package agent

import (
	"bilge-lib/core"
	"bilge-lib/internal/agent/middlewares"
	"bilge-lib/internal/approval"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	WorkspaceRoot string
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

func defaultExecutorDeepCapabilities(env core.EnvConfig) (ExecutorDeepCapabilities, error) {
	rootDir, err := resolveWorkspaceRoot(env.WorkspaceRoot)
	if err != nil {
		return ExecutorDeepCapabilities{}, err
	}

	return ExecutorDeepCapabilities{
		Filesystem: ExecutorDeepFilesystemConfig{
			Enabled:       true,
			WorkspaceRoot: rootDir,
			EnableExecute: false,
		},
	}, nil
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

	rootDir, err := resolveWorkspaceRoot(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}

	backend, err := localfs.NewBackend(ctx, &localfs.Config{})
	if err != nil {
		return nil, err
	}

	return &executorDeepFilesystemCapability{
		Backend: newWorkspaceBackend(rootDir, backend),
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

	rootDir, err := resolveWorkspaceRoot(capabilities.Filesystem.WorkspaceRoot)
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

func resolveWorkspaceRoot(rootDir string) (string, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve workspace root: %w", err)
		}
		rootDir = cwd
	}

	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	rootDir = filepath.Clean(rootDir)

	info, err := os.Stat(rootDir)
	if err != nil {
		return "", fmt.Errorf("workspace root %q: %w", rootDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root %q is not a directory", rootDir)
	}

	return rootDir, nil
}

type workspaceBackend struct {
	root  string
	inner adkfs.Backend
}

func newWorkspaceBackend(root string, inner adkfs.Backend) adkfs.Backend {
	return &workspaceBackend{
		root:  root,
		inner: inner,
	}
}

func (b *workspaceBackend) LsInfo(ctx context.Context, req *adkfs.LsInfoRequest) ([]adkfs.FileInfo, error) {
	path, err := b.resolvePath(req.Path)
	if err != nil {
		return nil, err
	}
	return b.inner.LsInfo(ctx, &adkfs.LsInfoRequest{Path: path})
}

func (b *workspaceBackend) Read(ctx context.Context, req *adkfs.ReadRequest) (*adkfs.FileContent, error) {
	path, err := b.resolvePath(req.FilePath)
	if err != nil {
		return nil, err
	}
	return b.inner.Read(ctx, &adkfs.ReadRequest{
		FilePath: path,
		Offset:   req.Offset,
		Limit:    req.Limit,
	})
}

func (b *workspaceBackend) GrepRaw(ctx context.Context, req *adkfs.GrepRequest) ([]adkfs.GrepMatch, error) {
	path, err := b.resolvePath(req.Path)
	if err != nil {
		return nil, err
	}
	return b.inner.GrepRaw(ctx, &adkfs.GrepRequest{
		Pattern:         req.Pattern,
		Path:            path,
		Glob:            req.Glob,
		FileType:        req.FileType,
		CaseInsensitive: req.CaseInsensitive,
		EnableMultiline: req.EnableMultiline,
		AfterLines:      req.AfterLines,
		BeforeLines:     req.BeforeLines,
	})
}

func (b *workspaceBackend) GlobInfo(ctx context.Context, req *adkfs.GlobInfoRequest) ([]adkfs.FileInfo, error) {
	path, err := b.resolvePath(req.Path)
	if err != nil {
		return nil, err
	}
	return b.inner.GlobInfo(ctx, &adkfs.GlobInfoRequest{
		Pattern: req.Pattern,
		Path:    path,
	})
}

func (b *workspaceBackend) Write(ctx context.Context, req *adkfs.WriteRequest) error {
	path, err := b.resolvePath(req.FilePath)
	if err != nil {
		return err
	}
	return b.inner.Write(ctx, &adkfs.WriteRequest{
		FilePath: path,
		Content:  req.Content,
	})
}

func (b *workspaceBackend) Edit(ctx context.Context, req *adkfs.EditRequest) error {
	path, err := b.resolvePath(req.FilePath)
	if err != nil {
		return err
	}
	return b.inner.Edit(ctx, &adkfs.EditRequest{
		FilePath:   path,
		OldString:  req.OldString,
		NewString:  req.NewString,
		ReplaceAll: req.ReplaceAll,
	})
}

func (b *workspaceBackend) resolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return b.root, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.root, path)
	}

	cleanPath := filepath.Clean(path)
	relPath, err := filepath.Rel(b.root, cleanPath)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes workspace root %q", path, b.root)
	}

	return cleanPath, nil
}
