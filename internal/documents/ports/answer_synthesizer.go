package ports

import (
	"bilge-lib/internal/documents/domain"
	"context"
)

type SynthesizeAnswerRequest struct {
	Question string
	Evidence domain.EvidenceBundle
}

type AnswerSynthesizer interface {
	Synthesize(ctx context.Context, req SynthesizeAnswerRequest) (domain.GroundedAnswer, error)
}
