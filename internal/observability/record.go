package observability

import "time"

type RunEvent struct {
	TS           time.Time
	Kind         string
	SessionID    string
	RunID        string
	CheckpointID string
	InterruptID  string
	Mode         string

	AgentName string
	ToolName  string
	Payload   map[string]any

	Message string
	Error   string
}
