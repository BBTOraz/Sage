package tools

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
	"encoding/json"
	"testing"
)

type nextPageStubSearchService struct {
	queryID domain.QueryID
	cursor  string
	page    ports.SearchPage
}

func (s *nextPageStubSearchService) Search(ctx context.Context, query domain.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}

func (s *nextPageStubSearchService) NextPage(ctx context.Context, queryID domain.QueryID, cursor string) (ports.SearchPage, error) {
	s.queryID = queryID
	s.cursor = cursor
	return s.page, nil
}

func TestNextPageToolMapsInputAndOutput(t *testing.T) {
	service := &nextPageStubSearchService{
		page: ports.SearchPage{
			QueryID:    "query-1",
			NextCursor: "cursor-2",
			HasMore:    true,
			Items: []ports.SearchHit{
				{
					DocumentID: "doc-1",
					ChunkID:    "doc-1:1",
					Score:      0.91,
					Snippet:    "snippet",
					Page:       3,
					Section:    "Restrictions",
				},
			},
		},
	}

	tool, err := NewNextPageTool(service)
	if err != nil {
		t.Fatalf("NewNextPageTool() error = %v", err)
	}

	output, err := tool.InvokableRun(context.Background(), `{"query_id":"query-1","cursor":"cursor-1"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	if service.queryID != "query-1" || service.cursor != "cursor-1" {
		t.Fatalf("expected next-page input to be forwarded, got queryID=%q cursor=%q", service.queryID, service.cursor)
	}

	var payload struct {
		QueryID    string `json:"query_id"`
		NextCursor string `json:"next_cursor"`
		HasMore    bool   `json:"has_more"`
		Results    []struct {
			DocumentID string `json:"document_id"`
			ChunkID    string `json:"chunk_id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if payload.QueryID != "query-1" || payload.NextCursor != "cursor-2" || !payload.HasMore {
		t.Fatalf("unexpected output payload %+v", payload)
	}
	if len(payload.Results) != 1 || payload.Results[0].DocumentID != "doc-1" || payload.Results[0].ChunkID != "doc-1:1" {
		t.Fatalf("unexpected tool results %+v", payload.Results)
	}
}
