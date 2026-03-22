package agent

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"bilge-lib/internal/approval"

	"github.com/cloudwego/eino/adk"
)

func TestMiddlewareStackExecutorDeep(t *testing.T) {
	handlers, err := BuildExecutorDeepHandlers(context.Background(), AgentHandlerConfig{
		ApprovalMode: approval.Guard,
		Model:        &stubBaseModel{},
		Capabilities: ExecutorDeepCapabilities{
			Filesystem: ExecutorDeepFilesystemConfig{
				Enabled:       true,
				WorkspaceRoot: t.TempDir(),
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildExecutorDeepHandlers() error = %v", err)
	}

	typeNames := handlerTypeNames(handlers)

	assertContainsHandler(t, typeNames, "middlewares.ApprovalMiddleware")
	assertContainsHandler(t, typeNames, "middlewares.SoftToolErrorMiddleware")
	patchIdx := assertContainsHandler(t, typeNames, "patchtoolcalls.")
	reductionIdx := assertContainsHandler(t, typeNames, "reduction.")
	summarizationIdx := assertContainsHandler(t, typeNames, "summarization.")

	if !(patchIdx < reductionIdx && reductionIdx < summarizationIdx) {
		t.Fatalf("executor handler order = %v, want patch -> reduction -> summarization", typeNames)
	}
}

func TestMiddlewareStackDocAgent(t *testing.T) {
	handlers, err := BuildDocAgentHandlers(context.Background(), AgentHandlerConfig{
		ApprovalMode: approval.Guard,
		Capabilities: ExecutorDeepCapabilities{
			Filesystem: ExecutorDeepFilesystemConfig{
				Enabled:       true,
				WorkspaceRoot: t.TempDir(),
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDocAgentHandlers() error = %v", err)
	}

	typeNames := handlerTypeNames(handlers)

	assertContainsHandler(t, typeNames, "middlewares.ApprovalMiddleware")
	assertContainsHandler(t, typeNames, "middlewares.SoftToolErrorMiddleware")
	assertContainsHandler(t, typeNames, "patchtoolcalls.")
	assertContainsHandler(t, typeNames, "reduction.")

	if idx := indexOfHandler(typeNames, "summarization."); idx >= 0 {
		t.Fatalf("doc agent handlers = %v, summarization must be absent", typeNames)
	}
}

func handlerTypeNames(handlers []adk.ChatModelAgentMiddleware) []string {
	typeNames := make([]string, 0, len(handlers))
	for _, handler := range handlers {
		typeNames = append(typeNames, reflect.TypeOf(handler).String())
	}
	return typeNames
}

func assertContainsHandler(t *testing.T, typeNames []string, needle string) int {
	t.Helper()

	idx := indexOfHandler(typeNames, needle)
	if idx < 0 {
		t.Fatalf("handler stack = %v, want handler containing %q", typeNames, needle)
	}
	return idx
}

func indexOfHandler(typeNames []string, needle string) int {
	for idx, typeName := range typeNames {
		if strings.Contains(typeName, needle) {
			return idx
		}
	}
	return -1
}
