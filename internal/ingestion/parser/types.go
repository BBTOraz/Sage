package parser

import (
	"bilge-lib/internal/ingestion/loader"
	"context"

	"github.com/cloudwego/eino/schema"
)

type ParseStatus string

const (
	ParseStatusOK        ParseStatus = "ok"
	ParseStatusUntrusted ParseStatus = "untrusted"
)

type ParsedFile struct {
	File      loader.FileDescriptor
	Documents []*schema.Document
	Content   string
	Status    ParseStatus
	Warnings  []string
}

type Parser interface {
	Parse(ctx context.Context, file loader.FileDescriptor) (*ParsedFile, error)
}
