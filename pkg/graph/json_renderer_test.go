package graph

// json_renderer_test.go — tests for RenderJSON.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRenderJSON_SerializesGraph(t *testing.T) {
	// Arrange
	g := &Graph{
		Nodes: []Node{
			{ID: "pkg/api", Label: "pkg/api", Package: "pkg/api", CallCount: 3},
			{ID: "pkg/lib", Label: "pkg/lib", Package: "pkg/lib", CallCount: 2},
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
	err := RenderJSON(g, &out)

	// Assert
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}

	var got Graph
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput=%q", err, out.String())
	}
	if !reflect.DeepEqual(got, *g) {
		t.Errorf("decoded graph mismatch:\n got: %#v\nwant: %#v", got, *g)
	}
}
