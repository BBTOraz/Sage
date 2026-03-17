package agent

import (
	"context"

	"github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/filesystem"
)

func newLocalBackend(ctx context.Context) (*local.Local, error) {
	backend, err := local.NewBackend(ctx, &local.Config{})
	if err != nil {
		return nil, err
	}
	return backend, nil
}

func FileSystemMiddleware(ctx context.Context) (adk.ChatModelAgentMiddleware, error) {
	backend, err := newLocalBackend(ctx)
	if err != nil {
		return nil, err
	}

	return filesystem.New(ctx, &filesystem.MiddlewareConfig{
		Backend: backend,
	})
}
