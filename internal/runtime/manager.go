package runtime

import (
	"bilge-lib/internal/approval"
	"bilge-lib/internal/ingestion/pipeline"
	"bilge-lib/internal/observability"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/google/uuid"
)

type Manager struct {
	mu              sync.Mutex
	runner          AgentRunner
	session         Session
	activeRun       *Run
	pendingApproval *PendingApproval
	historyStore    HistoryStore
	collector       *observability.Collector
	ingester        pipeline.Ingester
	ingestQueue     chan ingestJob
	ingestWorkers   int
	ingestOnce      sync.Once
}

func NewManager(mode approval.Mode, runner AgentRunner, ingester pipeline.Ingester, historyStores ...HistoryStore) *Manager {
	historyStore := HistoryStore(NoopHistoryStore{})
	if len(historyStores) > 0 && historyStores[0] != nil {
		historyStore = historyStores[0]
	}

	return &Manager{
		mu:            sync.Mutex{},
		runner:        runner,
		historyStore:  historyStore,
		ingester:      ingester,
		ingestQueue:   make(chan ingestJob, 32),
		ingestWorkers: 2,
		session:       newSession(mode),
	}
}

func (m *Manager) GetSession() Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session
}

func (m *Manager) RestoreLatestSession(ctx context.Context) (SessionSnapshot, []Event, error) {
	session, ok, err := m.historyStore.LatestSession(ctx)
	if err != nil || !ok {
		return SessionSnapshot{}, nil, err
	}

	return m.restoreStoredSession(ctx, session)
}

func (m *Manager) RestoreSession(ctx context.Context, sessionID SessionID) (SessionSnapshot, []Event, error) {
	session, ok, err := m.historyStore.LoadSession(ctx, sessionID)
	if err != nil || !ok {
		return SessionSnapshot{}, nil, err
	}

	return m.restoreStoredSession(ctx, session)
}

func (m *Manager) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	return m.historyStore.ListSessions(ctx)
}

func (m *Manager) StartNewSession() SessionSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.session = newSession(m.session.Mode)
	m.activeRun = nil
	m.pendingApproval = nil
	return m.session
}

