package middlewares

import (
	"bilge-lib/internal/middleware"
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

type SoftToolErrorMiddleware struct {
	adk.BaseChatModelAgentMiddleware
}

func (m *SoftToolErrorMiddleware) WrapInvokableToolCall(_ context.Context, next adk.InvokableToolCallEndpoint, tCtx *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	return func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		result, err := next(ctx, argumentsInJSON, opts...)
		if err == nil {
			return result, nil
		}

		if _, interrupted := compose.IsInterruptRerunError(err); interrupted {
			return "", err
		}

		return middleware.MapToolErrorResult(tCtx.Name, bestEffortRequestPath(argumentsInJSON), err), nil
	}, nil
}

func bestEffortRequestPath(argumentsInJSON string) string {
	if argumentsInJSON == "" {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &payload); err != nil {
		return ""
	}

	for _, key := range []string{"path", "file_path"} {
		if value, ok := payload[key].(string); ok {
			return value
		}
	}

	return ""
}
