package tools

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
	"encoding/json"
	"testing"
)

type stubSearchService struct {
	query domain.SearchQuery
	page  ports.SearchPage
}

func (s *stubSearchService) Search(ctx context.Context, query domain.SearchQuery) (ports.SearchPage, error) {
	s.query = query
	return s.page, nil
}

func (s *stubSearchService) NextPage(ctx context.Context, queryID domain.QueryID, cursor string) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}

func TestSearchDocsToolMapsInputAndOutput(t *testing.T) {
	service := &stubSearchService{
		page: ports.SearchPage{
			QueryID:    "query-1",
			NextCursor: "cursor-1",
			HasMore:    true,
			Items: []ports.SearchHit{
				{
					DocumentID: "doc-1",
					ChunkID:    "doc-1:0",
					Score:      0.42,
					Snippet:    "snippet",
					Page:       2,
					Section:    "Overview",
				},
			},
		},
	}

	tool, err := NewSearchDocsTool(service)
	if err != nil {
		t.Fatalf("NewSearchDocsTool() error = %v", err)
	}

	output, err := tool.InvokableRun(context.Background(), `{
		"question":"original query",
		"alternate_questions":["alternate query","alternate query ru"],
		"document_ids":["doc-2","doc-1"],
		"source":"C:\\docs\\policy.pdf",
		"page_size":5
	}`)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	if service.query.UserQuestion != "original query" {
		t.Fatalf("expected main question to be mapped, got %q", service.query.UserQuestion)
	}
	if len(service.query.AlternateQuestions) != 2 {
		t.Fatalf("expected alternate questions to be mapped, got %+v", service.query.AlternateQuestions)
	}
	if len(service.query.Filters.DocumentIDs) != 2 {
		t.Fatalf("expected multiple document ids to be mapped, got %+v", service.query.Filters.DocumentIDs)
	}
	if service.query.Filters.Source != "C:\\docs\\policy.pdf" {
		t.Fatalf("expected source to be mapped, got %q", service.query.Filters.Source)
	}
	if service.query.PageSize != 5 {
		t.Fatalf("expected page size to be mapped, got %d", service.query.PageSize)
	}

	var payload struct {
		QueryID    string `json:"query_id"`
		NextCursor string `json:"next_cursor"`
		HasMore    bool   `json:"has_more"`
		Results    []struct {
			DocumentID string `json:"document_id"`
			ChunkID    string `json:"chunk_id"`
			Section    string `json:"section"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if payload.QueryID != "query-1" || payload.NextCursor != "cursor-1" || !payload.HasMore {
		t.Fatalf("unexpected output payload %+v", payload)
	}
	if len(payload.Results) != 1 || payload.Results[0].DocumentID != "doc-1" || payload.Results[0].ChunkID != "doc-1:0" {
		t.Fatalf("unexpected tool results %+v", payload.Results)
	}
}
