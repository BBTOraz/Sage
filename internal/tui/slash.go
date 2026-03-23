package tui

import (
	"strings"
)

type slashCommand struct {
	Name        string
	Usage       string
	Description string
}

type slashState struct {
	Active   bool
	Matches  []slashCommand
	Selected int
	Offset   int
}

var availableSlashCommands = []slashCommand{
	{
		Name:        "ingest",
		Usage:       "/ingest <directory>",
		Description: "index documents from a directory in background",
	},
	{
		Name:        "session",
		Usage:       "/session",
		Description: "open the session picker and switch chats",
	},
}

func (m *model) updateSlashState() {
	m.syncDirSuggestFromArea()
	m.syncSessionSuggestFromArea()

	value := strings.TrimSpace(m.area.Value())
	if !strings.HasPrefix(value, "/") {
		m.slash = slashState{}
		return
	}

	if strings.Contains(value, " ") {
		m.slash = slashState{}
		return
	}

	query := strings.TrimPrefix(value, "/")
	matches := make([]slashCommand, 0, len(availableSlashCommands))
	for _, cmd := range availableSlashCommands {
		if query == "" || strings.HasPrefix(cmd.Name, query) {
			matches = append(matches, cmd)
		}
	}

	if len(matches) == 0 {
		m.slash = slashState{}
		return
	}

	selected := m.slash.Selected
	offset := m.slash.Offset
	if selected >= len(matches) {
		selected = len(matches) - 1
	}
	if selected < 0 {
		selected = 0
	}
	if offset > selected {
		offset = selected
	}
	if selected >= offset+maxVisible {
		offset = selected - maxVisible + 1
	}

	m.slash.Active = true
	m.slash.Matches = matches
	m.slash.Selected = selected
	m.slash.Offset = offset
}

func (m *model) moveSlashUp() {
	if m.slash.Selected > 0 {
		m.slash.Selected--
		if m.slash.Selected < m.slash.Offset {
			m.slash.Offset = m.slash.Selected
		}
	}
}

func (m *model) moveSlashDown() {
	if m.slash.Selected < len(m.slash.Matches)-1 {
		m.slash.Selected++
		if m.slash.Selected >= m.slash.Offset+maxVisible {
			m.slash.Offset = m.slash.Selected - maxVisible + 1
		}
	}
}

func (m *model) selectedSlashCommand() (slashCommand, bool) {
	if !m.slash.Active || len(m.slash.Matches) == 0 {
		return slashCommand{}, false
	}
	index := m.slash.Selected
	if index < 0 || index >= len(m.slash.Matches) {
		index = 0
	}
	return m.slash.Matches[index], true
}

func (m *model) slashOverlayView(width int) string {
	if !m.slash.Active || len(m.slash.Matches) == 0 {
		return ""
	}

	items := make([]overlayItem, 0, len(m.slash.Matches))
	for _, cmd := range m.slash.Matches {
		items = append(items, overlayItem{
			Primary:   cmd.Usage,
			Secondary: cmd.Description,
		})
	}

	return renderSelectionOverlay(selectionOverlayConfig{
		Title:     "Commands",
		QueryIcon: "/",
		Query:     strings.TrimPrefix(strings.TrimSpace(m.area.Value()), "/"),
		Items:     items,
		Selected:  m.slash.Selected,
		Offset:    m.slash.Offset,
		Width:     width,
		Accent:    colorBrand,
	})
}

func resolveSlashCommand(value string) (slashCommand, string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "/") {
		return slashCommand{}, "", false
	}

	parts := strings.Fields(value)
	if len(parts) == 0 {
		return slashCommand{}, "", false
	}

	token := strings.TrimPrefix(parts[0], "/")
	for _, cmd := range availableSlashCommands {
		if cmd.Name == token {
			args := ""
			if len(parts) > 1 {
				args = strings.TrimSpace(strings.Join(parts[1:], " "))
			}
			return cmd, args, true
		}
	}

	return slashCommand{}, "", false
}

func autocompleteSlashCommand(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "/") || strings.Contains(value, " ") {
		return "", false
	}

	query := strings.TrimPrefix(value, "/")
	var match *slashCommand
	for i := range availableSlashCommands {
		cmd := &availableSlashCommands[i]
		if query == "" || strings.HasPrefix(cmd.Name, query) {
			if match != nil {
				return "", false
			}
			match = cmd
		}
	}
	if match == nil {
		return "", false
	}

	return "/" + match.Name + " ", true
}
