package schema

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

const (
	extGVK          = "x-kubernetes-group-version-kind"
	extListType     = "x-kubernetes-list-type"
	extListMapKeys  = "x-kubernetes-list-map-keys"
	extMapType      = "x-kubernetes-map-type"
	extIntOrString  = "x-kubernetes-int-or-string"
	extPreserve     = "x-kubernetes-preserve-unknown-fields"
	extEmbeddedRes  = "x-kubernetes-embedded-resource"
	refPrefix       = "#/components/schemas/"
	maxRefHops      = 32
	arrayType       = "array"
	objectType      = "object"
	mapTypePrefix   = "map[string]"
	arrayTypePrefix = "[]"
	arrayPathSuffix = "[]"
	mapPathSuffix   = "{}"
	intOrStringName = "int-or-string"
)

// nonResourceKinds are schemas that carry a group-version-kind extension in
// every group but are not resources a user would browse.
var nonResourceKinds = map[string]bool{
	"DeleteOptions": true,
	"WatchEvent":    true,
	"Status":        true,
	"Patch":         true,
}

// document is one parsed group/version OpenAPI v3 document.
type document struct {
	gv      string
	schemas map[string]*spec.Schema
}

// parseDocument decodes an OpenAPI v3 document. It never panics on malformed
// input; decode failures are returned as errors.
func parseDocument(gv string, data []byte) (*document, error) {
	var api spec3.OpenAPI
	if err := json.Unmarshal(data, &api); err != nil {
		return nil, fmt.Errorf("decode %s: %w", gv, err)
	}
	d := &document{gv: gv, schemas: map[string]*spec.Schema{}}
	if api.Components != nil {
		for name, s := range api.Components.Schemas {
			if s != nil {
				d.schemas[name] = s
			}
		}
	}
	return d, nil
}

