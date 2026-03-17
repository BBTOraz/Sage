package runtime

type EventType string

const (
	EventRunStarted     EventType = "run_started"
	EventAssistantChunk EventType = "assistant_chunk"
	EventAssistantDone  EventType = "assistant_done"
	EventRunCompleted   EventType = "run_completed"
	EventRunFailed      EventType = "run_failed"
	EventRunInterrupted EventType = "run_interrupted"
	EventRunResumed     EventType = "run_resumed"
)

type Event struct {
	RunID    RunID
	Status   RunStatus
	Approval *PendingApproval
	Type     EventType `json:"event_type"`
	Text     string    `json:"text"`
	Err      error     `json:"error"`
}
