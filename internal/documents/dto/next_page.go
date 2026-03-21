package dto

type NextPageInput struct {
	QueryID string `json:"query_id" jsonschema_description:"Stable query identifier returned by a previous search_docs or next_page call." jsonschema:"required"`
	Cursor  string `json:"cursor" jsonschema_description:"Opaque next-page cursor returned by a previous search result page. Reuse it exactly as received." jsonschema:"required"`
}

type NextPageOutput struct {
	QueryID    string      `json:"query_id" jsonschema_description:"Stable query identifier for the continued search session."`
	Results    []SearchHit `json:"results" jsonschema_description:"Ranked search hits for the requested next page."`
	NextCursor string      `json:"next_cursor,omitempty" jsonschema_description:"Opaque cursor for fetching the following page of this same search session."`
	HasMore    bool        `json:"has_more" jsonschema_description:"Whether another page is available after this one."`
}
