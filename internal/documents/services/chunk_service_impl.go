package services

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
)

type DefaultChunkService struct {
	Reader ports.ChunkReader
}

func NewChunkService(reader ports.ChunkReader) *DefaultChunkService {
	return &DefaultChunkService{Reader: reader}
}

func (s *DefaultChunkService) OpenWindow(ctx context.Context, documentID domain.DocumentID, chunkID domain.ChunkID, opts ports.WindowOptions) (ports.ChunkWindow, error) {
	if opts.Before < 0 {
		opts.Before = 0
	}
	if opts.After < 0 {
		opts.After = 0
	}

	return s.Reader.OpenWindow(ctx, documentID, chunkID, opts)
}
