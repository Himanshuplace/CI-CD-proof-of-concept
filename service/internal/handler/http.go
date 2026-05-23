// Package handler contains all protocol handlers (HTTP, WebSocket, gRPC).
// Keeping them in one package means tests can share the same store instance.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gnexlayer/demo-service/internal/store"
)

// HTTP wires all REST endpoints onto the given mux.
// We accept *http.ServeMux instead of using the global DefaultServeMux so that
// tests can create isolated muxes — no bleed between test cases.
func HTTP(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /api/v1/users", handleListUsers(s))
	mux.HandleFunc("POST /api/v1/users", handleCreateUser(s))
	mux.HandleFunc("GET /api/v1/users/{id}", handleGetUser(s))
}

// handleHealth is the liveness probe endpoint.
// HAProxy and Kubernetes both hit /healthz — returning 200 means "I am alive".
// Never put DB checks here; that belongs on /readyz (readiness).
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleListUsers returns all users as a JSON array.
func handleListUsers(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.List())
	}
}

// handleCreateUser reads a JSON body, creates a user, returns 201 Created.
func handleCreateUser(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if body.Name == "" || body.Email == "" {
			writeError(w, http.StatusBadRequest, "name and email are required")
			return
		}

		u := s.Create(body.Name, body.Email)
		writeJSON(w, http.StatusCreated, u)
	}
}

// handleGetUser looks up a user by path parameter {id}.
// Go 1.22+ supports path parameters in ServeMux natively — no router library needed.
func handleGetUser(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id") // Go 1.22+ feature
		u, err := s.Get(id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}

// writeJSON serialises v as JSON and sets the correct Content-Type.
// All handlers share this helper so the response format is always consistent.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
