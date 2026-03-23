package agent

import (
	"bilge-lib/internal/agent/middlewares"
	"bilge-lib/internal/approval"
	"context"
	"fmt"
	"os"
	"path/filepath"

	localfs "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	adkfs "github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/patchtoolcalls"
	"github.com/cloudwego/eino/adk/middlewares/reduction"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
)

type AgentHandlerConfig struct {
	ApprovalMode approval.Mode
	Model        model.BaseChatModel
	Capabilities ExecutorDeepCapabilities
}

type ExecutorDeepCapabilities struct {
	Filesystem ExecutorDeepFilesystemConfig
}

type ExecutorDeepFilesystemConfig struct {
	Enabled       bool
	EnableExecute bool
}

type executorDeepRuntimeCapabilities struct {
	Backend        adkfs.Backend
	Shell          adkfs.Shell
	StreamingShell adkfs.StreamingShell
}

func BuildExecutorDeepHandlers(ctx context.Context, cfg AgentHandlerConfig) ([]adk.ChatModelAgentMiddleware, error) {
	handlers := append([]adk.ChatModelAgentMiddleware{}, middlewares.DefaultMiddlewares(cfg.ApprovalMode)...)

	patchMiddleware, err := patchtoolcalls.New(ctx, nil)
	if err != nil {
		return nil, err
	}
	handlers = append(handlers, patchMiddleware)

	reductionMiddleware, err := buildReductionHandler(ctx, cfg.Capabilities)
	if err != nil {
		return nil, err
	}
	if reductionMiddleware != nil {
		handlers = append(handlers, reductionMiddleware)
	}

	summarizationMiddleware, err := buildSummarizationHandler(ctx, cfg)
	if err != nil {
		return nil, err
	}
	handlers = append(handlers, summarizationMiddleware)

	return handlers, nil
}

func BuildDocAgentHandlers(ctx context.Context, cfg AgentHandlerConfig) ([]adk.ChatModelAgentMiddleware, error) {
	handlers := append([]adk.ChatModelAgentMiddleware{}, middlewares.DefaultMiddlewares(cfg.ApprovalMode)...)

	patchMiddleware, err := patchtoolcalls.New(ctx, nil)
	if err != nil {
		return nil, err
	}
	handlers = append(handlers, patchMiddleware)

	reductionMiddleware, err := buildReductionHandler(ctx, cfg.Capabilities)
	if err != nil {
		return nil, err
	}
	if reductionMiddleware != nil {
		handlers = append(handlers, reductionMiddleware)
	}

	return handlers, nil
}

func defaultExecutorDeepCapabilities() ExecutorDeepCapabilities {
	return ExecutorDeepCapabilities{
		Filesystem: ExecutorDeepFilesystemConfig{
			Enabled:       true,
			EnableExecute: false,
		},
	}
}

func buildExecutorDeepRuntimeCapabilities(ctx context.Context, cfg ExecutorDeepCapabilities) (*executorDeepRuntimeCapabilities, error) {
	filesystemCapability, err := buildExecutorDeepFilesystemCapability(ctx, cfg.Filesystem)
	if err != nil {
		return nil, err
	}

	return &executorDeepRuntimeCapabilities{
		Backend:        filesystemCapability.Backend,
		Shell:          filesystemCapability.Shell,
		StreamingShell: filesystemCapability.StreamingShell,
	}, nil
}

type executorDeepFilesystemCapability struct {
	Backend        adkfs.Backend
	Shell          adkfs.Shell
	StreamingShell adkfs.StreamingShell
}

func buildExecutorDeepFilesystemCapability(ctx context.Context, cfg ExecutorDeepFilesystemConfig) (*executorDeepFilesystemCapability, error) {
	if !cfg.Enabled {
		return &executorDeepFilesystemCapability{}, nil
	}
	if cfg.EnableExecute {
		return nil, fmt.Errorf("executor deep execute capability is not enabled in this migration stage")
	}

	backend, err := localfs.NewBackend(ctx, &localfs.Config{})
	if err != nil {
		return nil, err
	}

	return &executorDeepFilesystemCapability{
		Backend: backend,
	}, nil
}

func buildReductionHandler(ctx context.Context, capabilities ExecutorDeepCapabilities) (adk.ChatModelAgentMiddleware, error) {
	runtimeCapabilities, err := buildExecutorDeepRuntimeCapabilities(ctx, capabilities)
	if err != nil {
		return nil, err
	}
	if runtimeCapabilities.Backend == nil {
		return nil, nil
	}

	rootDir, err := resolveReductionRoot()
	if err != nil {
		return nil, err
	}

	return reduction.New(ctx, &reduction.Config{
		Backend:                   runtimeCapabilities.Backend,
		ReadFileToolName:          "read_file",
		RootDir:                   filepath.Join(rootDir, ".sage", "middleware", "reduction"),
		MaxLengthForTrunc:         12000,
		MaxTokensForClear:         24000,
		ClearRetentionSuffixLimit: 4,
	})
}

func buildSummarizationHandler(ctx context.Context, cfg AgentHandlerConfig) (adk.ChatModelAgentMiddleware, error) {
	if cfg.Model == nil {
		return nil, fmt.Errorf("summarization middleware requires model")
	}

	return summarization.New(ctx, &summarization.Config{
		Model: cfg.Model,
		Trigger: &summarization.TriggerCondition{
			ContextTokens: 24000,
		},
	})
}

func resolveReductionRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve reduction root: %w", err)
	}
	return filepath.Join(cwd, ".sage", "middleware", "reduction"), nil
}
