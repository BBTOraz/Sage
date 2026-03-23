package tui

import (
	"bilge-lib/internal/approval"
	"bilge-lib/internal/runtime"
	"context"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type model struct {
	width, height    int
	ctx              context.Context
	mode             approval.Mode
	area             textarea.Model
	help             help.Model
	viewport         viewport.Model
	spinner          spinner.Model
	messages         []Message
	transcript       *transcriptTree
	planStore        *planStore
	pinnedPlanRunID  runtime.RunID
	followTranscript bool
	assistantDraft   string
	activeRunID      runtime.RunID
	runStatus        runtime.RunStatus
	keys             keyMap
	manager          *runtime.Manager
	sessionID        runtime.SessionID
	sessionState     runtime.SessionState
	pendingApproval  *runtime.PendingApproval
	approvalToggle   approvalToggle
	fileSuggest      fileSuggest
	dirSuggest       fileSuggest
	focusedToolID    string
	slash            slashState
	sessionSuggest   sessionSuggestState
	activities       []activityEntry
}

func newModel(manager *runtime.Manager, ctx context.Context) *model {
	input := textarea.New()
	input.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "> "
		}
		return "  "
	})
	input.Placeholder = "write a message or type / for commands"
	input.CharLimit = 0
	input.SetWidth(60)
	input.SetHeight(3)

	// Reconfigure textarea: enter is for send, ctrl+j / shift+enter for newline
	km := input.KeyMap
	km.InsertNewline = key.NewBinding(
		key.WithKeys("ctrl+j", "shift+enter"),
		key.WithHelp("ctrl+j", "new line"),
	)
	input.KeyMap = km

	input.Focus()

	vport := viewport.New()
	h := help.New()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	vport.SetContent(
		helpBarStyle.Render("  No messages yet. Type a message and press enter."),
	)

	m := &model{
		ctx:              ctx,
		mode:             manager.GetSession().Mode,
		help:             h,
		keys:             keys,
		manager:          manager,
		area:             input,
		viewport:         vport,
		spinner:          sp,
		transcript:       newTranscriptTree(),
		planStore:        newPlanStore(),
		followTranscript: true,
		approvalToggle:   newApprovalToggle(),
		fileSuggest:      newFileSuggest("."),
		dirSuggest:       newDirSuggest("."),
		messages:         []Message{},
		sessionID:        manager.GetSession().ID,
		sessionState:     manager.GetSession().State,
	}
	return m
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.syncLayout()
		m.refreshViewport()

		return m, nil

	case spinner.TickMsg:
		if m.activeRunID != "" || m.hasActiveIngestJobs() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			m.refreshViewport()
			return m, cmd
		}
		return m, nil

	case runnerEventMsg:
		eventMsg := msg.Event
		if m.activeRunID != eventMsg.RunID {
			return m, nil
		}
		m.transcript.ApplyEvent(eventMsg)
		m.planStore.ApplyEvent(eventMsg)
		if m.planStore.planFor(eventMsg.RunID) != nil {
			m.pinnedPlanRunID = eventMsg.RunID
		}

		m.runStatus = eventMsg.Status

		switch eventMsg.Type {
		case runtime.EventRunStarted:
			// spinner already ticking

		case runtime.EventAssistantChunk:
			m.refreshViewport()

		case runtime.EventAssistantDone:
			m.assistantDraft = ""
			m.pendingApproval = nil
			m.syncLayout()
			m.refreshViewport()

		case runtime.EventRunCompleted:
			m.activeRunID = ""
			m.pendingApproval = nil
			m.syncLayout()
			m.refreshViewport()

		case runtime.EventRunFailed:
			m.assistantDraft = ""
			m.activeRunID = ""
			m.pendingApproval = nil
			m.area.Focus()
			m.syncLayout()
			if eventMsg.Err != nil {
				m.appendMessage(Message{
					Kind: MessageError,
					Text: eventMsg.Err.Error(),
				})
			} else {
				m.refreshViewport()
			}

		case runtime.EventRunInterrupted:
			m.pendingApproval = eventMsg.Approval
			m.approvalToggle = newApprovalToggle()
			m.activeRunID = ""
			m.area.Blur()
			m.syncLayout()
			m.refreshViewport()

		case runtime.EventRunResumed:
			// spinner will be re-started via handleApprove/handleDeny
		}

		return m, waitRunnerEvent(msg.Stream)

	case ingestEventMsg:
		m.upsertIngestActivity(msg.Event)
		m.syncLayout()
		m.refreshViewport()
		return m, waitIngestEvent(msg.Stream)

	case tea.KeyPressMsg:
		// Quit always works
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		// Approval mode: toggle with arrows, confirm with enter
		if m.pendingApproval != nil {
			switch {
			case key.Matches(msg, m.keys.ToggleLeft):
				m.approvalToggle.moveLeft()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, m.keys.ToggleRight):
				m.approvalToggle.moveRight()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, m.keys.ToggleConfirm):
				if m.approvalToggle.isApprove() {
					return m.handleApprove()
				}
				return m.handleDeny()
			case key.Matches(msg, m.keys.PageUp):
				m.scrollViewportPageUp()
				return m, nil
			case key.Matches(msg, m.keys.PageDown):
				m.scrollViewportPageDown()
				return m, nil
			}
			return m, nil
		}

		// Directory suggest mode for /ingest <dir>
		if m.dirSuggest.active {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
				m.dirSuggest.moveUp()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
				m.dirSuggest.moveDown()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("tab", "enter"))):
				selected := m.dirSuggest.confirm()
				if selected != "" {
					m.area.SetValue("/ingest " + selected)
				}
				m.dirSuggest.deactivate()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("escape"))):
				m.dirSuggest.deactivate()
				m.refreshViewport()
				return m, nil
			}
		}

		// File suggest mode: intercept navigation keys
		if m.fileSuggest.active {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
				m.fileSuggest.moveUp()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
				m.fileSuggest.moveDown()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("tab", "enter"))):
				selected := m.fileSuggest.confirm()
				if selected != "" {
					val := m.area.Value()
					atIdx := m.fileSuggest.atPos
					endIdx := atIdx + 1 + len(m.fileSuggest.query) // '@' + query
					if atIdx >= 0 && endIdx <= len(val) {
						newVal := val[:atIdx] + selected + val[endIdx:]
						m.area.SetValue(newVal)
					}
				}
				m.fileSuggest.deactivate()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("escape"))):
				m.fileSuggest.deactivate()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
				m.fileSuggest.backspaceQuery()
				m.area, _ = m.area.Update(msg)
				m.refreshViewport()
				return m, nil
			default:
				k := msg.String()
				if len(k) == 1 && k != "@" {
					m.fileSuggest.appendQuery(rune(k[0]))
				}
				m.area, _ = m.area.Update(msg)
				m.refreshViewport()
				return m, nil
			}
		}

		if m.sessionSuggest.active {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
				m.sessionSuggest.moveUp()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
				m.sessionSuggest.moveDown()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("tab", "enter"))):
				if err := m.chooseSelectedSession(); err != nil {
					m.addActivityNotice("failed", "Session", err.Error())
					m.refreshViewport()
				}
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("escape"))):
				m.area.SetValue("")
				m.updateSlashState()
				m.refreshViewport()
				return m, nil
			}
		}

		// Detect '@' to activate file suggestions
		if msg.String() == "@" && !m.fileSuggest.active {
			m.fileSuggest.activate(len(m.area.Value()))
			m.area, _ = m.area.Update(msg)
			m.refreshViewport()
			return m, nil
		}

		if m.slash.Active {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
				m.moveSlashUp()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
				m.moveSlashDown()
				m.refreshViewport()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("tab", "enter"))):
				if cmd, ok := m.selectedSlashCommand(); ok {
					m.area.SetValue("/" + cmd.Name + " ")
					m.updateSlashState()
					m.refreshViewport()
					return m, nil
				}
			case key.Matches(msg, key.NewBinding(key.WithKeys("escape"))):
				m.area.SetValue("")
				m.updateSlashState()
				m.refreshViewport()
				return m, nil
			}
		}

		// Normal input mode
		switch {
		case key.Matches(msg, m.keys.Send):
			return m.handleSend()
		case key.Matches(msg, m.keys.NextTool):
			m.focusNextTool()
			return m, nil
		case key.Matches(msg, m.keys.PrevTool):
			m.focusPrevTool()
			return m, nil
		case key.Matches(msg, m.keys.ExpandTools):
			m.toggleToolExpansion()
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			m.scrollViewportPageUp()
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			m.scrollViewportPageDown()
			return m, nil
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.area, cmd = m.area.Update(msg)
	cmds = append(cmds, cmd)
	m.syncDirSuggestFromArea()
	m.updateSlashState()
	m.refreshViewport()

	m.viewport, cmd = m.viewport.Update(msg)
	m.syncFollowTranscript()
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) handleSend() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.area.Value())
	if strings.HasPrefix(raw, "/") {
		return m.handleSlashCommand(raw)
	}

	if m.activeRunID != "" {
		m.appendMessage(Message{
			Kind: MessageSystem,
			Text: "current run still in progress",
		})
		return m, nil
	}

	if raw == "" {
		return m, nil
	}

	message := Message{Kind: MessageUser, Text: raw}
	m.followTranscript = true
	m.appendMessage(message)
	m.area.SetValue("")
	m.updateSlashState()

	handle, err := m.manager.StartRun(m.ctx, runtime.StartRunRequest{Input: raw})
	if err != nil {
		m.appendMessage(Message{Kind: MessageError, Text: err.Error()})
		return m, nil
	}

	m.activeRunID = handle.ID
	m.runStatus = handle.Status
	m.pinnedPlanRunID = handle.ID
	m.appendMessage(Message{Kind: MessageRun, RunID: handle.ID})

	return m, tea.Batch(
		m.spinner.Tick,
		waitRunnerEvent(handle.Events),
	)
}

