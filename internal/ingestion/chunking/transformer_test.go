package chunking

import (
	"bilge-lib/internal/ingestion/loader"
	"bilge-lib/internal/ingestion/parser"
	"context"
	"strings"
	"testing"
)

func TestDefaultTransformerSingleChunkForShortText(t *testing.T) {
	tr := NewDefaultTransformer()

	out, err := tr.Transform(context.Background(), &parser.ParsedFile{
		File: loader.FileDescriptor{
			Path:     "C:\\docs\\policy.pdf",
			FileType: loader.FileTypePDF,
		},
		Content: "INTRODUCTION\n\nThis is a short readable document.",
		Status:  parser.ParseStatusOK,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if len(out.Chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out.Chunks))
	}

	if out.SourceDocument.Title != "INTRODUCTION" {
		t.Fatalf("expected title to come from heading, got %q", out.SourceDocument.Title)
	}

	if out.Chunks[0].PrevChunkID != "" || out.Chunks[0].NextChunkID != "" {
		t.Fatalf("expected single chunk to have no neighbors, got prev=%q next=%q", out.Chunks[0].PrevChunkID, out.Chunks[0].NextChunkID)
	}
}

func TestDefaultTransformerCreatesMultipleChunksAndNeighborLinks(t *testing.T) {
	tr := NewDefaultTransformer()

	var paragraphs []string
	for i := 0; i < 6; i++ {
		paragraphs = append(paragraphs, strings.Repeat("This paragraph carries enough lexical content for Sage chunking. ", 8))
	}

	content := "SECTION ONE\n\n" + strings.Join(paragraphs[:3], "\n\n") + "\n\nSECTION TWO\n\n" + strings.Join(paragraphs[3:], "\n\n")

	out, err := tr.Transform(context.Background(), &parser.ParsedFile{
		File: loader.FileDescriptor{
			Path:     "C:\\docs\\long.pdf",
			FileType: loader.FileTypePDF,
		},
		Content: content,
		Status:  parser.ParseStatusOK,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if len(out.Chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(out.Chunks))
	}

	for i, chunk := range out.Chunks {
		if i > 0 && chunk.PrevChunkID == "" {
			t.Fatalf("expected chunk %d to have PrevChunkID", i)
		}
		if i < len(out.Chunks)-1 && chunk.NextChunkID == "" {
			t.Fatalf("expected chunk %d to have NextChunkID", i)
		}
	}
}

func TestDefaultTransformerRejectsUntrustedParsedFile(t *testing.T) {
	tr := NewDefaultTransformer()

	_, err := tr.Transform(context.Background(), &parser.ParsedFile{
		File: loader.FileDescriptor{
			Path:     "C:\\docs\\bad.pdf",
			FileType: loader.FileTypePDF,
		},
		Content:  "\x00\x01garbage",
		Status:   parser.ParseStatusUntrusted,
		Warnings: []string{"high NUL byte ratio"},
	})
	if err == nil {
		t.Fatal("expected untrusted parsed file to be rejected")
	}
}

func TestDefaultTransformerProducesDeterministicIDs(t *testing.T) {
	tr := NewDefaultTransformer()

	in := &parser.ParsedFile{
		File: loader.FileDescriptor{
			Path:     "C:\\docs\\stable.pdf",
			FileType: loader.FileTypePDF,
		},
		Content: "TITLE\n\n" + strings.Repeat("Stable content for deterministic IDs. ", 40),
		Status:  parser.ParseStatusOK,
	}

	left, err := tr.Transform(context.Background(), in)
	if err != nil {
		t.Fatalf("first Transform() error = %v", err)
	}

	right, err := tr.Transform(context.Background(), in)
	if err != nil {
		t.Fatalf("second Transform() error = %v", err)
	}

	if left.SourceDocument.ID != right.SourceDocument.ID {
		t.Fatalf("expected stable document id, got %q and %q", left.SourceDocument.ID, right.SourceDocument.ID)
	}

	if len(left.Chunks) != len(right.Chunks) {
		t.Fatalf("expected same chunk count, got %d and %d", len(left.Chunks), len(right.Chunks))
	}

	for i := range left.Chunks {
		if left.Chunks[i].ID != right.Chunks[i].ID {
			t.Fatalf("expected stable chunk id at %d, got %q and %q", i, left.Chunks[i].ID, right.Chunks[i].ID)
		}
	}
}

func TestDefaultTransformerIgnoresGenericWrapperHeadingsForTitle(t *testing.T) {
	tr := NewDefaultTransformer()

	out, err := tr.Transform(context.Background(), &parser.ParsedFile{
		File: loader.FileDescriptor{
			Path:     "C:\\docs\\guide.docx",
			FileType: loader.FileTypeDOCX,
		},
		Content: "=== MAIN CONTENT ===\n\n# Real Title\n\nThis paragraph should be indexed with the real title.",
		Status:  parser.ParseStatusOK,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if out.SourceDocument.Title != "Real Title" {
		t.Fatalf("expected real title, got %q", out.SourceDocument.Title)
	}
	if len(out.Chunks) != 1 {
		t.Fatalf("expected one chunk, got %d", len(out.Chunks))
	}
	if !strings.Contains(out.Chunks[0].SectionPath, "Real Title") {
		t.Fatalf("expected section path to include normalized heading, got %q", out.Chunks[0].SectionPath)
	}
}

func TestDefaultTransformerSplitsLongSingleFlowTextWithoutBlankLines(t *testing.T) {
	tr := NewDefaultTransformer()

	lines := make([]string, 0, 18)
	lines = append(lines, "# Long Guide")
	for i := 0; i < 16; i++ {
		lines = append(lines, "This line ends as a full paragraph for lexical retrieval and should not be glued into one giant chunk.")
	}

	out, err := tr.Transform(context.Background(), &parser.ParsedFile{
		File: loader.FileDescriptor{
			Path:     "C:\\docs\\flow.docx",
			FileType: loader.FileTypeDOCX,
		},
		Content: strings.Join(lines, "\n"),
		Status:  parser.ParseStatusOK,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if len(out.Chunks) < 2 {
		t.Fatalf("expected multiple chunks from long single-flow text, got %d", len(out.Chunks))
	}

	for _, chunk := range out.Chunks {
		if len(chunk.Content) > maxChunkChars+200 {
			t.Fatalf("expected chunk content to stay bounded, got %d chars", len(chunk.Content))
		}
	}
}

func TestIsHeadingLikeRejectsColonSentences(t *testing.T) {
	cases := []string{
		"A typical CSP will have directives such as:",
		"In practice, Spring Security will by default:",
		"This example shows several things:",
		"Key takeaways on password security in Spring Security:",
	}

	for _, text := range cases {
		if isHeadingLike(text) {
			t.Fatalf("expected %q not to be treated as heading", text)
		}
	}
}

func TestIsHeadingLikeKeepsRealTitleCaseHeadings(t *testing.T) {
	cases := []string{
		"Cross-Site Request Forgery (CSRF)",
		"PasswordEncoder and Password Storage",
		"Additional Cookie Protections:",
	}

	for _, text := range cases {
		if !isHeadingLike(text) {
			t.Fatalf("expected %q to be treated as heading", text)
		}
	}
}
