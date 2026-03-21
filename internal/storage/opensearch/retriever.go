package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

var (
	ErrEmptySearchQuestion = errors.New("search question is empty")
	ErrCursorQueryMismatch = errors.New("cursor query id does not match requested query id")
)

type Retriever struct {
	Client  *Client
	builder *queryBuilder
}

func NewRetriever(client *Client) *Retriever {
	return &Retriever{
		Client:  client,
		builder: newQueryBuilder(client.Config.ChunkIndex()),
	}
}

func (r *Retriever) Search(ctx context.Context, query domain.SearchQuery) (ports.SearchPage, error) {
	query = query.Normalize()
	query.UserQuestion = strings.TrimSpace(query.UserQuestion)
	if query.UserQuestion == "" {
		return ports.SearchPage{}, ErrEmptySearchQuestion
	}
	if query.ID == "" {
		query.ID = makeQueryID(query)
	}

	req, err := r.builder.BuildSearchRequest(query)
	if err != nil {
		return ports.SearchPage{}, err
	}

	resp, err := r.Client.API.Search(ctx, &req)
	if err != nil {
		return ports.SearchPage{}, fmt.Errorf("search chunks: %w", err)
	}

	cursor := newSearchCursor(query, nil)
	return mapSearchResponse(domain.QueryID(cursor.QueryID), query.PageSize, cursor, resp.Hits.Hits)
}

func (r *Retriever) NextPage(ctx context.Context, queryID domain.QueryID, rawCursor string) (ports.SearchPage, error) {
	cursor, err := decodeCursor(rawCursor)
	if err != nil {
		return ports.SearchPage{}, err
	}
	if queryID != "" && cursor.QueryID != string(queryID) {
		return ports.SearchPage{}, ErrCursorQueryMismatch
	}

	req, err := r.builder.BuildNextPageRequest(cursor)
	if err != nil {
		return ports.SearchPage{}, err
	}

	resp, err := r.Client.API.Search(ctx, &req)
	if err != nil {
		return ports.SearchPage{}, fmt.Errorf("search next page: %w", err)
	}

	return mapSearchResponse(domain.QueryID(cursor.QueryID), cursor.PageSize, cursor, resp.Hits.Hits)
}

func mapSearchResponse(queryID domain.QueryID, pageSize int, cursor searchCursor, hits []opensearchapi.SearchHit) (ports.SearchPage, error) {
	hasMore := len(hits) > pageSize
	if hasMore {
		hits = hits[:pageSize]
	}

	items := make([]ports.SearchHit, 0, len(hits))
	for _, hit := range hits {
		mapped, err := mapSearchHit(hit)
		if err != nil {
			return ports.SearchPage{}, err
		}
		items = append(items, mapped)
	}

	nextCursor := ""
	if hasMore && len(hits) != 0 {
		cursor.SearchAfter = hits[len(hits)-1].Sort
		encoded, err := encodeCursor(cursor)
		if err != nil {
			return ports.SearchPage{}, err
		}
		nextCursor = encoded
	}

	return ports.SearchPage{
		QueryID:    queryID,
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
