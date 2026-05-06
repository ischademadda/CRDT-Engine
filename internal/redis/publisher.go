// Package redis предоставляет тонкий Pub/Sub-адаптер поверх go-redis для
// межинстансной рассылки CRDT-дельт.
//
// Поток данных:
//
//	Local node → Publisher.Publish(docID, delta)  ─►  Redis channel "crdt:doc:<id>"
//	                                                     │
//	Other nodes ← Subscriber.Messages() chan      ◄──────┘
//
// Сериализация — JSON для MVP (по плану позже Protobuf).
package redis

import (
	"context"
	"encoding/json"
	"fmt"

	rds "github.com/redis/go-redis/v9"
)

// channelPrefix — namespace, чтобы не пересекаться с другими сервисами в общем Redis.
const channelPrefix = "crdt:doc:"

// channelFor возвращает имя Pub/Sub-канала для документа.
func channelFor(documentID string) string {
	return channelPrefix + documentID
}

// Delta — обёртка для рассылаемой CRDT-операции. Универсальный конверт:
// Type определяет, как десериализовать Payload на принимающей стороне.
//
// OriginNodeID нужен, чтобы получатель мог отфильтровать собственные эхо-сообщения
// (Redis Pub/Sub доставляет publishes всем подписчикам, включая отправителя).
type Delta struct {
	DocumentID   string          `json:"doc_id"`
	OriginNodeID string          `json:"origin"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
}

// Publisher публикует дельты в Redis Pub/Sub.
type Publisher struct {
	client rds.UniversalClient
}

// NewPublisher оборачивает уже сконфигурированный go-redis клиент.
// Жизненный цикл клиента — на стороне вызывающего.
func NewPublisher(client rds.UniversalClient) *Publisher {
	return &Publisher{client: client}
}

// Publish сериализует delta в JSON и отправляет в канал документа.
func (p *Publisher) Publish(ctx context.Context, delta Delta) error {
	if delta.DocumentID == "" {
		return fmt.Errorf("redis publish: empty DocumentID")
	}
	payload, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("redis publish: marshal: %w", err)
	}
	if err := p.client.Publish(ctx, channelFor(delta.DocumentID), payload).Err(); err != nil {
		return fmt.Errorf("redis publish: %w", err)
	}
	return nil
}
