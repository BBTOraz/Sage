package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

var ErrUnsupportedTagFilter = fmt.Errorf("tag filtering is not indexed yet")

type queryBuilder struct {
	index string
}

func newQueryBuilder(index string) *queryBuilder {
	return &queryBuilder{index: index}
}

func (b *queryBuilder) BuildSearchRequest(query domain.SearchQuery) (opensearchapi.SearchReq, error) {
	cursor := newSearchCursor(query, nil)
	return b.buildRequest(cursor)
}

func (b *queryBuilder) BuildNextPageRequest(cursor searchCursor) (opensearchapi.SearchReq, error) {
	return b.buildRequest(cursor)
}

func (b *queryBuilder) buildRequest(cursor searchCursor) (opensearchapi.SearchReq, error) {
	if len(cursor.Filters.Tags) != 0 {
		return opensearchapi.SearchReq{}, ErrUnsupportedTagFilter
	}

	should := make([]any, 0, 1+len(cursor.AlternateQuestions))
	for index, question := range append([]string{cursor.Question}, cursor.AlternateQuestions...) {
		boost := 1.0
		if index > 0 {
			boost = 0.6
		}

		should = append(should, map[string]any{
			"multi_match": map[string]any{
				"query": question,
				"fields": []string{
					"text^4",
					"title^3",
					"section^2",
					"aliases^3",
					"acronyms^3",
					"key_phrases^2",
					"top_terms",
				},
				"boost": boost,
			},
		})
	}

	boolQuery := map[string]any{
		"must": []any{
			map[string]any{
				"bool": map[string]any{
					"should":               should,
					"minimum_should_match": 1,
				},
			},
		},
	}

	filters := make([]any, 0, 2)
	if len(cursor.Filters.DocumentIDs) != 0 {
		filters = append(filters, map[string]any{
			"terms": map[string]any{
				"document_id": cursor.Filters.DocumentIDs,
			},
		})
	}
	if cursor.Filters.Source != "" {
		filters = append(filters, map[string]any{
			"term": map[string]any{
				"path": cursor.Filters.Source,
			},
		})
	}
	if len(filters) != 0 {
		boolQuery["filter"] = filters
	}

	body := map[string]any{
		"size":             cursor.PageSize + 1,
		"track_total_hits": true,
		"query": map[string]any{
			"bool": boolQuery,
		},
		"sort": []any{
			map[string]any{"_score": "desc"},
			map[string]any{"chunk_id": "asc"},
		},
		"highlight": map[string]any{
			"fields": map[string]any{
				"text": map[string]any{
					"fragment_size":       240,
					"number_of_fragments": 1,
				},
			},
		},
		"_source": []string{
			"document_id",
			"chunk_id",
			"path",
			"file_type",
			"title",
			"language",
			"document_type",
			"top_terms",
			"key_phrases",
			"acronyms",
			"aliases",
			"text",
			"section",
			"chunk_order",
			"prev_chunk_id",
			"next_chunk_id",
			"doc_hash",
			"warning_count",
			"ingested_at",
		},
	}
	if len(cursor.SearchAfter) != 0 {
		body["search_after"] = cursor.SearchAfter
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return opensearchapi.SearchReq{}, fmt.Errorf("marshal search body: %w", err)
	}

	return opensearchapi.SearchReq{
		Indices: []string{b.index},
		Body:    bytes.NewReader(rawBody),
	}, nil
}
