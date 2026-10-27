package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/elliottsam/kubectl-schema-tree/internal/tree"
)

// syncDetail refreshes the detail viewport when the selection changes.
func (m *Model) syncDetail() {
	n := m.state.Selected()
	if n == m.detailNode {
		return
	}
	m.detailNode = n
	m.detail.SetContent(m.detailContent(n, m.detail.Width()))
	m.detail.GotoTop()
}

func (m *Model) renderDetail(w, h int) string {
	m.detail.SetWidth(max(1, w-2))
	m.detail.SetHeight(max(1, h))
	return fitBlock(indentLines(strings.Split(m.detail.View(), "\n")), w, h)
}

func indentLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = " " + l
	}
	return out
}

// resourceOf returns the resource kind node that contains n.
func resourceOf(n *tree.Node) *tree.Node {
	for ; n != nil; n = n.Parent {
		if n.Schema != nil && n.Schema.GVK != nil {
			return n
		}
	}
	return nil
}

// FieldPath returns the dotted path for n including its kind, for example
// "Deployment.spec.replicas".
func FieldPath(n *tree.Node) string {
	if n == nil || n.Schema == nil {
		return ""
	}
	root := resourceOf(n)
	if root == nil || root == n {
		return n.Label
	}
	return root.Label + "." + n.Schema.Path
}

func (m *Model) detailContent(n *tree.Node, w int) string {
	st := m.styles
	if n == nil {
		return st.Muted.Render("nothing selected")
	}
	if n.Schema == nil {
		return m.groupDetail(n, w)
	}
	sn := n.Schema
	var b strings.Builder
	b.WriteString(st.Title.Render(FieldPath(n)) + "\n")
	if root := resourceOf(n); root != nil {
		b.WriteString(st.Muted.Render(root.Schema.GVK.String()) + "\n")
	}
	b.WriteString("\n")

	typ := sn.TypeName
	if sn.Format != "" {
		typ += " (" + sn.Format + ")"
	}
	req := "optional"
	switch {
	case sn.GVK != nil:
		req = "resource"
	case sn.Required:
		req = "required"
	}
	b.WriteString(st.Kind(sn.Kind).Render(typ) + st.Muted.Render(" · "+req))
	if sn.Nullable {
		b.WriteString(st.Muted.Render(" · nullable"))
	}
	b.WriteString("\n")
	if sn.Deprecated {
		b.WriteString(st.Error.Render("deprecated") + "\n")
	}
	if sn.Recursive {
		b.WriteString(st.Muted.Render("↻ recursive type: expansion stops here") + "\n")
	}
	kv := func(k, v string) {
		if v != "" {
			b.WriteString(st.Key.Render(k+": ") + v + "\n")
		}
	}
	kv("default", sn.Default)
	if len(sn.Enum) > 0 {
		kv("enum", strings.Join(sn.Enum, ", "))
	}
	h := sn.Hints
	kv("list-type", h.ListType)
	if len(h.ListMapKeys) > 0 {
		kv("list-map-keys", strings.Join(h.ListMapKeys, ", "))
	}
	kv("map-type", h.MapType)
	if h.PreserveUnknownFields {
		kv("preserve-unknown-fields", "true")
	}
	if h.EmbeddedResource {
		kv("embedded-resource", "true")
	}
	if sn.Description != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Width(max(1, w)).Render(sn.Description))
	}
	return b.String()
}

func (m *Model) groupDetail(n *tree.Node, w int) string {
	st := m.styles
	var b strings.Builder
	b.WriteString(st.Title.Render(n.Label) + "\n\n")
	switch n.State() {
	case tree.Unloaded:
		b.WriteString(st.Muted.Render("Not loaded. Press → or enter to load its resources."))
	case tree.Loading:
		b.WriteString(st.Muted.Render("Loading…"))
	case tree.Failed:
		b.WriteString(st.Error.Render(failReason(n.Err)) + "\n\n")
		b.WriteString(lipgloss.NewStyle().Width(max(1, w)).Render(n.Err.Error()))
	default:
		fmt.Fprintf(&b, "%d resources", len(n.Children()))
	}
	return b.String()
}
