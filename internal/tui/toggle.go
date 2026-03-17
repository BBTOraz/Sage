package tui

import "charm.land/lipgloss/v2"

const (
	toggleApprove = 0
	toggleDeny    = 1
)

type approvalToggle struct {
	selected int
}

func newApprovalToggle() approvalToggle {
	return approvalToggle{selected: toggleApprove}
}

func (t *approvalToggle) moveLeft() {
	t.selected = toggleApprove
}

func (t *approvalToggle) moveRight() {
	t.selected = toggleDeny
}

func (t *approvalToggle) isApprove() bool {
	return t.selected == toggleApprove
}

func (t approvalToggle) View() string {
	selectedApprove := lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
	selectedDeny := lipgloss.NewStyle().Bold(true).Foreground(colorError)
	unselected := lipgloss.NewStyle().Foreground(colorFaint)

	var approve, deny string
	if t.selected == toggleApprove {
		approve = selectedApprove.Render("► [Approve]")
		deny = unselected.Render("  Deny  ")
	} else {
		approve = unselected.Render("  Approve  ")
		deny = selectedDeny.Render("► [Deny]")
	}

	return approve + "     " + deny
}
