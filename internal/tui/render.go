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
	return indentBlock(renderToolBlockBody(name, args, "", status, expanded, false, width), "  ")
}

func renderToolBlockBody(name, args, result, status string, expanded bool, focused bool, width int) string {
	blockWidth := width - 4
	if blockWidth < 20 {
		blockWidth = 20
	}

	blockStyle := toolBlockStyle
	if focused {
		blockStyle = toolFocusedBlockStyle
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
		block := blockStyle.
			Width(blockWidth).
			Render(hint)
		block = injectBorderTitle(block, toolNameStyle.Render(name), badge)
		return indentBlock(block, "  ")
	}

	// Expanded: show full args and latest tool result.
	var sections []string
	if strings.TrimSpace(args) != "" {
		sections = append(sections, toolArgsStyle.Render("args:\n"+formatToolArgs(args)))
	}
	if strings.TrimSpace(result) != "" {
		sections = append(sections, toolResultStyle.Render("result:\n"+result))
	}
	if len(sections) == 0 {
		sections = append(sections, toolArgsStyle.Render("(no payload)"))
	}
	content := strings.Join(sections, "\n\n")

	block := blockStyle.
		Width(blockWidth).
		Render(content)

	block = injectBorderTitle(block, toolNameStyle.Render(name), badge)
	return block
}

func renderToolBlockWithApproval(pa *runtime.PendingApproval, toggle approvalToggle, width int) string {
	return indentBlock(renderToolBlockWithApprovalBody(pa, toggle, width), "  ")
}

func renderToolBlockWithApprovalBody(pa *runtime.PendingApproval, toggle approvalToggle, width int) string {
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
	return block
}

func renderSpinnerLine(spinnerView string) string {
	return "  " + spinnerView + spinnerLabelStyle.Render(" Thinking...")
}

func renderTranscript(
	msgs []Message,
	tree *transcriptTree,
	spinnerView string,
	pending *runtime.PendingApproval,
	toggle approvalToggle,
	activeRunID runtime.RunID,
	width int,
) string {
	if len(msgs) == 0 && (tree == nil || len(tree.nodes) == 0) && pending == nil && activeRunID == "" {
		return helpBarStyle.Render("  No messages yet. Type a message and press enter.")
	}

	var sections []string
	pendingRendered := false

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
		case MessageRun:
			block, renderedPending := renderRunTranscript(tree, msg.RunID, activeRunID, spinnerView, pending, toggle, width)
			if block != "" {
				sections = append(sections, block)
			}
			if renderedPending {
				pendingRendered = true
			}
		}
	}

	if pending != nil && !pendingRendered {
		sections = append(sections, renderToolBlockWithApproval(pending, toggle, width))
	}

	if activeRunID != "" && !hasRunAnchor(msgs, activeRunID) {
		sections = append(sections, renderSpinnerLine(spinnerView))
	}

	return strings.Join(sections, "\n\n")
}

func renderRunTranscript(
	tree *transcriptTree,
	runID runtime.RunID,
	activeRunID runtime.RunID,
	spinnerView string,
	pending *runtime.PendingApproval,
	toggle approvalToggle,
	width int,
) (string, bool) {
	if tree == nil {
		if activeRunID == runID {
			return renderSpinnerLine(spinnerView), false
		}
		return "", false
	}

	roots := tree.runRoots(runID)
	if len(roots) == 0 {
		if activeRunID == runID {
			return renderSpinnerLine(spinnerView), false
		}
		return "", false
	}

	blocks := make([]visibleRunBlock, 0, len(roots))
	for _, rootID := range roots {
		collectVisibleRunBlocks(tree, rootID, "", &blocks)
	}
	if len(blocks) == 0 {
		if activeRunID == runID {
			return renderSpinnerLine(spinnerView), false
		}
		return "", false
	}

	sections := make([]string, 0, len(blocks))
	renderedPending := false
	for i, block := range blocks {
		node := tree.nodeForID(block.NodeID)
		if node == nil {
			continue
		}

		switch block.Kind {
		case transcriptNodeAgent:
			isActiveLeaf := node.ID == tree.lastAgentByRun[activeRunID] && node.Status == string(runtime.RunStatusRunning)
			sections = append(sections, renderAgentTreeBlock(node, block.DisplayName, isActiveLeaf, spinnerView, width))
		case transcriptNodeTool:
			isLastInGroup := i == len(blocks)-1 ||
				blocks[i+1].Kind != transcriptNodeTool ||
				blocks[i+1].ParentVisibleAgentID != block.ParentVisibleAgentID
			toolOutput, toolRenderedPending := renderVisibleToolBlock(node, pending, toggle, width, isLastInGroup)
			if toolOutput != "" {
				sections = append(sections, toolOutput)
			}
			renderedPending = renderedPending || toolRenderedPending
		}
	}

	return strings.Join(sections, "\n"), renderedPending
}

