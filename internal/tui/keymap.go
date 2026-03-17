package tui

import (
	"charm.land/bubbles/v2/key"
)

type keyMap struct {
	Send          key.Binding
	NewLine       key.Binding
	Quit          key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	ToggleLeft    key.Binding
	ToggleRight   key.Binding
	ToggleConfirm key.Binding
	ExpandTools   key.Binding
}

func (k *keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Send,
		k.NewLine,
		k.Quit,
		k.PageUp,
		k.PageDown,
	}
}

func (k *keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.NewLine},
		{k.ToggleLeft, k.ToggleRight, k.ToggleConfirm},
		{k.PageUp, k.PageDown},
		{k.Quit},
	}
}

var keys = keyMap{
	Send: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send message"),
	),
	NewLine: key.NewBinding(
		key.WithKeys("ctrl+j", "shift+enter"),
		key.WithHelp("ctrl+j/shift+enter", "new line"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "scroll up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "scroll down"),
	),
	ToggleLeft: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "select left"),
	),
	ToggleRight: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "select right"),
	),
	ToggleConfirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	ExpandTools: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "expand/collapse tools"),
	),
}
