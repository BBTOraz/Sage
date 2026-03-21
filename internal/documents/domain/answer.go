package domain

type GroundedAnswer struct {
	Summary     string
	Citations   []Citation
	Uncertainty []string
}
