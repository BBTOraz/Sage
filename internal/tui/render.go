package tui

import (
	"bilge-lib/internal/runtime"
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

const streamingCursor = "▌"

func renderUserMessage(text string, width int) string {
	label := userLabelStyle.Render(">")
	body := userTextStyle.Width(width - 6).Render(text)
	return prefixBlock(label, body, "    ")
}

func renderAgentMessage(text string, width int) string {
	label := agentLabelStyle.Render("◆")
	rendered := renderMarkdown(text, width-6)
	return prefixBlock(label, rendered, "    ")
}

func renderAgentStreaming(text string, width int) string {
	label := agentLabelStyle.Render("◆")
	body := agentTextStyle.Width(width-6).Render(text) +
		streamingCursorStyle.Render(streamingCursor)
	return prefixBlock(label, body, "    ")
}

func renderSystemMessage(text string, width int) string {
	label := systemLabelStyle.Render("~")
	body := systemTextStyle.Width(width - 6).Render(text)
	return prefixBlock(label, body, "    ")
}

func renderErrorMessage(text string, width int) string {
	label := errorLabelStyle.Render("!")
	body := errorTextStyle.Width(width - 6).Render(text)
	return prefixBlock(label, body, "    ")
}

// prefixBlock renders a labeled block: first line gets "  label body",
// subsequent lines get "  indent body" so everything aligns.
func prefixBlock(label, body, indent string) string {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 {
		return "  " + label + " "
	}
	result := make([]string, len(lines))
	result[0] = "  " + label + " " + lines[0]
	for i := 1; i < len(lines); i++ {
		result[i] = indent + lines[i]
	}
	return strings.Join(result, "\n")
}

func renderToolBlock(name, args, status string, expanded bool, width int) string {
	blockWidth := width - 4
	if blockWidth < 20 {
		blockWidth = 20
	}

	var badge string
	switch status {
	case "approved":
		badge = toolApprovedBadge.Render("✓ approved")
	case "denied":
		badge = toolDeniedBadge.Render("✗ denied")
	}

	if !expanded {
		// Collapsed: just the border frame with title and badge, no inner content
		hint := toolArgsStyle.Render("  ▸ press enter to expand")
		block := toolBlockStyle.
			Width(blockWidth).
			Render(hint)
		block = injectBorderTitle(block, toolNameStyle.Render(name), badge)
		return indentBlock(block, "  ")
	}

	// Expanded: show full args
	content := toolArgsStyle.Render(formatToolArgs(args))

	block := toolBlockStyle.
		Width(blockWidth).
		Render(content)

	block = injectBorderTitle(block, toolNameStyle.Render(name), badge)
	return indentBlock(block, "  ")
}

func renderToolBlockWithApproval(pa *runtime.PendingApproval, toggle approvalToggle, width int) string {
	blockWidth := width - 4
	if blockWidth < 20 {
		blockWidth = 20
	}

	name := pa.ToolName
	if name == "" {
		name = "tool_call"
	}
	args := pa.Arguments
	if args == "" {
		args = pa.Summary
	}

	content := toolArgsStyle.Render(formatToolArgs(args))

	prompt := "\n" + approvalPromptStyle.Render("Allow this tool call?") +
		"\n" + toggle.View()

	inner := content + prompt

	block := toolBlockStyle.
		Width(blockWidth).
		Render(inner)

	block = injectBorderTitle(block, toolNameStyle.Render(name), "")
	return indentBlock(block, "  ")
}

func renderSpinnerLine(spinnerView string) string {
	return "  " + spinnerView + spinnerLabelStyle.Render(" Thinking...")
}

func renderTranscript(
	msgs []Message,
	draft string,
	spinnerView string,
	pending *runtime.PendingApproval,
	toggle approvalToggle,
	isProcessing bool,
	width int,
) string {
	if len(msgs) == 0 && draft == "" && pending == nil && !isProcessing {
		return helpBarStyle.Render("  No messages yet. Type a message and press enter.")
	}

	var sections []string

	for _, msg := range msgs {
		switch msg.Kind {
		case MessageUser:
			sections = append(sections, renderUserMessage(msg.Text, width))
		case MessageAgent:
			sections = append(sections, renderAgentMessage(msg.Text, width))
		case MessageSystem:
			sections = append(sections, renderSystemMessage(msg.Text, width))
		case MessageError:
			sections = append(sections, renderErrorMessage(msg.Text, width))
		case MessageTool:
			sections = append(sections, renderToolBlock(
				msg.ToolName, msg.ToolArgs, msg.ToolStatus, msg.Expanded, width,
			))
		}
	}

	if pending != nil {
		sections = append(sections, renderToolBlockWithApproval(pending, toggle, width))
	}

	if strings.TrimSpace(draft) != "" {
		sections = append(sections, renderAgentStreaming(draft, width))
	}

	if isProcessing && strings.TrimSpace(draft) == "" && pending == nil {
		sections = append(sections, renderSpinnerLine(spinnerView))
	}

	return strings.Join(sections, "\n\n")
}

func renderMarkdown(text string, width int) string {
	if width < 10 {
		width = 10
	}

	var result []string
	lines := strings.Split(text, "\n")
	inCodeBlock := false
	codeLang := ""
	var codeLines []string

	boldStyle := lipgloss.NewStyle().Bold(true).Foreground(colorBright)
	italicStyle := lipgloss.NewStyle().Italic(true).Foreground(colorText)
	codeInlineStyle := lipgloss.NewStyle().Foreground(colorSystem)
	codeFenceStyle := lipgloss.NewStyle().
		Foreground(colorFaint).
		PaddingLeft(1)
	codeLangStyle := lipgloss.NewStyle().Foreground(colorTool).Italic(true)
	headingStyle := lipgloss.NewStyle().Bold(true).Foreground(colorBright)

	reBold := regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic := regexp.MustCompile(`\*(.+?)\*`)
	reInlineCode := regexp.MustCompile("`([^`]+)`")

	for _, line := range lines {
		// Code fence toggle
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "```"))
				codeLines = nil
				continue
			}
			// Close code block
			inCodeBlock = false
			header := ""
			if codeLang != "" {
				header = codeLangStyle.Render(codeLang) + "\n"
			}
			code := codeFenceStyle.Width(width).Render(strings.Join(codeLines, "\n"))
			result = append(result, header+code)
			codeLang = ""
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Headings
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			result = append(result, headingStyle.Width(width).Render(strings.TrimPrefix(trimmed, "### ")))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			result = append(result, headingStyle.Width(width).Render(strings.TrimPrefix(trimmed, "## ")))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			result = append(result, headingStyle.Width(width).Render(strings.TrimPrefix(trimmed, "# ")))
			continue
		}

		// Inline formatting: bold, italic, inline code
		processed := line
		processed = reInlineCode.ReplaceAllStringFunc(processed, func(m string) string {
			inner := reInlineCode.FindStringSubmatch(m)
			if len(inner) > 1 {
				return codeInlineStyle.Render(inner[1])
			}
			return m
		})
		processed = reBold.ReplaceAllStringFunc(processed, func(m string) string {
			inner := reBold.FindStringSubmatch(m)
			if len(inner) > 1 {
				return boldStyle.Render(inner[1])
			}
			return m
		})
		processed = reItalic.ReplaceAllStringFunc(processed, func(m string) string {
			inner := reItalic.FindStringSubmatch(m)
			if len(inner) > 1 {
				return italicStyle.Render(inner[1])
			}
			return m
		})

		result = append(result, agentTextStyle.Width(width).Render(processed))
	}

	// Handle unclosed code block
	if inCodeBlock && len(codeLines) > 0 {
		code := codeFenceStyle.Width(width).Render(strings.Join(codeLines, "\n"))
		result = append(result, code)
	}

	return strings.Join(result, "\n")
}

func formatToolArgs(args string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(args), "", "  "); err == nil {
		return buf.String()
	}
	return args
}

func indentBlock(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func injectBorderTitle(block, title, badge string) string {
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return block
	}

	topRunes := []rune(lines[0])
	if len(topRunes) < 4 {
		return block
	}

	// Build: ╭─ title ──...── badge ─╮
	prefix := string(topRunes[0]) + "─ " + title + " "
	suffix := " " + string(topRunes[len(topRunes)-1])

	prefixWidth := runeWidth(prefix)
	suffixWidth := runeWidth(suffix)

	var badgeStr string
	if badge != "" {
		badgeStr = badge + " "
	}
	badgeWidth := runeWidth(badgeStr)

	fillLen := runeWidth(string(topRunes)) - prefixWidth - suffixWidth - badgeWidth
	if fillLen < 1 {
		fillLen = 1
	}

	newTop := prefix + strings.Repeat("─", fillLen) + badgeStr + suffix
	lines[0] = newTop
	return strings.Join(lines, "\n")
}

func runeWidth(s string) int {
	// Strip ANSI escape sequences for width calculation
	n := 0
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}
