package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type ChunkIndexer interface {
	IndexChunks(ctx context.Context, records []ChunkIndexRecord) error
}

type Indexer struct {
	Client *Client
}

func NewIndexer(client *Client) *Indexer {
	return &Indexer{Client: client}
}

func (i *Indexer) IndexChunks(ctx context.Context, records []ChunkIndexRecord) error {
	if len(records) == 0 {
		return nil
	}

	var body bytes.Buffer
	for _, record := range records {
		meta := map[string]map[string]string{
			"index": {"_id": record.ChunkID},
		}

		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("marshal bulk meta for %q: %w", record.ChunkID, err)
		}
		docJSON, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal bulk record for %q: %w", record.ChunkID, err)
		}

		body.Write(metaJSON)
		body.WriteByte('\n')
		body.Write(docJSON)
		body.WriteByte('\n')
	}

	resp, err := i.Client.API.Bulk(ctx, opensearchapi.BulkReq{
		Index: i.Client.Config.ChunkIndex(),
		Body:  &body,
	})
	if err != nil {
		return fmt.Errorf("bulk index chunks: %w", err)
	}
	if !resp.Errors {
		return nil
	}

	for _, item := range resp.Items {
		for _, result := range item {
			if result.Error != nil {
				return fmt.Errorf("bulk index chunks: status=%d type=%s reason=%s", result.Status, result.Error.Type, result.Error.Reason)
			}
		}
	}

	return fmt.Errorf("bulk index chunks: completed with errors")
}
