package domain

type Citation struct {
	DocumentID DocumentID
	ChunkID    ChunkID
	Title      string
	Page       int
	Section    string
	Snippet    string
}
