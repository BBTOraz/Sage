package runtime

import "bilge-lib/internal/approval"

type SessionID string
type SessionState string

const (
	SessionStateIdle    SessionState = "idle"
	SessionStateRunning SessionState = "running"
	SessionStateClosed  SessionState = "closed"
)

type Session struct {
	ID          SessionID
	Mode        approval.Mode
	State       SessionState
	ActiveRunID RunID
}
