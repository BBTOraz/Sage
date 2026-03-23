package runtime

import (
	"bilge-lib/internal/approval"
	"time"
)

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
	Title       string
	Archived    bool
}

type SessionSummary struct {
	ID          SessionID
	Mode        approval.Mode
	State       SessionState
	ActiveRunID RunID
	Title       string
	Preview     string
	Archived    bool
	UpdatedAt   time.Time
}
