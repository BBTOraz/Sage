package domain

import "strings"

const (
	DefaultSearchPageSize = 10
	MaxSearchPageSize     = 50
)

type QueryID string

type SearchFilters struct {
	DocumentIDs []DocumentID
	Tags        []string
	Source      string
}

type SearchQuery struct {
	ID                 QueryID
	UserQuestion       string
	AlternateQuestions []string
	Filters            SearchFilters
	PageSize           int
	Cursor             string
}

func (q SearchQuery) Normalize() SearchQuery {
	q.UserQuestion = strings.TrimSpace(q.UserQuestion)
	q.AlternateQuestions = normalizeAlternateQuestions(q.AlternateQuestions, q.UserQuestion)

	switch {
	case q.PageSize <= 0:
		q.PageSize = DefaultSearchPageSize
	case q.PageSize > MaxSearchPageSize:
		q.PageSize = MaxSearchPageSize
	}

	return q
}

func normalizeAlternateQuestions(values []string, mainQuestion string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := map[string]struct{}{
		mainQuestion: {},
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}

	return out
}
