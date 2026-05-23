// Package store holds the in-memory user store.
// In a real service this would be a Postgres/Redis client.
// We use sync.RWMutex because multiple goroutines (HTTP, gRPC, WS handlers)
// all hit this store concurrently.
package store

import (
	"errors"
	"fmt"
	"sync"
)

// ErrNotFound is returned when a user ID doesn't exist.
// We define our own error rather than using fmt.Errorf so callers can do:
//
//	if errors.Is(err, store.ErrNotFound) { ... }
var ErrNotFound = errors.New("user not found")

// User is the domain model. Fields are exported so encoding/json works without tags.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Store is a thread-safe in-memory user store.
type Store struct {
	mu    sync.RWMutex
	users map[string]User
	seq   int // simple auto-increment for IDs
}

// New returns an empty store. Always call New — never create Store{} directly
// because the map would be nil.
func New() *Store {
	return &Store{users: make(map[string]User)}
}

// Create adds a user and returns the assigned ID.
func (s *Store) Create(name, email string) User {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	id := formatID(s.seq)
	u := User{ID: id, Name: name, Email: email}
	s.users[id] = u
	return u
}

// Get returns a user by ID or ErrNotFound.
func (s *Store) Get(id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

// List returns all users as a slice. Order is non-deterministic (map iteration).
func (s *Store) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}

// Len returns the number of stored users.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

func formatID(n int) string {
	// Simple zero-padded ID — e.g. "001", "002". Not UUID because
	// this is a demo; the pattern is the same for any ID scheme.
	return fmt.Sprintf("%03d", n)
}
