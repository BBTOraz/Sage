package tools

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/dto"
	"bilge-lib/internal/documents/services"
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const getDocMetadataDescription = `Fetch metadata for one indexed document. 
Use this when you need the document title, source path, and structure before deciding how to search it more deeply. 
Enable section listing when the agent needs a lightweight map of the document to choose better follow-up searches or decide which chunk window to open next.`

func NewGetDocMetadataTool(metaService services.MetadataService) (tool.InvokableTool, error) {
	metaFunc := func(ctx context.Context, input dto.GetDocMetadataInput) (dto.GetDocMetadataOutput, error) {
		document, err := metaService.GetDocument(ctx, domain.DocumentID(input.DocumentID))
		if err != nil {
			return dto.GetDocMetadataOutput{}, err
		}
		out := dto.GetDocMetadataOutput{
			DocumentID: string(document.ID),
			Title:      document.Title,
			Source:     document.Source,
			Path:       document.Path,
		}
		if input.IncludeSections {
			list, err := metaService.ListSections(ctx, domain.DocumentID(input.DocumentID))
			if err != nil {
				return dto.GetDocMetadataOutput{}, err
			}
			out.Sections = make([]dto.Section, 0, len(list))
			for _, section := range list {
				out.Sections = append(out.Sections, dto.Section{
					ID:        string(section.ID),
					Title:     section.Title,
					Level:     section.Level,
					PageStart: section.PageStart,
					PageEnd:   section.PageEnd,
				})
			}
		}

		return out, nil
	}

	return utils.InferTool("get_doc_metadata", getDocMetadataDescription, metaFunc)
}
