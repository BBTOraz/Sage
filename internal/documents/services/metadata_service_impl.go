package services

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
)

type DefaultMetadataService struct {
	Store ports.MetadataStore
}

func NewMetadataService(store ports.MetadataStore) *DefaultMetadataService {
	return &DefaultMetadataService{Store: store}
}

func (s *DefaultMetadataService) GetDocument(ctx context.Context, id domain.DocumentID) (domain.Document, error) {
	return s.Store.GetDocument(ctx, id)
}

func (s *DefaultMetadataService) ListSections(ctx context.Context, id domain.DocumentID) ([]domain.Section, error) {
	return s.Store.ListSections(ctx, id)
}
