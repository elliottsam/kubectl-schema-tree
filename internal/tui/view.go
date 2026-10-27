package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/elliottsam/kubectl-schema-tree/internal/schema"
	"github.com/elliottsam/kubectl-schema-tree/internal/tree"
)

// View implements tea.Model.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "schema-tree"
	return v
}

func (m *Model) render() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	var body string
	switch {
	case m.fatal != nil:
		body = m.centered(m.styles.Error.Render(m.fatal.Error()) + "\n\n" + m.styles.Muted.Render("press q to quit"))
	case m.showHelp:
		body = m.centered(m.help.FullHelpView(m.keys.FullHelp()))
	default:
		body = m.renderBody()
	}
	return strings.Join([]string{header, body, footer}, "\n")
}

func (m *Model) centered(s string) string {
	return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, s)
}

func (m *Model) renderHeader() string {
	left := m.styles.Header.Render(" schema-tree")
	if m.cfg.Context != "" {
		left += m.styles.Muted.Render(" · ctx: " + m.cfg.Context)
	}
	right := ""
	if m.loading > 0 {
		right = m.spin.View() + " " + m.styles.Muted.Render(describeLoad(m.pendingGroups())) + " "
	}
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return fit(left+strings.Repeat(" ", gap)+right, m.width)
}

// pendingGroups counts groups that are currently loading.
func (m *Model) pendingGroups() int {
	n := 0
	for _, g := range m.roots {
		if g.State() == tree.Loading {
			n++
		}
	}
	return max(n, 1)
}

func (m *Model) renderFooter() string {
	var s string
	switch {
	case m.focus == FocusSearch:
		s = m.input.View()
		if m.query != "" {
			s += m.styles.Muted.Render(m.matchSummary())
		}
	case m.state.Filtering():
		s = m.styles.Muted.Render(" filter: ") + m.query + m.styles.Muted.Render(m.matchSummary()+" · n/N next · esc clear")
	case m.notice != "":
		s = " " + m.styles.Error.Render(m.notice)
	default:
		s = " " + m.help.ShortHelpView(m.keys.ShortHelp())
	}
	return fit(s, m.width)
}

func (m *Model) matchSummary() string {
	n := len(m.state.Matches())
	if n == 0 {
		return "  no matches"
	}
	return fmt.Sprintf("  %d matches", n)
}

