// Package tui implements the Bubble Tea interface for browsing schemas.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/elliottsam/kubectl-schema-tree/internal/schema"
	"github.com/elliottsam/kubectl-schema-tree/internal/search"
	"github.com/elliottsam/kubectl-schema-tree/internal/tree"
)

// searchSummaryWidth is the footer space reserved for the match count.
const searchSummaryWidth = 24

// narrowWidth is the terminal width below which only one pane is shown.
const narrowWidth = 80

// Source supplies schema data to the TUI. *schema.Loader satisfies it.
type Source interface {
	GroupVersions(ctx context.Context) ([]string, error)
	Resources(ctx context.Context, path string) ([]*schema.Node, error)
}

// Focus identifies which part of the UI receives key presses.
type Focus int

// Focus targets.
const (
	FocusTree Focus = iota
	FocusDetail
	FocusSearch
)

// Config configures a Model.
type Config struct {
	// Source supplies schema data. Required.
	Source Source
	// Context is the Kubernetes context name shown in the header.
	Context string
	// ExpandDepth is how many levels to pre-expand when a resource is
	// revealed by Open. Zero expands only the resource itself.
	ExpandDepth int
	// NoColor disables colours; the selected row uses reverse video.
	NoColor bool
	// Open, if set, selects a resource once loaded: a kind such as
	// "deployment" or a full "apps/v1/Deployment".
	Open string
	// Ctx bounds all cluster calls. Defaults to context.Background().
	Ctx context.Context
	// Keys overrides the key bindings. Defaults to DefaultKeyMap().
	Keys *KeyMap
}

// Validate reports whether the Config is usable.
func (c Config) Validate() error {
	if c.Source == nil {
		return errors.New("tui: Config.Source is required")
	}
	return nil
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg    Config
	keys   KeyMap
	styles Styles
	dark   bool

	width, height int

	state    *tree.State
	groups   map[string]*tree.Node
	roots    []*tree.Node
	offset   int
	loading  int
	fatal    error
	notice   string
	focus    Focus
	showHelp bool

	searcher *search.Searcher
	input    textinput.Model
	matchIdx map[*tree.Node][]int
	query    string

	pendingOpen string

	detail     viewport.Model
	detailNode *tree.Node

	spin spinner.Model
	help help.Model
}

// New returns a Model. Call Program.Run with it to start the TUI.
func New(cfg Config) (*Model, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Ctx == nil {
		cfg.Ctx = context.Background()
	}
	keys := DefaultKeyMap()
	if cfg.Keys != nil {
		keys = *cfg.Keys
	}
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "search kinds and fields"

	m := &Model{
		cfg:      cfg,
		keys:     keys,
		dark:     true,
		groups:   map[string]*tree.Node{},
		searcher: search.New(search.Config{}),
		input:    ti,
		detail:   viewport.New(),
		spin:     spinner.New(spinner.WithSpinner(spinner.Dot)),
		help:     help.New(),
		width:    120,
		height:   40,
	}
	m.restyle()
	m.state = tree.New(nil)
	return m, nil
}

func (m *Model) restyle() {
	m.styles = NewStyles(m.dark, m.cfg.NoColor)
	if m.cfg.NoColor {
		m.help.Styles = help.Styles{}
		m.input.SetStyles(textinput.Styles{Cursor: textinput.CursorStyle{Shape: tea.CursorBlock}})
		return
	}
	m.help.Styles = help.DefaultStyles(m.dark)
	st := textinput.DefaultStyles(m.dark)
	st.Cursor.Blink = false
	m.input.SetStyles(st)
}

// Selected returns the selected tree node, or nil.
func (m *Model) Selected() *tree.Node { return m.state.Selected() }

// Focus returns the focused component.
func (m *Model) Focus() Focus { return m.focus }

// Err returns the fatal error, if any.
func (m *Model) Err() error { return m.fatal }

// Filtering reports whether a search filter is active.
func (m *Model) Filtering() bool { return m.state.Filtering() }

type groupsLoadedMsg struct {
	paths []string
	err   error
}

type resourcesLoadedMsg struct {
	key   string
	nodes []*schema.Node
	err   error
}

// Init loads the group/version list and detects the terminal background.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.loadGroups(), tea.RequestBackgroundColor)
}

func (m *Model) loadGroups() tea.Cmd {
	src, ctx := m.cfg.Source, m.cfg.Ctx
	m.loading++
	return tea.Batch(func() tea.Msg {
		paths, err := src.GroupVersions(ctx)
		return groupsLoadedMsg{paths: paths, err: err}
	}, m.spin.Tick)
}

