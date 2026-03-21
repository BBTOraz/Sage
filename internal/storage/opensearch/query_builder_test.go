package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"encoding/json"
	"io"
	"testing"
)

func TestQueryBuilderBuildSearchRequest(t *testing.T) {
	builder := newQueryBuilder("sage_chunks")

	req, err := builder.BuildSearchRequest(domain.SearchQuery{
		UserQuestion: "termination clause",
		Filters: domain.SearchFilters{
			DocumentIDs: []domain.DocumentID{"doc-1"},
			Source:      "C:\\docs\\contract.pdf",
		},
		PageSize: 5,
	})
	if err != nil {
		t.Fatalf("BuildSearchRequest() error = %v", err)
	}

	if len(req.Indices) != 1 || req.Indices[0] != "sage_chunks" {
		t.Fatalf("unexpected indices: %+v", req.Indices)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if payload["size"].(float64) != 6 {
		t.Fatalf("expected lookahead page size 6, got %+v", payload["size"])
	}
	if _, ok := payload["highlight"]; !ok {
		t.Fatal("expected highlight block")
	}

	query := payload["query"].(map[string]any)
	boolQuery := query["bool"].(map[string]any)
	must := boolQuery["must"].([]any)
	innerBool := must[0].(map[string]any)["bool"].(map[string]any)
	should := innerBool["should"].([]any)
	if len(should) != 1 {
		t.Fatalf("expected a single primary query clause, got %+v", should)
	}
	multiMatch := should[0].(map[string]any)["multi_match"].(map[string]any)
	fields := multiMatch["fields"].([]any)
	rawFields := make([]string, 0, len(fields))
	for _, field := range fields {
		rawFields = append(rawFields, field.(string))
	}
	expected := []string{"text^4", "title^3", "section^2", "aliases^3", "acronyms^3", "key_phrases^2", "top_terms"}
	for _, field := range expected {
		found := false
		for _, item := range rawFields {
			if item == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected boosted field %q in %+v", field, rawFields)
		}
	}
}

func TestQueryBuilderRejectsTagFilter(t *testing.T) {
	builder := newQueryBuilder("sage_chunks")

	_, err := builder.BuildSearchRequest(domain.SearchQuery{
		UserQuestion: "termination clause",
		Filters: domain.SearchFilters{
			Tags: []string{"legal"},
		},
	})
	if err == nil {
		t.Fatal("expected tag filter to be rejected")
	}
}
