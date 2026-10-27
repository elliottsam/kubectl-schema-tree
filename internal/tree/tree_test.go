package tree_test

import (
	"errors"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/elliottsam/kubectl-schematree/internal/tree"
)

// build returns:
//
//	a
//	├── a1
//	│   └── a1x
//	└── a2
//	b
//	c (empty)
func build() (roots []*tree.Node, byName map[string]*tree.Node) {
	byName = map[string]*tree.Node{}
	mk := func(name string, parent *tree.Node) *tree.Node {
		n := tree.NewNode(name)
		if parent != nil {
			parent.AddChild(n)
		}
		byName[name] = n
		return n
	}
	a := mk("a", nil)
	a1 := mk("a1", a)
	mk("a1x", a1)
	mk("a2", a)
	b := mk("b", nil)
	c := mk("c", nil)
	return []*tree.Node{a, b, c}, byName
}

func labels(s *tree.State) []string {
	var out []string
	for _, r := range s.Rows() {
		out = append(out, r.Node.Label)
	}
	return out
}

func selected(s *tree.State) string {
	if n := s.Selected(); n != nil {
		return n.Label
	}
	return ""
}

func TestInitialRowsAreRoots(t *testing.T) {
	t.Parallel()
	roots, _ := build()
	s := tree.New(roots)
	if got, want := labels(s), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if s.Cursor() != 0 {
		t.Fatalf("cursor = %d", s.Cursor())
	}
}

func TestEmptyState(t *testing.T) {
	t.Parallel()
	s := tree.New(nil)
	if s.Cursor() != -1 || s.Selected() != nil || s.Len() != 0 {
		t.Fatalf("empty state: cursor=%d selected=%v len=%d", s.Cursor(), s.Selected(), s.Len())
	}
	// None of these may panic.
	s.Move(3)
	s.Top()
	s.Bottom()
	s.Left()
	s.CollapseAll()
	s.ExpandAll(3)
	if s.Right() != nil || s.Toggle() != nil {
		t.Fatal("empty state requested a load")
	}
}

