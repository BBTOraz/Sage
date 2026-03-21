package ports

import (
	"bilge-lib/internal/documents/domain"
	"context"
)

type SearchHit struct {
	DocumentID domain.DocumentID
	ChunkID    domain.ChunkID
	Score      float64
	Snippet    string
	Page       int
	Section    string
}

type SearchPage struct {
	QueryID    domain.QueryID
	Items      []SearchHit
	NextCursor string
	HasMore    bool
}

type Retriever interface {
	Search(ctx context.Context, query domain.SearchQuery) (SearchPage, error)
	NextPage(ctx context.Context, queryID domain.QueryID, cursor string) (SearchPage, error)
}
