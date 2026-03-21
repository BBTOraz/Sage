package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
)

type searchCursor struct {
	QueryID            string              `json:"query_id"`
	Question           string              `json:"question"`
	AlternateQuestions []string            `json:"alternate_questions,omitempty"`
	Filters            cursorSearchFilters `json:"filters"`
	PageSize           int                 `json:"page_size"`
	SearchAfter        []any               `json:"search_after,omitempty"`
}

type cursorSearchFilters struct {
	DocumentIDs []string `json:"document_ids,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Source      string   `json:"source,omitempty"`
}

func newSearchCursor(query domain.SearchQuery, searchAfter []any) searchCursor {
	normalized := query.Normalize()
	queryID := normalized.ID
	if queryID == "" {
		queryID = makeQueryID(normalized)
	}

	return searchCursor{
		QueryID:            string(queryID),
		Question:           normalized.UserQuestion,
		AlternateQuestions: append([]string(nil), normalized.AlternateQuestions...),
		Filters: cursorSearchFilters{
			DocumentIDs: normalizeDocumentIDs(normalized.Filters.DocumentIDs),
			Tags:        normalizeStrings(normalized.Filters.Tags),
			Source:      normalized.Filters.Source,
		},
		PageSize:    normalized.PageSize,
		SearchAfter: searchAfter,
	}
}

func (c searchCursor) Query() domain.SearchQuery {
	ids := make([]domain.DocumentID, len(c.Filters.DocumentIDs))
	for i, id := range c.Filters.DocumentIDs {
		ids[i] = domain.DocumentID(id)
	}

	return domain.SearchQuery{
		ID:                 domain.QueryID(c.QueryID),
		UserQuestion:       c.Question,
		AlternateQuestions: append([]string(nil), c.AlternateQuestions...),
		Filters: domain.SearchFilters{
			DocumentIDs: ids,
			Tags:        append([]string(nil), c.Filters.Tags...),
			Source:      c.Filters.Source,
		},
		PageSize: c.PageSize,
	}.Normalize()
}

func encodeCursor(cursor searchCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(raw string) (searchCursor, error) {
	var cursor searchCursor
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor, fmt.Errorf("decode cursor: %w", err)
	}
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return cursor, fmt.Errorf("unmarshal cursor: %w", err)
	}

	return cursor, nil
}

func makeQueryID(query domain.SearchQuery) domain.QueryID {
	normalized := query.Normalize()
	ids := normalizeDocumentIDs(normalized.Filters.DocumentIDs)
	tags := normalizeStrings(normalized.Filters.Tags)
	return domain.QueryID(sha256Hex(fmt.Sprintf("%s|%v|%d|%v|%v|%s", normalized.UserQuestion, normalized.AlternateQuestions, normalized.PageSize, ids, tags, normalized.Filters.Source)))
}

func normalizeDocumentIDs(ids []domain.DocumentID) []string {
	if len(ids) == 0 {
		return nil
	}

	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	sort.Strings(out)
	return out
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
