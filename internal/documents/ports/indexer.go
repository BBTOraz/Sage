package ports

import "context"

// IndexDocument is the thin application-facing indexing payload.
// An Eino-backed adapter can map this into schema.Document without leaking Eino into the domain layer.
type IndexDocument struct {
	ID       string
	Content  string
	Metadata map[string]any
}

// Indexer is the thin indexing seam. Default implementation path should use Eino first.
type Indexer interface {
	Store(ctx context.Context, docs []IndexDocument) ([]string, error)
}
