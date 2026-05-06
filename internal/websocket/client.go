package websocket

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// sendBuffer — размер per-client очереди исходящих сообщений.
// При переполнении клиент считается медленным и отключается.
const sendBuffer = 64

// Сетевые таймауты gorilla/websocket. Подобраны под интерактивный редактор.
const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 1 << 20 // 1 MiB
)

// Client — обёртка над websocket-соединением. Один клиент привязан к одному документу.
//
// readPump читает входящие операции и отправляет их в hub.inbound (Fan-In).
// writePump забирает сообщения из send-канала и пишет их в сокет (Fan-Out target).
type Client struct {
	id         string
	documentID string
	hub        *Hub
	conn       *websocket.Conn
	send       chan Message
	closed     atomic.Bool
}

// readPump — горутина, читающая фреймы из сокета.
// Завершается при ошибке чтения или таймауте pong.
func (c *Client) readPump() {
	defer c.hub.unregister(c)

	c.conn.SetReadLimit(maxMsgSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		// Принудительно проставляем идентификаторы — клиент не доверенный источник.
		msg.SenderID = c.id
		msg.DocumentID = c.documentID
		c.hub.inbound <- msg
	}
}

// writePump — горутина, пишущая фреймы в сокет, плюс ping раз в pingPeriod.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// trySend — неблокирующая попытка отправить клиенту. Возвращает false если очередь забита.
func (c *Client) trySend(msg Message) bool {
	if c.closed.Load() {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}
