package websocket

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gorilla/websocket"
)

// upgrader — конфиг gorilla/websocket. CheckOrigin открыт для MVP; в production
// здесь должна быть проверка whitelist доменов.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// HandleUpgrade возвращает http.HandlerFunc, апгрейдящий соединение и
// регистрирующий клиента в Hub.
//
// documentIDFn извлекает documentID из запроса (например, из query-параметра
// или path-сегмента). Это решение оставлено вызывающей стороне, чтобы пакет
// не зависел от роутинга.
func (h *Hub) HandleUpgrade(documentIDFn func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		docID := documentIDFn(r)
		if docID == "" {
			http.Error(w, "document id required", http.StatusBadRequest)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := &Client{
			id:         newClientID(),
			documentID: docID,
			hub:        h,
			conn:       conn,
			send:       make(chan Message, sendBuffer),
		}
		h.register(client)
		go client.writePump()
		go client.readPump()
	}
}

func newClientID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
