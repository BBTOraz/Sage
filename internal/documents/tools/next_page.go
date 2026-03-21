package tools

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/dto"
	"bilge-lib/internal/documents/services"
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const nextPageToolDescription = `Fetch the next page of an existing document search session. 
Use this only after search_docs or a previous next_page call when the agent wants more results from the same search strategy. 
Do not use it for a new question, new filters, or a different wording; start a fresh search_docs call instead.`

func NewNextPageTool(service services.SearchService) (tool.InvokableTool, error) {
	pageFunc := func(ctx context.Context, input dto.NextPageInput) (dto.NextPageOutput, error) {
		page, err := service.NextPage(ctx, domain.QueryID(input.QueryID), input.Cursor)
		if err != nil {
			return dto.NextPageOutput{}, err
		}
		hits := make([]dto.SearchHit, 0, len(page.Items))
		for _, item := range page.Items {
			hits = append(hits, dto.SearchHit{
				DocumentID: string(item.DocumentID),
				ChunkID:    string(item.ChunkID),
				Score:      item.Score,
				Snippet:    item.Snippet,
				Page:       item.Page,
				Section:    item.Section,
			})
		}
		return dto.NextPageOutput{
			QueryID:    string(page.QueryID),
			Results:    hits,
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
		}, nil
	}

	return utils.InferTool("next_page", nextPageToolDescription, pageFunc)
}
