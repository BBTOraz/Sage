package runtime

import (
	"bilge-lib/internal/approval"
	"context"
	"errors"
	"io"
	"testing"

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

type stubRunner struct {
	queryIter          *adk.AsyncIterator[*adk.AgentEvent]
	resumeIter         *adk.AsyncIterator[*adk.AgentEvent]
	resumeCheckPointID string
	resumeParams       *adk.ResumeParams
}

func (s *stubRunner) Query(context.Context, string, ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return s.queryIter
}

func (s *stubRunner) ResumeWithParams(_ context.Context, checkPointID string, params *adk.ResumeParams, _ ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	s.resumeCheckPointID = checkPointID
	s.resumeParams = params
	return s.resumeIter, nil
}

type stubHistoryStore struct {
	sessions []SessionSnapshot
	runs     []RunSnapshot
	events   []HistoryEventRecord
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

func init() {
	_ = io.EOF
}