func (m *model) handleSlashCommand(raw string) (tea.Model, tea.Cmd) {
	if completed, ok := autocompleteSlashCommand(raw); ok && strings.TrimSpace(raw) != strings.TrimSpace(completed) {
		m.area.SetValue(completed)
		m.updateSlashState()
		return m, nil
	}

	command, arg, ok := resolveSlashCommand(raw)
	if !ok {
		m.addActivityNotice("failed", "Command", "unknown slash command")
		m.area.SetValue("")
		m.updateSlashState()
		m.refreshViewport()
		return m, nil
	}

	switch command.Name {
	case "ingest":
		if strings.TrimSpace(arg) == "" {
			m.area.SetValue("/ingest ")
			m.updateSlashState()
			m.addActivityNotice("info", "Command", "provide a directory path for /ingest")
			m.refreshViewport()
			return m, nil
		}

		handle, err := m.manager.StartIngest(m.ctx, arg)
		if err != nil {
			m.addActivityNotice("failed", "Ingest", err.Error())
			m.area.SetValue("")
			m.updateSlashState()
			m.refreshViewport()
			return m, nil
		}

		m.area.SetValue("")
		m.updateSlashState()
		m.refreshViewport()
		return m, tea.Batch(
			m.spinner.Tick,
			waitIngestEvent(handle.Events),
		)
	case "session":
		m.area.SetValue("/session ")
		m.updateSlashState()
		m.refreshViewport()
		return m, nil
	default:
		m.addActivityNotice("failed", "Command", "unsupported slash command")
		m.area.SetValue("")
		m.updateSlashState()
		m.refreshViewport()
		return m, nil
	}
}

