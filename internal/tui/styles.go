package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/elliottsam/kubectl-schema-tree/internal/schema"
)

// Styles holds every lipgloss style used by the views. Views never build
// their own styles.
type Styles struct {
	Header     lipgloss.Style
	Muted      lipgloss.Style
	Error      lipgloss.Style
	Group      lipgloss.Style
	Label      lipgloss.Style
	Required   lipgloss.Style
	Deprecated lipgloss.Style
	Match      lipgloss.Style
	Title      lipgloss.Style
	Key        lipgloss.Style

	// PaneBorder and PaneBorderFocused colour the pane frames.
	PaneBorder        color.Color
	PaneBorderFocused color.Color

	kinds   map[schema.Kind]lipgloss.Style
	noColor bool
	selFoc  color.Color
	selBlur color.Color
}

// NewStyles builds the palette for a dark or light terminal. With noColor set
// no colours are used and the selection falls back to reverse video.
func NewStyles(dark, noColor bool) Styles {
	ld := lipgloss.LightDark(dark)
	c := func(light, dark string) color.Color {
		if noColor {
			return lipgloss.NoColor{}
		}
		return ld(lipgloss.Color(light), lipgloss.Color(dark))
	}
	fg := func(col color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(col) }

	s := Styles{
		noColor:           noColor,
		PaneBorder:        c("#c0c6cc", "#4b5260"),
		PaneBorderFocused: c("#0969da", "#58a6ff"),
		selFoc:            c("#bfdbfe", "#1f4a7a"),
		selBlur:           c("#e1e4e8", "#33363c"),
		kinds: map[schema.Kind]lipgloss.Style{
			schema.KindObject:  fg(c("#0550ae", "#79c0ff")),
			schema.KindArray:   fg(c("#6f42c1", "#d2a8ff")),
			schema.KindMap:     fg(c("#8a5a00", "#e3b341")),
			schema.KindString:  fg(c("#1a7f37", "#7ee787")),
			schema.KindInteger: fg(c("#bc4c00", "#ffa657")),
			schema.KindNumber:  fg(c("#bc4c00", "#ffa657")),
			schema.KindBoolean: fg(c("#cf222e", "#ff7b72")),
			schema.KindUnknown: fg(c("#6e7781", "#8b949e")),
		},
	}
	s.Header = fg(c("#24292f", "#e6edf3")).Bold(true)
	s.Muted = fg(c("#6e7781", "#8b949e"))
	s.Error = fg(c("#cf222e", "#ff7b72")).Bold(true)
	s.Group = fg(c("#24292f", "#e6edf3")).Bold(true)
	s.Label = fg(c("#24292f", "#e6edf3"))
	s.Required = lipgloss.NewStyle().Bold(true)
	s.Deprecated = s.Muted.Faint(true).Strikethrough(true)
	s.Match = lipgloss.NewStyle().Bold(true).Underline(true)
	s.Title = fg(c("#0550ae", "#79c0ff")).Bold(true)
	s.Key = fg(c("#6e7781", "#8b949e"))
	return s
}

// Kind returns the style for a schema kind.
func (s Styles) Kind(k schema.Kind) lipgloss.Style {
	if st, ok := s.kinds[k]; ok {
		return st
	}
	return s.kinds[schema.KindUnknown]
}

// Selected derives the style for text on the selected row from base. The row
// gets a background (reverse video without colour); it is stronger when the
// tree pane has focus and muted otherwise.
func (s Styles) Selected(base lipgloss.Style, focused bool) lipgloss.Style {
	if s.noColor {
		if focused {
			return base.Reverse(true)
		}
		return base.Reverse(true).Faint(true)
	}
	if focused {
		return base.Background(s.selFoc)
	}
	return base.Background(s.selBlur)
}
