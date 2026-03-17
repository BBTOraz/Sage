package tui

import "bilge-lib/internal/runtime"

type MessageKind string

const (
	MessageUser   MessageKind = "user"
	MessageAgent  MessageKind = "agent"
	MessageSystem MessageKind = "system"
	MessageTool   MessageKind = "tool"
	MessageError  MessageKind = "error"
)

type Message struct {
	Kind       MessageKind
	Text       string
	ToolName   string
	ToolArgs   string
	ToolStatus string
	Expanded   bool
}

type runnerEventMsg struct {
	Event  runtime.Event
	Stream <-chan runtime.Event
}
