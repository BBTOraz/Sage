package runtime

import (
	"bilge-lib/internal/approval"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestManagerStartRunPersistsSessionRunAndEvents(t *testing.T) {
	runner := &stubRunner{
		queryIter: iteratorFromAgentEvents(
			adk.EventFromMessage(schema.AssistantMessage("hello", nil), nil, schema.Assistant, ""),
		),
	}
	history := &stubHistoryStore{}
	manager := NewManager(approval.Guard, runner, nil, history)

	handle, err := manager.StartRun(context.Background(), StartRunRequest{Input: "hello"})
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	events := collectRuntimeEvents(handle.Events)
	if len(events) != 4 {
		t.Fatalf("events len = %d, want %d", len(events), 4)
	}

	if len(history.sessions) == 0 {
		t.Fatal("expected session snapshots to be persisted")
	}
	lastSession := history.sessions[len(history.sessions)-1]
	if lastSession.State != SessionStateIdle {
		t.Fatalf("last session state = %q, want %q", lastSession.State, SessionStateIdle)
	}

	if len(history.runs) == 0 {
		t.Fatal("expected run snapshots to be persisted")
	}
	lastRun := history.runs[len(history.runs)-1]
	if lastRun.Status != RunStatusCompleted {
		t.Fatalf("last run status = %q, want %q", lastRun.Status, RunStatusCompleted)
	}

	if len(history.events) != 4 {
		t.Fatalf("persisted events len = %d, want %d", len(history.events), 4)
	}
	if history.events[0].Type != EventRunStarted {
		t.Fatalf("first persisted event = %q, want %q", history.events[0].Type, EventRunStarted)
	}
	if history.events[1].Type != EventAssistantChunk {
		t.Fatalf("second persisted event = %q, want %q", history.events[1].Type, EventAssistantChunk)
	}
	if history.events[3].Type != EventRunCompleted {
		t.Fatalf("last persisted event = %q, want %q", history.events[3].Type, EventRunCompleted)
	}
}

func TestManagerApprovePendingUsesRunnerResumeAndPersistsEvents(t *testing.T) {
	runner := &stubRunner{
		resumeIter: iteratorFromAgentEvents(
			adk.EventFromMessage(schema.AssistantMessage("approved", nil), nil, schema.Assistant, ""),
		),
	}
	history := &stubHistoryStore{}
	manager := NewManager(approval.Guard, runner, nil, history)
	manager.pendingApproval = &PendingApproval{
		RunID:        RunID("run-1"),
		CheckPointID: "checkpoint-1",
		InterruptID:  "interrupt-1",
	}

	handle, err := manager.ApprovePending(context.Background())
	if err != nil {
		t.Fatalf("ApprovePending() error = %v", err)
	}

	events := collectRuntimeEvents(handle.Events)
	if len(events) != 4 {
		t.Fatalf("events len = %d, want %d", len(events), 4)
	}
	if runner.resumeCheckPointID != "checkpoint-1" {
		t.Fatalf("resume checkpoint id = %q, want %q", runner.resumeCheckPointID, "checkpoint-1")
	}

	decision, ok := runner.resumeParams.Targets["interrupt-1"].(*approval.ResumeDecision)
	if !ok {
		t.Fatalf("resume target type = %T, want *approval.ResumeDecision", runner.resumeParams.Targets["interrupt-1"])
	}
	if !decision.Approved {
		t.Fatal("resume decision approved = false, want true")
	}

	if len(history.events) == 0 || history.events[0].Type != EventRunResumed {
		t.Fatalf("persisted events = %+v, want first resumed event", history.events)
	}
	if history.events[len(history.events)-1].Type != EventRunCompleted {
		t.Fatalf("persisted events last = %q, want %q", history.events[len(history.events)-1].Type, EventRunCompleted)
	}
}

func TestManagerStartRunPersistsStructuredPayload(t *testing.T) {
	msg := schema.AssistantMessage("", []schema.ToolCall{
		{
			ID: "call-1",
			Function: schema.FunctionCall{
				Name:      "search_docs",
				Arguments: `{"query":"billing"}`,
			},
		},
	})
	event := adk.EventFromMessage(msg, nil, schema.Assistant, "")
	event.AgentName = "sage-doc"
	event.RunPath = mustRunSteps(t, "sage", "sage-doc")

	runner := &stubRunner{queryIter: iteratorFromAgentEvents(event)}
	history := &stubHistoryStore{}
	manager := NewManager(approval.Guard, runner, nil, history)

	handle, err := manager.StartRun(context.Background(), StartRunRequest{Input: "find billing"})
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	_ = collectRuntimeEvents(handle.Events)

	if len(history.events) < 2 {
		t.Fatalf("persisted events len = %d, want at least 2", len(history.events))
	}

	var payload struct {
		Payload *EventPayload `json:"payload"`
	}
	if err := json.Unmarshal([]byte(history.events[1].PayloadJSON), &payload); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}
	if payload.Payload == nil {
		t.Fatal("persisted payload = nil, want structured payload")
	}
	if payload.Payload.AgentName != "sage-doc" {
		t.Fatalf("persisted payload agent = %q, want %q", payload.Payload.AgentName, "sage-doc")
	}
	if len(payload.Payload.ToolCalls) != 1 || payload.Payload.ToolCalls[0].Name != "search_docs" {
		t.Fatalf("persisted payload tool calls = %#v, want search_docs", payload.Payload.ToolCalls)
	}
}

