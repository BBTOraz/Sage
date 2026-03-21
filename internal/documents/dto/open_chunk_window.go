package dto

type OpenChunkWindowInput struct {
	DocumentID string `json:"document_id" jsonschema_description:"Exact indexed document identifier that owns the anchor chunk." jsonschema:"required"`
	ChunkID    string `json:"chunk_id" jsonschema_description:"Exact anchor chunk identifier around which to open the window." jsonschema:"required"`
	Before     int    `json:"before,omitempty" jsonschema_description:"How many chunks before the anchor to include in the returned window."`
	After      int    `json:"after,omitempty" jsonschema_description:"How many chunks after the anchor to include in the returned window."`
}

type Chunk struct {
	ID      string `json:"id" jsonschema_description:"Exact indexed chunk identifier."`
	Text    string `json:"text" jsonschema_description:"Full chunk text returned for local context inspection."`
	Order   int    `json:"order" jsonschema_description:"Chunk order within the document."`
	Page    int    `json:"page,omitempty" jsonschema_description:"Best-effort page number for the chunk when available."`
	Section string `json:"section,omitempty" jsonschema_description:"Best-effort section or heading label for the chunk."`
}

type OpenChunkWindowOutput struct {
	DocumentID string  `json:"document_id" jsonschema_description:"Exact indexed document identifier for this window."`
	AnchorID   string  `json:"anchor_id" jsonschema_description:"Anchor chunk identifier that the window is centered on."`
	Window     []Chunk `json:"window" jsonschema_description:"Ordered chunk window containing the anchor and its requested neighbors."`
}
