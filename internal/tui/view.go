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
	plan := m.planPanelView(w)
	body := m.viewport.View()
	bottomDivider := renderDivider(w)
	activity := m.activityDockView(w, m.spinner.View())

	var input string
	if m.pendingApproval != nil {
		input = ""
	} else {
		input = m.area.View()
	}

	helpBar := m.contextHelp()

	sections := []string{
		header,
		topDivider,
	}
	if plan != "" {
		sections = append(sections, plan, renderDivider(w))
	}
	sections = append(sections, body)
	if activity != "" {
		sections = append(sections, renderDivider(w), activity)
	}
	sections = append(sections, bottomDivider, input)
	sections = append(sections, helpBar)

	content := pageStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, sections...),
	)

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m *model) viewportSize() (int, int) {
	availableWidth := m.width - pageStyle.GetHorizontalFrameSize()

	headerHeight := 1
	dividerHeight := 2 // top + bottom
	if m.planPanelHeight(availableWidth) > 0 {
		dividerHeight++
	}
	planHeight := m.planPanelHeight(availableWidth)
	inputHeight := 3
	if m.pendingApproval != nil {
		inputHeight = 0
	}
	helpHeight := 1
	activityHeight := m.activityDockHeight()
	if activityHeight > 0 {
		activityHeight++
	}

	availableHeight := m.height -
		pageStyle.GetVerticalFrameSize() -
		headerHeight -
		dividerHeight -
		planHeight -
		inputHeight -
		helpHeight -
		activityHeight

	if availableWidth < 20 {
		availableWidth = 20
	}
	if availableHeight < 3 {
		availableHeight = 3
	}

	return availableWidth, availableHeight
}

func renderHeader(mode, status string, width int) string {
	left := brandStyle.Render("Sage")
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
		"  enter send · / commands · shift+enter newline · tab next tool · shift+tab prev tool · ctrl+e toggle tool · ctrl+c quit",
	)
}

func (m *model) planPanelView(width int) string {
	plan := m.currentPinnedPlan()
	if plan == nil || len(plan.Items) == 0 {
		return ""
	}

	lines := make([]string, 0, len(plan.Items))
	for _, item := range plan.Items {
		lines = append(lines, renderPlanLine(item, maxInt(20, width-4)))
	}

	block := planBlockStyle.Width(maxInt(24, width-2)).Render(strings.Join(lines, "\n"))
	block = injectBorderTitle(block, planTitleStyle.Render("Plan"), "")
	return indentBlock(block, " ")
}

func (m *model) planPanelHeight(width int) int {
	plan := m.currentPinnedPlan()
	if plan == nil || len(plan.Items) == 0 {
		return 0
	}
	return len(plan.Items) + 2
}

func (m *model) currentPinnedPlan() *runPlanState {
	if m.planStore == nil {
		return nil
	}
	if m.pinnedPlanRunID != "" {
		if plan := m.planStore.planFor(m.pinnedPlanRunID); plan != nil {
			return plan
		}
	}
	if m.activeRunID != "" {
		if plan := m.planStore.planFor(m.activeRunID); plan != nil {
			return plan
		}
	}
	if m.planStore.lastRunID != "" {
		return m.planStore.planFor(m.planStore.lastRunID)
	}
	return nil
}

func renderPlanLine(item planItem, width int) string {
	text := strings.TrimSpace(item.Text)
	if text == "" {
		text = "Untitled step"
	}

	switch item.Status {
	case planItemDone:
		return planDoneStyle.Width(width).Render("[✓] " + text)
	case planItemActive:
		return planActiveStyle.Width(width).Render("[●] " + text)
	default:
		return planQueuedStyle.Width(width).Render("[ ] " + text)
	}
}
