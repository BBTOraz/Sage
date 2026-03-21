package middlewares

import (
	"bilge-lib/internal/approval"
	"context"
	"encoding/gob"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
)

type ApprovalMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	mode approval.Mode
}

var approvalBypassTools = map[string]struct{}{
	"task":        {},
	"write_todos": {},
}

func init() {
	gob.Register(approval.ToolInfo{})
}

func (am *ApprovalMiddleware) WrapInvokableToolCall(_ context.Context, next adk.InvokableToolCallEndpoint, tCtx *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	return func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		if am.mode == approval.Full || shouldBypassApproval(tCtx) {
			return next(ctx, argumentsInJSON, opts...)
		}

		wasInterrupted, _, args := tool.GetInterruptState[string](ctx)
		if !wasInterrupted {
			return "", tool.StatefulInterrupt(ctx, approval.ToolInfo{
				ToolName:   tCtx.Name,
				ToolCallID: tCtx.CallID,
				Args:       argumentsInJSON,
			}, argumentsInJSON)
		}

		isResumedTarget, hasData, data := tool.GetResumeContext[*approval.ResumeDecision](ctx)
		if !isResumedTarget {
			return "", tool.StatefulInterrupt(ctx, approval.ToolInfo{
				ToolName:   tCtx.Name,
				ToolCallID: tCtx.CallID,
				Args:       args,
			}, args)
		}

		if !hasData || data == nil || !data.Approved {
			return "tool call denied by user", nil
		}

		return next(ctx, args, opts...)
	}, nil
}

func shouldBypassApproval(tCtx *adk.ToolContext) bool {
	if tCtx == nil {
		return false
	}
	_, ok := approvalBypassTools[tCtx.Name]
	return ok
}
