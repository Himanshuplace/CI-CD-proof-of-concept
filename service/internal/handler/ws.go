package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// upgrader converts an HTTP connection to a WebSocket connection.
// CheckOrigin returns true to allow all origins — tighten this in production
// by checking r.Header.Get("Origin") against an allowlist.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

// LiveMessage is what the server pushes to connected clients every tick.
type LiveMessage struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	UserCount int    `json:"user_count"`
}

// WS registers the WebSocket live-feed endpoint.
// Each connection gets its own goroutine (started inside handleLive).
func WS(mux *http.ServeMux, userCount func() int) {
	mux.HandleFunc("GET /ws/live", handleLive(userCount))
}

// handleLive upgrades the HTTP request to WebSocket, then streams a
// LiveMessage every second. This simulates a real-time "users online" feed.
//
// Why a closure that takes userCount?
//   - The handler needs to read from the store, but we don't want to import
//     the store package here (that would couple handler ↔ store tightly).
//   - Instead we inject a function — this makes the handler testable without
//     a real store, and swappable (e.g. count from Redis in production).
func handleLive(userCount func() int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade writes its own error response — we just log and return.
			slog.Error("ws upgrade failed", "err", err)
			return
		}
		defer conn.Close()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case t := <-ticker.C:
				msg := LiveMessage{
					Type:      "tick",
					Timestamp: t.UTC().Format(time.RFC3339),
					UserCount: userCount(),
				}
				data, _ := json.Marshal(msg)

				// WriteMessage is not goroutine-safe — only one goroutine
				// should write per connection. We have exactly one (this one).
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					// Client disconnected — normal, not an error.
					return
				}

			case <-r.Context().Done():
				// Server shutting down or request cancelled.
				return
			}
		}
	}
}
