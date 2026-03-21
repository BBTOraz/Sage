package middlewares

import (
	"bilge-lib/internal/approval"

	"github.com/cloudwego/eino/adk"
)

func DefaultMiddlewares(mode approval.Mode) []adk.ChatModelAgentMiddleware {
	approvalMiddleware := &ApprovalMiddleware{
		mode: mode,
	}
	softToolErrorMiddleware := &SoftToolErrorMiddleware{}
	return []adk.ChatModelAgentMiddleware{
		approvalMiddleware, softToolErrorMiddleware,
	}

}
