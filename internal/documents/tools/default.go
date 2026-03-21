package tools

import (
	"bilge-lib/internal/documents/services"

	"github.com/cloudwego/eino/components/tool"
)

func DefaultTools(searchService services.SearchService, chunkService services.ChunkService, metaService services.MetadataService) (tools []tool.InvokableTool, err error) {
	tools = make([]tool.InvokableTool, 0)
	getDocMetadata, err := NewGetDocMetadataTool(metaService)
	if err != nil {
		return nil, err
	}
	tools = append(tools, getDocMetadata)

	nextPage, err := NewNextPageTool(searchService)
	if err != nil {
		return nil, err
	}
	tools = append(tools, nextPage)

	openChunkWindow, err := NewOpenChunkWindowTool(chunkService)
	if err != nil {
		return nil, err
	}
	tools = append(tools, openChunkWindow)

	searchDocs, err := NewSearchDocsTool(searchService)
	if err != nil {
		return nil, err
	}
	tools = append(tools, searchDocs)

	return tools, nil
}
