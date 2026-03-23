package runtime

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type AgentRunner interface {
	Run(ctx context.Context, messages []adk.Message, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent]
	ResumeWithParams(ctx context.Context, checkPointID string, params *adk.ResumeParams, opts ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error)
}

type SessionSnapshot = Session
type RunSnapshot = Run

type SessionTranscriptTurn struct {
	SessionID       SessionID
	RunID           RunID
	UserInput       string
	AssistantOutput string
}

type HistoryEventRecord struct {
	SessionID   SessionID
	RunID       RunID
	Sequence    int
	Type        EventType
	Status      RunStatus
	Text        string
	PayloadJSON string
}

type HistoryStore interface {
	SaveSession(ctx context.Context, snapshot SessionSnapshot) error
	SaveRun(ctx context.Context, snapshot RunSnapshot) error
	LastEventSequence(ctx context.Context, runID RunID) (int, error)
	AppendEvent(ctx context.Context, event HistoryEventRecord) error
	CreateTranscriptTurn(ctx context.Context, turn SessionTranscriptTurn) error
	CompleteTranscriptTurn(ctx context.Context, runID RunID, assistantOutput string) error
	LatestSession(ctx context.Context) (SessionSnapshot, bool, error)
	LoadSession(ctx context.Context, sessionID SessionID) (SessionSnapshot, bool, error)
	ListSessions(ctx context.Context) ([]SessionSummary, error)
	ListEvents(ctx context.Context, sessionID SessionID) ([]HistoryEventRecord, error)
	ListTranscriptTurns(ctx context.Context, sessionID SessionID) ([]SessionTranscriptTurn, error)
}

type NoopHistoryStore struct{}

func (NoopHistoryStore) SaveSession(context.Context, SessionSnapshot) error { return nil }
func (NoopHistoryStore) SaveRun(context.Context, RunSnapshot) error         { return nil }
func (NoopHistoryStore) LastEventSequence(context.Context, RunID) (int, error) {
	return 0, nil
}
func (NoopHistoryStore) AppendEvent(context.Context, HistoryEventRecord) error {
	return nil
}
func (NoopHistoryStore) CreateTranscriptTurn(context.Context, SessionTranscriptTurn) error {
	return nil
}
func (NoopHistoryStore) CompleteTranscriptTurn(context.Context, RunID, string) error {
	return nil
}
func (NoopHistoryStore) LatestSession(context.Context) (SessionSnapshot, bool, error) {
	return SessionSnapshot{}, false, nil
}
func (NoopHistoryStore) LoadSession(context.Context, SessionID) (SessionSnapshot, bool, error) {
	return SessionSnapshot{}, false, nil
}
func (NoopHistoryStore) ListSessions(context.Context) ([]SessionSummary, error) {
	return nil, nil
}
func (NoopHistoryStore) ListEvents(context.Context, SessionID) ([]HistoryEventRecord, error) {
	return nil, nil
}
func (NoopHistoryStore) ListTranscriptTurns(context.Context, SessionID) ([]SessionTranscriptTurn, error) {
	return nil, nil
}

func buildSessionMemoryMessages(turns []SessionTranscriptTurn, latestUserInput string) []adk.Message {
	messages := make([]adk.Message, 0, len(turns)*2+1)

	for _, turn := range turns {
		if turn.UserInput != "" {
			messages = append(messages, schema.UserMessage(turn.UserInput))
		}
		if turn.AssistantOutput != "" {
			messages = append(messages, schema.AssistantMessage(turn.AssistantOutput, nil))
		}
	}

	messages = append(messages, schema.UserMessage(latestUserInput))
	return messages
}

func newHistoryEventRecord(sessionID SessionID, sequence int, event Event) (HistoryEventRecord, error) {
	payload, err := json.Marshal(struct {
		Status   RunStatus        `json:"status"`
		Text     string           `json:"text,omitempty"`
		Error    string           `json:"error,omitempty"`
		Approval *PendingApproval `json:"approval,omitempty"`
		Payload  *EventPayload    `json:"payload,omitempty"`
	}{
		Status:   event.Status,
		Text:     event.Text,
		Error:    errorString(event.Err),
		Approval: event.Approval,
		Payload:  event.Payload,
	})
	if err != nil {
		return HistoryEventRecord{}, err
	}

	return HistoryEventRecord{
		SessionID:   sessionID,
		RunID:       event.RunID,
		Sequence:    sequence,
		Type:        event.Type,
		Status:      event.Status,
		Text:        event.Text,
		PayloadJSON: string(payload),
	}, nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func eventFromHistoryRecord(record HistoryEventRecord) (Event, error) {
	var payload struct {
		Status   RunStatus        `json:"status"`
		Text     string           `json:"text,omitempty"`
		Error    string           `json:"error,omitempty"`
		Approval *PendingApproval `json:"approval,omitempty"`
		Payload  *EventPayload    `json:"payload,omitempty"`
	}
	if record.PayloadJSON != "" {
		if err := json.Unmarshal([]byte(record.PayloadJSON), &payload); err != nil {
			return Event{}, err
		}
	}

	event := Event{
		RunID:    record.RunID,
		Status:   record.Status,
		Type:     record.Type,
		Text:     record.Text,
		Approval: payload.Approval,
		Payload:  payload.Payload,
	}
	if payload.Status != "" {
		event.Status = payload.Status
	}
	if payload.Text != "" {
		event.Text = payload.Text
	}
	if payload.Error != "" {
		event.Err = jsonError(payload.Error)
	}
	return event, nil
}

type jsonError string

func (e jsonError) Error() string {
	return string(e)
}
