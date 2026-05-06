package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialTestServer — поднимает httptest сервер с Hub и возвращает функцию dial.
func dialTestServer(t *testing.T, h *Hub) (*httptest.Server, func(docID string) *websocket.Conn) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.HandleUpgrade(func(r *http.Request) string {
		return r.URL.Query().Get("doc")
	}))
	srv := httptest.NewServer(mux)
	dial := func(docID string) *websocket.Conn {
		u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?doc=" + docID
		c, _, err := websocket.DefaultDialer.Dial(u, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return c
	}
	return srv, dial
}

func waitFor(t *testing.T, cond func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}

func TestHub_RegisterAndDisconnect(t *testing.T) {
	h := NewHub(0)
	go h.Run()
	defer h.Shutdown()

	srv, dial := dialTestServer(t, h)
	defer srv.Close()

	c := dial("doc1")
	waitFor(t, func() bool { return h.ClientCount("doc1") == 1 }, time.Second, "client registered")

	_ = c.Close()
	waitFor(t, func() bool { return h.ClientCount("doc1") == 0 }, time.Second, "client unregistered")
}

func TestHub_FanInDeliversMessage(t *testing.T) {
	h := NewHub(0)
	go h.Run()
	defer h.Shutdown()

	srv, dial := dialTestServer(t, h)
	defer srv.Close()

	c := dial("doc-x")
	defer c.Close()
	waitFor(t, func() bool { return h.ClientCount("doc-x") == 1 }, time.Second, "registered")

	out := Message{Type: "fugue_insert", Payload: json.RawMessage(`{"v":1}`)}
	if err := c.WriteJSON(out); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case got := <-h.Inbound():
		if got.Type != "fugue_insert" || got.DocumentID != "doc-x" {
			t.Fatalf("unexpected message: %+v", got)
		}
		if got.SenderID == "" {
			t.Fatal("expected SenderID to be set by hub")
		}
	case <-time.After(time.Second):
		t.Fatal("inbound timeout")
	}
}

func TestHub_BroadcastExcludesSender(t *testing.T) {
	h := NewHub(0)
	go h.Run()
	defer h.Shutdown()

	srv, dial := dialTestServer(t, h)
	defer srv.Close()

	a := dial("d")
	defer a.Close()
	b := dial("d")
	defer b.Close()
	waitFor(t, func() bool { return h.ClientCount("d") == 2 }, time.Second, "two clients")

	// Отправляем от A → ждём входящего, потом broadcast наружу.
	if err := a.WriteJSON(Message{Type: "x", Payload: json.RawMessage(`null`)}); err != nil {
		t.Fatal(err)
	}
	in := <-h.Inbound()

	h.Broadcast(Message{DocumentID: "d", SenderID: in.SenderID, Type: "broadcast", Payload: json.RawMessage(`null`)})

	// B должен получить, A — нет.
	_ = b.SetReadDeadline(time.Now().Add(time.Second))
	var got Message
	if err := b.ReadJSON(&got); err != nil {
		t.Fatalf("B did not receive: %v", err)
	}
	if got.Type != "broadcast" {
		t.Fatalf("unexpected type at B: %s", got.Type)
	}

	_ = a.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	var unwanted Message
	if err := a.ReadJSON(&unwanted); err == nil {
		t.Fatalf("A unexpectedly received broadcast: %+v", unwanted)
	}
}

func TestHub_DocumentIsolation(t *testing.T) {
	h := NewHub(0)
	go h.Run()
	defer h.Shutdown()

	srv, dial := dialTestServer(t, h)
	defer srv.Close()

	a := dial("doc-A")
	defer a.Close()
	b := dial("doc-B")
	defer b.Close()
	waitFor(t, func() bool {
		return h.ClientCount("doc-A") == 1 && h.ClientCount("doc-B") == 1
	}, time.Second, "both registered")

	h.Broadcast(Message{DocumentID: "doc-A", Type: "only-A", Payload: json.RawMessage(`null`)})

	_ = b.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	var got Message
	if err := b.ReadJSON(&got); err == nil {
		t.Fatalf("B leaked message from doc-A: %+v", got)
	}
}

func TestHub_ShutdownClosesClients(t *testing.T) {
	h := NewHub(0)
	go h.Run()

	srv, dial := dialTestServer(t, h)
	defer srv.Close()

	c := dial("z")
	defer c.Close()
	waitFor(t, func() bool { return h.ClientCount("z") == 1 }, time.Second, "registered")

	h.Shutdown()

	// После shutdown read должен завершиться (соединение закрыто writePump'ом).
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected read error after shutdown")
	}
}

func TestHandleUpgrade_RejectsMissingDocID(t *testing.T) {
	h := NewHub(0)
	go h.Run()
	defer h.Shutdown()

	srv, _ := dialTestServer(t, h)
	defer srv.Close()

	u := strings.Replace(srv.URL, "http", "ws", 1) + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if err == nil {
		t.Fatal("expected dial error without doc id")
	}
	if resp == nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", resp)
	}
}
