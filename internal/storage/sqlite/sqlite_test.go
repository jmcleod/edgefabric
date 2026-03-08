package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/storage/sqlite"
)

// newTestStore creates an in-memory SQLite store for testing.
// It runs migrations and returns the store. The store is closed when the test ends.
func newTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	store, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate test store: %v", err)
	}
	return store
}

func TestPing(t *testing.T) {
	store := newTestStore(t)
	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
