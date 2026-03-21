package opensearch

import (
	"bilge-lib/internal/ingestion/chunking"
	"time"
)

type ChunkIndexRecord struct {
	DocumentID   string    `json:"document_id"`
	ChunkID      string    `json:"chunk_id"`
	Path         string    `json:"path"`
	FileType     string    `json:"file_type"`
	Title        string    `json:"title"`
	Language     string    `json:"language,omitempty"`
	DocumentType string    `json:"document_type,omitempty"`
	TopTerms     []string  `json:"top_terms,omitempty"`
	KeyPhrases   []string  `json:"key_phrases,omitempty"`
	Acronyms     []string  `json:"acronyms,omitempty"`
	Aliases      []string  `json:"aliases,omitempty"`
	Text         string    `json:"text"`
	Section      string    `json:"section,omitempty"`
	ChunkOrder   int       `json:"chunk_order"`
	PrevChunkID  string    `json:"prev_chunk_id,omitempty"`
	NextChunkID  string    `json:"next_chunk_id,omitempty"`
	DocHash      string    `json:"doc_hash"`
	WarningCount int       `json:"warning_count"`
	IngestedAt   time.Time `json:"ingested_at"`
}

type IngestStatusRecord struct {
	RunID      string    `json:"run_id"`
	Path       string    `json:"path"`
	FileType   string    `json:"file_type"`
	Status     string    `json:"status"`
	Stage      string    `json:"stage"`
	Warnings   []string  `json:"warnings,omitempty"`
	Error      string    `json:"error,omitempty"`
	DocumentID string    `json:"document_id,omitempty"`
	ChunkCount int       `json:"chunk_count"`
	RecordedAt time.Time `json:"recorded_at"`
}

func MapChunkedFileToChunkRecords(file *chunking.ChunkedFile, warningCount int, ingestedAt time.Time) []ChunkIndexRecord {
	if file == nil {
		return nil
	}

	records := make([]ChunkIndexRecord, 0, len(file.Chunks))
	for _, chunk := range file.Chunks {
		if chunk == nil {
			continue
		}

		section := chunk.SectionPath
		if section == "" {
			section = chunk.Heading
		}

		records = append(records, ChunkIndexRecord{
			DocumentID:   string(file.SourceDocument.ID),
			ChunkID:      string(chunk.ID),
			Path:         file.SourceDocument.Path,
			FileType:     string(file.SourceDocument.FileType),
			Title:        file.SourceDocument.Title,
			Language:     file.SourceDocument.Passport.Language,
			DocumentType: file.SourceDocument.Passport.DocumentType,
			TopTerms:     cloneSlice(file.SourceDocument.Passport.TopTerms),
			KeyPhrases:   cloneSlice(file.SourceDocument.Passport.KeyPhrases),
			Acronyms:     cloneSlice(file.SourceDocument.Passport.Acronyms),
			Aliases:      cloneSlice(file.SourceDocument.Passport.Aliases),
			Text:         chunk.Content,
			Section:      section,
			ChunkOrder:   chunk.ChunkIndex,
			PrevChunkID:  string(chunk.PrevChunkID),
			NextChunkID:  string(chunk.NextChunkID),
			DocHash:      file.SourceDocument.DocHash,
			WarningCount: warningCount,
			IngestedAt:   ingestedAt,
		})
	}

	return records
}

func cloneSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