func (m *Model) loadResources(n *tree.Node) tea.Cmd {
	src, ctx, key := m.cfg.Source, m.cfg.Ctx, n.Key
	n.MarkLoading()
	m.loading++
	return tea.Batch(func() tea.Msg {
		nodes, err := src.Resources(ctx, key)
		return resourcesLoadedMsg{key: key, nodes: nodes, err: err}
	}, m.spin.Tick)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		m.restyle()
	case spinner.TickMsg:
		if m.loading > 0 {
			m.spin, cmd = m.spin.Update(msg)
		}
	case groupsLoadedMsg:
		m.loading--
		m.onGroups(msg)
		cmd = m.beginOpen()
	case resourcesLoadedMsg:
		m.loading--
		cmd = m.onResources(msg)
	case tea.KeyPressMsg:
		cmd = m.onKey(msg)
	}
	m.syncDetail()
	m.ensureVisible()
	return m, cmd
}

func (m *Model) onGroups(msg groupsLoadedMsg) {
	if msg.err != nil {
		m.fatal = msg.err
		return
	}
	m.roots = m.roots[:0]
	for _, p := range msg.paths {
		g := tree.NewGroup(schema.DisplayGroupVersion(p), p)
		m.groups[p] = g
		m.roots = append(m.roots, g)
	}
	m.state = tree.New(m.roots)
}

func (m *Model) onResources(msg resourcesLoadedMsg) tea.Cmd {
	g, ok := m.groups[msg.key]
	if !ok {
		return nil
	}
	if msg.err != nil {
		g.SetError(msg.err)
		if !errors.Is(msg.err, schema.ErrForbidden) {
			m.notice = msg.err.Error()
		}
	} else {
		g.SetResources(msg.nodes)
	}
	m.state.Refresh()
	if m.focus == FocusSearch || m.state.Filtering() {
		m.applySearch()
	}
	m.tryOpen()
	return nil
}

func (m *Model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	m.notice = ""
	if key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))) {
		return tea.Quit
	}
	if m.fatal != nil {
		if key.Matches(msg, m.keys.Quit) {
			return tea.Quit
		}
		return nil
	}
	if m.showHelp {
		m.showHelp = false
		return nil
	}
	switch m.focus {
	case FocusSearch:
		return m.onSearchKey(msg)
	case FocusDetail:
		return m.onDetailKey(msg)
	}
	return m.onTreeKey(msg)
}

func (m *Model) onTreeKey(msg tea.KeyPressMsg) tea.Cmd {
	k := m.keys
	page := max(1, m.treeHeight()/2)
	switch {
	case key.Matches(msg, k.Quit):
		return tea.Quit
	case key.Matches(msg, k.Help):
		m.showHelp = true
	case key.Matches(msg, k.Up):
		m.state.Move(-1)
	case key.Matches(msg, k.Down):
		m.state.Move(1)
	case key.Matches(msg, k.HalfDown):
		m.state.Move(page)
	case key.Matches(msg, k.HalfUp):
		m.state.Move(-page)
	case key.Matches(msg, k.Top):
		m.state.Top()
	case key.Matches(msg, k.Bottom):
		m.state.Bottom()
	case key.Matches(msg, k.Right):
		return m.requestLoad(m.state.Right())
	case key.Matches(msg, k.Toggle):
		return m.requestLoad(m.state.Toggle())
	case key.Matches(msg, k.Left):
		m.state.Left()
	case key.Matches(msg, k.ExpandAll):
		m.state.ExpandAll(m.cfg.ExpandDepth + 3)
	case key.Matches(msg, k.CollapseAll):
		m.state.CollapseAll()
	case key.Matches(msg, k.Tab):
		m.focus = FocusDetail
	case key.Matches(msg, k.Search):
		return m.startSearch()
	case key.Matches(msg, k.NextMatch):
		m.state.NextMatch()
	case key.Matches(msg, k.PrevMatch):
		m.state.PrevMatch()
	case key.Matches(msg, k.Cancel):
		m.clearSearch()
	}
	return nil
}

func (m *Model) onDetailKey(msg tea.KeyPressMsg) tea.Cmd {
	k := m.keys
	switch {
	case key.Matches(msg, k.Quit):
		return tea.Quit
	case key.Matches(msg, k.Help):
		m.showHelp = true
	case key.Matches(msg, k.Tab), key.Matches(msg, k.Cancel), key.Matches(msg, k.Left):
		m.focus = FocusTree
	case key.Matches(msg, k.Search):
		return m.startSearch()
	case key.Matches(msg, k.Up):
		m.detail.ScrollUp(1)
	case key.Matches(msg, k.Down):
		m.detail.ScrollDown(1)
	case key.Matches(msg, k.HalfUp):
		m.detail.HalfPageUp()
	case key.Matches(msg, k.HalfDown):
		m.detail.HalfPageDown()
	case key.Matches(msg, k.Top):
		m.detail.GotoTop()
	case key.Matches(msg, k.Bottom):
		m.detail.GotoBottom()
	}
	return nil
}

func (m *Model) requestLoad(n *tree.Node) tea.Cmd {
	if n == nil {
		return nil
	}
	return m.loadResources(n)
}

