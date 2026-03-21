package opensearch

import (
	"bilge-lib/internal/documents/domain"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

var ErrDocumentNotFound = errors.New("document not found")

type MetadataStore struct {
	Client *Client
}

func NewMetadataStore(client *Client) *MetadataStore {
	return &MetadataStore{Client: client}
}

func (s *MetadataStore) GetDocument(ctx context.Context, id domain.DocumentID) (domain.Document, error) {
	req, err := metadataSearchRequest(s.Client.Config.ChunkIndex(), string(id), []string{
		"document_id",
		"path",
		"title",
		"doc_hash",
		"chunk_order",
	}, 1)
	if err != nil {
		return domain.Document{}, err
	}

	resp, err := s.Client.API.Search(ctx, &req)
	if err != nil {
		return domain.Document{}, fmt.Errorf("get document metadata: %w", err)
	}
	if len(resp.Hits.Hits) == 0 {
		return domain.Document{}, ErrDocumentNotFound
	}

	record, err := decodeChunkRecord(resp.Hits.Hits[0].Source)
	if err != nil {
		return domain.Document{}, err
	}

	return mapChunkRecordToDocument(record), nil
}

func (s *MetadataStore) ListSections(ctx context.Context, id domain.DocumentID) ([]domain.Section, error) {
	req, err := metadataSearchRequest(s.Client.Config.ChunkIndex(), string(id), []string{
		"document_id",
		"section",
		"chunk_order",
	}, 1000)
	if err != nil {
		return nil, err
	}

	resp, err := s.Client.API.Search(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("list document sections: %w", err)
	}

	sections := make([]domain.Section, 0)
	seen := make(map[string]struct{})
	for _, hit := range resp.Hits.Hits {
		record, err := decodeChunkRecord(hit.Source)
		if err != nil {
			return nil, err
		}
		if record.Section == "" {
			continue
		}
		if _, ok := seen[record.Section]; ok {
			continue
		}

		order := len(sections)
		sections = append(sections, domain.Section{
			ID:         domain.SectionID(string(id) + ":" + strconv.Itoa(order)),
			DocumentID: id,
			Title:      record.Section,
			Order:      order,
		})
		seen[record.Section] = struct{}{}
	}

	return sections, nil
}

func metadataSearchRequest(index, documentID string, sourceFields []string, size int) (opensearchapi.SearchReq, error) {
	body := map[string]any{
		"size": size,
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{
						"term": map[string]any{
							"document_id": documentID,
						},
					},
				},
			},
		},
		"sort": []any{
			map[string]any{"chunk_order": "asc"},
		},
		"_source": sourceFields,
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return opensearchapi.SearchReq{}, fmt.Errorf("marshal metadata search body: %w", err)
	}

	return opensearchapi.SearchReq{
		Indices: []string{index},
		Body:    bytes.NewReader(rawBody),
	}, nil
}
