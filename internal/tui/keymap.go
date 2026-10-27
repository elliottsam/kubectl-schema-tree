package tui

import "charm.land/bubbles/v2/key"

// KeyMap lists every key binding the TUI responds to. It implements
// help.KeyMap so the footer help is generated from it.
type KeyMap struct {
	Up, Down, Left, Right  key.Binding
	Top, Bottom            key.Binding
	HalfDown, HalfUp       key.Binding
	Toggle                 key.Binding
	ExpandAll, CollapseAll key.Binding
	Tab                    key.Binding
	Search                 key.Binding
	NextMatch, PrevMatch   key.Binding
	Accept, Cancel         key.Binding
	Help                   key.Binding
	Quit                   key.Binding
}

// DefaultKeyMap returns the standard bindings: arrow keys and vim keys.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "collapse")),
		Right:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "expand")),
		Top:         key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:      key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		HalfDown:    key.NewBinding(key.WithKeys("ctrl+d", "pgdown"), key.WithHelp("ctrl+d", "half page down")),
		HalfUp:      key.NewBinding(key.WithKeys("ctrl+u", "pgup"), key.WithHelp("ctrl+u", "half page up")),
		Toggle:      key.NewBinding(key.WithKeys("enter", "space"), key.WithHelp("enter", "toggle")),
		ExpandAll:   key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "expand all")),
		CollapseAll: key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "collapse all")),
		Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
		Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		NextMatch:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next match")),
		PrevMatch:   key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev match")),
		Accept:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "accept")),
		Cancel:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Search, k.Tab, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Top, k.Bottom, k.HalfDown, k.HalfUp},
		{k.Toggle, k.ExpandAll, k.CollapseAll, k.Tab},
		{k.Search, k.NextMatch, k.PrevMatch, k.Cancel},
		{k.Help, k.Quit},
	}
}

// All returns every binding, for tests and documentation.
func (k KeyMap) All() []key.Binding {
	var out []key.Binding
	for _, g := range k.FullHelp() {
		out = append(out, g...)
	}
	return append(out, k.Accept)
}
