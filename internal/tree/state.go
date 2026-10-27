package tree

// Row is a visible tree row.
type Row struct {
	Node  *Node
	Depth int
}

// State is the navigation state of a forest of Nodes. It is not safe for
// concurrent use.
type State struct {
	roots  []*Node
	rows   []Row
	cursor int

	filter  map[*Node]bool // visible set while a filter is active
	matches []*Node        // matched nodes in row order
	matched map[*Node]bool
}

// New returns a State over roots with the cursor on the first row.
func New(roots []*Node) *State {
	s := &State{roots: roots}
	s.rebuild(nil)
	return s
}

// Rows returns the visible rows. The slice is valid until the next mutation.
func (s *State) Rows() []Row { return s.rows }

// Len returns the number of visible rows.
func (s *State) Len() int { return len(s.rows) }

// Cursor returns the index of the selected row, or -1 if there are no rows.
func (s *State) Cursor() int {
	if len(s.rows) == 0 {
		return -1
	}
	return s.cursor
}

// Selected returns the selected node, or nil if there are no rows.
func (s *State) Selected() *Node {
	if len(s.rows) == 0 {
		return nil
	}
	return s.rows[s.cursor].Node
}

// Move shifts the cursor by delta rows, clamped to the visible range.
func (s *State) Move(delta int) {
	s.cursor = clamp(s.cursor+delta, len(s.rows))
}

// Top moves the cursor to the first row.
func (s *State) Top() { s.cursor = 0 }

// Bottom moves the cursor to the last row.
func (s *State) Bottom() { s.cursor = clamp(len(s.rows)-1, len(s.rows)) }

// Select moves the cursor to n, revealing it by expanding its ancestors. It
// reports whether n was found.
func (s *State) Select(n *Node) bool {
	for _, a := range n.Ancestors() {
		a.expanded = true
	}
	s.rebuild(n)
	return s.indexOf(n) >= 0 && s.Selected() == n
}

// Toggle flips the selected node's expansion. It returns the node if it must
// be loaded before it can expand.
func (s *State) Toggle() *Node {
	n := s.Selected()
	if n == nil {
		return nil
	}
	if n.expanded {
		return s.collapse(n)
	}
	return s.expand(n)
}

// Right expands the selected node, or steps into its first child when it is
// already expanded. It returns the node if it must be loaded first.
func (s *State) Right() *Node {
	n := s.Selected()
	if n == nil {
		return nil
	}
	if !n.expanded {
		return s.expand(n)
	}
	if s.cursor+1 < len(s.rows) && s.rows[s.cursor+1].Node.Parent == n {
		s.cursor++
	}
	return nil
}

// Left collapses the selected node, or jumps to its parent when it is already
// collapsed or a leaf.
func (s *State) Left() {
	n := s.Selected()
	if n == nil {
		return
	}
	if n.expanded {
		s.collapse(n)
		return
	}
	if n.Parent != nil {
		s.rebuild(n.Parent)
	}
}

// Expand expands n (not just the selected node). See Toggle for the return value.
func (s *State) Expand(n *Node) *Node { return s.expand(n) }

// ExpandAll expands every node below the selected one (or every root when
// nothing is selected) down to maxDepth levels. Unloaded groups are skipped.
func (s *State) ExpandAll(maxDepth int) {
	sel := s.Selected()
	var walk func(n *Node, depth int)
	walk = func(n *Node, depth int) {
		if depth > maxDepth || !n.Expandable() || n.state != Loaded {
			return
		}
		n.expanded = true
		for _, c := range n.Children() {
			walk(c, depth+1)
		}
	}
	if sel == nil {
		for _, r := range s.roots {
			walk(r, 0)
		}
	} else {
		walk(sel, 0)
	}
	s.rebuild(sel)
}

