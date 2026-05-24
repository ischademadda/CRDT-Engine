package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type ReplicaActivity struct {
	ReplicaID string `json:"replica_id"`
	Inserts   int    `json:"inserts"`
	Deletes   int    `json:"deletes"`
}

type DocAnalytics struct {
	DocumentID   string            `json:"doc_id"`
	TotalInserts int               `json:"total_inserts"`
	TotalDeletes int               `json:"total_deletes"`
	TotalChars   int               `json:"total_chars"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Replicas     []ReplicaActivity `json:"replicas"`
}

type AnalyticsDB struct {
	db *sql.DB
}

// NewAnalyticsDB создает и инициализирует подключение к СУБД с логикой повторных попыток.
func NewAnalyticsDB(dsn string) (*AnalyticsDB, error) {
	var db *sql.DB
	var err error

	// PostgreSQL в контейнере может подниматься несколько секунд, делаем retry loop.
	for i := 1; i <= 10; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Printf("successfully connected to PostgreSQL on attempt %d", i)
				return &AnalyticsDB{db: db}, nil
			}
		}
		log.Printf("attempt %d to connect to PostgreSQL failed: %v. Retrying in 2s...", i, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after 10 attempts: %w", err)
}

func (adb *AnalyticsDB) Close() error {
	if adb.db != nil {
		return adb.db.Close()
	}
	return nil
}

// InitSchema накатывает схему данных в PostgreSQL.
func (adb *AnalyticsDB) InitSchema(ctx context.Context) error {
	queryDocStats := `
	CREATE TABLE IF NOT EXISTS document_stats (
		document_id VARCHAR(255) PRIMARY KEY,
		total_inserts INT DEFAULT 0,
		total_deletes INT DEFAULT 0,
		total_chars INT DEFAULT 0,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	queryReplicaStats := `
	CREATE TABLE IF NOT EXISTS replica_stats (
		document_id VARCHAR(255),
		replica_id VARCHAR(255),
		inserts INT DEFAULT 0,
		deletes INT DEFAULT 0,
		PRIMARY KEY (document_id, replica_id)
	);`

	tx, err := adb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, queryDocStats); err != nil {
		return fmt.Errorf("create document_stats: %w", err)
	}

	if _, err := tx.ExecContext(ctx, queryReplicaStats); err != nil {
		return fmt.Errorf("create replica_stats: %w", err)
	}

	return tx.Commit()
}

// RecordInsert атомарно регистрирует вставку символа.
func (adb *AnalyticsDB) RecordInsert(ctx context.Context, docID string, replicaID string) error {
	tx, err := adb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Обновляем общую статистику документа
	docQuery := `
	INSERT INTO document_stats (document_id, total_inserts, total_deletes, total_chars, updated_at)
	VALUES ($1, 1, 0, 1, NOW())
	ON CONFLICT (document_id)
	DO UPDATE SET
		total_inserts = document_stats.total_inserts + 1,
		total_chars = document_stats.total_chars + 1,
		updated_at = NOW();`

	if _, err := tx.ExecContext(ctx, docQuery, docID); err != nil {
		return fmt.Errorf("upsert document_stats: %w", err)
	}

	// 2. Обновляем статистику конкретного автора (реплики)
	replicaQuery := `
	INSERT INTO replica_stats (document_id, replica_id, inserts, deletes)
	VALUES ($1, $2, 1, 0)
	ON CONFLICT (document_id, replica_id)
	DO UPDATE SET
		inserts = replica_stats.inserts + 1;`

	if _, err := tx.ExecContext(ctx, replicaQuery, docID, replicaID); err != nil {
		return fmt.Errorf("upsert replica_stats: %w", err)
	}

	return tx.Commit()
}

// RecordDelete атомарно регистрирует удаление символа.
func (adb *AnalyticsDB) RecordDelete(ctx context.Context, docID string, replicaID string) error {
	tx, err := adb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Обновляем общую статистику документа
	docQuery := `
	INSERT INTO document_stats (document_id, total_inserts, total_deletes, total_chars, updated_at)
	VALUES ($1, 0, 1, 0, NOW())
	ON CONFLICT (document_id)
	DO UPDATE SET
		total_deletes = document_stats.total_deletes + 1,
		total_chars = GREATEST(0, document_stats.total_chars - 1),
		updated_at = NOW();`

	if _, err := tx.ExecContext(ctx, docQuery, docID); err != nil {
		return fmt.Errorf("upsert document_stats: %w", err)
	}

	// 2. Обновляем статистику автора
	replicaQuery := `
	INSERT INTO replica_stats (document_id, replica_id, inserts, deletes)
	VALUES ($1, $2, 0, 1)
	ON CONFLICT (document_id, replica_id)
	DO UPDATE SET
		deletes = replica_stats.deletes + 1;`

	if _, err := tx.ExecContext(ctx, replicaQuery, docID, replicaID); err != nil {
		return fmt.Errorf("upsert replica_stats: %w", err)
	}

	return tx.Commit()
}

// GetStats получает собранные метрики документа и активность участников.
func (adb *AnalyticsDB) GetStats(ctx context.Context, docID string) (*DocAnalytics, error) {
	stats := &DocAnalytics{
		DocumentID: docID,
		Replicas:   []ReplicaActivity{},
	}

	// 1. Получаем общую статистику
	docQuery := `
	SELECT total_inserts, total_deletes, total_chars, updated_at 
	FROM document_stats 
	WHERE document_id = $1`

	err := adb.db.QueryRowContext(ctx, docQuery, docID).Scan(
		&stats.TotalInserts, &stats.TotalDeletes, &stats.TotalChars, &stats.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Документа еще нет в статистике, возвращаем пустую структуру
			stats.UpdatedAt = time.Now()
			return stats, nil
		}
		return nil, fmt.Errorf("select document_stats: %w", err)
	}

	// 2. Получаем разбивку по репликам
	replicaQuery := `
	SELECT replica_id, inserts, deletes 
	FROM replica_stats 
	WHERE document_id = $1
	ORDER BY inserts DESC`

	rows, err := adb.db.QueryContext(ctx, replicaQuery, docID)
	if err != nil {
		return nil, fmt.Errorf("select replica_stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ra ReplicaActivity
		if err := rows.Scan(&ra.ReplicaID, &ra.Inserts, &ra.Deletes); err != nil {
			return nil, fmt.Errorf("scan replica_stats: %w", err)
		}
		stats.Replicas = append(stats.Replicas, ra)
	}

	return stats, nil
}

// GetAllDocs возвращает список всех известных документов.
func (adb *AnalyticsDB) GetAllDocs(ctx context.Context) ([]string, error) {
	query := `SELECT document_id FROM document_stats ORDER BY updated_at DESC`
	rows, err := adb.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []string
	for rows.Next() {
		var doc string
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
