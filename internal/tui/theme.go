package tui

import "charm.land/lipgloss/v2"

// ── Color Palette (One Dark Pro inspired) ──────────────────

var (
	colorBrand   = lipgloss.Color("#7B68EE")
	colorUser    = lipgloss.Color("#5B9BD5")
	colorAgent   = lipgloss.Color("#98C379")
	colorSystem  = lipgloss.Color("#E5C07B")
	colorTool    = lipgloss.Color("#C678DD")
	colorError   = lipgloss.Color("#E06C75")
	colorSuccess = lipgloss.Color("#98C379")
	colorBorder  = lipgloss.Color("#3E4451")
	colorFaint   = lipgloss.Color("#5C6370")
	colorText    = lipgloss.Color("#ABB2BF")
	colorBright  = lipgloss.Color("#E0E0E0")
)

// ── Header ─────────────────────────────────────────────────

var (
	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrand)

	headerModeStyle = lipgloss.NewStyle().
			Foreground(colorSystem)

	headerStatusStyle = lipgloss.NewStyle().
				Foreground(colorFaint)
)

// ── Divider ────────────────────────────────────────────────

var dividerStyle = lipgloss.NewStyle().
	Foreground(colorBorder)

// ── Message Labels ─────────────────────────────────────────

var (
	userLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorUser)

	agentLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAgent)

	systemLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSystem)

	errorLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError)
)

// ── Message Text ───────────────────────────────────────────

var (
	userTextStyle = lipgloss.NewStyle().
			Foreground(colorBright)

	agentTextStyle = lipgloss.NewStyle().
			Foreground(colorText)

	systemTextStyle = lipgloss.NewStyle().
			Foreground(colorSystem).
			Italic(true)

	errorTextStyle = lipgloss.NewStyle().
			Foreground(colorError)
)

// ── Tool Call Block ────────────────────────────────────────

var (
	toolBlockStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorTool).
			Padding(0, 1)

	toolNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTool)

	toolArgsStyle = lipgloss.NewStyle().
			Foreground(colorFaint)

	toolApprovedBadge = lipgloss.NewStyle().
				Foreground(colorSuccess)

	toolDeniedBadge = lipgloss.NewStyle().
			Foreground(colorError)
)

// ── Approval Panel ─────────────────────────────────────────

var (
	approveKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSuccess)

	denyKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError)

	approvalPromptStyle = lipgloss.NewStyle().
				Foreground(colorText)
)

// ── Spinner / Streaming ────────────────────────────────────

var (
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorAgent)

	spinnerLabelStyle = lipgloss.NewStyle().
				Foreground(colorFaint)

	streamingCursorStyle = lipgloss.NewStyle().
				Foreground(colorAgent).
				Bold(true)
)

// ── Help Bar ───────────────────────────────────────────────

var helpBarStyle = lipgloss.NewStyle().
	Foreground(colorFaint)

// ── Activity / Commands ────────────────────────────────────

var (
	activityBlockStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(0, 1)

	activityTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSystem)

	activityTextStyle = lipgloss.NewStyle().
				Foreground(colorText)

	activityQueuedStyle = lipgloss.NewStyle().
				Foreground(colorSystem)

	activityRunningStyle = lipgloss.NewStyle().
				Foreground(colorAgent)

	activitySuccessStyle = lipgloss.NewStyle().
				Foreground(colorSuccess)

	activityErrorStyle = lipgloss.NewStyle().
				Foreground(colorError)

	activityInfoStyle = lipgloss.NewStyle().
				Foreground(colorFaint)

	slashBlockStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBrand).
			Padding(0, 1)

	slashTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrand)

	slashNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBright)

	slashDescStyle = lipgloss.NewStyle().
			Foreground(colorFaint)
)

// ── Page / Layout ──────────────────────────────────────────

var pageStyle = lipgloss.NewStyle().
	Padding(0, 1)
