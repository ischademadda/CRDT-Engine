package websocket

import "encoding/json"

// Message — единица обмена между клиентом и Hub.
//
// DocumentID маршрутизирует сообщение к подписчикам конкретного документа.
// Type — тип CRDT-операции ("fugue_insert", "fugue_delete", и т. п.).
// Payload — сырое тело операции (JSON), парсится на уровне use-case.
type Message struct {
	DocumentID string          `json:"doc_id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	SenderID   string          `json:"sender_id,omitempty"`
}
