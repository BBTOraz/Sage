package observability

import (
	"context"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/tool"
	template "github.com/cloudwego/eino/utils/callbacks"
)

type Meta struct {
	SessionID    string
	RunID        string
	CheckpointID string
	Mode         string
}

func NewHandler(meta Meta, collector Collector) *template.HandlerHelper {
	handler := template.NewHandlerHelper().
		Agent(&template.AgentCallbackHandler{
			OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *adk.AgentCallbackInput) context.Context {
				collector.Emit(RunEvent{
					TS:           time.Now(),
					Kind:         "agent_start",
					RunID:        meta.RunID,
					SessionID:    meta.SessionID,
					CheckpointID: meta.CheckpointID,
					Mode:         meta.Mode,
					AgentName:    info.Name,
				})
				return ctx
			},
			OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *adk.AgentCallbackOutput) context.Context {
				collector.Emit(RunEvent{
					TS:           time.Now(),
					Kind:         "agent_end",
					RunID:        meta.RunID,
					SessionID:    meta.SessionID,
					CheckpointID: meta.CheckpointID,
					Mode:         meta.Mode,
					AgentName:    info.Name,
				})
				go func() {
					if output != nil && output.Events != nil {
						ev, ok := output.Events.Next()
						if !ok {
							return
						}

						if ev.Err != nil {
							collector.Emit(RunEvent{
								TS:           time.Now(),
								Kind:         "agent_error",
								RunID:        meta.RunID,
								SessionID:    meta.SessionID,
								CheckpointID: meta.CheckpointID,
								Mode:         meta.Mode,
								AgentName:    info.Name,
								Error:        ev.Err.Error(),
							})
						}
					}
				}()
				return ctx
			},
		}).
		Tool(&template.ToolCallbackHandler{
			OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
				collector.Emit(RunEvent{
					TS:           time.Now(),
					Kind:         "tool_start",
					RunID:        meta.RunID,
					SessionID:    meta.SessionID,
					CheckpointID: meta.CheckpointID,
					Mode:         meta.Mode,
					ToolName:     info.Name,
				})
				return ctx
			},
			OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context {
				collector.Emit(RunEvent{
					TS:           time.Now(),
					Kind:         "tool_end",
					RunID:        meta.RunID,
					SessionID:    meta.SessionID,
					CheckpointID: meta.CheckpointID,
					Mode:         meta.Mode,
					ToolName:     info.Name,
				})
				return ctx
			},
			OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
				collector.Emit(RunEvent{
					TS:           time.Now(),
					Kind:         "tool_error",
					RunID:        meta.RunID,
					SessionID:    meta.SessionID,
					CheckpointID: meta.CheckpointID,
					Mode:         meta.Mode,
					ToolName:     info.Name,
					Error:        err.Error(),
				})
				return ctx
			},
		})
	
	return handler
}
