package ports

import (
	"bilge-lib/internal/documents/domain"
	"context"
)

type WindowOptions struct {
	Before int
	After  int
}

type ChunkWindow struct {
	DocumentID domain.DocumentID
	AnchorID   domain.ChunkID
	Chunks     []domain.Chunk
}

type ChunkReader interface {
	OpenWindow(ctx context.Context, documentID domain.DocumentID, chunkID domain.ChunkID, opts WindowOptions) (ChunkWindow, error)
}
