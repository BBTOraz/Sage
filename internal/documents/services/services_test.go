package services

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
	"context"
	"testing"
)

type stubRetriever struct {
	query    domain.SearchQuery
	queryID  domain.QueryID
	cursor   string
	search   ports.SearchPage
	nextPage ports.SearchPage
}

func (s *stubRetriever) Search(ctx context.Context, query domain.SearchQuery) (ports.SearchPage, error) {
	s.query = query
	return s.search, nil
}

func (s *stubRetriever) NextPage(ctx context.Context, queryID domain.QueryID, cursor string) (ports.SearchPage, error) {
	s.queryID = queryID
	s.cursor = cursor
	return s.nextPage, nil
}

type stubChunkReader struct {
	documentID domain.DocumentID
	chunkID    domain.ChunkID
	opts       ports.WindowOptions
	window     ports.ChunkWindow
}

func (s *stubChunkReader) OpenWindow(ctx context.Context, documentID domain.DocumentID, chunkID domain.ChunkID, opts ports.WindowOptions) (ports.ChunkWindow, error) {
	s.documentID = documentID
	s.chunkID = chunkID
	s.opts = opts
	return s.window, nil
}

type stubMetadataStore struct {
	documentID domain.DocumentID
	document   domain.Document
	sections   []domain.Section
}

func (s *stubMetadataStore) GetDocument(ctx context.Context, id domain.DocumentID) (domain.Document, error) {
	s.documentID = id
	return s.document, nil
}

func (s *stubMetadataStore) ListSections(ctx context.Context, id domain.DocumentID) ([]domain.Section, error) {
	s.documentID = id
	return s.sections, nil
}

func TestDefaultSearchServiceNormalizesQuery(t *testing.T) {
	retriever := &stubRetriever{}
	service := NewSearchService(retriever)

	if _, err := service.Search(context.Background(), domain.SearchQuery{UserQuestion: "test", PageSize: 0}); err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if retriever.query.PageSize != domain.DefaultSearchPageSize {
		t.Fatalf("expected query to be normalized, got page size %d", retriever.query.PageSize)
	}
}

func TestDefaultChunkServiceClampsWindowOptions(t *testing.T) {
	reader := &stubChunkReader{}
	service := NewChunkService(reader)

	if _, err := service.OpenWindow(context.Background(), "doc-1", "chunk-1", ports.WindowOptions{Before: -3, After: -1}); err != nil {
		t.Fatalf("OpenWindow() error = %v", err)
	}

	if reader.opts.Before != 0 || reader.opts.After != 0 {
		t.Fatalf("expected negative window options to clamp to zero, got %+v", reader.opts)
	}
}

func TestDefaultMetadataServiceDelegatesToStore(t *testing.T) {
	store := &stubMetadataStore{
		document: domain.Document{ID: "doc-1", Title: "Title"},
		sections: []domain.Section{{ID: "doc-1:0", Title: "Overview"}},
	}
	service := NewMetadataService(store)

	doc, err := service.GetDocument(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if doc.ID != "doc-1" {
		t.Fatalf("expected document to be returned, got %+v", doc)
	}

	sections, err := service.ListSections(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("ListSections() error = %v", err)
	}
	if len(sections) != 1 || sections[0].Title != "Overview" {
		t.Fatalf("expected sections to be returned, got %+v", sections)
	}
}
