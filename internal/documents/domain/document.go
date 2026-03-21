package domain

type DocumentID string
type SectionID string

type Document struct {
	ID       DocumentID
	Title    string
	Source   string
	Path     string
	Version  string
	Language string
}
