package sqlite

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func TestCheckpointStoreSetAndGet(t *testing.T) {
	db := openTestDB(t)
	store := NewCheckpointStore(db)

	want := []byte("checkpoint-v1")
	if err := store.Set(context.Background(), "cp-1", want); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, ok, err := store.Get(context.Background(), "cp-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Get() value = %q, want %q", got, want)
	}
}

func TestCheckpointStoreSetOverwritesExistingValue(t *testing.T) {
	db := openTestDB(t)
	store := NewCheckpointStore(db)

	if err := store.Set(context.Background(), "cp-1", []byte("first")); err != nil {
		t.Fatalf("Set(first) error = %v", err)
	}
	if err := store.Set(context.Background(), "cp-1", []byte("second")); err != nil {
		t.Fatalf("Set(second) error = %v", err)
	}

	got, ok, err := store.Get(context.Background(), "cp-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if string(got) != "second" {
		t.Fatalf("Get() value = %q, want %q", got, "second")
	}
}

func openTestDB(t *testing.T) DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(context.Background(), Config{Path: dbPath})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})

	return db
}
