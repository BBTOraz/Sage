package tui

import (
	"bilge-lib/internal/runtime"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type activityEntry struct {
	ID        string
	Title     string
	Detail    string
	Status    string
	UpdatedAt time.Time
}

func (m *model) upsertIngestActivity(event runtime.IngestEvent) {
	title := event.Root
	if title == "" {
		title = "ingest"
	}

	detail := ingestEventDetail(event)
	entry := activityEntry{
		ID:        string(event.JobID),
		Title:     title,
		Detail:    detail,
		Status:    string(event.Status),
		UpdatedAt: event.OccurredAt,
	}

	m.upsertActivity(entry)
}

func (m *model) addActivityNotice(status, title, detail string) {
	m.upsertActivity(activityEntry{
		ID:        fmt.Sprintf("notice:%d", time.Now().UnixNano()),
		Title:     title,
		Detail:    detail,
		Status:    status,
		UpdatedAt: time.Now(),
	})
}

func (m *model) upsertActivity(entry activityEntry) {
	if entry.Title == "" {
		entry.Title = "activity"
	}

	index := -1
	for i := range m.activities {
		if m.activities[i].ID == entry.ID {
			index = i
			break
		}
	}

	if index >= 0 {
		m.activities[index] = entry
		m.activities = append(append([]activityEntry{entry}, m.activities[:index]...), m.activities[index+1:]...)
	} else {
		m.activities = append([]activityEntry{entry}, m.activities...)
	}

	if len(m.activities) > 8 {
		m.activities = m.activities[:8]
	}

	m.syncLayout()
}

func ingestEventDetail(event runtime.IngestEvent) string {
	switch event.Type {
	case runtime.EventIngestQueued:
		return "queued for background indexing"
	case runtime.EventIngestStarted:
		return "indexing documents in background"
	case runtime.EventIngestCompleted:
		if event.Report == nil {
			return "documents ready"
		}

		skipped := 0
		failed := 0
		for _, file := range event.Report.Files {
			switch file.Status {
			case "skipped":
				skipped++
			case "failed":
				failed++
			}
		}

		parts := []string{
			fmt.Sprintf("ready: %d indexed", event.Report.IndexedDocuments),
			fmt.Sprintf("%d chunks", event.Report.IndexedChunks),
		}
		if skipped > 0 {
			parts = append(parts, fmt.Sprintf("%d skipped", skipped))
		}
		if failed > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", failed))
		}
		return strings.Join(parts, " · ")
	case runtime.EventIngestFailed:
		if event.Err != nil {
			return event.Err.Error()
		}
		return "ingest failed"
	default:
		return ""
	}
}

func (m *model) activeIngestCount() int {
	count := 0
	for _, entry := range m.activities {
		if entry.Status == string(runtime.IngestStatusQueued) || entry.Status == string(runtime.IngestStatusRunning) {
			count++
		}
	}
	return count
}

func (m *model) hasActiveIngestJobs() bool {
	return m.activeIngestCount() > 0
}

func (m *model) activityDockHeight() int {
	if len(m.activities) == 0 {
		return 0
	}

	lines := minInt(len(m.activities), 4) + 2
	return lines
}

func (m *model) activityDockView(width int, spinnerView string) string {
	if len(m.activities) == 0 {
		return ""
	}

	items := make([]string, 0, minInt(len(m.activities), 4))
	for i := 0; i < len(m.activities) && i < 4; i++ {
		items = append(items, renderActivityLine(m.activities[i], spinnerView, width-8))
	}

	content := strings.Join(items, "\n")
	block := activityBlockStyle.Width(maxInt(24, width-2)).Render(content)
	block = injectBorderTitle(block, activityTitleStyle.Render("Activity"), "")
	return indentBlock(block, " ")
}

func renderActivityLine(entry activityEntry, spinnerView string, width int) string {
	iconStyle := activityInfoStyle
	icon := "•"

	switch entry.Status {
	case string(runtime.IngestStatusQueued):
		icon = "◌"
		iconStyle = activityQueuedStyle
	case string(runtime.IngestStatusRunning):
		icon = spinnerView
		iconStyle = activityRunningStyle
	case string(runtime.IngestStatusCompleted):
		icon = "✓"
		iconStyle = activitySuccessStyle
	case string(runtime.IngestStatusFailed):
		icon = "✗"
		iconStyle = activityErrorStyle
	default:
		iconStyle = activityInfoStyle
	}

	title := filepath.Base(entry.Title)
	if title == "." || title == string(filepath.Separator) || title == "" {
		title = entry.Title
	}
	text := fmt.Sprintf("%s  %s", title, entry.Detail)
	return iconStyle.Render(icon) + " " + activityTextStyle.Width(maxInt(12, width)).Render(text)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
