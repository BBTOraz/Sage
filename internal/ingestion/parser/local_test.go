package parser

import (
	"bilge-lib/internal/ingestion/loader"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	docparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

type stubParser struct {
	docs []*schema.Document
	err  error
}

func (s stubParser) Parse(ctx context.Context, reader io.Reader, opts ...docparser.Option) ([]*schema.Document, error) {
	return s.docs, s.err
}

func TestLocalParseAggregatesReadableTextWithoutWarning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.pdf")
	if err := os.WriteFile(path, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := NewLocal(stubParser{
		docs: []*schema.Document{
			{Content: "Introduction to Sage."},
			{Content: "This document contains readable text."},
		},
	})

	out, err := p.Parse(context.Background(), loader.FileDescriptor{
		Path:     path,
		FileType: loader.FileTypePDF,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if out.Content == "" {
		t.Fatal("expected aggregated content to be populated")
	}

	if out.Status != ParseStatusOK {
		t.Fatalf("expected status %q, got %q", ParseStatusOK, out.Status)
	}

	if len(out.Warnings) != 0 {
		t.Fatalf("expected no warnings for readable text, got %v", out.Warnings)
	}
}

func TestLocalParseMarksSuspiciousExtractedText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.pdf")
	if err := os.WriteFile(path, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := NewLocal(stubParser{
		docs: []*schema.Document{
			{Content: "\x01�\x01�\x00Q\x00W\x00U\x00R"},
		},
	})

	out, err := p.Parse(context.Background(), loader.FileDescriptor{
		Path:     path,
		FileType: loader.FileTypePDF,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if out.Status != ParseStatusUntrusted {
		t.Fatalf("expected status %q, got %q", ParseStatusUntrusted, out.Status)
	}

	if len(out.Warnings) == 0 {
		t.Fatal("expected suspicious extracted text to produce warnings")
	}
}

func TestLocalParseMarksEmptyExtractedTextAsUntrusted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.pdf")
	if err := os.WriteFile(path, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := NewLocal(stubParser{
		docs: []*schema.Document{
			{Content: ""},
			{Content: "   "},
		},
	})

	out, err := p.Parse(context.Background(), loader.FileDescriptor{
		Path:     path,
		FileType: loader.FileTypePDF,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if out.Status != ParseStatusUntrusted {
		t.Fatalf("expected status %q, got %q", ParseStatusUntrusted, out.Status)
	}

	if len(out.Warnings) == 0 {
		t.Fatal("expected warnings for empty extracted text")
	}
}
