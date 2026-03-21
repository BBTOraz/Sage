package services

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
)

type ChunkService interface {
	OpenWindow(ctx context.Context, documentID domain.DocumentID, chunkID domain.ChunkID, opts ports.WindowOptions) (ports.ChunkWindow, error)
}
