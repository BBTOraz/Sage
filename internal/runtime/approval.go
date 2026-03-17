package runtime

type PendingApproval struct {
	RunID        RunID
	CheckPointID string
	InterruptID  string
	Summary      string
	ToolName     string
	Arguments    string
}
