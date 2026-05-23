package store_test

// We use the _test suffix package (external test package) so tests can only
// call exported methods — same API surface your real callers see.

import (
	"errors"
	"testing"

	"github.com/gnexlayer/demo-service/internal/store"
)

func TestCreate(t *testing.T) {
	s := store.New()
	u := s.Create("Alice", "alice@example.com")

	if u.Name != "Alice" {
		t.Errorf("want Name=Alice, got %q", u.Name)
	}
	if u.ID == "" {
		t.Error("Create should assign a non-empty ID")
	}
	if s.Len() != 1 {
		t.Errorf("want Len=1, got %d", s.Len())
	}
}

func TestGetFound(t *testing.T) {
	s := store.New()
	created := s.Create("Bob", "bob@example.com")

	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != "bob@example.com" {
		t.Errorf("want Email=bob@example.com, got %q", got.Email)
	}
}

func TestGetNotFound(t *testing.T) {
	s := store.New()

	_, err := s.Get("does-not-exist")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	s := store.New()
	s.Create("Alice", "alice@example.com")
	s.Create("Bob", "bob@example.com")

	users := s.List()
	if len(users) != 2 {
		t.Errorf("want 2 users, got %d", len(users))
	}
}

// TestConcurrent ensures the RWMutex actually protects concurrent writes.
// The -race flag in CI (go test -race) detects data races even when this
// test passes — together they prove thread safety.
func TestConcurrent(t *testing.T) {
	s := store.New()
	done := make(chan struct{})

	for i := 0; i < 50; i++ {
		go func() {
			s.Create("User", "u@example.com")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}

	if s.Len() != 50 {
		t.Errorf("want 50, got %d — possible race condition", s.Len())
	}
}
