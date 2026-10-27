package tree_test

import (
	"testing"

	"github.com/elliottsam/kubectl-schematree/internal/schematest"
	"github.com/elliottsam/kubectl-schematree/internal/tree"
)

func TestFromSchemaLazyChildren(t *testing.T) {
	t.Parallel()
	l := schematest.Loader(t, schematest.NewDiscovery())
	group := tree.NewGroup("apps/v1", "apis/apps/v1")
	s := tree.New([]*tree.Node{group})

	if !group.NeedsLoad() {
		t.Fatal("group should need load")
	}
	group.SetResources(schematest.Resources(t, l, "apis/apps/v1"))
	s.Expand(group)
	s.Move(1) // Gadget
	s.Move(1) // Widget
	if selected(s) != "Widget" {
		t.Fatalf("selected = %q", selected(s))
	}
	s.Right() // expand Widget
	s.Right() // into metadata
	s.Move(1) // spec
	s.Right() // expand spec
	s.Right() // into color
	s.Move(1) // replicas
	if selected(s) != "replicas" {
		t.Fatalf("selected = %q, want replicas", selected(s))
	}
	if p := s.Selected().Schema.Path; p != "spec.replicas" {
		t.Fatalf("schema path = %q", p)
	}
}