func TestManagerApprovePendingContinuesEventSequenceForExistingRun(t *testing.T) {
	runner := &stubRunner{
		resumeIter: iteratorFromAgentEvents(
			adk.EventFromMessage(schema.AssistantMessage("approved", nil), nil, schema.Assistant, ""),
		),
	}
	history := &stubHistoryStore{
		events: []HistoryEventRecord{
			{RunID: RunID("run-1"), Sequence: 1, Type: EventRunStarted},
			{RunID: RunID("run-1"), Sequence: 2, Type: EventAssistantChunk},
			{RunID: RunID("run-1"), Sequence: 3, Type: EventRunInterrupted},
		},
	}
	manager := NewManager(approval.Guard, runner, nil, history)
	manager.pendingApproval = &PendingApproval{
		RunID:        RunID("run-1"),
		CheckPointID: "checkpoint-1",
		InterruptID:  "interrupt-1",
	}

	handle, err := manager.ApprovePending(context.Background())
	if err != nil {
		t.Fatalf("ApprovePending() error = %v", err)
	}

	_ = collectRuntimeEvents(handle.Events)

	if len(history.events) != 7 {
		t.Fatalf("persisted events len = %d, want %d", len(history.events), 7)
	}

	for idx, want := range []int{1, 2, 3, 4, 5, 6, 7} {
		if history.events[idx].Sequence != want {
			t.Fatalf("history.events[%d].Sequence = %d, want %d", idx, history.events[idx].Sequence, want)
		}
	}
}

func TestManagerRestoreSessionLoadsRequestedHistory(t *testing.T) {
	history := &stubHistoryStore{}
	manager := NewManager(approval.Guard, nil, nil, history)

	wantSession := SessionSnapshot{
		ID:          SessionID("session-2"),
		Mode:        approval.Guard,
		State:       SessionStateIdle,
		ActiveRunID: RunID("run-22"),
		Title:       "Incident review",
	}
	history.loadedSession = &wantSession

	record, err := newHistoryEventRecord(wantSession.ID, 1, Event{
		RunID:  RunID("run-22"),
		Status: RunStatusRunning,
		Type:   EventAssistantChunk,
		Text:   "Investigating prod incident",
	})
	if err != nil {
		t.Fatalf("newHistoryEventRecord() error = %v", err)
	}
	history.events = []HistoryEventRecord{record}

	session, events, err := manager.RestoreSession(context.Background(), wantSession.ID)
	if err != nil {
		t.Fatalf("RestoreSession() error = %v", err)
	}
	if history.loadedSessionID != wantSession.ID {
		t.Fatalf("loaded session id = %q, want %q", history.loadedSessionID, wantSession.ID)
	}
	if session.ID != wantSession.ID {
		t.Fatalf("restored session id = %q, want %q", session.ID, wantSession.ID)
	}
	if manager.GetSession().ID != wantSession.ID {
		t.Fatalf("manager session id = %q, want %q", manager.GetSession().ID, wantSession.ID)
	}
	if len(events) != 1 || events[0].Text != "Investigating prod incident" {
		t.Fatalf("replayed events = %#v, want requested session history", events)
	}
}

