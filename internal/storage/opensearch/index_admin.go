package opensearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type IndexEnsurer interface {
	EnsureIndexes(ctx context.Context) error
}

type IndexAdmin struct {
	Client *Client
}

func NewIndexAdmin(client *Client) *IndexAdmin {
	return &IndexAdmin{Client: client}
}

func (a *IndexAdmin) EnsureIndexes(ctx context.Context) error {
	if err := a.ensureIndex(ctx, a.Client.Config.ChunkIndex(), chunkIndexMapping()); err != nil {
		return err
	}
	if err := a.ensureIndex(ctx, a.Client.Config.StatusIndex(), ingestStatusIndexMapping()); err != nil {
		return err
	}

	return nil
}

func (a *IndexAdmin) ensureIndex(ctx context.Context, index string, mapping map[string]any) error {
	existsResp, err := a.Client.API.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{
		Indices: []string{index},
	})
	if existsResp != nil {
		defer existsResp.Body.Close()
	}
	if err != nil && (existsResp == nil || existsResp.StatusCode != 404) {
		return fmt.Errorf("check index %q existence: %w", index, err)
	}
	if existsResp.StatusCode == 200 {
		return nil
	}
	if existsResp.StatusCode != 404 {
		return fmt.Errorf("check index %q existence: unexpected status %s", index, existsResp.Status())
	}

	body, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshal mapping for %q: %w", index, err)
	}

	if _, err := a.Client.API.Indices.Create(ctx, opensearchapi.IndicesCreateReq{
		Index: index,
		Body:  strings.NewReader(string(body)),
	}); err != nil {
		return fmt.Errorf("create index %q: %w", index, err)
	}

	return nil
}

func chunkIndexMapping() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"index": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"document_id":   keywordField(),
				"chunk_id":      keywordField(),
				"path":          keywordField(),
				"file_type":     keywordField(),
				"language":      keywordField(),
				"document_type": keywordField(),
				"title":         textField(),
				"top_terms":     textField(),
				"key_phrases":   textField(),
				"acronyms":      textField(),
				"aliases":       textField(),
				"text":          textField(),
				"section":       textField(),
				"chunk_order":   map[string]any{"type": "integer"},
				"prev_chunk_id": keywordField(),
				"next_chunk_id": keywordField(),
				"doc_hash":      keywordField(),
				"warning_count": map[string]any{"type": "integer"},
				"ingested_at":   map[string]any{"type": "date"},
			},
		},
	}
}

func ingestStatusIndexMapping() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"index": map[string]any{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"run_id":      keywordField(),
				"path":        keywordField(),
				"file_type":   keywordField(),
				"status":      keywordField(),
				"stage":       keywordField(),
				"warnings":    map[string]any{"type": "keyword"},
				"error":       textField(),
				"document_id": keywordField(),
				"chunk_count": map[string]any{"type": "integer"},
				"recorded_at": map[string]any{"type": "date"},
			},
		},
	}
}

func keywordField() map[string]any {
	return map[string]any{"type": "keyword"}
}

func textField() map[string]any {
	return map[string]any{"type": "text"}
}
