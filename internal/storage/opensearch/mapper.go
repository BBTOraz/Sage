package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

func mapSearchHit(hit opensearchapi.SearchHit) (ports.SearchHit, error) {
	record, err := decodeChunkRecord(hit.Source)
	if err != nil {
		return ports.SearchHit{}, err
	}

	return ports.SearchHit{
		DocumentID: domain.DocumentID(record.DocumentID),
		ChunkID:    domain.ChunkID(record.ChunkID),
		Score:      float64(hit.Score),
		Snippet:    bestSnippet(hit, record.Text),
		Page:       0,
		Section:    record.Section,
	}, nil
}

func mapChunkRecordToDocument(record ChunkIndexRecord) domain.Document {
	return domain.Document{
		ID:      domain.DocumentID(record.DocumentID),
		Title:   record.Title,
		Source:  filepath.Base(record.Path),
		Path:    record.Path,
		Version: record.DocHash,
	}
}

func mapChunkRecordToDomainChunk(record ChunkIndexRecord) domain.Chunk {
	return domain.Chunk{
		ID:         domain.ChunkID(record.ChunkID),
		DocumentID: domain.DocumentID(record.DocumentID),
		Text:       record.Text,
		Order:      record.ChunkOrder,
		Page:       0,
		Section:    record.Section,
	}
}

func decodeChunkRecord(raw json.RawMessage) (ChunkIndexRecord, error) {
	var record ChunkIndexRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return record, fmt.Errorf("decode chunk record: %w", err)
	}

	return record, nil
}

func bestSnippet(hit opensearchapi.SearchHit, fallback string) string {
	if highlights, ok := hit.Highlight["text"]; ok && len(highlights) != 0 {
		return strings.TrimSpace(highlights[0])
	}

	fallback = strings.TrimSpace(fallback)
	if len(fallback) <= 240 {
		return fallback
	}

	return fallback[:240] + "..."
}
