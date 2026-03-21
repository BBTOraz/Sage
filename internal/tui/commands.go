package tui

import (
	"bilge-lib/internal/runtime"

	tea "charm.land/bubbletea/v2"
)

func waitRunnerEvent(stream <-chan runtime.Event) tea.Cmd {
	if stream == nil {
		return nil
	}

	return func() tea.Msg {
		for e := range stream {
			return runnerEventMsg{
				Event:  e,
				Stream: stream,
			}
		}
		return nil
	}
}

func waitIngestEvent(stream <-chan runtime.IngestEvent) tea.Cmd {
	if stream == nil {
		return nil
	}

	return func() tea.Msg {
		for e := range stream {
			return ingestEventMsg{
				Event:  e,
				Stream: stream,
			}
		}
		return nil
	}
}
