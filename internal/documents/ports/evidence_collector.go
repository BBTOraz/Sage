package ports

import (
	"bilge-lib/internal/documents/domain"
	"context"
)

type CollectEvidenceRequest struct {
	Question   string
	Candidates []domain.Chunk
}

type EvidenceCollector interface {
	Collect(ctx context.Context, req CollectEvidenceRequest) (domain.EvidenceBundle, error)
}
