package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

var ErrChunkNotFound = errors.New("chunk not found")

type ChunkReader struct {
	Client *Client
}

func NewChunkReader(client *Client) *ChunkReader {
	return &ChunkReader{Client: client}
}

func (r *ChunkReader) OpenWindow(ctx context.Context, documentID domain.DocumentID, chunkID domain.ChunkID, opts ports.WindowOptions) (ports.ChunkWindow, error) {
	anchor, err := r.getChunk(ctx, chunkID)
	if err != nil {
		return ports.ChunkWindow{}, err
	}
	if anchor.DocumentID != string(documentID) {
		return ports.ChunkWindow{}, ErrChunkNotFound
	}

	before := opts.Before
	if before < 0 {
		before = 0
	}
	after := opts.After
	if after < 0 {
		after = 0
	}

	start := anchor.ChunkOrder - before
	if start < 0 {
		start = 0
	}
	end := anchor.ChunkOrder + after

	req, err := chunkWindowRequest(r.Client.Config.ChunkIndex(), string(documentID), start, end)
	if err != nil {
		return ports.ChunkWindow{}, err
	}

	resp, err := r.Client.API.Search(ctx, &req)
	if err != nil {
		return ports.ChunkWindow{}, fmt.Errorf("open chunk window: %w", err)
	}

	chunks := make([]domain.Chunk, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		record, err := decodeChunkRecord(hit.Source)
		if err != nil {
			return ports.ChunkWindow{}, err
		}
		chunks = append(chunks, mapChunkRecordToDomainChunk(record))
	}

	return ports.ChunkWindow{
		DocumentID: documentID,
		AnchorID:   chunkID,
		Chunks:     chunks,
	}, nil
}

func (r *ChunkReader) getChunk(ctx context.Context, chunkID domain.ChunkID) (ChunkIndexRecord, error) {
	resp, err := r.Client.API.Document.Get(ctx, opensearchapi.DocumentGetReq{
		Index:      r.Client.Config.ChunkIndex(),
		DocumentID: string(chunkID),
	})
	if err != nil {
		return ChunkIndexRecord{}, fmt.Errorf("get chunk %q: %w", chunkID, err)
	}
	if !resp.Found {
		return ChunkIndexRecord{}, ErrChunkNotFound
	}

	return decodeChunkRecord(resp.Source)
}

func chunkWindowRequest(index, documentID string, start, end int) (opensearchapi.SearchReq, error) {
	body := map[string]any{
		"size": end - start + 1,
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{"term": map[string]any{"document_id": documentID}},
					map[string]any{"range": map[string]any{"chunk_order": map[string]any{
						"gte": start,
						"lte": end,
					}}},
				},
			},
		},
		"sort": []any{
			map[string]any{"chunk_order": "asc"},
		},
		"_source": []string{
			"document_id",
			"chunk_id",
			"text",
			"section",
			"chunk_order",
		},
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return opensearchapi.SearchReq{}, fmt.Errorf("marshal chunk window body: %w", err)
	}

	return opensearchapi.SearchReq{
		Indices: []string{index},
		Body:    bytes.NewReader(rawBody),
	}, nil
}
