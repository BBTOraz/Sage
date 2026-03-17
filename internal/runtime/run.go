package runtime

import "time"

type RunID string
type RunStatus string

const (
	RunStatusCreated     RunStatus = "created"
	RunStatusRunning     RunStatus = "running"
	RunStatusCompleted   RunStatus = "completed"
	RunStatusFailed      RunStatus = "failed"
	RunStatusInterrupted RunStatus = "interrupted"
)

type Run struct {
	ID        RunID
	SessionID SessionID
	Status    RunStatus
	StartedAt time.Time
	UpdatedAt time.Time
}

type StartRunRequest struct {
	Input string `json:"input"`
}

type RunHandle struct {
	ID     RunID
	Status RunStatus
	Events <-chan Event
}