func (m *Manager) StartRun(ctx context.Context, in StartRunRequest) (RunHandle, error) {
	runId := uuid.New().String()
	turns, err := m.historyStore.ListTranscriptTurns(ctx, m.GetSession().ID)
	if err != nil {
		return RunHandle{}, err
	}
	messages := buildSessionMemoryMessages(turns, in.Input)

	now := time.Now().UTC()
	m.mu.Lock()
	m.session.State = SessionStateRunning
	m.session.ActiveRunID = RunID(runId)
	m.activeRun = &Run{
		ID:        RunID(runId),
		SessionID: m.session.ID,
		Status:    RunStatusRunning,
		StartedAt: now,
		UpdatedAt: now,
	}
	sessionSnapshot := m.session
	runSnapshot := *m.activeRun
	m.mu.Unlock()

	if err := m.historyStore.SaveSession(ctx, sessionSnapshot); err != nil {
		return RunHandle{}, err
	}
	if err := m.historyStore.SaveRun(ctx, runSnapshot); err != nil {
		return RunHandle{}, err
	}
	if err := m.historyStore.CreateTranscriptTurn(ctx, SessionTranscriptTurn{
		SessionID: sessionSnapshot.ID,
		RunID:     RunID(runId),
		UserInput: in.Input,
	}); err != nil {
		return RunHandle{}, err
	}

	iter := m.runner.Run(ctx, messages, adk.WithCheckPointID(runId))

	return RunHandle{
		ID:     RunID(runId),
		Status: RunStatusRunning,
		Events: m.streamAgentEvents(ctx, RunID(runId), EventRunStarted, iter),
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
	if m.activeRun == nil || m.activeRun.ID != pa.RunID {
		m.activeRun = &Run{
			ID:        pa.RunID,
			SessionID: m.session.ID,
			StartedAt: time.Now().UTC(),
		}
	}
	m.activeRun.Status = RunStatusRunning
	m.activeRun.UpdatedAt = time.Now().UTC()
	m.session.State = SessionStateRunning
	m.session.ActiveRunID = pa.RunID
	sessionSnapshot := m.session
	runSnapshot := *m.activeRun
	m.mu.Unlock()

	if err := m.historyStore.SaveSession(ctx, sessionSnapshot); err != nil {
		return RunHandle{}, err
	}
	if err := m.historyStore.SaveRun(ctx, runSnapshot); err != nil {
		return RunHandle{}, err
	}

	return RunHandle{
		ID:     pa.RunID,
		Status: RunStatusRunning,
		Events: m.streamAgentEvents(ctx, pa.RunID, EventRunResumed, iter),
	}, nil

}

func (m *Manager) streamAgentEvents(ctx context.Context, runID RunID, start EventType, iter *adk.AsyncIterator[*adk.AgentEvent]) <-chan Event {
	out := make(chan Event)
	go func() {
		defer close(out)
		sequence, err := m.historyStore.LastEventSequence(ctx, runID)
		if err != nil {
			out <- Event{
				RunID:  runID,
				Status: RunStatusFailed,
				Type:   EventRunFailed,
				Err:    err,
			}
			return
		}
		emit := func(event Event) bool {
			out <- event
			sequence++
			record, err := newHistoryEventRecord(m.GetSession().ID, sequence, event)
			if err != nil {
				out <- Event{
					RunID:  runID,
					Status: RunStatusFailed,
					Type:   EventRunFailed,
					Err:    err,
				}
				return false
			}
			if err := m.historyStore.AppendEvent(ctx, record); err != nil {
				out <- Event{
					RunID:  runID,
					Status: RunStatusFailed,
					Type:   EventRunFailed,
					Err:    err,
				}
				return false
			}
			return true
		}

		var currentAssistantMessage strings.Builder
		lastCompletedAssistantMessage := ""
		flushAssistantMessage := func() {
			if currentAssistantMessage.Len() == 0 {
				return
			}
			lastCompletedAssistantMessage = currentAssistantMessage.String()
			currentAssistantMessage.Reset()
		}

		if !emit(Event{
			RunID:  runID,
			Status: RunStatusRunning,
			Type:   start,
		}) {
			return
		}
		for {
			event, ok := iter.Next()
			if !ok {
				if err := m.completeTranscriptTurn(ctx, runID, lastCompletedAssistantMessage); err != nil {
					m.finishRun(ctx, runID, RunStatusFailed)
					emit(Event{
						RunID:  runID,
						Status: RunStatusFailed,
						Type:   EventRunFailed,
						Err:    err,
					})
					break
				}
				m.finishRun(ctx, runID, RunStatusCompleted)
				emit(Event{
					RunID:  runID,
					Status: RunStatusCompleted,
					Type:   EventRunCompleted,
				})
				break
			}

			if event.Err != nil {
				m.finishRun(ctx, runID, RunStatusFailed)
				emit(Event{
					RunID:  runID,
					Status: RunStatusFailed,
					Type:   EventRunFailed,
					Err:    event.Err,
				})
				break
			}

			if event.Action != nil && event.Action.Interrupted != nil {
				ctxs := event.Action.Interrupted.InterruptContexts
				for _, interruptCtx := range ctxs {
					if interruptCtx.IsRootCause {
						pending := PendingApproval{
							RunID:        runID,
							CheckPointID: string(runID),
							InterruptID:  interruptCtx.ID,
							Summary:      fmt.Sprintf("%v", interruptCtx.Info),
						}
						if info, ok := interruptCtx.Info.(approval.ToolInfo); ok {
							pending.ToolName = info.ToolName
							pending.Arguments = info.Args
						}
						if event.Action.Interrupted.Data != nil {
							pending.Summary = fmt.Sprintf("data:%v | info:%v", event.Action.Interrupted.Data, interruptCtx.Info)
						}
						m.mu.Lock()
						m.pendingApproval = &pending
						m.mu.Unlock()

						m.markRunInterrupted(ctx, runID)
						emit(Event{
							RunID:    runID,
							Status:   RunStatusInterrupted,
							Type:     EventRunInterrupted,
							Approval: &pending,
						})
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
						if !emit(Event{
							RunID:  runID,
							Type:   EventAssistantDone,
							Status: RunStatusRunning,
						}) {
							return
						}
						flushAssistantMessage()
						break
					}
					if err != nil {
						m.finishRun(ctx, runID, RunStatusFailed)
						emit(Event{
							RunID:  runID,
							Status: RunStatusFailed,
							Type:   EventRunFailed,
							Err:    err,
						})
						return
					}

					if !emit(Event{
						RunID:  runID,
						Status: RunStatusRunning,
						Type:   EventAssistantChunk,
						Text:   chunk.Content,
						Payload: buildEventPayload(
							event,
							event.Output.MessageOutput,
							chunk,
						),
					}) {
						return
					}
					currentAssistantMessage.WriteString(chunk.Content)
				}
			}

			if !event.Output.MessageOutput.IsStreaming {
				msg, err := event.Output.MessageOutput.GetMessage()
				if err != nil {
					m.finishRun(ctx, runID, RunStatusFailed)
					emit(Event{
						RunID:  runID,
						Status: RunStatusFailed,
						Type:   EventRunFailed,
						Err:    err,
					})
					break
				}
				if !emit(Event{
					RunID:  runID,
					Status: RunStatusRunning,
					Type:   EventAssistantChunk,
					Text:   msg.Content,
					Payload: buildEventPayload(
						event,
						event.Output.MessageOutput,
						msg,
					),
				}) {
					return
				}
				currentAssistantMessage.WriteString(msg.Content)
				if !emit(Event{
					RunID:  runID,
					Type:   EventAssistantDone,
					Status: RunStatusRunning,
				}) {
					return
				}
				flushAssistantMessage()
			}
		}
	}()

	return out
}

func (m *Manager) completeTranscriptTurn(ctx context.Context, runID RunID, assistantOutput string) error {
	if strings.TrimSpace(assistantOutput) == "" {
		return nil
	}
	return m.historyStore.CompleteTranscriptTurn(ctx, runID, assistantOutput)
}

func (m *Manager) finishRun(ctx context.Context, runID RunID, status RunStatus) {
	m.mu.Lock()
	m.session.State = SessionStateIdle
	m.session.ActiveRunID = ""
	sessionSnapshot := m.session
	var runSnapshot *Run
	if m.activeRun != nil && m.activeRun.ID == runID {
		m.activeRun.Status = status
		m.activeRun.UpdatedAt = time.Now().UTC()
		snapshot := *m.activeRun
		runSnapshot = &snapshot
	}
	m.mu.Unlock()

	if runSnapshot != nil {
		_ = m.historyStore.SaveRun(ctx, *runSnapshot)
	}
	_ = m.historyStore.SaveSession(ctx, sessionSnapshot)
}

func (m *Manager) markRunInterrupted(ctx context.Context, runID RunID) {
	m.mu.Lock()
	m.session.State = SessionStateIdle
	sessionSnapshot := m.session
	var runSnapshot *Run
	if m.activeRun != nil && m.activeRun.ID == runID {
		m.activeRun.Status = RunStatusInterrupted
		m.activeRun.UpdatedAt = time.Now().UTC()
		snapshot := *m.activeRun
		runSnapshot = &snapshot
	}
	m.mu.Unlock()

	if runSnapshot != nil {
		_ = m.historyStore.SaveRun(ctx, *runSnapshot)
	}
	_ = m.historyStore.SaveSession(ctx, sessionSnapshot)
}

func (m *Manager) restoreStoredSession(ctx context.Context, session SessionSnapshot) (SessionSnapshot, []Event, error) {
	records, err := m.historyStore.ListEvents(ctx, session.ID)
	if err != nil {
		return SessionSnapshot{}, nil, err
	}

	events := make([]Event, 0, len(records))
	var pending *PendingApproval
	var activeRunStatus RunStatus

	for _, record := range records {
		event, err := eventFromHistoryRecord(record)
		if err != nil {
			return SessionSnapshot{}, nil, err
		}
		events = append(events, event)
		if event.RunID == session.ActiveRunID {
			activeRunStatus = event.Status
		}
		if event.Type == EventRunInterrupted && event.Approval != nil {
			approval := *event.Approval
			pending = &approval
		}
	}

	m.mu.Lock()
	m.session = session
	m.pendingApproval = pending
	if session.ActiveRunID != "" {
		m.activeRun = &Run{
			ID:        session.ActiveRunID,
			SessionID: session.ID,
			Status:    activeRunStatus,
		}
	} else {
		m.activeRun = nil
	}
	m.mu.Unlock()

	return session, events, nil
}

func newSession(mode approval.Mode) Session {
	return Session{
		ID:    SessionID(uuid.New().String()),
		Mode:  mode,
		State: SessionStateIdle,
	}
}
