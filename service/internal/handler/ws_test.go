package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gnexlayer/demo-service/internal/handler"
)

func TestWSLive(t *testing.T) {
	// Start a test server that only has the WS handler.
	mux := http.NewServeMux()
	userCount := func() int { return 42 }
	handler.WS(mux, userCount)

	// httptest.NewServer starts a real TCP server — required for WebSocket.
	// (httptest.NewRecorder doesn't work for WebSocket because it can't hold
	// an open connection.)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Dial the WebSocket endpoint.
	// Replace http:// with ws:// — same host, same port.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/live"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Read exactly one message and verify its shape.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var msg handler.LiveMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if msg.Type != "tick" {
		t.Errorf("want type=tick, got %q", msg.Type)
	}
	if msg.UserCount != 42 {
		t.Errorf("want UserCount=42, got %d", msg.UserCount)
	}
	if msg.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}
