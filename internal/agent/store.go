package agent

import (
	"context"
	"sync"
)

type InMemoryCheckPointStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewInMemoryCheckPointStore() *InMemoryCheckPointStore {
	return &InMemoryCheckPointStore{
		mu:   sync.Mutex{},
		data: make(map[string][]byte),
	}
}

func (store *InMemoryCheckPointStore) Get(ctx context.Context, id string) ([]byte, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	v, ok := store.data[id]
	return v, ok, nil
}

func (store *InMemoryCheckPointStore) Set(ctx context.Context, id string, value []byte) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.data[id] = value
	return nil
}
