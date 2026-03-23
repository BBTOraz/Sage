package tui

import (
	"bilge-lib/internal/runtime"
	"fmt"
	"strings"
	"time"
)

type sessionOptionKind string

const (
	sessionOptionNew    sessionOptionKind = "new"
	sessionOptionStored sessionOptionKind = "stored"
)

type sessionOption struct {
	Kind    sessionOptionKind
	Summary runtime.SessionSummary
}

type sessionSuggestState struct {
	active   bool
	query    string
	all      []sessionOption
	matches  []sessionOption
	selected int
	offset   int
}

func (m *model) syncSessionSuggestFromArea() {
	value := strings.ReplaceAll(m.area.Value(), "\r\n", "\n")
	if !strings.HasPrefix(value, "/session ") {
		m.sessionSuggest.deactivate()
		return
	}

	query := strings.TrimSpace(strings.TrimPrefix(value, "/session "))
	if !m.sessionSuggest.active {
		options, err := m.sessionOptions()
		if err != nil {
			m.sessionSuggest.deactivate()
			m.addActivityNotice("failed", "Session", err.Error())
			return
		}
		m.sessionSuggest.activate(options)
	}
	m.sessionSuggest.applyQuery(query)
}

func (m *model) sessionOptions() ([]sessionOption, error) {
	options := []sessionOption{{Kind: sessionOptionNew}}
	if m.manager == nil {
		return options, nil
	}

	summaries, err := m.manager.ListSessions(m.ctx)
	if err != nil {
		return nil, err
	}

	for _, summary := range summaries {
		options = append(options, sessionOption{
			Kind:    sessionOptionStored,
			Summary: summary,
		})
	}

	return options, nil
}

func (s *sessionSuggestState) activate(options []sessionOption) {
	s.active = true
	s.all = options
	s.selected = 0
	for i, option := range options {
		if option.Kind == sessionOptionStored {
			s.selected = i
			break
		}
	}
	s.offset = 0
	s.applyQuery("")
}

func (s *sessionSuggestState) deactivate() {
	s.active = false
	s.query = ""
	s.all = nil
	s.matches = nil
	s.selected = 0
	s.offset = 0
}

func (s *sessionSuggestState) applyQuery(query string) {
	s.query = query
	q := strings.ToLower(strings.TrimSpace(query))

	s.matches = s.matches[:0]
	for _, option := range s.all {
		if option.Kind == sessionOptionNew {
			s.matches = append(s.matches, option)
			continue
		}
		if q == "" || strings.Contains(strings.ToLower(sessionSearchText(option)), q) {
			s.matches = append(s.matches, option)
		}
	}

	if len(s.matches) == 0 {
		s.selected = 0
		s.offset = 0
		return
	}
	if s.selected >= len(s.matches) {
		s.selected = len(s.matches) - 1
	}
	if s.selected < 0 {
		s.selected = 0
	}
	if s.offset > s.selected {
		s.offset = s.selected
	}
	if s.selected >= s.offset+maxVisible {
		s.offset = s.selected - maxVisible + 1
	}
}

func (s *sessionSuggestState) moveUp() {
	if s.selected > 0 {
		s.selected--
		if s.selected < s.offset {
			s.offset = s.selected
		}
	}
}

func (s *sessionSuggestState) moveDown() {
	if s.selected < len(s.matches)-1 {
		s.selected++
		if s.selected >= s.offset+maxVisible {
			s.offset = s.selected - maxVisible + 1
		}
	}
}

func (s *sessionSuggestState) selectedOption() (sessionOption, bool) {
	if !s.active || len(s.matches) == 0 {
		return sessionOption{}, false
	}

	index := s.selected
	if index < 0 || index >= len(s.matches) {
		index = 0
	}
	return s.matches[index], true
}

func (m *model) sessionOverlayView(width int) string {
	if !m.sessionSuggest.active || len(m.sessionSuggest.matches) == 0 {
		return ""
	}

	items := make([]overlayItem, 0, len(m.sessionSuggest.matches))
	for _, option := range m.sessionSuggest.matches {
		items = append(items, overlayItem{
			Primary:   sessionOptionPrimary(option, m.sessionID),
			Secondary: sessionOptionSecondary(option, m.sessionID),
		})
	}

	return renderSelectionOverlay(selectionOverlayConfig{
		Title:     "Sessions",
		QueryIcon: "/",
		Query:     m.sessionSuggest.query,
		Items:     items,
		Selected:  m.sessionSuggest.selected,
		Offset:    m.sessionSuggest.offset,
		Width:     width,
		Accent:    colorBrand,
	})
}

func (m *model) chooseSelectedSession() error {
	if m.activeRunID != "" {
		return fmt.Errorf("finish the current run before switching sessions")
	}

	option, ok := m.sessionSuggest.selectedOption()
	if !ok {
		return fmt.Errorf("no session selected")
	}

	switch option.Kind {
	case sessionOptionNew:
		session := m.manager.StartNewSession()
		m.loadSession(session, nil)
		m.addActivityNotice("info", "Session", "started a new chat")
	case sessionOptionStored:
		session, events, err := m.manager.RestoreSession(m.ctx, option.Summary.ID)
		if err != nil {
			return err
		}
		m.loadSession(session, events)
		m.addActivityNotice("info", "Session", "switched to "+sessionDisplayTitle(option.Summary))
	default:
		return fmt.Errorf("unsupported session selection")
	}

	m.area.SetValue("")
	m.updateSlashState()
	m.refreshViewport()
	return nil
}

func sessionSearchText(option sessionOption) string {
	if option.Kind == sessionOptionNew {
		return "new chat fresh session"
	}
	return strings.Join([]string{
		option.Summary.Title,
		string(option.Summary.ID),
		option.Summary.Preview,
	}, " ")
}

func sessionOptionPrimary(option sessionOption, currentID runtime.SessionID) string {
	if option.Kind == sessionOptionNew {
		return "New chat"
	}

	title := sessionDisplayTitle(option.Summary)
	if option.Summary.ID == currentID {
		return title + "  current"
	}
	return title
}

func sessionOptionSecondary(option sessionOption, currentID runtime.SessionID) string {
	if option.Kind == sessionOptionNew {
		return "Start a fresh empty session"
	}

	parts := make([]string, 0, 3)
	if option.Summary.UpdatedAt.IsZero() {
		parts = append(parts, "history")
	} else {
		parts = append(parts, formatSessionTime(option.Summary.UpdatedAt))
	}
	if option.Summary.State != "" && option.Summary.State != runtime.SessionStateIdle {
		parts = append(parts, string(option.Summary.State))
	}
	if option.Summary.ID == currentID {
		parts = append(parts, "open now")
	}

	preview := strings.TrimSpace(option.Summary.Preview)
	if preview == "" {
		preview = "No messages yet"
	}
	parts = append(parts, preview)
	return strings.Join(parts, " · ")
}

func sessionDisplayTitle(summary runtime.SessionSummary) string {
	if strings.TrimSpace(summary.Title) != "" {
		return summary.Title
	}

	id := string(summary.ID)
	if len(id) > 8 {
		id = id[:8]
	}
	return "Session " + id
}

func formatSessionTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	local := value.In(time.Local)
	now := time.Now().In(time.Local)
	if local.YearDay() == now.YearDay() && local.Year() == now.Year() {
		return "Today " + local.Format("15:04")
	}
	if local.Year() == now.Year() {
		return local.Format("Jan 02 15:04")
	}
	return local.Format("2006-01-02 15:04")
}
