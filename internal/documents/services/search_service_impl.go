package services

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
)

type DefaultSearchService struct {
	Retriever ports.Retriever
}

func NewSearchService(retriever ports.Retriever) *DefaultSearchService {
	return &DefaultSearchService{Retriever: retriever}
}

func (s *DefaultSearchService) Search(ctx context.Context, query domain.SearchQuery) (ports.SearchPage, error) {
	return s.Retriever.Search(ctx, query.Normalize())
}

func (s *DefaultSearchService) NextPage(ctx context.Context, queryID domain.QueryID, cursor string) (ports.SearchPage, error) {
	return s.Retriever.NextPage(ctx, queryID, cursor)
}
