// Package main — демо-сервер CRDT-движка.
//
// Связывает все слои:
//   HTTP/WebSocket (gorilla)  →  Hub (Fan-In/Fan-Out)
//                                  ↓
//                              Worker Pool
//                                  ↓
//                              SyncUseCase  ──►  InMemoryRepository
//                                  ↕
//                              Redis Pub/Sub  ◄──►  другие узлы
//
// ENV:
//
//	HTTP_ADDR   — адрес HTTP-сервера (по умолчанию ":8080")
//	REDIS_ADDR  — адрес Redis (по умолчанию "localhost:6379"). Пустая строка — отключает Redis.
//	NODE_ID     — идентификатор узла (генерируется случайно, если пусто)
//	DOC_ID      — документ, на который узел подписывается в Redis (по умолчанию "demo")
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/ischademadda/CRDT-Engine/internal/redis"
	"github.com/ischademadda/CRDT-Engine/internal/repository"
	"github.com/ischademadda/CRDT-Engine/internal/usecase"
	"github.com/ischademadda/CRDT-Engine/internal/websocket"
	"github.com/ischademadda/CRDT-Engine/internal/worker"
)

func main() {
	httpAddr := envOr("HTTP_ADDR", ":8080")
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	nodeID := envOr("NODE_ID", randomID())
	docID := envOr("DOC_ID", "demo")

	log.Printf("crdt-engine: nodeID=%s http=%s redis=%s", nodeID, httpAddr, redisAddr)

	repo := repository.NewInMemory()
	hub := websocket.NewHub(0)
	pool := worker.New(8, 256)

	go hub.Run()

	var publisher usecase.Publisher
	var subscriber *redis.Subscriber
	var rclient goredis.UniversalClient
	if redisAddr != "" {
		rclient = goredis.NewClient(&goredis.Options{Addr: redisAddr})
		// pre-flight ping чтобы быстро узнать о неработающем Redis.
		ctxPing, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := rclient.Ping(ctxPing).Err(); err != nil {
			log.Printf("warning: redis ping failed (%v) — running stand-alone", err)
			rclient = nil
		}
		cancel()

		if rclient != nil {
			pub := redis.NewPublisher(rclient)
			publisher = &pubAdapter{pub: pub}
			subscriber = redis.NewSubscriber(rclient, 0)
			if err := subscriber.Subscribe(context.Background(), docID); err != nil {
				log.Fatalf("redis subscribe: %v", err)
			}
		}
	}

	syncUC := usecase.NewSyncUseCase(repo, &hubAdapter{hub: hub}, publisher, nodeID)
	docUC := usecase.NewDocumentUseCase(repo, nodeID)

	dispatcher := newDispatcher(repo, syncUC, pool, nodeID)
	go dispatcher.runWS(hub.Inbound())
	if subscriber != nil {
		go dispatcher.runRedis(subscriber.Messages())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleUpgrade(func(r *http.Request) string {
		if d := r.URL.Query().Get("doc"); d != "" {
			return d
		}
		return docID
	}))
	mux.HandleFunc("/snapshot", snapshotHandler(docUC, docID))
	mux.HandleFunc("/", indexHandler())

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("listening on %s", httpAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = srv.Shutdown(shutdownCtx)
	hub.Shutdown()
	_ = pool.Stop(shutdownCtx)
	if subscriber != nil {
		_ = subscriber.Close()
	}
	if rclient != nil {
		_ = rclient.Close()
	}
	log.Println("bye")
}

// --- адаптеры портов use-case к конкретным транспортам ---

type hubAdapter struct{ hub *websocket.Hub }

func (a *hubAdapter) Broadcast(doc, typ string, payload json.RawMessage, exclude string) {
	a.hub.Broadcast(websocket.Message{
		DocumentID: doc,
		Type:       typ,
		Payload:    payload,
		SenderID:   exclude,
	})
}

type pubAdapter struct{ pub *redis.Publisher }

func (a *pubAdapter) Publish(ctx context.Context, doc, typ string, payload json.RawMessage, origin string) error {
	return a.pub.Publish(ctx, redis.Delta{
		DocumentID:   doc,
		OriginNodeID: origin,
		Type:         typ,
		Payload:      payload,
	})
}

// --- dispatcher: WS inbound + Redis fan-in → use-case через worker pool ---

type dispatcher struct {
	repo   repository.DocumentRepository
	uc     *usecase.SyncUseCase
	pool   *worker.Pool
	nodeID string
}

func newDispatcher(r repository.DocumentRepository, uc *usecase.SyncUseCase, p *worker.Pool, nodeID string) *dispatcher {
	return &dispatcher{repo: r, uc: uc, pool: p, nodeID: nodeID}
}

// runWS обрабатывает входящие сообщения от WebSocket-клиентов.
//
// Клиент шлёт интент ({"type":"insert_intent","payload":{"pos":N,"char":"x"}}),
// сервер резолвит его в FugueInsertOp/FugueDeleteOp на локальном дереве и
// прогоняет через SyncUseCase.HandleDelta как OriginLocal-дельту.
func (d *dispatcher) runWS(in <-chan websocket.Message) {
	for msg := range in {
		msg := msg
		_ = d.pool.Submit(func(ctx context.Context) {
			if err := d.handleClientMsg(ctx, msg); err != nil {
				log.Printf("ws handle: %v", err)
			}
		})
	}
}

// runRedis обрабатывает дельты, пришедшие от других узлов через Redis Pub/Sub.
func (d *dispatcher) runRedis(in <-chan redis.Delta) {
	for delta := range in {
		if delta.OriginNodeID == d.nodeID {
			continue // эхо собственного публикации
		}
		delta := delta
		_ = d.pool.Submit(func(ctx context.Context) {
			err := d.uc.HandleDelta(ctx, usecase.Delta{
				DocumentID:   delta.DocumentID,
				Type:         delta.Type,
				Payload:      delta.Payload,
				OriginNodeID: delta.OriginNodeID,
				Origin:       usecase.OriginRemote,
			})
			if err != nil {
				log.Printf("redis handle: %v", err)
			}
		})
	}
}

// intentInsert/intentDelete — JSON-схемы клиентских интентов.
type intentInsert struct {
	Pos  int    `json:"pos"`
	Char string `json:"char"`
}
type intentDelete struct {
	Pos int `json:"pos"`
}

const (
	intentTypeInsert = "insert_intent"
	intentTypeDelete = "delete_intent"
)

func (d *dispatcher) handleClientMsg(ctx context.Context, msg websocket.Message) error {
	tree, err := d.repo.GetOrCreate(ctx, msg.DocumentID, d.nodeID)
	if err != nil {
		return err
	}

	switch msg.Type {
	case intentTypeInsert:
		var in intentInsert
		if err := json.Unmarshal(msg.Payload, &in); err != nil {
			return err
		}
		runes := []rune(in.Char)
		if len(runes) == 0 {
			return errors.New("empty char")
		}
		op, err := tree.InsertAt(in.Pos, runes[0])
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(op)
		return d.uc.HandleDelta(ctx, usecase.Delta{
			DocumentID: msg.DocumentID,
			Type:       usecase.OpTypeFugueInsert,
			Payload:    payload,
			SenderID:   msg.SenderID,
			Origin:     usecase.OriginLocal,
		})

	case intentTypeDelete:
		var in intentDelete
		if err := json.Unmarshal(msg.Payload, &in); err != nil {
			return err
		}
		op, err := tree.DeleteAt(in.Pos)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(op)
		return d.uc.HandleDelta(ctx, usecase.Delta{
			DocumentID: msg.DocumentID,
			Type:       usecase.OpTypeFugueDelete,
			Payload:    payload,
			SenderID:   msg.SenderID,
			Origin:     usecase.OriginLocal,
		})

	case usecase.OpTypeFugueInsert, usecase.OpTypeFugueDelete:
		// Прямая операция (например, от другого инстанса того же сервера).
		return d.uc.HandleDelta(ctx, usecase.Delta{
			DocumentID: msg.DocumentID,
			Type:       msg.Type,
			Payload:    msg.Payload,
			SenderID:   msg.SenderID,
			Origin:     usecase.OriginLocal,
		})

	default:
		return errors.New("unknown msg type: " + msg.Type)
	}
}

// --- HTTP handlers ---

func snapshotHandler(uc *usecase.DocumentUseCase, defaultDoc string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		doc := r.URL.Query().Get("doc")
		if doc == "" {
			doc = defaultDoc
		}
		text, err := uc.Text(r.Context(), doc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"doc": doc, "text": text})
	}
}

func indexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	}
}

// --- утилиты ---

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func randomID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "node-" + hex.EncodeToString(b[:])
}

