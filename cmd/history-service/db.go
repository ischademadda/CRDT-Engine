package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type Revision struct {
	ID        int64           `json:"id"`
	OpType    string          `json:"op_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

type HistoryDB struct {
	db *sql.DB
}

// NewHistoryDB создает и инициализирует подключение к СУБД с логикой повторных попыток.
func NewHistoryDB(dsn string) (*HistoryDB, error) {
	var db *sql.DB
	var err error

	for i := 1; i <= 10; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Printf("history-db: successfully connected to PostgreSQL on attempt %d", i)
				return &HistoryDB{db: db}, nil
			}
		}
		log.Printf("history-db: attempt %d to connect to PostgreSQL failed: %v. Retrying in 2s...", i, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after 10 attempts: %w", err)
}

func (hdb *HistoryDB) Close() error {
	if hdb.db != nil {
		return hdb.db.Close()
	}
	return nil
}

// InitSchema накатывает таблицу истории в PostgreSQL.
func (hdb *HistoryDB) InitSchema(ctx context.Context) error {
	queryHistoryTable := `
	CREATE TABLE IF NOT EXISTS document_history (
		id SERIAL PRIMARY KEY,
		document_id VARCHAR(255) NOT NULL,
		op_type VARCHAR(50) NOT NULL,
		payload JSONB NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	queryIndex := `
	CREATE INDEX IF NOT EXISTS idx_doc_history_doc ON document_history (document_id);`

	tx, err := hdb.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, queryHistoryTable); err != nil {
		return fmt.Errorf("create document_history table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, queryIndex); err != nil {
		return fmt.Errorf("create index on document_history: %w", err)
	}

	return tx.Commit()
}

// AppendOp сохраняет новую операцию в неизменяемый лог (Event Store).
func (hdb *HistoryDB) AppendOp(ctx context.Context, docID string, opType string, payload []byte) error {
	query := `
	INSERT INTO document_history (document_id, op_type, payload)
	VALUES ($1, $2, $3)`

	_, err := hdb.db.ExecContext(ctx, query, docID, opType, payload)
	if err != nil {
		return fmt.Errorf("append op to history: %w", err)
	}
	return nil
}

// GetOpsLimit выбирает операции для конкретного документа до порядкового номера limit включительно.
// Сортировка по id ASC гарантирует строго детерминированное воспроизведение (Event Sourcing).
func (hdb *HistoryDB) GetOpsLimit(ctx context.Context, docID string, limit int) ([]Revision, error) {
	query := `
	SELECT id, op_type, payload, created_at
	FROM document_history
	WHERE document_id = $1
	ORDER BY id ASC
	LIMIT $2`

	rows, err := hdb.db.QueryContext(ctx, query, docID, limit)
	if err != nil {
		return nil, fmt.Errorf("select history limit: %w", err)
	}
	defer rows.Close()

	var revs []Revision
	for rows.Next() {
		var r Revision
		if err := rows.Scan(&r.ID, &r.OpType, &r.Payload, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		revs = append(revs, r)
	}

	return revs, nil
}

// GetAllOps возвращает полную историю изменений документа.
func (hdb *HistoryDB) GetAllOps(ctx context.Context, docID string) ([]Revision, error) {
	query := `
	SELECT id, op_type, payload, created_at
	FROM document_history
	WHERE document_id = $1
	ORDER BY id ASC`

	rows, err := hdb.db.QueryContext(ctx, query, docID)
	if err != nil {
		return nil, fmt.Errorf("select all history: %w", err)
	}
	defer rows.Close()

	var revs []Revision
	for rows.Next() {
		var r Revision
		if err := rows.Scan(&r.ID, &r.OpType, &r.Payload, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		revs = append(revs, r)
	}

	return revs, nil
}
