package domain

type Section struct {
	ID         SectionID
	DocumentID DocumentID
	Title      string
	Level      int
	Order      int
	PageStart  int
	PageEnd    int
}
