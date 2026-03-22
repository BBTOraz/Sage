package agent

import (
	"bilge-lib/core"
	"bilge-lib/internal/approval"
	docservices "bilge-lib/internal/documents/services"
	doctools "bilge-lib/internal/documents/tools"
	"bilge-lib/internal/ingestion/pipeline"
	"bilge-lib/internal/storage/opensearch"
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
)

type ApplicationConfig struct {
	Env             core.EnvConfig
	ApprovalMode    approval.Mode
	OpenSearch      opensearch.Config
	CheckPointStore adk.CheckPointStore
}

type Application struct {
	config                   ApplicationConfig
	model                    model.BaseChatModel
	searchService            docservices.SearchService
	metadataService          docservices.MetadataService
	chunkService             docservices.ChunkService
	docTools                 []tool.BaseTool
	checkpoints              adk.CheckPointStore
	executorDeepCapabilities ExecutorDeepCapabilities
}

func NewApplication(ctx context.Context, cfg ApplicationConfig) (*Application, error) {
	cfg.OpenSearch = withDefaultOpenSearchConfig(cfg.OpenSearch)

	chatModel, err := core.NewChatModel(ctx, cfg.Env)
	if err != nil {
		return nil, err
	}

	client, err := opensearch.NewClient(cfg.OpenSearch)
	if err != nil {
		return nil, err
	}

	retriever := opensearch.NewRetriever(client)
	metadataStore := opensearch.NewMetadataStore(client)
	chunkReader := opensearch.NewChunkReader(client)

	searchService := docservices.NewSearchService(retriever)
	metadataService := docservices.NewMetadataService(metadataStore)
	chunkService := docservices.NewChunkService(chunkReader)

	invokableTools, err := doctools.DefaultTools(searchService, chunkService, metadataService)
	if err != nil {
		return nil, err
	}

	docTools := make([]tool.BaseTool, 0, len(invokableTools))
	for _, t := range invokableTools {
		docTools = append(docTools, t)
	}

	executorDeepCapabilities, err := defaultExecutorDeepCapabilities(cfg.Env)
	if err != nil {
		return nil, err
	}
	checkPointStore := cfg.CheckPointStore
	if checkPointStore == nil {
		checkPointStore = NewInMemoryCheckPointStore()
	}

	return &Application{
		config:                   cfg,
		model:                    chatModel,
		searchService:            searchService,
		metadataService:          metadataService,
		chunkService:             chunkService,
		docTools:                 docTools,
		checkpoints:              checkPointStore,
		executorDeepCapabilities: executorDeepCapabilities,
	}, nil
}

func withDefaultOpenSearchConfig(cfg opensearch.Config) opensearch.Config {
	defaults := opensearch.DefaultConfig

	if len(cfg.Addresses) == 0 {
		cfg.Addresses = append([]string(nil), defaults.Addresses...)
	}
	if cfg.ChunkIndexName == "" {
		cfg.ChunkIndexName = defaults.ChunkIndexName
	}
	if cfg.StatusIndexName == "" {
		cfg.StatusIndexName = defaults.StatusIndexName
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = defaults.RequestTimeout
	}

	return cfg
}

var _ pipeline.Ingester = (*Application)(nil)
