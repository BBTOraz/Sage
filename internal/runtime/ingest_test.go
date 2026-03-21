package runtime

import (
	"bilge-lib/internal/ingestion/pipeline"
	"context"
	"os"
	"path/filepath"
	"testing"
)

type stubIngester struct {
	report *pipeline.IngestReport
	err    error
	root   string
}

func (s *stubIngester) Ingest(ctx context.Context, root string) (*pipeline.IngestReport, error) {
	s.root = root
	return s.report, s.err
}

func TestManagerStartIngest(t *testing.T) {
	dir := t.TempDir()
	stub := &stubIngester{report: &pipeline.IngestReport{RunID: "run-1", Root: dir}}
	manager := NewManager("", nil, stub)

	handle, err := manager.StartIngest(context.Background(), dir)
	if err != nil {
		t.Fatalf("StartIngest() error = %v", err)
	}
	if handle.Status != IngestStatusQueued {
		t.Fatalf("StartIngest() status = %q, want %q", handle.Status, IngestStatusQueued)
	}

	first := <-handle.Events
	if first.Type != EventIngestQueued {
		t.Fatalf("first event type = %q, want %q", first.Type, EventIngestQueued)
	}

	second := <-handle.Events
	if second.Type != EventIngestStarted {
		t.Fatalf("second event type = %q, want %q", second.Type, EventIngestStarted)
	}

	third := <-handle.Events
	if third.Type != EventIngestCompleted {
		t.Fatalf("third event type = %q, want %q", third.Type, EventIngestCompleted)
	}
	if stub.root != dir {
		t.Fatalf("ingest root = %q, want %q", stub.root, dir)
	}
}

func TestValidateIngestRoot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := validateIngestRoot(""); err == nil {
		t.Fatal("validateIngestRoot(\"\") expected error")
	}
	if _, err := validateIngestRoot(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("validateIngestRoot(missing) expected error")
	}
	if _, err := validateIngestRoot(file); err == nil {
		t.Fatal("validateIngestRoot(file) expected error")
	}
	got, err := validateIngestRoot(dir)
	if err != nil {
		t.Fatalf("validateIngestRoot(dir) error = %v", err)
	}
	if got != dir {
		t.Fatalf("validateIngestRoot(dir) = %q, want %q", got, dir)
	}
}
