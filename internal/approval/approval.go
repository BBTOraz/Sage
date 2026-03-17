package approval

type ResumeDecision struct {
	Approved bool
}

type ToolInfo struct {
	ToolName   string
	Args       string
	ToolCallID string
}
