package middleware

import (
	"bilge-lib/internal/apperr"
	"context"
	"fmt"
)

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