func TestMoveClamps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		moves []int
		want  string
	}{
		{"down", []int{1}, "b"},
		{"down-too-far", []int{10}, "c"},
		{"up-at-top", []int{-1}, "a"},
		{"down-up", []int{2, -1}, "b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			roots, _ := build()
			s := tree.New(roots)
			for _, m := range tt.moves {
				s.Move(m)
			}
			if got := selected(s); got != tt.want {
				t.Fatalf("selected = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTopBottom(t *testing.T) {
	t.Parallel()
	roots, _ := build()
	s := tree.New(roots)
	s.Bottom()
	if selected(s) != "c" {
		t.Fatalf("bottom = %q", selected(s))
	}
	s.Top()
	if selected(s) != "a" {
		t.Fatalf("top = %q", selected(s))
	}
}

func TestExpandCollapse(t *testing.T) {
	t.Parallel()
	roots, _ := build()
	s := tree.New(roots)

	s.Toggle()
	if got, want := labels(s), []string{"a", "a1", "a2", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("after expand rows = %v, want %v", got, want)
	}
	s.Toggle()
	if got, want := labels(s), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("after collapse rows = %v, want %v", got, want)
	}
}

func TestRightStepsIntoChild(t *testing.T) {
	t.Parallel()
	roots, _ := build()
	s := tree.New(roots)
	s.Right() // expand a
	if selected(s) != "a" {
		t.Fatalf("selected = %q after first right", selected(s))
	}
	s.Right() // step into a1
	if selected(s) != "a1" {
		t.Fatalf("selected = %q after second right", selected(s))
	}
}

func TestRightOnLeafIsNoOp(t *testing.T) {
	t.Parallel()
	roots, _ := build()
	s := tree.New(roots)
	s.Move(2) // c, no children
	s.Right()
	if selected(s) != "c" || s.Len() != 3 {
		t.Fatalf("leaf right changed state: sel=%q len=%d", selected(s), s.Len())
	}
}

func TestLeftCollapsesThenJumpsToParent(t *testing.T) {
	t.Parallel()
	roots, by := build()
	s := tree.New(roots)
	s.Right()
	s.Right() // on a1
	s.Right() // expand a1
	s.Right() // on a1x
	if selected(s) != "a1x" {
		t.Fatalf("setup: selected = %q", selected(s))
	}
	s.Left() // leaf -> parent
	if selected(s) != "a1" {
		t.Fatalf("selected = %q, want a1", selected(s))
	}
	s.Left() // collapse a1
	if by["a1"].Expanded() {
		t.Fatal("a1 still expanded")
	}
	s.Left() // jump to a
	if selected(s) != "a" {
		t.Fatalf("selected = %q, want a", selected(s))
	}
}

func TestLeftOnRootIsNoOp(t *testing.T) {
	t.Parallel()
	roots, _ := build()
	s := tree.New(roots)
	s.Left()
	if selected(s) != "a" {
		t.Fatalf("selected = %q", selected(s))
	}
}

func TestCollapseKeepsCursorVisible(t *testing.T) {
	t.Parallel()
	roots, by := build()
	s := tree.New(roots)
	s.Expand(by["a"])
	s.Select(by["a2"])
	s.Select(by["a"])
	s.Toggle() // collapse a while a2 was previously selected elsewhere
	if selected(s) != "a" {
		t.Fatalf("selected = %q", selected(s))
	}
}

func TestCollapseAllMovesToRootAncestor(t *testing.T) {
	t.Parallel()
	roots, by := build()
	s := tree.New(roots)
	s.ExpandAll(5)
	s.Select(by["a1x"])
	s.CollapseAll()
	if selected(s) != "a" || s.Len() != 3 {
		t.Fatalf("selected=%q len=%d", selected(s), s.Len())
	}
}

func TestExpandAllRespectsDepth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		depth int
		want  []string
	}{
		{0, []string{"a", "a1", "a2", "b", "c"}},
		{1, []string{"a", "a1", "a1x", "a2", "b", "c"}},
		{5, []string{"a", "a1", "a1x", "a2", "b", "c"}},
	}
	for _, tt := range tests {
		roots, _ := build()
		s := tree.New(roots)
		s.ExpandAll(tt.depth)
		if got := labels(s); !slices.Equal(got, tt.want) {
			t.Errorf("depth %d rows = %v, want %v", tt.depth, got, tt.want)
		}
	}
}

func TestSelectRevealsAncestors(t *testing.T) {
	t.Parallel()
	roots, by := build()
	s := tree.New(roots)
	if !s.Select(by["a1x"]) {
		t.Fatal("Select returned false")
	}
	if selected(s) != "a1x" {
		t.Fatalf("selected = %q", selected(s))
	}
	if !by["a"].Expanded() || !by["a1"].Expanded() {
		t.Fatal("ancestors not expanded")
	}
}

