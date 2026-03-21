package services

import (
	"bilge-lib/internal/documents/domain"
	"context"
)

type MetadataService interface {
	GetDocument(ctx context.Context, id domain.DocumentID) (domain.Document, error)
	ListSections(ctx context.Context, id domain.DocumentID) ([]domain.Section, error)
}