func (m *Model) renderBody() string {
	h := m.bodyHeight()
	treeW, detailW := m.paneWidths()
	treeFocused := m.focus != FocusDetail
	if m.narrow() {
		if m.focus == FocusDetail {
			return m.pane(m.renderDetail(detailW-2, h-2), true)
		}
		return m.pane(m.renderTree(treeW-2, h-2), true)
	}
	left := m.pane(m.renderTree(treeW-2, h-2), treeFocused)
	right := m.pane(m.renderDetail(detailW-2, h-2), !treeFocused)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// pane frames content, which must already be exactly the inner size.
func (m *Model) pane(content string, focused bool) string {
	col := m.styles.PaneBorder
	if focused {
		col = m.styles.PaneBorderFocused
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(col).
		Render(content)
}

func (m *Model) renderTree(w, h int) string {
	rows := m.state.Rows()
	lines := make([]string, 0, h)
	if len(rows) == 0 {
		msg := "loading…"
		switch {
		case m.state.Filtering():
			msg = "no matches"
		case m.loading == 0:
			msg = "nothing to show"
		}
		lines = append(lines, " "+m.styles.Muted.Render(msg))
	}
	cursor := m.state.Cursor()
	for i := m.offset; i < len(rows) && len(lines) < h; i++ {
		lines = append(lines, m.renderRow(rows[i], w, i == cursor))
	}
	return fitBlock(lines, w, h)
}

type seg struct {
	text  string
	style lipgloss.Style
}

// renderRow draws one tree row. The selected row is padded to the full pane
// width and drawn with a background so the cursor is obvious.
func (m *Model) renderRow(r tree.Row, w int, selected bool) string {
	segs := m.rowSegments(r)
	var b strings.Builder
	used := 0
	for _, sg := range segs {
		if sg.text == "" {
			continue
		}
		room := w - used
		if room <= 0 {
			break
		}
		text := sg.text
		if lipgloss.Width(text) > room {
			text = ansi.Truncate(text, room, "…")
		}
		b.WriteString(m.paint(sg.style, text, selected))
		used += lipgloss.Width(text)
	}
	if pad := w - used; pad > 0 {
		b.WriteString(m.paint(lipgloss.NewStyle(), strings.Repeat(" ", pad), selected))
	}
	return b.String()
}

func (m *Model) paint(st lipgloss.Style, text string, selected bool) string {
	if selected {
		st = m.styles.Selected(st, m.focus != FocusDetail)
	}
	return st.Render(text)
}

func (m *Model) rowSegments(r tree.Row) []seg {
	n := r.Node
	st := m.styles
	segs := []seg{{text: " " + strings.Repeat("  ", r.Depth)}, {text: m.marker(n) + " "}}

	if n.Schema == nil {
		label := st.Group
		if n.State() == tree.Failed {
			label = st.Muted
		}
		segs = append(segs, m.labelSegs(n, label)...)
		if n.State() == tree.Failed {
			segs = append(segs, seg{text: "  " + failReason(n.Err), style: st.Error})
		}
		return segs
	}

	sn := n.Schema
	label := st.Label
	switch {
	case sn.Deprecated:
		label = st.Deprecated
	case sn.GVK != nil:
		label = st.Group
	}
	segs = append(segs, m.labelSegs(n, label)...)
	if sn.Required {
		segs = append(segs, seg{text: " *", style: st.Required})
	}
	typ := sn.TypeName
	if sn.Kind == schema.KindObject || sn.Kind == schema.KindArray || sn.Kind == schema.KindMap {
		typ = "<" + typ + ">"
	}
	if sn.Recursive {
		typ += " ↻"
	}
	segs = append(segs, seg{text: "  "}, seg{text: typ, style: st.Kind(sn.Kind)})
	return segs
}

// labelSegs splits the label into runs so fuzzy-matched characters stand out.
func (m *Model) labelSegs(n *tree.Node, base lipgloss.Style) []seg {
	idx := m.matchIdx[n]
	if len(idx) == 0 {
		return []seg{{text: n.Label, style: base}}
	}
	hit := map[int]bool{}
	for _, i := range idx {
		hit[i] = true
	}
	var out []seg
	var cur strings.Builder
	curHit := false
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		style := base
		if curHit {
			style = base.Bold(true).Underline(true)
		}
		out = append(out, seg{text: cur.String(), style: style})
		cur.Reset()
	}
	for i, r := range []rune(n.Label) {
		if hit[i] != curHit {
			flush()
			curHit = hit[i]
		}
		cur.WriteRune(r)
	}
	flush()
	return out
}

func (m *Model) marker(n *tree.Node) string {
	switch n.State() {
	case tree.Loading:
		return m.spin.View()
	case tree.Failed:
		return "✗"
	}
	switch {
	case !n.Expandable():
		return " "
	case n.Expanded() || m.state.Filtering():
		return "▾"
	default:
		return "▸"
	}
}

func failReason(err error) string {
	if err != nil && strings.Contains(err.Error(), schema.ErrForbidden.Error()) {
		return "forbidden"
	}
	return "failed to load"
}

// fit truncates or pads s to exactly w columns.
func fit(s string, w int) string {
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if pad := w - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// fitBlock returns exactly h lines of exactly w columns.
func fitBlock(lines []string, w, h int) string {
	out := make([]string, h)
	for i := range out {
		if i < len(lines) {
			out[i] = fit(lines[i], w)
		} else {
			out[i] = strings.Repeat(" ", w)
		}
	}
	return strings.Join(out, "\n")
}
