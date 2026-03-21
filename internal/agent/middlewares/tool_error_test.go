package middlewares

import (
	"bilge-lib/internal/apperr"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

func TestSoftToolErrorMiddlewareWrapInvokableToolCallConvertsRegularErrors(t *testing.T) {
	mw := &SoftToolErrorMiddleware{}
	endpoint, err := mw.WrapInvokableToolCall(context.Background(), func(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
		return "", &apperr.BaseError{
			ErrorType: apperr.InvalidToolArgument,
			Reason:    "bad document id",
			HINT:      "provide a non-empty document_id",
		}
	}, &adk.ToolContext{Name: "get_doc_metadata"})
	if err != nil {
		t.Fatalf("WrapInvokableToolCall() error = %v", err)
	}

	result, err := endpoint(context.Background(), `{"document_id":""}`)
	if err != nil {
		t.Fatalf("expected soft error result, got hard error %v", err)
	}

	if !strings.Contains(result, string(apperr.InvalidToolArgument)) || !strings.Contains(result, "provide a non-empty document_id") {
		t.Fatalf("expected mapped soft error result, got %q", result)
	}
}

func TestSoftToolErrorMiddlewarePreservesInterrupts(t *testing.T) {
	mw := &SoftToolErrorMiddleware{}
	endpoint, err := mw.WrapInvokableToolCall(context.Background(), func(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
		return "", compose.NewInterruptAndRerunErr("pause")
	}, &adk.ToolContext{Name: "search_docs"})
	if err != nil {
		t.Fatalf("WrapInvokableToolCall() error = %v", err)
	}

	_, gotErr := endpoint(context.Background(), `{}`)
	if gotErr == nil {
		t.Fatal("expected interrupt error to be preserved")
	}
	if !errors.Is(gotErr, compose.NewInterruptAndRerunErr("pause")) {
		if _, ok := compose.IsInterruptRerunError(gotErr); !ok {
			t.Fatalf("expected interrupt error, got %v", gotErr)
		}
	}
}
