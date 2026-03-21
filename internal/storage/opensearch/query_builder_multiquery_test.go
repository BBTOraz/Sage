package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"encoding/json"
	"io"
	"testing"
)

func TestQueryBuilderBuildSearchRequestUsesAlternateQuestionsInSingleQuery(t *testing.T) {
	builder := newQueryBuilder("sage_chunks")

	req, err := builder.BuildSearchRequest(domain.SearchQuery{
		UserQuestion:       "spring security csrf",
		AlternateQuestions: []string{"cross-site request forgery", "защита от csrf"},
		PageSize:           5,
	})
	if err != nil {
		t.Fatalf("BuildSearchRequest() error = %v", err)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	query := payload["query"].(map[string]any)
	boolQuery := query["bool"].(map[string]any)
	must := boolQuery["must"].([]any)
	innerBool := must[0].(map[string]any)["bool"].(map[string]any)
	should := innerBool["should"].([]any)
	if len(should) != 3 {
		t.Fatalf("expected one should clause per question variant, got %d", len(should))
	}

	first := should[0].(map[string]any)["multi_match"].(map[string]any)
	if first["query"].(string) != "spring security csrf" {
		t.Fatalf("expected original query first, got %+v", first)
	}
	if first["boost"].(float64) != 1 {
		t.Fatalf("expected primary query boost 1, got %+v", first["boost"])
	}

	second := should[1].(map[string]any)["multi_match"].(map[string]any)
	if second["query"].(string) != "cross-site request forgery" {
		t.Fatalf("unexpected first alternate query %+v", second)
	}
	if second["boost"].(float64) >= 1 {
		t.Fatalf("expected alternate query to have lower boost, got %+v", second["boost"])
	}
}
