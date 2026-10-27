// Package search provides fuzzy matching over the nodes of a schema tree.
package search

import (
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/elliottsam/kubectl-schema-tree/internal/tree"
)

// DefaultMaxResults bounds the number of results returned for one query.
const DefaultMaxResults = 2000

// Config configures a Searcher.
type Config struct {
	// MaxDepth is how many levels below each loaded resource kind are
	// searched. Zero means DefaultMaxDepth.
	MaxDepth int
	// MaxResults bounds the results returned. Zero means DefaultMaxResults.
	MaxResults int
}

// DefaultMaxDepth is the field depth searched below each resource kind.
const DefaultMaxDepth = 10

// Result is one fuzzy match.
type Result struct {
	// Node is the matched tree node.
	Node *tree.Node
	// Text is the string that was matched, for example
	// "Deployment.spec.replicas".
	Text string
	// LabelIndexes are the matched character positions within Node.Label.
	LabelIndexes []int
}

// Searcher matches queries against loaded tree nodes.
type Searcher struct {
	maxDepth   int
	maxResults int
}

// New returns a Searcher with defaults applied to cfg.
func New(cfg Config) *Searcher {
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = DefaultMaxDepth
	}
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = DefaultMaxResults
	}
	return &Searcher{maxDepth: cfg.MaxDepth, maxResults: cfg.MaxResults}
}

type candidate struct {
	node *tree.Node
	text string
}

type source []candidate

func (s source) String(i int) string { return s[i].text }
func (s source) Len() int            { return len(s) }

// Find is not safe to call concurrently with other use of the same nodes,
// because it expands lazy children. It returns nodes under roots whose path fuzzy-matches query, best match
// first. Only loaded group/versions are searched. An empty query matches
// nothing.
func (s *Searcher) Find(roots []*tree.Node, query string) []Result {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	cands := s.collect(roots)
	matches := fuzzy.FindFrom(query, source(cands))
	if len(matches) > s.maxResults {
		matches = matches[:s.maxResults]
	}
	out := make([]Result, len(matches))
	for i, m := range matches {
		c := cands[m.Index]
		out[i] = Result{Node: c.node, Text: c.text, LabelIndexes: labelIndexes(c, m.MatchedIndexes)}
	}
	return out
}

func (s *Searcher) collect(roots []*tree.Node) []candidate {
	var out []candidate
	var walk func(n *tree.Node, kind string, depth int)
	walk = func(n *tree.Node, kind string, depth int) {
		switch {
		case n.Schema == nil:
			out = append(out, candidate{n, n.Label})
		case n.Schema.GVK != nil:
			kind, depth = n.Label, 0
			out = append(out, candidate{n, kind})
		default:
			out = append(out, candidate{n, kind + "." + n.Schema.Path})
		}
		if n.State() != tree.Loaded || depth >= s.maxDepth {
			return
		}
		for _, c := range n.Children() {
			walk(c, kind, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, "", 0)
	}
	return out
}

// labelIndexes keeps the matched positions that fall inside the node's label,
// which is always the final segment of the matched text.
func labelIndexes(c candidate, idx []int) []int {
	offset := len(c.text) - len(c.node.Label)
	var out []int
	for _, i := range idx {
		if i >= offset {
			out = append(out, i-offset)
		}
	}
	return out
}
