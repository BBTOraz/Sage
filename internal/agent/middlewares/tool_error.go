package middlewares

import (
	"bilge-lib/internal/apperr"
	"bilge-lib/internal/middleware"
	"context"
	"encoding/json"
	"strings"

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

		if isToolArgumentParseError(err) {
			return (&middleware.ToolResponseError{
				Type:        apperr.InvalidToolArgument,
				Tool:        tCtx.Name,
				RequestPath: bestEffortRequestPath(argumentsInJSON),
				Reason:      err.Error(),
				Hint:        "return strict JSON arguments matching the tool schema exactly; do not use markdown fences, XML tags, or commentary",
			}).Error(), nil
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

func isToolArgumentParseError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "failed to unmarshal arguments in json") ||
		strings.Contains(msg, "invalid character") ||
		strings.Contains(msg, "syntax error at index")
}
