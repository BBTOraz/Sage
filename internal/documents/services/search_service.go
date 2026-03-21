package services

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
)

type SearchService interface {
	Search(ctx context.Context, query domain.SearchQuery) (ports.SearchPage, error)
	NextPage(ctx context.Context, queryID domain.QueryID, cursor string) (ports.SearchPage, error)
}
