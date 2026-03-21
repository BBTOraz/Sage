package domain

type EvidenceID string

type ClaimEvidence struct {
	ID         EvidenceID
	Claim      string
	Support    []Citation
	Confidence float64
}

type EvidenceGap struct {
	Claim  string
	Reason string
}

type EvidenceConflict struct {
	Claim     string
	Citations []Citation
}

type EvidenceBundle struct {
	Claims    []ClaimEvidence
	Gaps      []EvidenceGap
	Conflicts []EvidenceConflict
}