func TestManagerListSessionsReturnsHistorySummaries(t *testing.T) {
	history := &stubHistoryStore{
		sessionSummaries: []SessionSummary{
			{
				ID:        SessionID("session-2"),
				Title:     "Incident review",
				Preview:   "Investigating prod incident",
				State:     SessionStateIdle,
				UpdatedAt: mustTime(t, "2026-03-22T15:04:05Z"),
			},
			{
				ID:        SessionID("session-1"),
				Title:     "Planner migration",
				Preview:   "Need to adapt executor context",
				State:     SessionStateIdle,
				UpdatedAt: mustTime(t, "2026-03-21T11:30:00Z"),
			},
		},
	}
	manager := NewManager(approval.Guard, nil, nil, history)

	summaries, err := manager.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("ListSessions() len = %d, want %d", len(summaries), 2)
	}
	if summaries[0].Title != "Incident review" || summaries[0].Preview != "Investigating prod incident" {
		t.Fatalf("ListSessions()[0] = %#v, want most recent summary", summaries[0])
	}
}

func TestManagerStartRunBuildsRunnerInputFromCompletedTranscriptTurns(t *testing.T) {
	runner := &stubRunner{
		queryIter: iteratorFromAgentEvents(
			adk.EventFromMessage(schema.AssistantMessage("second answer", nil), nil, schema.Assistant, ""),
		),
	}
	history := &stubHistoryStore{
		transcriptTurns: []stubTranscriptTurn{
			{UserInput: "first question", AssistantOutput: "first answer"},
		},
	}
	manager := NewManager(approval.Guard, runner, nil, history)

	handle, err := manager.StartRun(context.Background(), StartRunRequest{Input: "second question"})
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	_ = collectRuntimeEvents(handle.Events)

	if len(runner.runMessages) != 3 {
		t.Fatalf("runner input len = %d, want %d", len(runner.runMessages), 3)
	}
	if runner.runMessages[0].Role != schema.User || runner.runMessages[0].Content != "first question" {
		t.Fatalf("runner input[0] = %#v, want first user turn", runner.runMessages[0])
	}
	if runner.runMessages[1].Role != schema.Assistant || runner.runMessages[1].Content != "first answer" {
		t.Fatalf("runner input[1] = %#v, want first assistant turn", runner.runMessages[1])
	}
	if runner.runMessages[2].Role != schema.User || runner.runMessages[2].Content != "second question" {
		t.Fatalf("runner input[2] = %#v, want latest user turn", runner.runMessages[2])
	}
}

func TestManagerStartRunPersistsOnlyLastCompletedAssistantMessageAsTranscriptTurn(t *testing.T) {
	runner := &stubRunner{
		queryIter: iteratorFromAgentEvents(
			adk.EventFromMessage(schema.AssistantMessage(`{"steps":["draft a plan"]}`, nil), nil, schema.Assistant, ""),
			adk.EventFromMessage(schema.AssistantMessage("final answer", nil), nil, schema.Assistant, ""),
		),
	}
	history := &stubHistoryStore{}
	manager := NewManager(approval.Guard, runner, nil, history)

	handle, err := manager.StartRun(context.Background(), StartRunRequest{Input: "question"})
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	_ = collectRuntimeEvents(handle.Events)

	if len(history.transcriptTurns) != 1 {
		t.Fatalf("persisted transcript turns len = %d, want %d", len(history.transcriptTurns), 1)
	}
	if history.transcriptTurns[0].UserInput != "question" {
		t.Fatalf("persisted user input = %q, want %q", history.transcriptTurns[0].UserInput, "question")
	}
	if history.transcriptTurns[0].AssistantOutput != "final answer" {
		t.Fatalf("persisted assistant output = %q, want %q", history.transcriptTurns[0].AssistantOutput, "final answer")
	}
}

type stubRunner struct {
	queryIter          *adk.AsyncIterator[*adk.AgentEvent]
	resumeIter         *adk.AsyncIterator[*adk.AgentEvent]
	resumeCheckPointID string
	resumeParams       *adk.ResumeParams
	runMessages        []adk.Message
}

func (s *stubRunner) Run(_ context.Context, messages []adk.Message, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	s.runMessages = append([]adk.Message(nil), messages...)
	return s.queryIter
}

