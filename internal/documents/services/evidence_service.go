package services

import (
	"bilge-lib/internal/documents/domain"
	"context"
)

type EvidenceService interface {
	Collect(ctx context.Context, question string, candidates []domain.Chunk) (domain.EvidenceBundle, error)
}
