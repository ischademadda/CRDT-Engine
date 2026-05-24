package main

import (
	"context"
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
	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

func main() {
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	dbDSN := envOr("DB_DSN", "postgres://crdt:crdt_password@localhost:5432/crdt_analytics?sslmode=disable")
	httpAddr := envOr("HTTP_ADDR", ":8082")

	log.Printf("starting crdt-analytics-service: redis=%s, http=%s", redisAddr, httpAddr)

	// 1. Инициализация PostgreSQL
	adb, err := NewAnalyticsDB(dbDSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer adb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = adb.InitSchema(ctx)
	cancel()
	if err != nil {
		log.Fatalf("failed to init database schema: %v", err)
	}
	log.Println("database schema checked/initialized successfully")

	// 2. Инициализация Redis
	rclient := goredis.NewClient(&goredis.Options{Addr: redisAddr})
	ctxPing, cancelPing := context.WithTimeout(context.Background(), 3*time.Second)
	if err := rclient.Ping(ctxPing).Err(); err != nil {
		log.Fatalf("failed to ping Redis: %v", err)
	}
	cancelPing()
	log.Println("successfully connected to Redis")

	// 3. Запуск асинхронного Event Consumer
	ctxConsumer, stopConsumer := context.WithCancel(context.Background())
	defer stopConsumer()
	go runRedisPatternConsumer(ctxConsumer, rclient, adb)

	// 4. Настройка HTTP-сервера с CORS
	mux := http.NewServeMux()
	mux.HandleFunc("OPTIONS /", corsHandler)
	mux.HandleFunc("GET /api/analytics", getDocumentsHandler(adb))
	mux.HandleFunc("GET /api/analytics/{doc_id}", getDocumentStatsHandler(adb))

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("analytics HTTP server listening on %s", httpAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http listen and serve: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down analytics service...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	stopConsumer()
	_ = srv.Shutdown(shutdownCtx)
	_ = rclient.Close()
	log.Println("analytics service stopped gracefully. Bye!")
}

// runRedisPatternConsumer подписывается на паттерн crdt:doc:* и обрабатывает дельты в фоне.
func runRedisPatternConsumer(ctx context.Context, client goredis.UniversalClient, adb *AnalyticsDB) {
	pubsub := client.PSubscribe(ctx, "crdt:doc:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("redis pattern consumer subscribed to 'crdt:doc:*'")

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				log.Println("redis pubsub channel closed")
				return
			}

			var delta redis.Delta
			if err := json.Unmarshal([]byte(msg.Payload), &delta); err != nil {
				log.Printf("consumer: failed to unmarshal delta JSON: %v", err)
				continue
			}

			// Обрабатываем конкретные типы операций
			switch delta.Type {
			case "fugue_insert":
				var op crdt.FugueInsertOp
				if err := json.Unmarshal(delta.Payload, &op); err != nil {
					log.Printf("consumer: failed to unmarshal insert op: %v", err)
					continue
				}
				err := adb.RecordInsert(ctx, delta.DocumentID, op.NodeID.ReplicaID)
				if err != nil {
					log.Printf("consumer: failed to save insert for %s: %v", delta.DocumentID, err)
				}

			case "fugue_delete":
				var op crdt.FugueDeleteOp
				if err := json.Unmarshal(delta.Payload, &op); err != nil {
					log.Printf("consumer: failed to unmarshal delete op: %v", err)
					continue
				}
				err := adb.RecordDelete(ctx, delta.DocumentID, op.SourceID.ReplicaID)
				if err != nil {
					log.Printf("consumer: failed to save delete for %s: %v", delta.DocumentID, err)
				}
			}
		}
	}
}

// corsHandler возвращает правильные заголовки для preflight CORS-запросов.
func corsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)
	w.WriteHeader(http.StatusNoContent)
}

func getDocumentsHandler(adb *AnalyticsDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		docs, err := adb.GetAllDocs(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(docs)
	}
}

func getDocumentStatsHandler(adb *AnalyticsDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		docID := r.PathValue("doc_id")
		if docID == "" {
			http.Error(w, "missing document ID", http.StatusBadRequest)
			return
		}

		stats, err := adb.GetStats(r.Context(), docID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stats)
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
