package runtime

import (
	"bilge-lib/internal/approval"
	"bilge-lib/internal/observability"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/google/uuid"
)

type Manager struct {
	mu              sync.Mutex
	runner          *adk.Runner
	session         Session
	activeRun       *Run
	pendingApproval *PendingApproval
	collector       *observability.Collector
}

func NewManager(mode approval.Mode, runner *adk.Runner) *Manager {
	return &Manager{
		mu:     sync.Mutex{},
		runner: runner,
		session: Session{
			ID:    SessionID(uuid.New().String()),
			Mode:  mode,
			State: SessionStateIdle,
		},
	}
}

func (m *Manager) GetSession() Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session
}

func (m *Manager) StartRun(ctx context.Context, in StartRunRequest) (RunHandle, error) {
	runId := uuid.New().String()

	iter := m.runner.Query(ctx, in.Input, adk.WithCheckPointID(runId))

	return RunHandle{
		ID:     RunID(runId),
		Status: RunStatusRunning,
		Events: m.streamAgentEvents(RunID(runId), EventRunStarted, iter),
	}, nil
}

func (m *Manager) PendingApproval() *PendingApproval {
	var tmp PendingApproval
	m.mu.Lock()
	if m.pendingApproval != nil {
		tmp = *m.pendingApproval
		m.mu.Unlock()
		return &tmp
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) ApprovePending(ctx context.Context) (RunHandle, error) {

	pending, err := m.resumePending(ctx, &approval.ResumeDecision{
		Approved: true,
	})

	if err != nil {
		return RunHandle{}, err
	}

	return pending, nil
}

func (m *Manager) DenyPending(ctx context.Context) (RunHandle, error) {
	pending, err := m.resumePending(ctx, &approval.ResumeDecision{
		Approved: false,
	})
	if err != nil {
		return RunHandle{}, err
	}
	return pending, nil
}

func (m *Manager) resumePending(ctx context.Context, data any) (RunHandle, error) {
	m.mu.Lock()
	pa := m.pendingApproval
	if pa == nil {
		m.mu.Unlock()
		return RunHandle{}, errors.New("no pending approval")
	}
	m.mu.Unlock()

	iter, err := m.runner.ResumeWithParams(ctx, pa.CheckPointID, &adk.ResumeParams{
		Targets: map[string]any{
			pa.InterruptID: data,
		},
	})

	if err != nil {
		return RunHandle{}, err
	}

	m.mu.Lock()
	m.pendingApproval = nil
	m.mu.Unlock()

	return RunHandle{
		ID:     pa.RunID,
		Status: RunStatusRunning,
		Events: m.streamAgentEvents(pa.RunID, EventRunResumed, iter),
	}, nil

}

func (m *Manager) streamAgentEvents(runID RunID, start EventType, iter *adk.AsyncIterator[*adk.AgentEvent]) <-chan Event {
	out := make(chan Event)
	go func() {
		defer close(out)
		out <- Event{
			RunID:  runID,
			Status: RunStatusRunning,
			Type:   start,
		}
		for {
			event, ok := iter.Next()
			if !ok {
				out <- Event{
					RunID:  runID,
					Status: RunStatusCompleted,
					Type:   EventRunCompleted,
				}
				break
			}

			if event.Err != nil {
				out <- Event{
					RunID:  runID,
					Status: RunStatusFailed,
					Type:   EventRunFailed,
					Err:    event.Err,
				}
				break
			}

			if event.Action != nil && event.Action.Interrupted != nil {
				ctxs := event.Action.Interrupted.InterruptContexts
				for _, ctx := range ctxs {
					if ctx.IsRootCause {
						pending := PendingApproval{
							RunID:        runID,
							CheckPointID: string(runID),
							InterruptID:  ctx.ID,
							Summary:      fmt.Sprintf("%v", ctx.Info),
						}
						if info, ok := ctx.Info.(approval.ToolInfo); ok {
							pending.ToolName = info.ToolName
							pending.Arguments = info.Args
						}
						if event.Action.Interrupted.Data != nil {
							pending.Summary = fmt.Sprintf("data:%v | info:%v", event.Action.Interrupted.Data, ctx.Info)
						}
						m.mu.Lock()
						m.pendingApproval = &pending
						m.mu.Unlock()

						out <- Event{
							RunID:    runID,
							Status:   RunStatusInterrupted,
							Type:     EventRunInterrupted,
							Approval: &pending,
						}
						return
					}
				}
			}

			if event.Output == nil || event.Output.MessageOutput == nil {
				continue
			}

			if event.Output.MessageOutput.IsStreaming {
				stream := event.Output.MessageOutput.MessageStream
				for {
					chunk, err := stream.Recv()
					if errors.Is(err, io.EOF) {
						out <- Event{
							RunID:  runID,
							Type:   EventAssistantDone,
							Status: RunStatusRunning,
						}
						break
					}
					if err != nil {
						out <- Event{
							RunID:  runID,
							Status: RunStatusFailed,
							Type:   EventRunFailed,
							Err:    err,
						}
						return
					}

					out <- Event{
						RunID:  runID,
						Status: RunStatusRunning,
						Type:   EventAssistantChunk,
						Text:   chunk.Content,
					}
				}
			}

			if !event.Output.MessageOutput.IsStreaming {
				msg, err := event.Output.MessageOutput.GetMessage()
				if err != nil {
					out <- Event{
						RunID:  runID,
						Status: RunStatusFailed,
						Type:   EventRunFailed,
						Err:    err,
					}
					break
				}
				out <- Event{
					RunID:  runID,
					Status: RunStatusRunning,
					Type:   EventAssistantChunk,
					Text:   msg.Content,
				}
				out <- Event{
					RunID:  runID,
					Type:   EventAssistantDone,
					Status: RunStatusRunning,
				}
			}
		}
	}()

	return out
}
