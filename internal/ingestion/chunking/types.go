package chunking

import (
	"bilge-lib/internal/ingestion/loader"
	"bilge-lib/internal/ingestion/parser"
	"context"
	"time"
)

type DocumentID string
type ChunkID string

type DocumentPassport struct {
	Language     string
	DocumentType string
	TopTerms     []string
	KeyPhrases   []string
	Acronyms     []string
	Aliases      []string
}

type SourceDocument struct {
	ID        DocumentID
	Path      string
	Name      string
	FileType  loader.FileType
	Title     string
	DocHash   string
	UpdatedAt time.Time
	Language  string
	Passport  DocumentPassport
}

type Chunk struct {
	ID         ChunkID
	DocumentID DocumentID
	ChunkIndex int
	Content    string

	SectionPath  string
	Heading      string
	HeadingLevel int
	Page         int
	StartOffset  int
	EndOffset    int

	PrevChunkID ChunkID
	NextChunkID ChunkID
}

type ChunkedFile struct {
	SourceDocument SourceDocument
	Chunks         []*Chunk
}

type Transformer interface {
	Transform(ctx context.Context, parsed *parser.ParsedFile) (*ChunkedFile, error)
}
