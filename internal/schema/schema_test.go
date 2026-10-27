package schema_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi"

	s "github.com/elliottsam/kubectl-schema-tree/internal/schema"
)

// fakeDiscovery serves canned OpenAPI v3 documents.
type fakeDiscovery struct {
	docs     map[string][]byte
	pathsErr error
	docErr   error
	fetches  atomic.Int32
}

func (f *fakeDiscovery) OpenAPIV3() openapi.Client { return f }

func (f *fakeDiscovery) Paths() (map[string]openapi.GroupVersion, error) {
	if f.pathsErr != nil {
		return nil, f.pathsErr
	}
	out := map[string]openapi.GroupVersion{
		".well-known/openid-configuration": &fakeGV{f: f},
		"api":                              &fakeGV{f: f},
		"apis":                             &fakeGV{f: f},
		"apis/apps":                        &fakeGV{f: f},
		"version":                          &fakeGV{f: f},
	}
	for p := range f.docs {
		out[p] = &fakeGV{f: f, path: p}
	}
	return out, nil
}

type fakeGV struct {
	f    *fakeDiscovery
	path string
}

func (g *fakeGV) Schema(string) ([]byte, error) {
	g.f.fetches.Add(1)
	if g.f.docErr != nil {
		return nil, g.f.docErr
	}
	return g.f.docs[g.path], nil
}

func (g *fakeGV) ServerRelativeURL() string { return "/openapi/v3/" + g.path }

func fixture(tb testing.TB, name string) []byte {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		tb.Fatalf("read fixture: %v", err)
	}
	return data
}

func newFake(tb testing.TB) *fakeDiscovery {
	tb.Helper()
	return &fakeDiscovery{docs: map[string][]byte{
		"api/v1":                       fixture(tb, "api_v1.json"),
		"apis/apps/v1":                 fixture(tb, "apis_apps_v1.json"),
		"apis/apiextensions.k8s.io/v1": fixture(tb, "apis_apiextensions.k8s.io_v1.json"),
	}}
}

func newLoader(tb testing.TB, d s.Discovery) *s.Loader {
	tb.Helper()
	l, err := s.NewLoader(s.Config{Discovery: d})
	if err != nil {
		tb.Fatalf("NewLoader: %v", err)
	}
	return l
}

// descend follows a path of child names from n, failing the test if any is missing.
func descend(tb testing.TB, n *s.Node, names ...string) *s.Node {
	tb.Helper()
	for _, name := range names {
		var next *s.Node
		for _, c := range n.Children() {
			if c.Name == name {
				next = c
				break
			}
		}
		if next == nil {
			tb.Fatalf("%q has no child %q", n.Path, name)
		}
		n = next
	}
	return n
}

func findRoot(tb testing.TB, roots []*s.Node, kind string) *s.Node {
	tb.Helper()
	for _, r := range roots {
		if r.Name == kind {
			return r
		}
	}
	tb.Fatalf("no root %q", kind)
	return nil
}

