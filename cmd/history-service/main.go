package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/ischademadda/CRDT-Engine/internal/redis"
	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

func main() {
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	dbDSN := envOr("DB_DSN", "postgres://crdt:crdt_password@localhost:5432/crdt_analytics?sslmode=disable")
	httpAddr := envOr("HTTP_ADDR", ":8083")

	log.Printf("starting crdt-history-service: redis=%s, http=%s", redisAddr, httpAddr)

	// 1. Инициализация СУБД (PostgreSQL)
	hdb, err := NewHistoryDB(dbDSN)
	if err != nil {
		log.Fatalf("failed to connect to history database: %v", err)
	}
	defer hdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = hdb.InitSchema(ctx)
	cancel()
	if err != nil {
		log.Fatalf("failed to init history schema: %v", err)
	}
	log.Println("history database schema checked/initialized successfully")

	// 2. Инициализация Redis
	rclient := goredis.NewClient(&goredis.Options{Addr: redisAddr})
	ctxPing, cancelPing := context.WithTimeout(context.Background(), 3*time.Second)
	if err := rclient.Ping(ctxPing).Err(); err != nil {
		log.Fatalf("failed to ping Redis: %v", err)
	}
	cancelPing()
	log.Println("history-service: successfully connected to Redis")

	// 3. Запуск асинхронного подписчика на события (Event Sourcing Collector)
	ctxConsumer, stopConsumer := context.WithCancel(context.Background())
	defer stopConsumer()
	go runRedisHistoryConsumer(ctxConsumer, rclient, hdb)

	// 4. Настройка HTTP-сервера
	mux := http.NewServeMux()
	mux.HandleFunc("OPTIONS /", corsHandler)
	mux.HandleFunc("GET /api/history/{doc_id}/revisions", getRevisionsHandler(hdb))
	mux.HandleFunc("GET /api/history/{doc_id}/checkout", checkoutHandler(hdb))

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("history HTTP server listening on %s", httpAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("history HTTP server failed: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down history service...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	stopConsumer()
	_ = srv.Shutdown(shutdownCtx)
	_ = rclient.Close()
	log.Println("history service stopped gracefully. Bye!")
}

// runRedisHistoryConsumer слушает все изменения документов и сохраняет их в БД.
func runRedisHistoryConsumer(ctx context.Context, client goredis.UniversalClient, hdb *HistoryDB) {
	pubsub := client.PSubscribe(ctx, "crdt:doc:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("history-service: pattern consumer subscribed to 'crdt:doc:*'")

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				log.Println("history-service: redis pubsub channel closed")
				return
			}

			var delta redis.Delta
			if err := json.Unmarshal([]byte(msg.Payload), &delta); err != nil {
				log.Printf("history consumer: failed to unmarshal delta: %v", err)
				continue
			}

			// Логируем все операции (вставки и удаления)
			if delta.Type == "fugue_insert" || delta.Type == "fugue_delete" {
				err := hdb.AppendOp(ctx, delta.DocumentID, delta.Type, delta.Payload)
				if err != nil {
					log.Printf("history consumer: failed to save op for doc %s: %v", delta.DocumentID, err)
				}
			}
		}
	}
}

func corsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
	w.WriteHeader(http.StatusNoContent)
}

// getRevisionsHandler отдает весь список операций для рендеринга лога изменений
func getRevisionsHandler(hdb *HistoryDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		docID := r.PathValue("doc_id")
		if docID == "" {
			http.Error(w, "missing document ID", http.StatusBadRequest)
			return
		}

		revs, err := hdb.GetAllOps(r.Context(), docID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(revs)
	}
}

// checkoutHandler выполняет реконструкцию (Event Sourcing) документа
// до версии (количества операций) X.
func checkoutHandler(hdb *HistoryDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		docID := r.PathValue("doc_id")
		if docID == "" {
			http.Error(w, "missing document ID", http.StatusBadRequest)
			return
		}

		versionStr := r.URL.Query().Get("version")
		if versionStr == "" {
			http.Error(w, "missing version parameter", http.StatusBadRequest)
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil || version < 0 {
			http.Error(w, "invalid version parameter", http.StatusBadRequest)
			return
		}

		// 1. Извлекаем первые X операций из Event Store
		revs, err := hdb.GetOpsLimit(r.Context(), docID, version)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. Инициализируем пустое дерево CRDT
		// Для сборки используем специальный системный ID реплики
		tree := crdt.NewFugueTree("crdt-reconstructor")

		// 3. Последовательно накатываем (Event Sourcing) операции
		for _, rev := range revs {
			switch rev.OpType {
			case "fugue_insert":
				var op crdt.FugueInsertOp
				if err := json.Unmarshal(rev.Payload, &op); err != nil {
					http.Error(w, "failed to decode insert op: "+err.Error(), http.StatusInternalServerError)
					return
				}
				tree.ApplyRemoteInsert(op)

			case "fugue_delete":
				var op crdt.FugueDeleteOp
				if err := json.Unmarshal(rev.Payload, &op); err != nil {
					http.Error(w, "failed to decode delete op: "+err.Error(), http.StatusInternalServerError)
					return
				}
				tree.ApplyRemoteDelete(op)
			}
		}

		// 4. Возвращаем восстановленный текст
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"doc_id":  docID,
			"version": version,
			"text":    tree.ToString(),
		})
	}
}

func enableCors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
