package runtime

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/adk"
)

type AgentRunner interface {
	Query(ctx context.Context, query string, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent]
	ResumeWithParams(ctx context.Context, checkPointID string, params *adk.ResumeParams, opts ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error)
}

type SessionSnapshot = Session
type RunSnapshot = Run

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

func newHistoryEventRecord(sessionID SessionID, sequence int, event Event) (HistoryEventRecord, error) {
	payload, err := json.Marshal(struct {
		Status   RunStatus        `json:"status"`
		Text     string           `json:"text,omitempty"`
		Error    string           `json:"error,omitempty"`
		Approval *PendingApproval `json:"approval,omitempty"`
	}{
		Status:   event.Status,
		Text:     event.Text,
		Error:    errorString(event.Err),
		Approval: event.Approval,
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
