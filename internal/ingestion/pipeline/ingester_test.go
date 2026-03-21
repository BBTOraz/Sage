package pipeline

import (
	"bilge-lib/internal/ingestion/chunking"
	"bilge-lib/internal/ingestion/loader"
	parsepkg "bilge-lib/internal/ingestion/parser"
	storage "bilge-lib/internal/storage/opensearch"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type stubSurveyor struct {
	descriptors []*loader.FileDescriptor
	err         error
}

func (s stubSurveyor) Survey(ctx context.Context, dir string) ([]*loader.FileDescriptor, error) {
	return s.descriptors, s.err
}

type stubParser struct {
	files map[string]*parsepkg.ParsedFile
	errs  map[string]error
}

func (s stubParser) Parse(ctx context.Context, file loader.FileDescriptor) (*parsepkg.ParsedFile, error) {
	if err, ok := s.errs[file.Path]; ok {
		return nil, err
	}
	return s.files[file.Path], nil
}

type blockingParser struct {
	files        map[string]*parsepkg.ParsedFile
	release      <-chan struct{}
	active       int32
	maxActive    int32
	startedCount int32
}

func (p *blockingParser) Parse(ctx context.Context, file loader.FileDescriptor) (*parsepkg.ParsedFile, error) {
	current := atomic.AddInt32(&p.active, 1)
	atomic.AddInt32(&p.startedCount, 1)
	for {
		maxSeen := atomic.LoadInt32(&p.maxActive)
		if current <= maxSeen || atomic.CompareAndSwapInt32(&p.maxActive, maxSeen, current) {
			break
		}
	}

	defer atomic.AddInt32(&p.active, -1)

	select {
	case <-p.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return p.files[file.Path], nil
}

type stubTransformer struct {
	files map[string]*chunking.ChunkedFile
	errs  map[string]error
}

func (s stubTransformer) Transform(ctx context.Context, parsed *parsepkg.ParsedFile) (*chunking.ChunkedFile, error) {
	if err, ok := s.errs[parsed.File.Path]; ok {
		return nil, err
	}
	return s.files[parsed.File.Path], nil
}

type stubIndexes struct {
	calls int
	err   error
}

func (s *stubIndexes) EnsureIndexes(ctx context.Context) error {
	s.calls++
	return s.err
}

type stubIndexer struct {
	records [][]storage.ChunkIndexRecord
	err     error
}

func (s *stubIndexer) IndexChunks(ctx context.Context, records []storage.ChunkIndexRecord) error {
	copied := make([]storage.ChunkIndexRecord, len(records))
	copy(copied, records)
	s.records = append(s.records, copied)
	return s.err
}

type stubStatusStore struct {
	mu      sync.Mutex
	records []storage.IngestStatusRecord
	err     error
}

func (s *stubStatusStore) WriteStatus(ctx context.Context, record storage.IngestStatusRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return s.err
}

func TestLocalIngesterContinuesPastBadFilesAndWritesStatuses(t *testing.T) {
	good := &loader.FileDescriptor{Path: "C:\\docs\\good.pdf", FileType: loader.FileTypePDF}
	bad := &loader.FileDescriptor{Path: "C:\\docs\\bad.pdf", FileType: loader.FileTypePDF}
	untrusted := &loader.FileDescriptor{Path: "C:\\docs\\suspect.docx", FileType: loader.FileTypeDOCX}

	indexes := &stubIndexes{}
	indexer := &stubIndexer{}
	statuses := &stubStatusStore{}
	ingester := NewLocalIngester(
		stubSurveyor{descriptors: []*loader.FileDescriptor{good, bad, untrusted}},
		stubParser{
			files: map[string]*parsepkg.ParsedFile{
				good.Path: {
					File:     *good,
					Content:  "good",
					Status:   parsepkg.ParseStatusOK,
					Warnings: nil,
				},
				untrusted.Path: {
					File:     *untrusted,
					Content:  "\x00garbage",
					Status:   parsepkg.ParseStatusUntrusted,
					Warnings: []string{"high NUL byte ratio"},
				},
			},
			errs: map[string]error{
				bad.Path: errors.New("parse failed"),
			},
		},
		stubTransformer{
			files: map[string]*chunking.ChunkedFile{
				good.Path: {
					SourceDocument: chunking.SourceDocument{
						ID:       chunking.DocumentID("doc-1"),
						Path:     good.Path,
						FileType: loader.FileTypePDF,
						Title:    "Good",
						DocHash:  "hash-1",
					},
					Chunks: []*chunking.Chunk{
						{ID: chunking.ChunkID("doc-1:0"), DocumentID: chunking.DocumentID("doc-1"), ChunkIndex: 0, Content: "a"},
						{ID: chunking.ChunkID("doc-1:1"), DocumentID: chunking.DocumentID("doc-1"), ChunkIndex: 1, Content: "b"},
					},
				},
			},
		},
		indexes,
		indexer,
		statuses,
	)
	ingester.now = func() time.Time {
		return time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	}

	report, err := ingester.Ingest(context.Background(), "C:\\docs")
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	if indexes.calls != 1 {
		t.Fatalf("expected EnsureIndexes to be called once, got %d", indexes.calls)
	}
	if report.IndexedDocuments != 1 {
		t.Fatalf("expected 1 indexed document, got %d", report.IndexedDocuments)
	}
	if report.IndexedChunks != 2 {
		t.Fatalf("expected 2 indexed chunks, got %d", report.IndexedChunks)
	}
	if len(report.Files) != 3 {
		t.Fatalf("expected 3 file results, got %d", len(report.Files))
	}
	if len(indexer.records) != 1 || len(indexer.records[0]) != 2 {
		t.Fatalf("expected one chunk indexing call with 2 records, got %+v", indexer.records)
	}
	if len(statuses.records) != 3 {
		t.Fatalf("expected 3 status records, got %d", len(statuses.records))
	}

	if report.Files[0].Status != FileStatusIndexed {
		t.Fatalf("expected good file to be indexed, got %+v", report.Files[0])
	}
	if report.Files[1].Status != FileStatusFailed || report.Files[1].Stage != FileStageParse {
		t.Fatalf("expected bad file to fail at parse, got %+v", report.Files[1])
	}
	if report.Files[2].Status != FileStatusSkipped || report.Files[2].Stage != FileStageParse {
		t.Fatalf("expected untrusted file to be skipped at parse, got %+v", report.Files[2])
	}
}

func TestLocalIngesterAbortsOnIndexSetupFailure(t *testing.T) {
	ingester := NewLocalIngester(
		stubSurveyor{descriptors: []*loader.FileDescriptor{{Path: "C:\\docs\\good.pdf", FileType: loader.FileTypePDF}}},
		stubParser{},
		stubTransformer{},
		&stubIndexes{err: errors.New("cluster down")},
		&stubIndexer{},
		&stubStatusStore{},
	)

	report, err := ingester.Ingest(context.Background(), "C:\\docs")
	if err == nil {
		t.Fatal("expected EnsureIndexes failure to abort ingest")
	}
	if report == nil {
		t.Fatal("expected partial report even on setup failure")
	}
}

func TestLocalIngesterBatchesIndexedFilesIntoSingleBulkCall(t *testing.T) {
	first := &loader.FileDescriptor{Path: "C:\\docs\\first.docx", FileType: loader.FileTypeDOCX}
	second := &loader.FileDescriptor{Path: "C:\\docs\\second.docx", FileType: loader.FileTypeDOCX}

	indexes := &stubIndexes{}
	indexer := &stubIndexer{}
	statuses := &stubStatusStore{}

	ingester := NewLocalIngester(
		stubSurveyor{descriptors: []*loader.FileDescriptor{first, second}},
		stubParser{
			files: map[string]*parsepkg.ParsedFile{
				first.Path: {
					File:    *first,
					Content: "First content",
					Status:  parsepkg.ParseStatusOK,
				},
				second.Path: {
					File:    *second,
					Content: "Second content",
					Status:  parsepkg.ParseStatusOK,
				},
			},
		},
		stubTransformer{
			files: map[string]*chunking.ChunkedFile{
				first.Path: {
					SourceDocument: chunking.SourceDocument{
						ID:       chunking.DocumentID("doc-1"),
						Path:     first.Path,
						FileType: loader.FileTypeDOCX,
						Title:    "First",
						DocHash:  "hash-1",
					},
					Chunks: []*chunking.Chunk{
						{ID: chunking.ChunkID("doc-1:0"), DocumentID: chunking.DocumentID("doc-1"), ChunkIndex: 0, Content: "a"},
					},
				},
				second.Path: {
					SourceDocument: chunking.SourceDocument{
						ID:       chunking.DocumentID("doc-2"),
						Path:     second.Path,
						FileType: loader.FileTypeDOCX,
						Title:    "Second",
						DocHash:  "hash-2",
					},
					Chunks: []*chunking.Chunk{
						{ID: chunking.ChunkID("doc-2:0"), DocumentID: chunking.DocumentID("doc-2"), ChunkIndex: 0, Content: "b"},
						{ID: chunking.ChunkID("doc-2:1"), DocumentID: chunking.DocumentID("doc-2"), ChunkIndex: 1, Content: "c"},
					},
				},
			},
		},
		indexes,
		indexer,
		statuses,
	)

	report, err := ingester.Ingest(context.Background(), "C:\\docs")
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	if report.IndexedDocuments != 2 {
		t.Fatalf("expected 2 indexed documents, got %d", report.IndexedDocuments)
	}
	if report.IndexedChunks != 3 {
		t.Fatalf("expected 3 indexed chunks, got %d", report.IndexedChunks)
	}
	if len(indexer.records) != 1 {
		t.Fatalf("expected a single bulk indexing call, got %d", len(indexer.records))
	}
	if len(indexer.records[0]) != 3 {
		t.Fatalf("expected 3 records in the single bulk call, got %d", len(indexer.records[0]))
	}
	if len(statuses.records) != 2 {
		t.Fatalf("expected 2 status writes, got %d", len(statuses.records))
	}
	for _, file := range report.Files {
		if file.Status != FileStatusIndexed || file.Stage != FileStageIndex {
			t.Fatalf("expected indexed file result, got %+v", file)
		}
	}
}

func TestLocalIngesterUsesConfiguredWorkerPoolForParseStage(t *testing.T) {
	release := make(chan struct{})
	files := make([]*loader.FileDescriptor, 0, 4)
	parserFiles := make(map[string]*parsepkg.ParsedFile, 4)
	transformFiles := make(map[string]*chunking.ChunkedFile, 4)

	for i := 0; i < 4; i++ {
		path := "C:\\docs\\file-" + string(rune('a'+i)) + ".docx"
		desc := &loader.FileDescriptor{Path: path, FileType: loader.FileTypeDOCX}
		files = append(files, desc)
		parserFiles[path] = &parsepkg.ParsedFile{
			File:    *desc,
			Content: "content",
			Status:  parsepkg.ParseStatusOK,
		}
		transformFiles[path] = &chunking.ChunkedFile{
			SourceDocument: chunking.SourceDocument{
				ID:       chunking.DocumentID("doc-" + string(rune('1'+i))),
				Path:     path,
				FileType: loader.FileTypeDOCX,
				Title:    "Doc",
				DocHash:  "hash",
			},
			Chunks: []*chunking.Chunk{
				{ID: chunking.ChunkID("doc:0"), DocumentID: chunking.DocumentID("doc"), ChunkIndex: 0, Content: "chunk"},
			},
		}
	}

	parser := &blockingParser{
		files:   parserFiles,
		release: release,
	}
	ingester := NewLocalIngester(
		stubSurveyor{descriptors: files},
		parser,
		stubTransformer{files: transformFiles},
		&stubIndexes{},
		&stubIndexer{},
		&stubStatusStore{},
	).WithWorkers(4)

	done := make(chan error, 1)
	go func() {
		_, err := ingester.Ingest(context.Background(), "C:\\docs")
		done <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&parser.maxActive) >= 4 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if atomic.LoadInt32(&parser.maxActive) < 4 {
		close(release)
		t.Fatalf("expected parser stage to reach 4 concurrent workers, got %d", atomic.LoadInt32(&parser.maxActive))
	}

	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Ingest() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ingest did not finish after releasing parser workers")
	}
}
