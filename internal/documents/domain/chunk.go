package domain

type ChunkID string

type Chunk struct {
	ID          ChunkID
	DocumentID  DocumentID
	Text        string
	Order       int
	Page        int
	Section     string
	StartOffset int
	EndOffset   int
}