func TestLazyGroupLoading(t *testing.T) {
	t.Parallel()
	g := tree.NewGroup("apps/v1", "apis/apps/v1")
	s := tree.New([]*tree.Node{g})

	if load := s.Toggle(); load != g {
		t.Fatalf("Toggle returned %v, want the group to load", load)
	}
	g.MarkLoading()
	if g.State() != tree.Loading {
		t.Fatalf("state = %v", g.State())
	}
	g.SetError(errors.New("boom"))
	if g.State() != tree.Failed || g.Expanded() || g.Expandable() {
		t.Fatalf("failed group: state=%v expanded=%v expandable=%v", g.State(), g.Expanded(), g.Expandable())
	}
	if s.Toggle() != nil {
		t.Fatal("failed group requested another load")
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()
	roots, by := build()
	s := tree.New(roots)
	s.SetFilter([]*tree.Node{by["a1x"], by["b"]})

	if got, want := labels(s), []string{"a", "a1", "a1x", "b"}; !slices.Equal(got, want) {
		t.Fatalf("filtered rows = %v, want %v", got, want)
	}
	if selected(s) != "a1x" {
		t.Fatalf("selected = %q, want first match", selected(s))
	}
	if !s.Filtering() || !s.IsMatch(by["b"]) || s.IsMatch(by["a"]) {
		t.Fatal("match bookkeeping wrong")
	}

	s.NextMatch()
	if selected(s) != "b" {
		t.Fatalf("next = %q", selected(s))
	}
	s.NextMatch() // wraps
	if selected(s) != "a1x" {
		t.Fatalf("wrap next = %q", selected(s))
	}
	s.PrevMatch() // wraps back
	if selected(s) != "b" {
		t.Fatalf("prev = %q", selected(s))
	}

	s.ClearFilter()
	if s.Filtering() {
		t.Fatal("still filtering")
	}
	if selected(s) != "b" {
		t.Fatalf("selection lost after clear: %q", selected(s))
	}
}

func TestFilterNoMatches(t *testing.T) {
	t.Parallel()
	roots, _ := build()
	s := tree.New(roots)
	s.SetFilter(nil)
	if s.Len() != 0 || s.Cursor() != -1 {
		t.Fatalf("len=%d cursor=%d", s.Len(), s.Cursor())
	}
	s.NextMatch() // must not panic
	s.ClearFilter()
	if s.Len() != 3 {
		t.Fatalf("len after clear = %d", s.Len())
	}
}

func TestDepthAndAncestors(t *testing.T) {
	t.Parallel()
	_, by := build()
	n := by["a1x"]
	if n.Depth() != 2 {
		t.Fatalf("depth = %d", n.Depth())
	}
	anc := n.Ancestors()
	if len(anc) != 2 || anc[0].Label != "a" || anc[1].Label != "a1" {
		t.Fatalf("ancestors = %v", anc)
	}
}

// TestCursorInvariant applies random operations and checks the cursor always
// addresses a visible row.
func TestCursorInvariant(t *testing.T) {
	t.Parallel()
	for seed := uint64(0); seed < 50; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed+1))
		roots, by := build()
		s := tree.New(roots)
		names := []string{"a", "a1", "a1x", "a2", "b", "c"}
		for step := 0; step < 200; step++ {
			switch rng.IntN(12) {
			case 0:
				s.Move(rng.IntN(7) - 3)
			case 1:
				s.Top()
			case 2:
				s.Bottom()
			case 3:
				s.Right()
			case 4:
				s.Left()
			case 5:
				s.Toggle()
			case 6:
				s.ExpandAll(rng.IntN(3))
			case 7:
				s.CollapseAll()
			case 8:
				s.Select(by[names[rng.IntN(len(names))]])
			case 9:
				s.SetFilter([]*tree.Node{by[names[rng.IntN(len(names))]]})
			case 10:
				s.ClearFilter()
			case 11:
				s.NextMatch()
			}
			if s.Len() == 0 {
				if s.Cursor() != -1 {
					t.Fatalf("seed %d step %d: empty view cursor=%d", seed, step, s.Cursor())
				}
				continue
			}
			if c := s.Cursor(); c < 0 || c >= s.Len() {
				t.Fatalf("seed %d step %d: cursor %d out of range [0,%d)", seed, step, c, s.Len())
			}
		}
	}
}

func BenchmarkRebuildLargeTree(b *testing.B) {
	root := tree.NewNode("root")
	for i := range 200 {
		c := root.AddChild(tree.NewNode("c"))
		for j := range 250 {
			_ = i + j
			c.AddChild(tree.NewNode("g"))
		}
	}
	s := tree.New([]*tree.Node{root})
	s.ExpandAll(5)
	b.ReportAllocs()
	for b.Loop() {
		s.Refresh()
	}
}