func TestNewLoaderValidatesConfig(t *testing.T) {
	t.Parallel()
	_, err := s.NewLoader(s.Config{})
	if !errors.Is(err, s.ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestGroupVersions(t *testing.T) {
	t.Parallel()
	l := newLoader(t, newFake(t))
	got, err := l.GroupVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api/v1", "apis/apiextensions.k8s.io/v1", "apis/apps/v1"}
	if !slices.Equal(got, want) {
		t.Fatalf("GroupVersions = %v, want %v", got, want)
	}
}

func TestDisplayGroupVersion(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"api/v1":       "v1",
		"apis/apps/v1": "apps/v1",
		"other":        "other",
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if got := s.DisplayGroupVersion(in); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestResourcesRoots(t *testing.T) {
	t.Parallel()
	l := newLoader(t, newFake(t))
	roots, err := l.Resources(context.Background(), "apis/apps/v1")
	if err != nil {
		t.Fatal(err)
	}
	dep := findRoot(t, roots, "Deployment")
	wantGVK := s.GVK{Group: "apps", Version: "v1", Kind: "Deployment"}
	if dep.GVK == nil || *dep.GVK != wantGVK {
		t.Fatalf("GVK = %v, want %v", dep.GVK, wantGVK)
	}
	for _, r := range roots {
		if strings.HasSuffix(r.Name, "List") || r.Name == "DeleteOptions" {
			t.Errorf("unexpected non-resource root %q", r.Name)
		}
	}
}

func TestNodeNavigation(t *testing.T) {
	t.Parallel()
	l := newLoader(t, newFake(t))
	roots, err := l.Resources(context.Background(), "apis/apps/v1")
	if err != nil {
		t.Fatal(err)
	}
	dep := findRoot(t, roots, "Deployment")

	tests := []struct {
		name     string
		path     []string
		wantPath string
		kind     s.Kind
		typeName string
		required bool
		format   string
	}{
		{"replicas", []string{"spec", "replicas"}, "spec.replicas", s.KindInteger, "integer", false, "int32"},
		{"selector-required", []string{"spec", "selector"}, "spec.selector", s.KindObject, "LabelSelector", true, ""},
		{"containers-array", []string{"spec", "template", "spec", "containers"}, "spec.template.spec.containers", s.KindArray, "[]Container", true, ""},
		{"container-image", []string{"spec", "template", "spec", "containers", "image"}, "spec.template.spec.containers[].image", s.KindString, "string", false, ""},
		{"labels-map", []string{"metadata", "labels"}, "metadata.labels", s.KindMap, "map[string]string", false, ""},
		{"int-or-string", []string{"spec", "strategy", "rollingUpdate", "maxSurge"}, "spec.strategy.rollingUpdate.maxSurge", s.KindString, "int-or-string", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := descend(t, dep, tt.path...)
			if n.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", n.Path, tt.wantPath)
			}
			if n.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", n.Kind, tt.kind)
			}
			if n.TypeName != tt.typeName {
				t.Errorf("TypeName = %q, want %q", n.TypeName, tt.typeName)
			}
			if n.Required != tt.required {
				t.Errorf("Required = %v, want %v", n.Required, tt.required)
			}
			if n.Format != tt.format {
				t.Errorf("Format = %q, want %q", n.Format, tt.format)
			}
			if n.Description == "" {
				t.Error("Description is empty")
			}
		})
	}
}

func TestEnumAndHints(t *testing.T) {
	t.Parallel()
	l := newLoader(t, newFake(t))
	roots, err := l.Resources(context.Background(), "api/v1")
	if err != nil {
		t.Fatal(err)
	}
	svc := findRoot(t, roots, "Service")

	typ := descend(t, svc, "spec", "type")
	if len(typ.Enum) == 0 {
		t.Errorf("spec.type has no enum values")
	}
	ports := descend(t, svc, "spec", "ports")
	if ports.Hints.ListType != "map" || !slices.Equal(ports.Hints.ListMapKeys, []string{"port", "protocol"}) {
		t.Errorf("ports hints = %+v", ports.Hints)
	}
}

func TestExpandable(t *testing.T) {
	t.Parallel()
	l := newLoader(t, newFake(t))
	roots, _ := l.Resources(context.Background(), "apis/apps/v1")
	dep := findRoot(t, roots, "Deployment")
	if !dep.Expandable() {
		t.Error("Deployment should be expandable")
	}
	if replicas := descend(t, dep, "spec", "replicas"); replicas.Expandable() {
		t.Error("replicas should not be expandable")
	}
}

func TestRecursiveSchemaTerminates(t *testing.T) {
	t.Parallel()
	l := newLoader(t, newFake(t))
	roots, err := l.Resources(context.Background(), "apis/apiextensions.k8s.io/v1")
	if err != nil {
		t.Fatal(err)
	}
	crd := findRoot(t, roots, "CustomResourceDefinition")

	var visited, recursive int
	crd.Walk(30, func(n *s.Node) bool {
		visited++
		if n.Recursive {
			recursive++
			if n.Expandable() || len(n.Children()) != 0 {
				t.Errorf("recursive node %q must not expand", n.Path)
			}
		}
		return true
	})
	if recursive == 0 {
		t.Fatal("expected recursive nodes in JSONSchemaProps")
	}
	if visited > 200000 {
		t.Fatalf("walk visited %d nodes; recursion not bounded", visited)
	}
}