type visibleRunBlock struct {
	Kind                 transcriptNodeKind
	NodeID               string
	DisplayName          string
	ParentVisibleAgentID string
}

func collectVisibleRunBlocks(tree *transcriptTree, nodeID string, visibleAgentID string, out *[]visibleRunBlock) {
	node := tree.nodeForID(nodeID)
	if node == nil {
		return
	}

	switch node.Kind {
	case transcriptNodeAgent:
		nextVisibleAgentID := visibleAgentID
		if displayName, ok := userFacingAgentName(node); ok {
			*out = append(*out, visibleRunBlock{
				Kind:        transcriptNodeAgent,
				NodeID:      node.ID,
				DisplayName: displayName,
			})
			nextVisibleAgentID = node.ID
		}
		for _, childID := range node.Children {
			collectVisibleRunBlocks(tree, childID, nextVisibleAgentID, out)
		}
	case transcriptNodeTool:
		if visibleAgentID == "" || hideToolFromTranscript(node) {
			return
		}
		*out = append(*out, visibleRunBlock{
			Kind:                 transcriptNodeTool,
			NodeID:               node.ID,
			ParentVisibleAgentID: visibleAgentID,
		})
	}
}

func renderAgentTreeBlock(node *transcriptNode, displayName string, active bool, spinnerView string, width int) string {
	if width < 10 {
		width = 10
	}

	body := agentNameStyle.Render(displayName)
	if strings.TrimSpace(node.Summary) != "" {
		body += "\n" + renderMarkdown(node.Summary, width-6)
	} else if active {
		body += "\n" + spinnerView + spinnerLabelStyle.Render(" thinking…")
	}

	return prefixBlock(agentLabelStyle.Render("A"), body, "    ")
}

func renderVisibleToolBlock(
	node *transcriptNode,
	pending *runtime.PendingApproval,
	toggle approvalToggle,
	width int,
	isLast bool,
) (string, bool) {
	firstPrefix := "   " + treeGuideStyle.Render("├─ ")
	continuePrefix := "   " + treeGuideStyle.Render("│  ")
	if isLast {
		firstPrefix = "   " + treeGuideStyle.Render("└─ ")
		continuePrefix = "      "
	}

	blockWidth := width - runeWidth(firstPrefix)
	if blockWidth < 20 {
		blockWidth = 20
	}

	if node.Approval != nil && pending != nil && pending.RunID == node.Approval.RunID {
		block := renderToolBlockWithApprovalBody(node.Approval, toggle, blockWidth)
		return prefixTreeBlock(block, firstPrefix, continuePrefix), true
	}

	block := renderToolBlockBody(node.Title, node.Summary, node.Result, node.Status, node.Expanded, node.Focused, blockWidth)
	return prefixTreeBlock(block, firstPrefix, continuePrefix), false
}

func userFacingAgentName(node *transcriptNode) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(node.AgentName)) {
	case "sage":
		return "Sage", true
	default:
		return "", false
	}
}

func hideToolFromTranscript(node *transcriptNode) bool {
	switch strings.ToLower(strings.TrimSpace(node.Title)) {
	case "plan", "respond", "write_todos":
		return true
	default:
		return false
	}
}

func prefixTreeBlock(block, firstPrefix, continuePrefix string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		prefix := continuePrefix
		if i == 0 {
			prefix = firstPrefix
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func hasRunAnchor(msgs []Message, runID runtime.RunID) bool {
	for _, msg := range msgs {
		if msg.Kind == MessageRun && msg.RunID == runID {
			return true
		}
	}
	return false
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
