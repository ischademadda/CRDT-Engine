package websocket

import (
	"context"
	"sync"
)

// Hub — центральный диспетчер WebSocket-соединений.
//
// Архитектура:
//   - clients  : documentID → set(Client) — список подписчиков на документ
//   - inbound  : единый канал входящих сообщений от ВСЕХ клиентов (Fan-In)
//   - register/unregister: административные каналы для безопасной (без race) работы с clients
//
// Inbound отдаётся наружу через Inbound() — use-case применяет операцию к CRDT,
// затем дёргает Hub.Broadcast для рассылки результата (Fan-Out).
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{} // documentID → clients

	inbound chan Message

	registerCh   chan *Client
	unregisterCh chan *Client

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewHub создаёт Hub с буферизованным inbound-каналом.
// inboundBuf — размер буфера Fan-In канала. 0 → дефолт 256.
func NewHub(inboundBuf int) *Hub {
	if inboundBuf <= 0 {
		inboundBuf = 256
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:      make(map[string]map[*Client]struct{}),
		inbound:      make(chan Message, inboundBuf),
		registerCh:   make(chan *Client, 16),
		unregisterCh: make(chan *Client, 16),
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
	}
}

// Run — диспетчерский цикл. Блокирующий, обычно запускается в отдельной горутине.
// Завершается по Shutdown().
func (h *Hub) Run() {
	defer close(h.done)
	for {
		select {
		case <-h.ctx.Done():
			h.closeAll()
			return
		case c := <-h.registerCh:
			h.mu.Lock()
			set, ok := h.clients[c.documentID]
			if !ok {
				set = make(map[*Client]struct{})
				h.clients[c.documentID] = set
			}
			set[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unregisterCh:
			h.removeClient(c)
		}
	}
}

// Inbound — канал входящих сообщений (Fan-In). Только для чтения снаружи.
func (h *Hub) Inbound() <-chan Message { return h.inbound }

// Broadcast рассылает сообщение всем клиентам документа msg.DocumentID
// КРОМЕ отправителя (msg.SenderID). Медленные клиенты отключаются.
func (h *Hub) Broadcast(msg Message) {
	h.mu.RLock()
	set := h.clients[msg.DocumentID]
	if len(set) == 0 {
		h.mu.RUnlock()
		return
	}
	targets := make([]*Client, 0, len(set))
	for c := range set {
		if c.id == msg.SenderID {
			continue
		}
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		if !c.trySend(msg) {
			// Медленный клиент: отключаем во избежание роста backlog.
			h.unregister(c)
		}
	}
}

// Shutdown останавливает Hub: закрывает все соединения и завершает Run.
func (h *Hub) Shutdown() {
	h.cancel()
	<-h.done
}

// register регистрирует нового клиента (вызывается из HandleUpgrade).
func (h *Hub) register(c *Client) { h.registerCh <- c }

// unregister помечает клиента отключённым и закрывает send-канал.
// Идемпотентен — повторные вызовы безопасны.
func (h *Hub) unregister(c *Client) {
	if c.closed.Swap(true) {
		return
	}
	close(c.send)
	select {
	case h.unregisterCh <- c:
	case <-h.ctx.Done():
	}
}

// removeClient вычищает клиента из реестра. Вызывается только из Run.
func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[c.documentID]
	if !ok {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.clients, c.documentID)
	}
}

// closeAll — graceful close всех клиентов при shutdown.
func (h *Hub) closeAll() {
	h.mu.Lock()
	all := make([]*Client, 0)
	for _, set := range h.clients {
		for c := range set {
			all = append(all, c)
		}
	}
	h.clients = make(map[string]map[*Client]struct{})
	h.mu.Unlock()

	for _, c := range all {
		if !c.closed.Swap(true) {
			close(c.send)
		}
	}
}

// ClientCount возвращает число активных клиентов документа (для метрик/тестов).
func (h *Hub) ClientCount(documentID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[documentID])
}