func (s *stubRunner) ResumeWithParams(_ context.Context, checkPointID string, params *adk.ResumeParams, _ ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	s.resumeCheckPointID = checkPointID
	s.resumeParams = params
	return s.resumeIter, nil
}

type stubHistoryStore struct {
	latestSession    *SessionSnapshot
	loadedSession    *SessionSnapshot
	loadedSessionID  SessionID
	sessionSummaries []SessionSummary
	sessions         []SessionSnapshot
	runs             []RunSnapshot
	events           []HistoryEventRecord
	transcriptTurns  []stubTranscriptTurn
}

func (s *stubHistoryStore) SaveSession(_ context.Context, snapshot SessionSnapshot) error {
	s.sessions = append(s.sessions, snapshot)
	return nil
}

func (s *stubHistoryStore) SaveRun(_ context.Context, snapshot RunSnapshot) error {
	s.runs = append(s.runs, snapshot)
	return nil
}

func (s *stubHistoryStore) LastEventSequence(_ context.Context, runID RunID) (int, error) {
	last := 0
	for _, event := range s.events {
		if event.RunID == runID && event.Sequence > last {
			last = event.Sequence
		}
	}
	return last, nil
}

func (s *stubHistoryStore) AppendEvent(_ context.Context, event HistoryEventRecord) error {
	for _, existing := range s.events {
		if existing.RunID == event.RunID && existing.Sequence == event.Sequence {
			return errors.New("duplicate history event sequence")
		}
	}
	s.events = append(s.events, event)
	return nil
}

func (s *stubHistoryStore) CreateTranscriptTurn(_ context.Context, turn SessionTranscriptTurn) error {
	s.transcriptTurns = append(s.transcriptTurns, stubTranscriptTurn{
		UserInput:       turn.UserInput,
		AssistantOutput: turn.AssistantOutput,
	})
	return nil
}

func (s *stubHistoryStore) CompleteTranscriptTurn(_ context.Context, runID RunID, assistantOutput string) error {
	for idx := len(s.transcriptTurns) - 1; idx >= 0; idx-- {
		if s.transcriptTurns[idx].AssistantOutput == "" {
			s.transcriptTurns[idx].AssistantOutput = assistantOutput
			return nil
		}
	}
	return nil
}

func (s *stubHistoryStore) LatestSession(_ context.Context) (SessionSnapshot, bool, error) {
	if s.latestSession == nil {
		return SessionSnapshot{}, false, nil
	}
	return *s.latestSession, true, nil
}

func (s *stubHistoryStore) LoadSession(_ context.Context, sessionID SessionID) (SessionSnapshot, bool, error) {
	s.loadedSessionID = sessionID
	if s.loadedSession == nil {
		return SessionSnapshot{}, false, nil
	}
	return *s.loadedSession, true, nil
}

func (s *stubHistoryStore) ListSessions(_ context.Context) ([]SessionSummary, error) {
	return s.sessionSummaries, nil
}

func (s *stubHistoryStore) ListEvents(_ context.Context, sessionID SessionID) ([]HistoryEventRecord, error) {
	var events []HistoryEventRecord
	for _, event := range s.events {
		if event.SessionID == "" || event.SessionID == sessionID {
			events = append(events, event)
		}
	}
	return events, nil
}

func (s *stubHistoryStore) ListTranscriptTurns(_ context.Context, _ SessionID) ([]SessionTranscriptTurn, error) {
	turns := make([]SessionTranscriptTurn, 0, len(s.transcriptTurns))
	for _, turn := range s.transcriptTurns {
		if turn.AssistantOutput == "" {
			continue
		}
		turns = append(turns, SessionTranscriptTurn{
			UserInput:       turn.UserInput,
			AssistantOutput: turn.AssistantOutput,
		})
	}
	return turns, nil
}

func iteratorFromAgentEvents(events ...*adk.AgentEvent) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, writer := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer writer.Close()
		for _, event := range events {
			writer.Send(event)
		}
	}()
	return iter
}

func collectRuntimeEvents(ch <-chan Event) []Event {
	var events []Event
	for event := range ch {
		events = append(events, event)
	}
	return events
}

func mustTime(t *testing.T, raw string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", raw, err)
	}
	return parsed
}

func init() {
	_ = io.EOF
}

type stubTranscriptTurn struct {
	UserInput       string
	AssistantOutput string
}
