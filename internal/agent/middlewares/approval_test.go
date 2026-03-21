package middlewares

import (
	"bilge-lib/internal/approval"
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
)

func TestApprovalMiddlewareBypassesDeepAgentControlTools(t *testing.T) {
	mw := &ApprovalMiddleware{mode: approval.Guard}

	called := false
	endpoint, err := mw.WrapInvokableToolCall(context.Background(), func(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
		called = true
		return argumentsInJSON, nil
	}, &adk.ToolContext{Name: "task"})
	if err != nil {
		t.Fatalf("WrapInvokableToolCall() error = %v", err)
	}

	result, err := endpoint(context.Background(), `{"subagent_type":"sage-doc","description":"find evidence"}`)
	if err != nil {
		t.Fatalf("endpoint() error = %v", err)
	}
	if !called {
		t.Fatal("expected task tool to bypass approval and call next endpoint")
	}
	if result == "" {
		t.Fatal("expected passthrough result from next endpoint")
	}
}

func TestShouldBypassApproval(t *testing.T) {
	tests := []struct {
		name string
		ctx  *adk.ToolContext
		want bool
	}{
		{name: "nil", ctx: nil, want: false},
		{name: "task", ctx: &adk.ToolContext{Name: "task"}, want: true},
		{name: "write_todos", ctx: &adk.ToolContext{Name: "write_todos"}, want: true},
		{name: "document tool", ctx: &adk.ToolContext{Name: "search_docs"}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldBypassApproval(tc.ctx); got != tc.want {
				t.Fatalf("shouldBypassApproval() = %v, want %v", got, tc.want)
			}
		})
	}
}
