package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"testing"
)

func TestCursorRoundTrip(t *testing.T) {
	raw, err := encodeCursor(newSearchCursor(domain.SearchQuery{
		ID:                 "query-1",
		UserQuestion:       "find restrictions",
		AlternateQuestions: []string{"restriction clause", "policy limit"},
		Filters: domain.SearchFilters{
			DocumentIDs: []domain.DocumentID{"doc-2", "doc-1"},
			Source:      "C:\\docs\\policy.pdf",
		},
		PageSize: 7,
	}, []any{0.42, "doc-1:3"}))
	if err != nil {
		t.Fatalf("encodeCursor() error = %v", err)
	}

	cursor, err := decodeCursor(raw)
	if err != nil {
		t.Fatalf("decodeCursor() error = %v", err)
	}

	if cursor.QueryID != "query-1" || cursor.PageSize != 7 {
		t.Fatalf("unexpected decoded cursor %+v", cursor)
	}
	if len(cursor.AlternateQuestions) != 2 || cursor.AlternateQuestions[0] != "restriction clause" {
		t.Fatalf("expected alternate questions to round-trip, got %+v", cursor.AlternateQuestions)
	}
	if len(cursor.Filters.DocumentIDs) != 2 || cursor.Filters.DocumentIDs[0] != "doc-1" {
		t.Fatalf("expected normalized document ids, got %+v", cursor.Filters.DocumentIDs)
	}
}
