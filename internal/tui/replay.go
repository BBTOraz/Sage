package tui

import "bilge-lib/internal/runtime"

func (m *model) loadSession(session runtime.SessionSnapshot, events []runtime.Event) {
	m.mode = session.Mode
	m.sessionID = session.ID
	m.sessionState = session.State
	m.pendingApproval = nil
	m.activeRunID = ""
	m.runStatus = ""
	m.assistantDraft = ""
	m.focusedToolID = ""
	m.messages = nil
	m.transcript = newTranscriptTree()
	m.planStore = newPlanStore()
	m.pinnedPlanRunID = ""
	m.followTranscript = true

	seenRuns := make(map[runtime.RunID]struct{})
	for _, event := range events {
		if event.RunID != "" {
			if _, ok := seenRuns[event.RunID]; !ok {
				m.messages = append(m.messages, Message{
					Kind:  MessageRun,
					RunID: event.RunID,
				})
				seenRuns[event.RunID] = struct{}{}
			}
		}

		m.transcript.ApplyEvent(event)
		m.planStore.ApplyEvent(event)
		if m.planStore.planFor(event.RunID) != nil {
			m.pinnedPlanRunID = event.RunID
		}
		m.runStatus = event.Status

		switch event.Type {
		case runtime.EventRunInterrupted:
			m.pendingApproval = event.Approval
			m.activeRunID = ""
		case runtime.EventRunCompleted, runtime.EventRunFailed:
			if m.activeRunID == event.RunID {
				m.activeRunID = ""
			}
		default:
			if event.RunID == session.ActiveRunID && event.Status == runtime.RunStatusRunning {
				m.activeRunID = event.RunID
			}
		}
	}

	if m.pendingApproval != nil {
		m.area.Blur()
	} else {
		m.area.Focus()
	}

	m.syncLayout()
	m.refreshViewport()
}
