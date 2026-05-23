package handler_test

// Using net/http/httptest — Go's built-in test server.
// No external libraries needed. This is the same approach used in Google's
// internal Go codebase.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gnexlayer/demo-service/internal/handler"
	"github.com/gnexlayer/demo-service/internal/store"
)

// newTestMux creates a fresh mux + store for each test.
// Isolation matters: if tests share state, a failing test can corrupt others.
func newTestMux(t *testing.T) (*http.ServeMux, *store.Store) {
	t.Helper()
	s := store.New()
	mux := http.NewServeMux()
	handler.HTTP(mux, s)
	return mux, s
}

func TestHealthz(t *testing.T) {
	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestCreateUser(t *testing.T) {
	mux, s := newTestMux(t)

	body := bytes.NewBufferString(`{"name":"Alice","email":"alice@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d — body: %s", w.Code, w.Body.String())
	}
	if s.Len() != 1 {
		t.Error("user was not persisted in the store")
	}
}

func TestCreateUserMissingFields(t *testing.T) {
	mux, _ := newTestMux(t)

	body := bytes.NewBufferString(`{"name":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGetUser(t *testing.T) {
	mux, s := newTestMux(t)

	// Seed the store directly — don't depend on the POST handler working
	// (unit tests should be isolated by layer)
	created := s.Create("Bob", "bob@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+created.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}

	var resp store.User
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if resp.Email != "bob@example.com" {
		t.Errorf("want email=bob@example.com, got %q", resp.Email)
	}
}

func TestGetUserNotFound(t *testing.T) {
	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/999", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestListUsers(t *testing.T) {
	mux, s := newTestMux(t)

	s.Create("Alice", "alice@example.com")
	s.Create("Bob", "bob@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}

	var users []store.User
	if err := json.NewDecoder(w.Body).Decode(&users); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("want 2 users, got %d", len(users))
	}
}
