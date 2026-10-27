//go:build integration

// Package integration exercises the schema loader and TUI model against a real
// cluster. Run with `make test-integration`.
package integration_test

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/elliottsam/kubectl-schema-tree/internal/schema"
	"github.com/elliottsam/kubectl-schema-tree/internal/tui"
)

func newLoader(t *testing.T) *schema.Loader {
	t.Helper()
	kctx := os.Getenv("KUBECTL_SCHEMATREE_CONTEXT")
	if kctx == "" {
		t.Skip("KUBECTL_SCHEMATREE_CONTEXT not set")
	}
	flags := genericclioptions.NewConfigFlags(true)
	flags.Context = &kctx
	cache := t.TempDir()
	flags.CacheDir = &cache
	dc, err := flags.ToDiscoveryClient()
	if err != nil {
		t.Fatalf("discovery client: %v", err)
	}
	l, err := schema.NewLoader(schema.Config{Discovery: dc})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestClusterGroupVersions(t *testing.T) {
	l := newLoader(t)
	gvs, err := l.GroupVersions(ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"api/v1", "apis/apps/v1", "apis/apiextensions.k8s.io/v1"} {
		if !slices.Contains(gvs, want) {
			t.Errorf("group/versions %v missing %q", gvs, want)
		}
	}
	if gvs[0] != "api/v1" {
		t.Errorf("core group not first: %v", gvs[:3])
	}
}

func TestClusterDeploymentSchema(t *testing.T) {
	l := newLoader(t)
	roots, err := l.Resources(ctx(t), "apis/apps/v1")
	if err != nil {
		t.Fatal(err)
	}
	var dep *schema.Node
	for _, r := range roots {
		if r.Name == "Deployment" {
			dep = r
		}
	}
	if dep == nil {
		t.Fatal("no Deployment resource")
	}
	find := func(n *schema.Node, names ...string) *schema.Node {
		for _, name := range names {
			var next *schema.Node
			for _, c := range n.Children() {
				if c.Name == name {
					next = c
				}
			}
			if next == nil {
				t.Fatalf("%s has no child %q", n.Path, name)
			}
			n = next
		}
		return n
	}
	replicas := find(dep, "spec", "replicas")
	if replicas.Kind != schema.KindInteger || replicas.Format != "int32" {
		t.Errorf("replicas = %v/%q", replicas.Kind, replicas.Format)
	}
	image := find(dep, "spec", "template", "spec", "containers", "image")
	if image.Path != "spec.template.spec.containers[].image" {
		t.Errorf("image path = %q", image.Path)
	}
}

func TestClusterRecursiveCRDSchemaTerminates(t *testing.T) {
	l := newLoader(t)
	roots, err := l.Resources(ctx(t), "apis/apiextensions.k8s.io/v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roots {
		if r.Name != "CustomResourceDefinition" {
			continue
		}
		recursive := false
		r.Walk(40, func(n *schema.Node) bool {
			recursive = recursive || n.Recursive
			return true
		})
		if !recursive {
			t.Fatal("expected recursive JSONSchemaProps nodes")
		}
		return
	}
	t.Fatal("no CustomResourceDefinition resource")
}

// TestClusterTUIOpensResource drives the real model against the cluster.
func TestClusterTUIOpensResource(t *testing.T) {
	l := newLoader(t)
	m, err := tui.New(tui.Config{Source: l, Context: "it", Open: "deployments", Ctx: ctx(t)})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	pump(t, m, m.Init())

	if got := m.Selected(); got == nil || got.Label != "Deployment" {
		t.Fatalf("selected = %v", got)
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "apps/v1/Deployment") {
		t.Errorf("detail pane missing GVK:\n%s", view)
	}
}

// pump runs commands and feeds their messages back until none remain,
// dropping timers.
func pump(t *testing.T, m *tui.Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			pump(t, m, c)
		}
		return
	}
	switch msg.(type) {
	case nil, spinner.TickMsg:
		return
	}
	_, next := m.Update(msg)
	pump(t, m, next)
}
