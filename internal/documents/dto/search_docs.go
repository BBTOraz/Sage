package dto

type SearchDocsInput struct {
	Question           string   `json:"question" jsonschema_description:"Original natural-language search request from the user. This should stay close to the user's wording." jsonschema:"required"`
	AlternateQuestions []string `json:"alternate_questions,omitempty" jsonschema_description:"Optional alternate phrasings, translations, synonyms, acronym expansions, or keyword-focused queries to search together with the original question in one call."`
	DocumentIDs        []string `json:"document_ids,omitempty" jsonschema_description:"Optional exact document ids to limit the search to specific indexed documents."`
	Source             string   `json:"source,omitempty" jsonschema_description:"Optional exact source path or source identifier to narrow the search."`
	PageSize           int      `json:"page_size,omitempty" jsonschema_description:"Maximum number of search hits to return. Prefer a small value such as 3 to 8."`
}

type SearchHit struct {
	DocumentID string  `json:"document_id" jsonschema_description:"Exact indexed document identifier that owns the matching chunk."`
	ChunkID    string  `json:"chunk_id" jsonschema_description:"Exact indexed chunk identifier for the matching fragment."`
	Score      float64 `json:"score" jsonschema_description:"OpenSearch relevance score for this hit. Higher usually means a better lexical match."`
	Snippet    string  `json:"snippet" jsonschema_description:"Highlighted fragment or best-effort excerpt showing why this chunk matched the search."`
	Page       int     `json:"page,omitempty" jsonschema_description:"Best-effort page number when available from document extraction."`
	Section    string  `json:"section,omitempty" jsonschema_description:"Best-effort section or heading label for the matching chunk."`
}

type SearchDocsOutput struct {
	QueryID    string      `json:"query_id" jsonschema_description:"Stable query identifier for this search session. Reuse it when requesting the next page."`
	Results    []SearchHit `json:"results" jsonschema_description:"Ranked search hits for the current page."`
	NextCursor string      `json:"next_cursor,omitempty" jsonschema_description:"Opaque cursor for fetching the next page of this exact search."`
	HasMore    bool        `json:"has_more" jsonschema_description:"Whether more results are available after this page."`
}
