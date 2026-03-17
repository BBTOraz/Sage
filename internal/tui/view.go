package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *model) View() tea.View {
	w := m.width - pageStyle.GetHorizontalFrameSize()
	if w < 20 {
		w = 20
	}

	header := renderHeader(string(m.mode), m.helpStatus(), w)
	topDivider := renderDivider(w)
	body := m.viewport.View()
	bottomDivider := renderDivider(w)

	var input string
	if m.pendingApproval != nil {
		input = ""
	} else {
		input = m.area.View()
	}

	helpBar := m.contextHelp()

	content := pageStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			topDivider,
			body,
			bottomDivider,
			input,
			helpBar,
		),
	)

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m *model) viewportSize() (int, int) {
	availableWidth := m.width - pageStyle.GetHorizontalFrameSize()

	headerHeight := 1
	dividerHeight := 2 // top + bottom
	inputHeight := 3
	if m.pendingApproval != nil {
		inputHeight = 0
	}
	helpHeight := 1

	availableHeight := m.height -
		pageStyle.GetVerticalFrameSize() -
		headerHeight -
		dividerHeight -
		inputHeight -
		helpHeight

	if availableWidth < 20 {
		availableWidth = 20
	}
	if availableHeight < 3 {
		availableHeight = 3
	}

	return availableWidth, availableHeight
}

func renderHeader(mode, status string, width int) string {
	left := brandStyle.Render("DossierForge")
	right := headerModeStyle.Render(mode) +
		headerStatusStyle.Render(" · ") +
		headerStatusStyle.Render(status)

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}

	return fmt.Sprintf(" %s%s%s", left, strings.Repeat(" ", gap), right)
}

func renderDivider(width int) string {
	return dividerStyle.Render(strings.Repeat("─", width))
}

func (m *model) contextHelp() string {
	if m.pendingApproval != nil {
		return helpBarStyle.Render(
			"  ←/→ select · enter confirm · ctrl+c quit",
		)
	}
	return helpBarStyle.Render(
		"  enter send · shift+enter newline · ctrl+e expand tools · ctrl+c quit",
	)
}
