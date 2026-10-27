package schema

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/openapi"
)

const contentTypeJSON = "application/json"

// Discovery is the part of a Kubernetes discovery client the Loader needs.
// discovery.DiscoveryInterface satisfies it.
type Discovery interface {
	OpenAPIV3() openapi.Client
}

// Config configures a Loader.
type Config struct {
	// Discovery supplies the OpenAPI v3 client. Required.
	Discovery Discovery
}

// Validate reports whether the Config can be used to build a Loader.
func (c Config) Validate() error {
	if c.Discovery == nil {
		return fmt.Errorf("%w: Discovery is required", ErrInvalidConfig)
	}
	return nil
}

// Loader reads group/version schema documents from a cluster and caches the
// parsed result. It is safe for concurrent use.
type Loader struct {
	discovery Discovery

	mu   sync.Mutex
	docs map[string]*document
}

// NewLoader validates cfg and returns a Loader.
func NewLoader(cfg Config) (*Loader, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Loader{discovery: cfg.Discovery, docs: map[string]*document{}}, nil
}

// GroupVersions lists the served group/version paths such as "api/v1" and
// "apis/apps/v1", sorted with the core group first.
func (l *Loader) GroupVersions(ctx context.Context) ([]string, error) {
	paths, err := l.paths(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(paths))
	for p := range paths {
		if isGroupVersionPath(p) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ci, cj := strings.HasPrefix(out[i], "api/"), strings.HasPrefix(out[j], "api/")
		if ci != cj {
			return ci
		}
		return out[i] < out[j]
	})
	return out, nil
}

// Resources returns the resource kinds served by the group/version at path.
func (l *Loader) Resources(ctx context.Context, path string) ([]*Node, error) {
	doc, err := l.document(ctx, path)
	if err != nil {
		return nil, err
	}
	return doc.roots(), nil
}

// DisplayGroupVersion converts a discovery path to the kubectl notation:
// "api/v1" becomes "v1" and "apis/apps/v1" becomes "apps/v1".
func DisplayGroupVersion(path string) string {
	if v, ok := strings.CutPrefix(path, "apis/"); ok {
		return v
	}
	if v, ok := strings.CutPrefix(path, "api/"); ok {
		return v
	}
	return path
}

func (l *Loader) paths(ctx context.Context) (map[string]openapi.GroupVersionWithContext, error) {
	client := openapi.ToClientWithContext(l.discovery.OpenAPIV3())
	paths, err := client.PathsWithContext(ctx)
	if err != nil {
		return nil, classify("list OpenAPI v3 paths", err, true)
	}
	return paths, nil
}

func (l *Loader) document(ctx context.Context, path string) (*document, error) {
	l.mu.Lock()
	doc, ok := l.docs[path]
	l.mu.Unlock()
	if ok {
		return doc, nil
	}

	paths, err := l.paths(ctx)
	if err != nil {
		return nil, err
	}
	gv, ok := paths[path]
	if !ok {
		return nil, fmt.Errorf("%w: group/version %q", ErrNotFound, path)
	}
	data, err := gv.SchemaWithContext(ctx, contentTypeJSON)
	if err != nil {
		return nil, classify("load "+path, err, false)
	}
	doc, err = parseDocument(DisplayGroupVersion(path), data)
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	l.docs[path] = doc
	l.mu.Unlock()
	return doc, nil
}

// classify maps API errors onto the package sentinels while keeping the
// original error in the chain.
func classify(op string, err error, discovery bool) error {
	switch {
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		return fmt.Errorf("%s: %w: %w", op, ErrForbidden, err)
	case discovery && apierrors.IsNotFound(err):
		return fmt.Errorf("%s: %w: %w", op, ErrOpenAPIV3Unsupported, err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s: %w", op, err)
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}

func isGroupVersionPath(p string) bool {
	parts := strings.Split(p, "/")
	switch parts[0] {
	case "api":
		return len(parts) == 2
	case "apis":
		return len(parts) == 3
	default:
		return false
	}
}
