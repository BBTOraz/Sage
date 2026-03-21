package services

import (
	"bilge-lib/internal/documents/domain"
	"context"
)

type AnswerService interface {
	BuildGroundedAnswer(ctx context.Context, question string, evidence domain.EvidenceBundle) (domain.GroundedAnswer, error)
}
