// Package schematest provides fakes and small documents for testing code that
// consumes the schema package without a cluster.
package schematest

import (
	"context"
	"sync/atomic"
	"testing"

	"k8s.io/client-go/openapi"

	"github.com/elliottsam/kubectl-schematree/internal/schema"
)

// Docs is a small two-group document set:
//
//	apis/apps/v1:  Widget (spec.replicas, spec.color), Gadget (replicas)
//	api/v1:        Pod (spec.nodeName, spec.recursive -> recursive Pod spec)
var Docs = map[string]string{
	"apis/apps/v1": `{"components":{"schemas":{
 "io.k8s.api.apps.v1.Widget":{"type":"object","description":"Widget is a thing.",
  "x-kubernetes-group-version-kind":[{"group":"apps","version":"v1","kind":"Widget"}],
  "required":["spec"],
  "properties":{
   "metadata":{"type":"object","description":"Standard metadata."},
   "spec":{"$ref":"#/components/schemas/io.k8s.api.apps.v1.WidgetSpec"}}},
 "io.k8s.api.apps.v1.Gadget":{"type":"object","description":"Gadget is another thing.",
  "x-kubernetes-group-version-kind":[{"group":"apps","version":"v1","kind":"Gadget"}],
  "properties":{"replicas":{"type":"integer","format":"int32","description":"Desired count.","default":1}}},
 "io.k8s.api.apps.v1.WidgetSpec":{"type":"object","description":"WidgetSpec is the spec.","required":["replicas"],
  "properties":{
   "replicas":{"type":"integer","format":"int32","description":"Number of replicas."},
   "color":{"type":"string","description":"Paint colour.","enum":["red","green"]},
   "tags":{"type":"array","items":{"type":"string"},"description":"Free tags."}}}}}}`,
	"api/v1": `{"components":{"schemas":{
 "io.k8s.api.core.v1.Pod":{"type":"object","description":"Pod is a pod.",
  "x-kubernetes-group-version-kind":[{"group":"","version":"v1","kind":"Pod"}],
  "properties":{"spec":{"$ref":"#/components/schemas/io.k8s.api.core.v1.PodSpec"}}},
 "io.k8s.api.core.v1.PodSpec":{"type":"object","description":"PodSpec.",
  "properties":{
   "nodeName":{"type":"string","description":"Node to run on."},
   "child":{"$ref":"#/components/schemas/io.k8s.api.core.v1.PodSpec"}}}}}}`,
}

// Discovery is a fake schema.Discovery serving canned documents.
type Discovery struct {
	// Docs maps a discovery path such as "api/v1" to its JSON document.
	Docs map[string]string
	// PathsErr, if set, is returned from Paths.
	PathsErr error
	// DocErr, if set, is returned when fetching any document.
	DocErr error
	// DocErrFor, if set, returns an error only for the given path.
	DocErrFor map[string]error

	fetches atomic.Int32
}

// NewDiscovery returns a Discovery over Docs.
func NewDiscovery() *Discovery { return &Discovery{Docs: Docs} }

// Fetches returns how many documents have been fetched.
func (d *Discovery) Fetches() int { return int(d.fetches.Load()) }

// OpenAPIV3 implements schema.Discovery.
func (d *Discovery) OpenAPIV3() openapi.Client { return client{d} }

type client struct{ d *Discovery }

func (c client) Paths() (map[string]openapi.GroupVersion, error) {
	if c.d.PathsErr != nil {
		return nil, c.d.PathsErr
	}
	out := map[string]openapi.GroupVersion{}
	for p := range c.d.Docs {
		out[p] = groupVersion{d: c.d, path: p}
	}
	return out, nil
}

type groupVersion struct {
	d    *Discovery
	path string
}

func (g groupVersion) Schema(string) ([]byte, error) {
	g.d.fetches.Add(1)
	if err := g.d.DocErrFor[g.path]; err != nil {
		return nil, err
	}
	if g.d.DocErr != nil {
		return nil, g.d.DocErr
	}
	return []byte(g.d.Docs[g.path]), nil
}

func (g groupVersion) ServerRelativeURL() string { return "/openapi/v3/" + g.path }

// Loader returns a schema.Loader over d, failing the test on error.
func Loader(tb testing.TB, d *Discovery) *schema.Loader {
	tb.Helper()
	l, err := schema.NewLoader(schema.Config{Discovery: d})
	if err != nil {
		tb.Fatalf("schematest: NewLoader: %v", err)
	}
	return l
}

// Resources loads the resources for path, failing the test on error.
func Resources(tb testing.TB, l *schema.Loader, path string) []*schema.Node {
	tb.Helper()
	res, err := l.Resources(context.Background(), path)
	if err != nil {
		tb.Fatalf("schematest: Resources(%q): %v", path, err)
	}
	return res
}
