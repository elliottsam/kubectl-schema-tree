// Package tree holds the pure navigation state behind the schema tree: which
// rows are visible, where the cursor is, and what is expanded. It has no
// terminal or network dependencies.
package tree

import "github.com/elliottsam/kubectl-schema-tree/internal/schema"

// LoadState tracks lazy loading of a group/version node.
type LoadState int

// Load states.
const (
	// Loaded means children are available (possibly none).
	Loaded LoadState = iota
	// Unloaded means children must be fetched before the node can expand.
	Unloaded
	// Loading means a fetch is in flight.
	Loading
	// Failed means the last fetch failed; see Node.Err.
	Failed
)

// Node is one row candidate in the tree.
type Node struct {
	// Label is the text shown for the node.
	Label string
	// Schema is the backing schema node, or nil for group/version headers.
	Schema *schema.Node
	// Parent is nil for roots.
	Parent *Node
	// Key identifies a group/version header to the loader (a discovery path).
	Key string
	// Err is set when State is Failed.
	Err error

	state    LoadState
	expanded bool
	children []*Node
	lazy     func() []*Node
}

// NewNode returns a loaded node with no children.
func NewNode(label string) *Node { return &Node{Label: label} }

// NewGroup returns a group/version header whose children are loaded on demand
// via SetResources or SetError.
func NewGroup(label, key string) *Node {
	return &Node{Label: label, Key: key, state: Unloaded}
}

// FromSchema wraps a schema node, materialising its children lazily.
func FromSchema(sn *schema.Node, parent *Node) *Node {
	n := &Node{Label: sn.Name, Schema: sn, Parent: parent}
	if sn.Expandable() {
		n.lazy = func() []*Node {
			kids := sn.Children()
			out := make([]*Node, len(kids))
			for i, k := range kids {
				out[i] = FromSchema(k, n)
			}
			return out
		}
	}
	return n
}

// AddChild appends c as a child of n.
func (n *Node) AddChild(c *Node) *Node {
	c.Parent = n
	n.children = append(n.children, c)
	return c
}

// Children returns the node's children, materialising lazy schema children on
// first use.
func (n *Node) Children() []*Node {
	if n.lazy != nil {
		n.children = append(n.children, n.lazy()...)
		n.lazy = nil
	}
	return n.children
}

// Expandable reports whether expanding the node could reveal children.
func (n *Node) Expandable() bool {
	switch n.state {
	case Unloaded, Loading:
		return true
	case Failed:
		return false
	}
	return n.lazy != nil || len(n.children) > 0
}

// Expanded reports whether the node's children are shown.
func (n *Node) Expanded() bool { return n.expanded }

// State returns the node's load state.
func (n *Node) State() LoadState { return n.state }

// NeedsLoad reports whether the node is an unloaded group/version header.
func (n *Node) NeedsLoad() bool { return n.state == Unloaded }

// MarkLoading records that a fetch has started.
func (n *Node) MarkLoading() { n.state = Loading }

// SetResources completes a load with the group's resource kinds.
func (n *Node) SetResources(resources []*schema.Node) {
	n.children = n.children[:0]
	for _, r := range resources {
		n.children = append(n.children, FromSchema(r, n))
	}
	n.state = Loaded
	n.Err = nil
}

// SetError completes a load with a failure and collapses the node.
func (n *Node) SetError(err error) {
	n.state = Failed
	n.Err = err
	n.expanded = false
}

// Depth returns the number of ancestors.
func (n *Node) Depth() int {
	d := 0
	for p := n.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

// Ancestors returns the chain from the root down to (not including) n.
func (n *Node) Ancestors() []*Node {
	var out []*Node
	for p := n.Parent; p != nil; p = p.Parent {
		out = append([]*Node{p}, out...)
	}
	return out
}
