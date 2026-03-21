package tools

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/dto"
	"bilge-lib/internal/documents/ports"
	"bilge-lib/internal/documents/services"
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const openChunkWindowDescription = `Open the local context around a previously found chunk. 
Use this after search when the snippet alone is not enough and the agent needs surrounding paragraphs or neighboring chunks to interpret a clause, exception, definition, or example accurately.`

func NewOpenChunkWindowTool(service services.ChunkService) (tool.InvokableTool, error) {
	chunkFunk := func(ctx context.Context, input dto.OpenChunkWindowInput) (dto.OpenChunkWindowOutput, error) {

		chunk, err := service.OpenWindow(ctx, domain.DocumentID(input.DocumentID), domain.ChunkID(input.ChunkID), ports.WindowOptions{
			Before: input.Before,
			After:  input.After,
		})

		if err != nil {
			return dto.OpenChunkWindowOutput{}, err
		}

		window := make([]dto.Chunk, 0, len(chunk.Chunks))
		for _, item := range chunk.Chunks {
			window = append(window, dto.Chunk{
				ID:      string(item.ID),
				Text:    item.Text,
				Order:   item.Order,
				Page:    item.Page,
				Section: item.Section,
			})
		}

		out := dto.OpenChunkWindowOutput{
			DocumentID: string(chunk.DocumentID),
			AnchorID:   string(chunk.AnchorID),
			Window:     window,
		}
		return out, nil
	}

	return utils.InferTool("open_chunk_window", openChunkWindowDescription, chunkFunk)
}
