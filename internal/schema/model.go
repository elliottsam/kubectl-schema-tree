// Package schema loads Kubernetes OpenAPI v3 documents and exposes them as a
// lazily expanded tree of Nodes that does not depend on any OpenAPI library
// types.
package schema

import (
	"strings"
	"sync"
)

// Kind classifies the shape of a Node for display and navigation.
type Kind int

// Node kinds.
const (
	KindObject Kind = iota
	KindArray
	KindMap
	KindString
	KindInteger
	KindNumber
	KindBoolean
	KindUnknown
)

// String returns the lower-case name of the kind.
func (k Kind) String() string {
	switch k {
	case KindObject:
		return "object"
	case KindArray:
		return "array"
	case KindMap:
		return "map"
	case KindString:
		return "string"
	case KindInteger:
		return "integer"
	case KindNumber:
		return "number"
	case KindBoolean:
		return "boolean"
	default:
		return "unknown"
	}
}

// GVK identifies a Kubernetes resource kind.
type GVK struct {
	Group   string
	Version string
	Kind    string
}

// String renders the GVK as group/version/Kind, omitting an empty group.
func (g GVK) String() string {
	if g.Group == "" {
		return g.Version + "/" + g.Kind
	}
	return g.Group + "/" + g.Version + "/" + g.Kind
}

// Hints carries the x-kubernetes-* extensions that matter for display.
type Hints struct {
	ListType              string
	ListMapKeys           []string
	MapType               string
	IntOrString           bool
	PreserveUnknownFields bool
	EmbeddedResource      bool
}

// Node is one entry in the schema tree. Children are computed on first use and
// cached; a Node must be used from a single goroutine at a time.
type Node struct {
	// Name is the field name, or the resource kind for a root node.
	Name string
	// Path is the dotted field path from the resource root, using "[]" after
	// array fields, for example "spec.containers[].image".
	Path string
	// Kind is the shape of the value.
	Kind Kind
	// TypeName is the display type, for example "integer", "[]Container" or
	// "map[string]string".
	TypeName string
	// Format is the OpenAPI format, for example "int32".
	Format string
	// Description is the field documentation.
	Description string
	// Required reports whether the parent lists the field as required.
	Required bool
	// Deprecated reports whether the description marks the field deprecated.
	Deprecated bool
	// Nullable reports whether the schema permits null.
	Nullable bool
	// Enum lists the allowed values, rendered as strings.
	Enum []string
	// Default is the default value rendered as JSON, or empty.
	Default string
	// Hints holds x-kubernetes-* extensions.
	Hints Hints
	// Recursive marks a node whose type already appears among its ancestors;
	// it has no children to avoid infinite expansion.
	Recursive bool
	// GVK is set on resource root nodes only.
	GVK *GVK

	once     sync.Once
	children []*Node
	build    func() []*Node
}

// Children returns the child nodes, expanding them on first call.
func (n *Node) Children() []*Node {
	n.once.Do(func() {
		if n.build != nil {
			n.children = n.build()
			n.build = nil
		}
	})
	return n.children
}

// Expandable reports whether the node may have children. It does not trigger
// expansion, so it can be used when rendering collapsed rows.
func (n *Node) Expandable() bool {
	if n.Recursive {
		return false
	}
	return n.build != nil || len(n.children) > 0
}

// Walk visits the node and its descendants depth-first, up to maxDepth levels
// below the node (0 visits only the node). Returning false from fn skips the
// node's children.
func (n *Node) Walk(maxDepth int, fn func(*Node) bool) {
	if !fn(n) || maxDepth <= 0 {
		return
	}
	for _, c := range n.Children() {
		c.Walk(maxDepth-1, fn)
	}
}

func isDeprecated(desc string) bool {
	return strings.Contains(strings.ToLower(desc), "deprecated")
}
