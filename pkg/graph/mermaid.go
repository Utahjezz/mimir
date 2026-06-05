package graph

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// RenderMermaid renders g as a valid Mermaid flowchart to w.
// It is a pure renderer with no side effects, no database access,
// and no output except through the provided io.Writer.
func RenderMermaid(g *Graph, w io.Writer) error {
	if _, err := io.WriteString(w, "flowchart LR\n"); err != nil {
		return fmt.Errorf("write mermaid header: %w", err)
	}

	for _, node := range g.Nodes {
		id := sanitizeNodeID(node.ID)
		label := node.Label
		if node.RepoID != "" {
			label = node.RepoID + "." + label
		}
		if _, err := fmt.Fprintf(w, "    %s[\"%s\"]\n", id, mermaidEscapeLabel(label)); err != nil {
			return fmt.Errorf("write mermaid node: %w", err)
		}
	}

	for _, edge := range g.Edges {
		src := sanitizeNodeID(edge.Source)
		dst := sanitizeNodeID(edge.Target)

		var line string
		if edge.IsCrossRepo {
			line = fmt.Sprintf("    %s -.->|%d calls [ext]| %s", src, edge.CallCount, dst)
		} else {
			line = fmt.Sprintf("    %s -->|%d calls| %s", src, edge.CallCount, dst)
		}
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return fmt.Errorf("write mermaid edge: %w", err)
		}
	}

	return nil
}

// sanitizeNodeID converts an arbitrary string into a valid Mermaid node ID.
// Mermaid node IDs may only contain alphanumeric characters and underscores,
// and must start with a letter. Non-conforming characters are replaced with
// underscores, and a "n_" prefix is added if the result would start with a digit.
//
// This is a best-effort transformation for Go package paths (e.g. "pkg/graph"
// becomes "pkg_graph"). Two distinct inputs that collide after sanitization
// (e.g. "a/b" and "a_b") are not disambiguated; such collisions are extremely
// rare in practice and can be addressed if they arise.
func sanitizeNodeID(id string) string {
	if id == "" {
		return "n"
	}

	var b strings.Builder
	b.Grow(len(id) + 2)

	// If the first character is not a letter, prepend "n_" to ensure
	// the resulting ID starts with a letter as Mermaid requires.
	needsPrefix := false
	for i, r := range id {
		if i == 0 && !unicode.IsLetter(r) {
			needsPrefix = true
		}
		break
	}
	if needsPrefix {
		b.WriteString("n_")
	}

	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}

	result := b.String()
	if result == "" || result == "n_" {
		return "n"
	}
	return result
}

// mermaidEscapeLabel escapes special characters in a Mermaid label string.
// Double quotes are replaced with single quotes to avoid breaking the
// "Label" syntax used in node declarations.
func mermaidEscapeLabel(label string) string {
	return strings.ReplaceAll(label, "\"", "'")
}
