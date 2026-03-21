package dto

type CandidateRef struct {
	DocumentID string `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
}

type CollectEvidenceInput struct {
	Question   string         `json:"question"`
	Candidates []CandidateRef `json:"candidates"`
}

type ClaimEvidence struct {
	Claim      string   `json:"claim"`
	Citations  []string `json:"citations"`
	Confidence float64  `json:"confidence"`
}

type CollectEvidenceOutput struct {
	Claims    []ClaimEvidence `json:"claims"`
	Gaps      []string        `json:"gaps,omitempty"`
	Conflicts []string        `json:"conflicts,omitempty"`
}