// CollapseAll collapses every node and moves the cursor to the top-level
// ancestor of the selection.
func (s *State) CollapseAll() {
	sel := s.Selected()
	var walk func(n *Node)
	walk = func(n *Node) {
		n.expanded = false
		for _, c := range n.children {
			walk(c)
		}
	}
	for _, r := range s.roots {
		walk(r)
	}
	for sel != nil && sel.Parent != nil {
		sel = sel.Parent
	}
	s.rebuild(sel)
}

// Refresh recomputes visible rows after nodes were loaded or changed, keeping
// the selection where possible.
func (s *State) Refresh() { s.rebuild(s.Selected()) }

// SetFilter restricts the view to matched nodes and their ancestors, expanding
// ancestors virtually. matched must be in the order matches should be cycled.
// An empty matched slice yields an empty view; use ClearFilter to remove it.
func (s *State) SetFilter(matched []*Node) {
	s.filter = map[*Node]bool{}
	s.matched = map[*Node]bool{}
	s.matches = matched
	for _, m := range matched {
		s.matched[m] = true
		for n := m; n != nil; n = n.Parent {
			s.filter[n] = true
		}
	}
	s.rebuild(nil)
	if len(matched) > 0 {
		s.Select(matched[0])
	}
}

// ClearFilter removes any active filter, keeping the selected node selected.
func (s *State) ClearFilter() {
	sel := s.Selected()
	s.filter, s.matched, s.matches = nil, nil, nil
	if sel != nil {
		for _, a := range sel.Ancestors() {
			a.expanded = true
		}
	}
	s.rebuild(sel)
}

// Filtering reports whether a filter is active.
func (s *State) Filtering() bool { return s.filter != nil }

// IsMatch reports whether n is one of the filter's matched nodes.
func (s *State) IsMatch(n *Node) bool { return s.matched[n] }

// Matches returns the filter's matched nodes.
func (s *State) Matches() []*Node { return s.matches }

// NextMatch moves the cursor to the next matched row after the cursor, wrapping.
func (s *State) NextMatch() { s.stepMatch(1) }

// PrevMatch moves the cursor to the previous matched row, wrapping.
func (s *State) PrevMatch() { s.stepMatch(-1) }

func (s *State) stepMatch(dir int) {
	n := len(s.rows)
	if n == 0 || len(s.matched) == 0 {
		return
	}
	for i := 1; i <= n; i++ {
		idx := ((s.cursor+dir*i)%n + n) % n
		if s.matched[s.rows[idx].Node] {
			s.cursor = idx
			return
		}
	}
}

func (s *State) expand(n *Node) *Node {
	if !n.Expandable() {
		return nil
	}
	n.expanded = true
	s.rebuild(s.Selected())
	if n.NeedsLoad() {
		return n
	}
	return nil
}

func (s *State) collapse(n *Node) *Node {
	n.expanded = false
	s.rebuild(s.Selected())
	return nil
}

func (s *State) indexOf(n *Node) int {
	for i, r := range s.rows {
		if r.Node == n {
			return i
		}
	}
	return -1
}

// rebuild recomputes rows and places the cursor on keep, or on its nearest
// visible ancestor, or clamps the previous index.
func (s *State) rebuild(keep *Node) {
	prev := s.cursor
	s.rows = s.rows[:0]
	var walk func(n *Node, depth int)
	walk = func(n *Node, depth int) {
		if s.filter != nil && !s.filter[n] {
			return
		}
		s.rows = append(s.rows, Row{Node: n, Depth: depth})
		if n.expanded || s.filter != nil {
			for _, c := range n.Children() {
				walk(c, depth+1)
			}
		}
	}
	for _, r := range s.roots {
		walk(r, 0)
	}
	for n := keep; n != nil; n = n.Parent {
		if i := s.indexOf(n); i >= 0 {
			s.cursor = i
			return
		}
	}
	s.cursor = clamp(prev, len(s.rows))
}

func clamp(i, n int) int {
	if n == 0 || i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
