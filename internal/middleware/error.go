package middleware

import (
	"bilge-lib/internal/apperr"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

type ToolResponseError struct {
	Type        apperr.ErrorType `json:"type"`
	Tool        string           `json:"tool"`
	RequestPath string           `json:"request_path,omitempty"`
	Reason      string           `json:"reason"`
	Hint        string           `json:"hint,omitempty"`
}

func (e *ToolResponseError) Error() string {
	if e.RequestPath != "" {
		return fmt.Sprintf("type of error %s, tool name %s, request path %s, reason why denied %s, hint %s", e.Type, e.Tool, e.RequestPath, e.Reason, e.Hint)
	}
	return fmt.Sprintf("type of error %s, called tool name %s, reason why denied %s, hint: %s", e.Type, e.Tool, e.Reason, e.Hint)
}

func MapToolErrorResult(name, path string, err error) string {
	if err == nil {
		return ""
	}
	var Err apperr.AppError
	code := apperr.UnknownError
	hint := "retry or inspect the underlying error"
	if errors.As(err, &Err) {
		code = Err.Type()
		hint = Err.Hint()
	}
	resp := ToolResponseError{
		Type:        code,
		Tool:        name,
		RequestPath: path,
		Reason:      err.Error(),
		Hint:        hint,
	}
	return resp.Error()
}

func MapToolError(name, path string, err error) *compose.ToolOutput {
	if err == nil {
		return nil
	}
	return &compose.ToolOutput{
		Result: MapToolErrorResult(name, path, err),
	}
}
