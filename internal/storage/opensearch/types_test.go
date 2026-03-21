package opensearch

import (
	"bilge-lib/internal/ingestion/chunking"
	"bilge-lib/internal/ingestion/loader"
	"testing"
	"time"
)

func TestMapChunkedFileToChunkRecords(t *testing.T) {
	now := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	file := &chunking.ChunkedFile{
		SourceDocument: chunking.SourceDocument{
			ID:       chunking.DocumentID("doc-1"),
			Path:     "C:\\docs\\policy.pdf",
			FileType: loader.FileTypePDF,
			Title:    "Policy",
			DocHash:  "hash-1",
			Passport: chunking.DocumentPassport{
				Language:     "en",
				DocumentType: "guide",
				TopTerms:     []string{"policy", "security"},
				KeyPhrases:   []string{"Security Policy"},
				Acronyms:     []string{"CSP"},
				Aliases:      []string{"Content Security Policy"},
			},
		},
		Chunks: []*chunking.Chunk{
			{
				ID:          chunking.ChunkID("doc-1:0"),
				DocumentID:  chunking.DocumentID("doc-1"),
				ChunkIndex:  0,
				Content:     "first",
				Heading:     "Overview",
				NextChunkID: chunking.ChunkID("doc-1:1"),
			},
			{
				ID:          chunking.ChunkID("doc-1:1"),
				DocumentID:  chunking.DocumentID("doc-1"),
				ChunkIndex:  1,
				Content:     "second",
				SectionPath: "Restrictions",
				PrevChunkID: chunking.ChunkID("doc-1:0"),
			},
		},
	}

	records := MapChunkedFileToChunkRecords(file, 2, now)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].Section != "Overview" {
		t.Fatalf("expected heading fallback section, got %q", records[0].Section)
	}
	if records[1].Section != "Restrictions" {
		t.Fatalf("expected explicit section, got %q", records[1].Section)
	}
	if records[0].WarningCount != 2 || records[1].WarningCount != 2 {
		t.Fatalf("expected warning count to propagate, got %+v", records)
	}
	if records[0].Language != "en" || records[0].DocumentType != "guide" {
		t.Fatalf("expected passport metadata to propagate, got %+v", records[0])
	}
	if len(records[0].TopTerms) == 0 || len(records[0].Aliases) == 0 {
		t.Fatalf("expected passport arrays to propagate, got %+v", records[0])
	}
	if !records[0].IngestedAt.Equal(now) || !records[1].IngestedAt.Equal(now) {
		t.Fatalf("expected ingested_at to propagate, got %+v", records)
	}
}
