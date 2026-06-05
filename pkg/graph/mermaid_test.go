package graph

// mermaid_test.go — tests for RenderMermaid and Mermaid helpers.

import (
	"strings"
	"testing"
)

func TestRenderMermaid_RendersFlowchart(t *testing.T) {
	// Arrange
	g := &Graph{
		Nodes: []Node{
			{ID: "pkg/api", Label: "pkg/api", Package: "pkg/api"},
			{ID: "pkg/lib", Label: "pkg/lib", Package: "pkg/lib"},
		},
		Edges: []Edge{{
			Source:    "pkg/api",
			Target:    "pkg/lib",
			CallCount: 2,
			Functions: []string{"Helper", "Serve"},
		}},
	}
	var out strings.Builder

	// Act
	err := RenderMermaid(g, &out)

	// Assert
	if err != nil {
		t.Fatalf("RenderMermaid: %v", err)
	}
	text := out.String()
	if !strings.HasPrefix(text, "flowchart LR\n") {
		t.Fatalf("expected Mermaid header, got %q", text)
	}
	if !strings.Contains(text, `pkg_api["pkg/api"]`) {
		t.Errorf("expected node line for pkg/api, got %q", text)
	}
	if !strings.Contains(text, `pkg_lib["pkg/lib"]`) {
		t.Errorf("expected node line for pkg/lib, got %q", text)
	}
	if !strings.Contains(text, `pkg_api -->|2 calls| pkg_lib`) {
		t.Errorf("expected edge line, got %q", text)
	}
}

func TestSanitizeNodeID_PrefixesDigitAndNormalizesSeparators(t *testing.T) {
	// Arrange
	id := "1/pkg-api"

	// Act
	got := sanitizeNodeID(id)

	// Assert
	if got != "n_1_pkg_api" {
		t.Errorf("sanitizeNodeID: got %q, want %q", got, "n_1_pkg_api")
	}
}

func TestMermaidEscapeLabel_ReplacesDoubleQuotes(t *testing.T) {
	// Arrange
	label := `pkg/"api"`

	// Act
	got := mermaidEscapeLabel(label)

	// Assert
	if got != "pkg/'api'" {
		t.Errorf("mermaidEscapeLabel: got %q, want %q", got, "pkg/'api'")
	}
}
