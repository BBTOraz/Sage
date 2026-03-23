package agent

import (
	"bilge-lib/internal/ingestion/chunking"
	"bilge-lib/internal/ingestion/loader"
	parsepkg "bilge-lib/internal/ingestion/parser"
	"bilge-lib/internal/ingestion/pipeline"
	"bilge-lib/internal/storage/opensearch"
	"context"

	"github.com/cloudwego/eino-ext/components/document/parser/docx"
	"github.com/cloudwego/eino-ext/components/document/parser/pdf"
	docparser "github.com/cloudwego/eino/components/document/parser"
)

func (a *Application) Ingest(ctx context.Context, root string) (*pipeline.IngestReport, error) {
	ingester, err := a.newIngester(ctx)
	if err != nil {
		return nil, err
	}

	return ingester.Ingest(ctx, root)
}

func (a *Application) newIngester(ctx context.Context) (*pipeline.LocalIngester, error) {
	surveyor := loader.NewSurveyor(map[string]loader.FileType{
		".pdf":  loader.FileTypePDF,
		".docx": loader.FileTypeDOCX,
		".md":   loader.FileTypeMD,
		".txt":  loader.FileTypeTXT,
	})

	parser, err := newDocumentParser(ctx)
	if err != nil {
		return nil, err
	}

	client, err := opensearch.NewClient(a.config.OpenSearch)
	if err != nil {
		return nil, err
	}

	ingester := pipeline.NewLocalIngester(
		surveyor,
		parsepkg.NewLocal(parser),
		chunking.NewDefaultTransformer(),
		opensearch.NewIndexAdmin(client),
		opensearch.NewIndexer(client),
		opensearch.NewStatusStore(client),
	).WithWorkers(4)

	return ingester, nil
}

func newDocumentParser(ctx context.Context) (docparser.Parser, error) {
	pdfParser, err := pdf.NewPDFParser(ctx, &pdf.Config{})
	if err != nil {
		return nil, err
	}

	docxParser, err := docx.NewDocxParser(ctx, &docx.Config{
		IncludeTables: true,
	})
	if err != nil {
		return nil, err
	}

	return docparser.NewExtParser(ctx, &docparser.ExtParserConfig{
		Parsers: map[string]docparser.Parser{
			".pdf":  pdfParser,
			".docx": docxParser,
		},
	})
}
