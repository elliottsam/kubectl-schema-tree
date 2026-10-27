package search_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/elliottsam/kubectl-schematree/internal/schematest"
	"github.com/elliottsam/kubectl-schematree/internal/search"
	"github.com/elliottsam/kubectl-schematree/internal/tree"
)

func loadedRoots(tb testing.TB) []*tree.Node {
	tb.Helper()
	l := schematest.Loader(tb, schematest.NewDiscovery())
	g := tree.NewGroup("apps/v1", "apis/apps/v1")
	g.SetResources(schematest.Resources(tb, l, "apis/apps/v1"))
	unloaded := tree.NewGroup("batch/v1", "apis/batch/v1")
	return []*tree.Node{g, unloaded}
}

func texts(rs []search.Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Text
	}
	return out
}

func TestFind(t *testing.T) {
	t.Parallel()
	s := search.New(search.Config{})

	tests := []struct {
		name  string
		query string
		want  []string // must all be present
		none  []string // must all be absent
	}{
		{"kind", "widget", []string{"Widget"}, []string{"Gadget"}},
		{"field-across-kinds", "replicas", []string{"Widget.spec.replicas", "Gadget.replicas"}, []string{"Widget.spec.color"}},
		{"fuzzy-subsequence", "wspcol", []string{"Widget.spec.color"}, nil},
		{"group-label", "apps", []string{"apps/v1"}, nil},
		{"case-insensitive", "COLOR", []string{"Widget.spec.color"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := texts(s.Find(loadedRoots(t), tt.query))
			for _, w := range tt.want {
				if !slices.Contains(got, w) {
					t.Errorf("results %v missing %q", got, w)
				}
			}
			for _, n := range tt.none {
				if slices.Contains(got, n) {
					t.Errorf("results %v unexpectedly contain %q", got, n)
				}
			}
		})
	}
}

func TestEmptyQueryMatchesNothing(t *testing.T) {
	t.Parallel()
	s := search.New(search.Config{})
	for _, q := range []string{"", "   "} {
		if got := s.Find(loadedRoots(t), q); len(got) != 0 {
			t.Errorf("query %q returned %d results", q, len(got))
		}
	}
}

func TestUnloadedGroupsNotSearchedInside(t *testing.T) {
	t.Parallel()
	got := texts(search.New(search.Config{}).Find(loadedRoots(t), "batch"))
	if !slices.Equal(got, []string{"batch/v1"}) {
		t.Fatalf("results = %v", got)
	}
}

func TestLabelIndexes(t *testing.T) {
	t.Parallel()
	for _, r := range search.New(search.Config{}).Find(loadedRoots(t), "color") {
		if r.Text != "Widget.spec.color" {
			continue
		}
		if want := []int{0, 1, 2, 3, 4}; !slices.Equal(r.LabelIndexes, want) {
			t.Fatalf("LabelIndexes = %v, want %v", r.LabelIndexes, want)
		}
		return
	}
	t.Fatal("no result for color")
}

func TestMaxResultsAndDepth(t *testing.T) {
	t.Parallel()
	if got := search.New(search.Config{MaxResults: 1}).Find(loadedRoots(t), "replicas"); len(got) != 1 {
		t.Fatalf("MaxResults: got %d results", len(got))
	}
	// Depth 1 reaches spec but not spec.color.
	got := texts(search.New(search.Config{MaxDepth: 1}).Find(loadedRoots(t), "color"))
	if slices.Contains(got, "Widget.spec.color") {
		t.Fatalf("depth limit ignored: %v", got)
	}
}

func BenchmarkFindLargeTree(b *testing.B) {
	root := tree.NewNode("root")
	for i := range 500 {
		kind := root.AddChild(tree.NewNode(fmt.Sprintf("kind%d", i)))
		for j := range 100 {
			kind.AddChild(tree.NewNode(fmt.Sprintf("field%d", j)))
		}
	}
	roots := []*tree.Node{root}
	s := search.New(search.Config{})
	b.ReportAllocs()
	for b.Loop() {
		s.Find(roots, "kind42fld7")
	}
}