// roots returns one root Node per resource kind served by this document.
func (d *document) roots() []*Node {
	group, version := splitGV(d.gv)
	names := make([]string, 0, len(d.schemas))
	for name := range d.schemas {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []*Node
	for _, name := range names {
		s := d.schemas[name]
		gvk, ok := matchGVK(s, group, version)
		if !ok || nonResourceKinds[gvk.Kind] || strings.HasSuffix(gvk.Kind, "List") {
			continue
		}
		n := d.newNode(gvk.Kind, "", s, false, []string{name})
		n.GVK = &gvk
		n.Path = ""
		n.TypeName = gvk.Kind
		out = append(out, n)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// resolved is a schema with its $ref chain followed.
type resolved struct {
	schema  *spec.Schema
	refName string
	chain   []string
	cycle   bool
}

// refOf returns the schema name s points at, handling the allOf-wrapped form
// the Kubernetes API server emits for fields with descriptions.
func refOf(s *spec.Schema) string {
	if name, ok := strings.CutPrefix(s.Ref.String(), refPrefix); ok {
		return name
	}
	if len(s.AllOf) == 1 {
		if name, ok := strings.CutPrefix(s.AllOf[0].Ref.String(), refPrefix); ok {
			return name
		}
	}
	return ""
}

// resolve follows $refs from s, stopping at a cycle or an unknown reference.
func (d *document) resolve(s *spec.Schema, chain []string) resolved {
	r := resolved{schema: s, chain: chain}
	for range maxRefHops {
		name := refOf(r.schema)
		if name == "" {
			return r
		}
		r.refName = name
		if slices.Contains(r.chain, name) {
			r.cycle = true
			return r
		}
		target, ok := d.schemas[name]
		if !ok {
			return r
		}
		r.chain = append(slices.Clone(r.chain), name)
		r.schema = target
	}
	r.cycle = true
	return r
}

// shape describes the displayable form of a schema after unwrapping arrays
// and maps down to the value that supplies children.
type shape struct {
	kind     Kind
	typeName string
	suffix   string
	value    resolved
	cycle    bool
}

func (d *document) shape(s *spec.Schema, chain []string) shape {
	r := d.resolve(s, chain)
	if r.cycle {
		return shape{kind: KindObject, typeName: shortName(r.refName), value: r, cycle: true}
	}
	sc := r.schema
	switch {
	case sc.Type.Contains(arrayType) && sc.Items != nil && sc.Items.Schema != nil:
		inner := d.shape(sc.Items.Schema, r.chain)
		return shape{kind: KindArray, typeName: arrayTypePrefix + inner.typeName, suffix: arrayPathSuffix + inner.suffix, value: inner.value, cycle: inner.cycle}
	case len(sc.Properties) == 0 && sc.AdditionalProperties != nil && sc.AdditionalProperties.Schema != nil:
		inner := d.shape(sc.AdditionalProperties.Schema, r.chain)
		return shape{kind: KindMap, typeName: mapTypePrefix + inner.typeName, suffix: mapPathSuffix + inner.suffix, value: inner.value, cycle: inner.cycle}
	case isIntOrString(sc):
		return shape{kind: KindString, typeName: intOrStringName, value: r}
	case sc.Type.Contains("string"):
		return shape{kind: KindString, typeName: "string", value: r}
	case sc.Type.Contains("integer"):
		return shape{kind: KindInteger, typeName: "integer", value: r}
	case sc.Type.Contains("number"):
		return shape{kind: KindNumber, typeName: "number", value: r}
	case sc.Type.Contains("boolean"):
		return shape{kind: KindBoolean, typeName: "boolean", value: r}
	case len(sc.Properties) > 0 || sc.Type.Contains(objectType) || r.refName != "":
		name := objectType
		if r.refName != "" {
			name = shortName(r.refName)
		}
		return shape{kind: KindObject, typeName: name, value: r}
	default:
		return shape{kind: KindUnknown, typeName: "any", value: r}
	}
}

// newNode builds the Node for schema s. The returned node expands lazily.
func (d *document) newNode(name, parentPath string, s *spec.Schema, required bool, chain []string) *Node {
	sh := d.shape(s, chain)
	r := sh.value
	desc := s.Description
	if desc == "" && r.schema != nil {
		desc = r.schema.Description
	}
	n := &Node{
		Name:        name,
		Path:        joinPath(parentPath, name),
		Kind:        sh.kind,
		TypeName:    sh.typeName,
		Description: desc,
		Required:    required,
		Deprecated:  isDeprecated(desc),
		Recursive:   sh.cycle,
	}
	if sc := r.schema; sc != nil {
		n.Format = sc.Format
		if n.Format == intOrStringName {
			n.Format = ""
		}
		n.Nullable = s.Nullable || sc.Nullable
		n.Enum = enumStrings(firstNonEmpty(s.Enum, sc.Enum))
		n.Default = jsonString(firstNonNil(s.Default, sc.Default))
		n.Hints = mergeHints(hintsOf(s), hintsOf(sc))
	}
	if !sh.cycle && r.schema != nil && len(r.schema.Properties) > 0 {
		n.build = func() []*Node { return d.children(n.Path+sh.suffix, r) }
	}
	return n
}

func (d *document) children(path string, r resolved) []*Node {
	names := make([]string, 0, len(r.schema.Properties))
	for name := range r.schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]*Node, 0, len(names))
	for _, name := range names {
		prop := r.schema.Properties[name]
		out = append(out, d.newNode(name, path, &prop, slices.Contains(r.schema.Required, name), r.chain))
	}
	return out
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

func shortName(ref string) string {
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func splitGV(gv string) (group, version string) {
	group, version, ok := strings.Cut(gv, "/")
	if !ok {
		return "", gv
	}
	return group, version
}

func matchGVK(s *spec.Schema, group, version string) (GVK, bool) {
	raw, ok := s.Extensions[extGVK]
	if !ok {
		return GVK{}, false
	}
	var entries []map[string]any
	switch v := raw.(type) {
	case []any:
		for _, e := range v {
			if m, ok := e.(map[string]any); ok {
				entries = append(entries, m)
			}
		}
	case []map[string]any:
		entries = v
	}
	for _, m := range entries {
		g, _ := m["group"].(string)
		v, _ := m["version"].(string)
		k, _ := m["kind"].(string)
		if g == group && v == version && k != "" {
			return GVK{Group: g, Version: v, Kind: k}, true
		}
	}
	return GVK{}, false
}

func isIntOrString(s *spec.Schema) bool {
	return boolExt(s, extIntOrString) || s.Format == intOrStringName
}

func firstNonEmpty[T any](a, b []T) []T {
	if len(a) > 0 {
		return a
	}
	return b
}

func firstNonNil(a, b any) any {
	if a != nil {
		return a
	}
	return b
}

// mergeHints prefers values set on the referencing (outer) schema and falls
// back to the referenced one.
func mergeHints(outer, inner Hints) Hints {
	if outer.ListType == "" {
		outer.ListType = inner.ListType
	}
	if len(outer.ListMapKeys) == 0 {
		outer.ListMapKeys = inner.ListMapKeys
	}
	if outer.MapType == "" {
		outer.MapType = inner.MapType
	}
	outer.IntOrString = outer.IntOrString || inner.IntOrString
	outer.PreserveUnknownFields = outer.PreserveUnknownFields || inner.PreserveUnknownFields
	outer.EmbeddedResource = outer.EmbeddedResource || inner.EmbeddedResource
	return outer
}

func boolExt(s *spec.Schema, key string) bool {
	b, _ := s.Extensions[key].(bool)
	return b
}

func stringExt(s *spec.Schema, key string) string {
	v, _ := s.Extensions[key].(string)
	return v
}

func hintsOf(s *spec.Schema) Hints {
	h := Hints{
		ListType:              stringExt(s, extListType),
		MapType:               stringExt(s, extMapType),
		IntOrString:           isIntOrString(s),
		PreserveUnknownFields: boolExt(s, extPreserve),
		EmbeddedResource:      boolExt(s, extEmbeddedRes),
	}
	if keys, ok := s.Extensions[extListMapKeys].([]any); ok {
		for _, k := range keys {
			if str, ok := k.(string); ok {
				h.ListMapKeys = append(h.ListMapKeys, str)
			}
		}
	}
	return h
}

func enumStrings(vals []any) []string {
	if len(vals) == 0 {
		return nil
	}
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = fmt.Sprint(v)
	}
	return out
}

func jsonString(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