func (m *model) handleApprove() (tea.Model, tea.Cmd) {
	m.followTranscript = true
	pending := m.pendingApproval

	handle, err := m.manager.ApprovePending(m.ctx)
	if err != nil {
		m.appendMessage(Message{Kind: MessageError, Text: err.Error()})
		m.pendingApproval = nil
		m.area.Focus()
		return m, nil
	}

	if pending != nil {
		m.transcript.resolveApproval(pending.RunID, toolName(pending), "approved")
	}
	m.pendingApproval = nil
	m.activeRunID = handle.ID
	m.runStatus = handle.Status
	m.area.Focus()
	m.syncLayout()

	return m, tea.Batch(
		m.spinner.Tick,
		waitRunnerEvent(handle.Events),
	)
}

func (m *model) handleDeny() (tea.Model, tea.Cmd) {
	m.followTranscript = true
	pending := m.pendingApproval

	handle, err := m.manager.DenyPending(m.ctx)
	if err != nil {
		m.appendMessage(Message{Kind: MessageError, Text: err.Error()})
		m.pendingApproval = nil
		m.area.Focus()
		return m, nil
	}

	if pending != nil {
		m.transcript.resolveApproval(pending.RunID, toolName(pending), "denied")
	}
	m.pendingApproval = nil
	m.activeRunID = handle.ID
	m.runStatus = handle.Status
	m.area.Focus()
	m.syncLayout()

	return m, tea.Batch(
		m.spinner.Tick,
		waitRunnerEvent(handle.Events),
	)
}

func (m *model) appendMessage(message Message) {
	m.messages = append(m.messages, message)
	m.refreshViewport()
}

func (m *model) toggleToolExpansion() {
	toolIDs := m.transcript.toolIDsForMessages(m.messages)
	if len(toolIDs) == 0 {
		return
	}
	targetID := m.focusedToolID
	if targetID == "" || m.transcript.nodeForID(targetID) == nil {
		targetID = toolIDs[len(toolIDs)-1]
		m.setFocusedTool(targetID)
	}
	if node := m.transcript.nodeForID(targetID); node != nil {
		node.Expanded = !node.Expanded
	}
	m.refreshViewport()
}

