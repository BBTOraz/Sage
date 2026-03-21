package parser

import (
	"bilge-lib/internal/ingestion/loader"
	"context"
	"os"
	"strings"
	"unicode"

	docparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

type Local struct {
	Parser docparser.Parser
}

func NewLocal(parser docparser.Parser) *Local {
	return &Local{Parser: parser}
}

func (p *Local) Parse(ctx context.Context, descriptor loader.FileDescriptor) (*ParsedFile, error) {
	file, err := os.Open(descriptor.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	docs, err := p.Parser.Parse(ctx, file, docparser.WithURI(descriptor.Path))
	if err != nil {
		return nil, err
	}

	content := aggregateContent(docs)

	return &ParsedFile{
		File:      descriptor,
		Documents: docs,
		Content:   content,
		Status:    assessExtractedText(content),
		Warnings:  assessExtractedTextWarnings(content),
	}, nil
}

func aggregateContent(docs []*schema.Document) string {
	if len(docs) == 0 {
		return ""
	}

	parts := make([]string, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}

		content := strings.TrimSpace(doc.Content)
		if content == "" {
			continue
		}

		parts = append(parts, content)
	}

	return strings.Join(parts, "\n\n")
}

func assessExtractedText(content string) ParseStatus {
	if len(assessExtractedTextWarnings(content)) != 0 {
		return ParseStatusUntrusted
	}

	return ParseStatusOK
}

func assessExtractedTextWarnings(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return []string{"empty extracted text"}
	}

	var (
		total       int
		nulls       int
		controls    int
		replacement int
		printable   int
	)

	for _, r := range content {
		total++

		switch {
		case r == 0:
			nulls++
		case r == unicode.ReplacementChar:
			replacement++
		case r == '\n' || r == '\r' || r == '\t':
			printable++
		case unicode.IsPrint(r):
			printable++
		case unicode.IsControl(r):
			controls++
		}
	}

	if total == 0 {
		return []string{"empty extracted text"}
	}

	nullRatio := float64(nulls) / float64(total)
	controlRatio := float64(controls) / float64(total)
	replacementRatio := float64(replacement) / float64(total)
	printableRatio := float64(printable) / float64(total)

	var warnings []string
	if nullRatio > 0.01 {
		warnings = append(warnings, "high NUL byte ratio")
	}
	if controlRatio > 0.02 {
		warnings = append(warnings, "high control character ratio")
	}
	if replacementRatio > 0.005 {
		warnings = append(warnings, "many replacement characters")
	}
	if printableRatio < 0.85 {
		warnings = append(warnings, "low printable text ratio")
	}

	return warnings
}
