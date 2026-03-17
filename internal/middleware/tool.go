package middleware

import (
	"bilge-lib/internal/apperr"
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

func LoggerMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(endpoint compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				fmt.Printf("tool name: %s, args: %s, opts: %v", input.Name, input.Arguments, input.CallOptions)
				return endpoint(ctx, input)
			}
		},
	}
}

func UnknownToolHandler() func(ctx context.Context, name string, input string) (string, error) {
	return func(ctx context.Context, name string, input string) (string, error) {
		res := ToolResponseError{
			Type:   apperr.InvalidToolCall,
			Tool:   name,
			Reason: fmt.Sprintf("wrong tool name: %s", name),
			Hint:   "try again, but know u need to check the actual tool name",
		}
		return res.Error(), nil
	}
}