// startSearch focuses the search input and begins loading every group so
// results cover the whole API.
func (m *Model) startSearch() tea.Cmd {
	m.focus = FocusSearch
	m.input.SetValue("")
	cmds := []tea.Cmd{m.input.Focus()}
	for _, g := range m.roots {
		if g.NeedsLoad() {
			cmds = append(cmds, m.loadResources(g))
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) onSearchKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.clearSearch()
		return nil
	case key.Matches(msg, m.keys.Accept):
		m.input.Blur()
		m.focus = FocusTree
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != m.query {
		m.query = m.input.Value()
		m.applySearch()
	}
	return cmd
}

func (m *Model) applySearch() {
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		m.matchIdx = nil
		m.state.ClearFilter()
		return
	}
	results := m.searcher.Find(m.roots, q)
	nodes := make([]*tree.Node, len(results))
	m.matchIdx = make(map[*tree.Node][]int, len(results))
	for i, r := range results {
		nodes[i] = r.Node
		m.matchIdx[r.Node] = r.LabelIndexes
	}
	m.state.SetFilter(nodes)
}

func (m *Model) clearSearch() {
	m.input.Blur()
	m.input.SetValue("")
	m.query = ""
	m.matchIdx = nil
	m.state.ClearFilter()
	m.focus = FocusTree
}

// beginOpen starts loading the groups that may contain cfg.Open.
func (m *Model) beginOpen() tea.Cmd {
	if m.cfg.Open == "" {
		return nil
	}
	m.pendingOpen = m.cfg.Open
	var cmds []tea.Cmd
	for _, g := range m.openCandidates() {
		if g.NeedsLoad() {
			cmds = append(cmds, m.loadResources(g))
		}
	}
	if len(cmds) == 0 {
		m.tryOpen()
	}
	return tea.Batch(cmds...)
}

// openParts splits the pending query into an optional group/version prefix
// and the kind.
func (m *Model) openParts() (gv, kind string) {
	q := strings.ToLower(m.pendingOpen)
	if i := strings.LastIndex(q, "/"); i >= 0 {
		return q[:i], q[i+1:]
	}
	return "", q
}

func (m *Model) openCandidates() []*tree.Node {
	gv, _ := m.openParts()
	if gv == "" {
		return m.roots
	}
	var out []*tree.Node
	for _, g := range m.roots {
		if strings.ToLower(g.Label) == gv {
			out = append(out, g)
		}
	}
	return out
}

// tryOpen selects the pending resource once every candidate group has
// settled, preferring earlier groups (core first).
func (m *Model) tryOpen() {
	if m.pendingOpen == "" {
		return
	}
	_, kind := m.openParts()
	cands := m.openCandidates()
	for _, g := range cands {
		if g.State() == tree.Unloaded || g.State() == tree.Loading {
			return
		}
	}
	defer func() { m.pendingOpen = "" }()
	for _, g := range cands {
		if g.State() != tree.Loaded {
			continue
		}
		for _, c := range g.Children() {
			if kindMatches(c.Label, kind) {
				m.state.Select(c)
				m.state.ExpandAll(m.cfg.ExpandDepth)
				return
			}
		}
	}
	m.notice = fmt.Sprintf("resource %q not found", m.pendingOpen)
}

// kindMatches reports whether query names kind, allowing simple plurals.
func kindMatches(kind, query string) bool {
	k := strings.ToLower(kind)
	return k == query || k+"s" == query || k+"es" == query
}

// layout sizes child components for the current terminal size.
func (m *Model) layout() {
	_, detailW := m.paneWidths()
	m.detail.SetWidth(max(1, detailW-4))
	m.detail.SetHeight(max(1, m.bodyHeight()-2))
	m.input.SetWidth(max(1, m.width-searchSummaryWidth))
	m.help.SetWidth(m.width)
	m.detailNode = nil // force re-render at the new width
}

func (m *Model) bodyHeight() int { return max(1, m.height-2) }

func (m *Model) treeHeight() int { return max(1, m.bodyHeight()-2) }

func (m *Model) narrow() bool { return m.width < narrowWidth }

func (m *Model) paneWidths() (treeW, detailW int) {
	if m.narrow() {
		return m.width, m.width
	}
	treeW = m.width / 2
	return treeW, m.width - treeW
}

func (m *Model) ensureVisible() {
	h := m.treeHeight()
	c := m.state.Cursor()
	if c < 0 {
		m.offset = 0
		return
	}
	if c < m.offset {
		m.offset = c
	}
	if c >= m.offset+h {
		m.offset = c - h + 1
	}
	maxOff := max(0, m.state.Len()-h)
	m.offset = min(m.offset, maxOff)
}

func describeLoad(n int) string {
	if n == 1 {
		return "loading 1 group"
	}
	return fmt.Sprintf("loading %d groups", n)
}
