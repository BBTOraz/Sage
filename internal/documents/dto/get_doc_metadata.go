package dto

type GetDocMetadataInput struct {
	DocumentID      string `json:"document_id" jsonschema_description:"Exact indexed document identifier whose metadata should be returned." jsonschema:"required"`
	IncludeSections bool   `json:"include_sections,omitempty" jsonschema_description:"When true, include the document's known section list so the agent can understand its structure before searching more deeply."`
}

type Section struct {
	ID        string `json:"id" jsonschema_description:"Stable section identifier within the document."`
	Title     string `json:"title" jsonschema_description:"Human-readable section heading or title."`
	Level     int    `json:"level" jsonschema_description:"Section nesting level. Lower usually means a higher-level heading."`
	PageStart int    `json:"page_start,omitempty" jsonschema_description:"Best-effort starting page for the section when available."`
	PageEnd   int    `json:"page_end,omitempty" jsonschema_description:"Best-effort ending page for the section when available."`
}

type GetDocMetadataOutput struct {
	DocumentID string    `json:"document_id" jsonschema_description:"Exact indexed document identifier for this metadata result."`
	Title      string    `json:"title" jsonschema_description:"Best available document title."`
	Source     string    `json:"source,omitempty" jsonschema_description:"Logical source label when the document has one."`
	Path       string    `json:"path,omitempty" jsonschema_description:"Filesystem path or source path used during ingest."`
	Sections   []Section `json:"sections,omitempty" jsonschema_description:"Optional section map for the document when requested."`
}
