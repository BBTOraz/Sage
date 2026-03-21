package tools

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/dto"
	"bilge-lib/internal/documents/services"
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const searchDocsDescription = `
Search indexed document fragments and return the most relevant chunks for the user's request. 
Use this as the first step when exploring a document collection. You may provide alternate phrasings, translations, synonyms, or acronym expansions in the same call to improve lexical recall while keeping one search session. 
If the returned snippet is not enough, inspect metadata or open the surrounding chunk window next.`

func NewSearchDocsTool(searchService services.SearchService) (tool.InvokableTool, error) {
	searchFunc := func(ctx context.Context, input dto.SearchDocsInput) (dto.SearchDocsOutput, error) {
		documentIDs := make([]domain.DocumentID, 0, len(input.DocumentIDs))
		for _, id := range input.DocumentIDs {
			if id == "" {
				continue
			}
			documentIDs = append(documentIDs, domain.DocumentID(id))
		}

		q := domain.SearchQuery{
			UserQuestion:       input.Question,
			AlternateQuestions: input.AlternateQuestions,
			PageSize:           input.PageSize,
			Filters: domain.SearchFilters{
				DocumentIDs: documentIDs,
				Source:      input.Source,
			},
		}
		page, err := searchService.Search(ctx, q)
		if err != nil {
			return dto.SearchDocsOutput{}, err
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

		return dto.SearchDocsOutput{
			QueryID:    string(page.QueryID),
			Results:    hits,
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
		}, nil

	}

	return utils.InferTool(
		"search_docs",
		searchDocsDescription,
		searchFunc,
	)
}