func (m *model) refreshViewport() {
	w, _ := m.viewportSize()
	content := renderTranscript(
		m.messages,
		m.transcript,
		m.spinner.View(),
		m.pendingApproval,
		m.approvalToggle,
		m.activeRunID,
		w,
	)
	if m.fileSuggest.active {
		overlay := m.fileSuggest.View(w)
		if overlay != "" {
			content += "\n\n" + overlay
		}
	} else if m.dirSuggest.active {
		overlay := m.dirSuggest.View(w)
		if overlay != "" {
			content += "\n\n" + overlay
		}
	} else if m.sessionSuggest.active {
		overlay := m.sessionOverlayView(w)
		if overlay != "" {
			content += "\n\n" + overlay
		}
	} else if m.slash.Active {
		overlay := m.slashOverlayView(w)
		if overlay != "" {
			content += "\n\n" + overlay
		}
	}
	m.viewport.SetContent(content)
	if m.followTranscript || m.viewport.PastBottom() {
		m.viewport.GotoBottom()
	}
}

func (m *model) syncLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	w, h := m.viewportSize()
	m.area.SetWidth(w)
	m.viewport.SetWidth(w)
	m.viewport.SetHeight(h)
	m.help.SetWidth(m.width)
}

func (m *model) helpStatus() string {
	base := ""
	if m.pendingApproval != nil {
		base = "waiting"
	} else if m.activeRunID == "" {
		base = "idle"
	} else {
		base = string(m.runStatus)
	}

	activeIngest := m.activeIngestCount()
	if activeIngest == 0 {
		return base
	}
	return base + " · ingest " + strconv.Itoa(activeIngest)
}

func toolName(pa *runtime.PendingApproval) string {
	if pa != nil && pa.ToolName != "" {
		return pa.ToolName
	}
	return "tool_call"
}

func toolArgs(pa *runtime.PendingApproval) string {
	if pa != nil && pa.Arguments != "" {
		return pa.Arguments
	}
	if pa != nil {
		return pa.Summary
	}
	return ""
}

func (m *model) scrollViewportPageUp() {
	m.viewport.PageUp()
	m.syncFollowTranscript()
}

func (m *model) scrollViewportPageDown() {
	m.viewport.PageDown()
	m.syncFollowTranscript()
}

func (m *model) syncFollowTranscript() {
	m.followTranscript = m.viewport.AtBottom()
}

func (m *model) syncDirSuggestFromArea() {
	value := strings.ReplaceAll(m.area.Value(), "\r\n", "\n")
	if !(value == "/ingest" || strings.HasPrefix(value, "/ingest ")) {
		m.dirSuggest.deactivate()
		return
	}

	query := strings.TrimSpace(strings.TrimPrefix(value, "/ingest"))
	atPos := len("/ingest ")
	if !m.dirSuggest.active {
		m.dirSuggest.activate(atPos)
	}
	m.dirSuggest.query = query
	m.dirSuggest.items = m.dirSuggest.filter(query)
	if len(m.dirSuggest.items) == 0 {
		m.dirSuggest.selected = 0
		m.dirSuggest.offset = 0
		return
	}
	if m.dirSuggest.selected >= len(m.dirSuggest.items) {
		m.dirSuggest.selected = len(m.dirSuggest.items) - 1
	}
	if m.dirSuggest.selected < 0 {
		m.dirSuggest.selected = 0
	}
	if m.dirSuggest.offset > m.dirSuggest.selected {
		m.dirSuggest.offset = m.dirSuggest.selected
	}
	if m.dirSuggest.selected >= m.dirSuggest.offset+maxVisible {
		m.dirSuggest.offset = m.dirSuggest.selected - maxVisible + 1
	}
}

func (m *model) focusNextTool() {
	m.moveToolFocus(1)
}

func (m *model) focusPrevTool() {
	m.moveToolFocus(-1)
}

func (m *model) moveToolFocus(delta int) {
	toolIDs := m.transcript.toolIDsForMessages(m.messages)
	if len(toolIDs) == 0 {
		return
	}

	index := -1
	for i, nodeID := range toolIDs {
		if nodeID == m.focusedToolID {
			index = i
			break
		}
	}

	switch {
	case index == -1 && delta >= 0:
		index = 0
	case index == -1 && delta < 0:
		index = len(toolIDs) - 1
	default:
		index = (index + delta + len(toolIDs)) % len(toolIDs)
	}

	m.setFocusedTool(toolIDs[index])
	m.refreshViewport()
}

func (m *model) setFocusedTool(nodeID string) {
	m.focusedToolID = nodeID
	m.transcript.setFocusedTool(nodeID)
}