func TestWalkDepth(t *testing.T) {
	t.Parallel()
	l := newLoader(t, newFake(t))
	roots, _ := l.Resources(context.Background(), "apis/apps/v1")
	dep := findRoot(t, roots, "Deployment")
	var names []string
	dep.Walk(0, func(n *s.Node) bool { names = append(names, n.Name); return true })
	if !slices.Equal(names, []string{"Deployment"}) {
		t.Fatalf("depth 0 visited %v", names)
	}
	count := 0
	dep.Walk(1, func(*s.Node) bool { count++; return false })
	if count != 1 {
		t.Fatalf("skip children visited %d, want 1", count)
	}
}

func TestErrorMapping(t *testing.T) {
	t.Parallel()
	gr := schema.GroupResource{Group: "x", Resource: "y"}
	tests := []struct {
		name  string
		setup func(*fakeDiscovery)
		call  func(*s.Loader) error
		want  error
	}{
		{
			name:  "openapi-v3-missing",
			setup: func(f *fakeDiscovery) { f.pathsErr = apierrors.NewNotFound(gr, "openapi") },
			call:  func(l *s.Loader) error { _, err := l.GroupVersions(context.Background()); return err },
			want:  s.ErrOpenAPIV3Unsupported,
		},
		{
			name:  "paths-forbidden",
			setup: func(f *fakeDiscovery) { f.pathsErr = apierrors.NewForbidden(gr, "openapi", errors.New("no")) },
			call:  func(l *s.Loader) error { _, err := l.GroupVersions(context.Background()); return err },
			want:  s.ErrForbidden,
		},
		{
			name:  "document-forbidden",
			setup: func(f *fakeDiscovery) { f.docErr = apierrors.NewForbidden(gr, "doc", errors.New("no")) },
			call:  func(l *s.Loader) error { _, err := l.Resources(context.Background(), "apis/apps/v1"); return err },
			want:  s.ErrForbidden,
		},
		{
			name:  "unknown-group-version",
			setup: func(*fakeDiscovery) {},
			call:  func(l *s.Loader) error { _, err := l.Resources(context.Background(), "apis/nope/v1"); return err },
			want:  s.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFake(t)
			tt.setup(f)
			err := tt.call(newLoader(t, f))
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestMalformedDocument(t *testing.T) {
	t.Parallel()
	f := &fakeDiscovery{docs: map[string][]byte{"api/v1": []byte(`{"components": "oops"`)}}
	_, err := newLoader(t, f).Resources(context.Background(), "api/v1")
	if err == nil {
		t.Fatal("expected error for malformed document")
	}
	if errors.Is(err, s.ErrForbidden) || errors.Is(err, s.ErrOpenAPIV3Unsupported) {
		t.Fatalf("malformed document mapped to wrong sentinel: %v", err)
	}
}

func TestDocumentCached(t *testing.T) {
	t.Parallel()
	f := newFake(t)
	l := newLoader(t, f)
	for range 3 {
		if _, err := l.Resources(context.Background(), "apis/apps/v1"); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.fetches.Load(); got != 1 {
		t.Fatalf("document fetched %d times, want 1", got)
	}
}

func FuzzLoadDocument(f *testing.F) {
	for _, name := range []string{"api_v1.json", "apis_apps_v1.json"} {
		f.Add(fixture(f, name))
	}
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"components":{"schemas":{"a":{"$ref":"#/components/schemas/a"}}}}`))
	f.Add([]byte(`{"components":{"schemas":{"a":{"x-kubernetes-group-version-kind":[{"group":"","version":"v1","kind":"A"}],"properties":{"b":{"$ref":"#/components/schemas/a"}}}}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		d := &fakeDiscovery{docs: map[string][]byte{"api/v1": data}}
		l, err := s.NewLoader(s.Config{Discovery: d})
		if err != nil {
			t.Fatal(err)
		}
		roots, err := l.Resources(context.Background(), "api/v1")
		if err != nil {
			return
		}
		for _, r := range roots {
			r.Walk(8, func(*s.Node) bool { return true })
		}
	})
}

func BenchmarkWalkDeployment(b *testing.B) {
	l := newLoader(b, newFake(b))
	b.ReportAllocs()
	for b.Loop() {
		roots, err := l.Resources(context.Background(), "apis/apps/v1")
		if err != nil {
			b.Fatal(err)
		}
		for _, r := range roots {
			if r.Name == "Deployment" {
				r.Walk(12, func(*s.Node) bool { return true })
			}
		}
	}
}
