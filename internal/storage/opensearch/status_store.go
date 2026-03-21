package opensearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type StatusWriter interface {
	WriteStatus(ctx context.Context, record IngestStatusRecord) error
}

type StatusStore struct {
	Client *Client
}

func NewStatusStore(client *Client) *StatusStore {
	return &StatusStore{Client: client}
}

func (s *StatusStore) WriteStatus(ctx context.Context, record IngestStatusRecord) error {
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal ingest status for %q: %w", record.Path, err)
	}

	if _, err := s.Client.API.Index(ctx, opensearchapi.IndexReq{
		Index:      s.Client.Config.StatusIndex(),
		DocumentID: statusRecordID(record),
		Body:       bytes.NewReader(body),
	}); err != nil {
		return fmt.Errorf("index ingest status for %q: %w", record.Path, err)
	}

	return nil
}

func statusRecordID(record IngestStatusRecord) string {
	sum := sha256.Sum256([]byte(record.RunID + ":" + record.Path))
	return hex.EncodeToString(sum[:])
}
