package graph

// filter_test.go — tests for FilterGraph.

import (
	"reflect"
	"testing"
)

func TestFilterGraph_FocusKeepsMatchingNodesAndEdges(t *testing.T) {
	// Arrange
	g := &Graph{
		Nodes: []Node{
			{ID: "svc/foo", Package: "svc/foo"},
			{ID: "svc/foo/internal", Package: "svc/foo/internal"},
			{ID: "svc/bar", Package: "svc/bar"},
		},
		Edges: []Edge{
			{Source: "svc/foo", Target: "svc/foo/internal", CallCount: 3},
			{Source: "svc/foo/internal", Target: "svc/bar", CallCount: 2},
		},
	}

	// Act
	got := FilterGraph(g, GraphOptions{Focus: "foo"})

	// Assert
	if len(got.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %#v", len(got.Nodes), got.Nodes)
	}
	if len(got.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %#v", len(got.Edges), got.Edges)
	}
	if got.Edges[0].Source != "svc/foo" || got.Edges[0].Target != "svc/foo/internal" {
		t.Errorf("unexpected remaining edge: %#v", got.Edges[0])
	}
}

func TestFilterGraph_ExcludeRemovesMatchingNodesAndEdges(t *testing.T) {
	// Arrange
	g := &Graph{
		Nodes: []Node{
			{ID: "svc/foo", Package: "svc/foo"},
			{ID: "svc/bar", Package: "svc/bar"},
			{ID: "svc/baz", Package: "svc/baz"},
		},
		Edges: []Edge{
			{Source: "svc/foo", Target: "svc/bar", CallCount: 2},
			{Source: "svc/bar", Target: "svc/baz", CallCount: 2},
		},
	}

	// Act
	got := FilterGraph(g, GraphOptions{Exclude: "bar"})

	// Assert
	if len(got.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %#v", len(got.Nodes), got.Nodes)
	}
	if len(got.Edges) != 0 {
		t.Fatalf("expected 0 edges after excluding incident node, got %d: %#v", len(got.Edges), got.Edges)
	}
	for _, node := range got.Nodes {
		if node.ID == "svc/bar" {
			t.Fatalf("excluded node still present: %#v", node)
		}
	}
}

func TestFilterGraph_MinCallsRemovesLowCountEdgesAndKeepsNodes(t *testing.T) {
	// Arrange
	g := &Graph{
		Nodes: []Node{
			{ID: "pkg/a", Package: "pkg/a"},
			{ID: "pkg/b", Package: "pkg/b"},
			{ID: "pkg/c", Package: "pkg/c"},
		},
		Edges: []Edge{
			{Source: "pkg/a", Target: "pkg/b", CallCount: 1},
			{Source: "pkg/b", Target: "pkg/c", CallCount: 5},
		},
	}

	// Act
	got := FilterGraph(g, GraphOptions{MinCalls: 2})

	// Assert
	if len(got.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %#v", len(got.Nodes), got.Nodes)
	}
	if len(got.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %#v", len(got.Edges), got.Edges)
	}
	if got.Edges[0].Source != "pkg/b" || got.Edges[0].Target != "pkg/c" {
		t.Errorf("unexpected remaining edge: %#v", got.Edges[0])
	}
}

func TestFilterGraph_DepthOneFromFocusedNodeKeepsOneHopNeighbors(t *testing.T) {
	// Arrange
	g := &Graph{
		Nodes: []Node{
			{ID: "pkg/a", Package: "pkg/a"},
			{ID: "pkg/b", Package: "pkg/b"},
			{ID: "pkg/c", Package: "pkg/c"},
		},
		Edges: []Edge{
			{Source: "pkg/a", Target: "pkg/b", CallCount: 3},
			{Source: "pkg/b", Target: "pkg/c", CallCount: 3},
		},
	}

	// Act
	got := FilterGraph(g, GraphOptions{Focus: "pkg/a", Depth: 1})

	// Assert
	if len(got.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %#v", len(got.Nodes), got.Nodes)
	}
	if len(got.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %#v", len(got.Edges), got.Edges)
	}
	wantNodeIDs := []string{"pkg/a", "pkg/b"}
	gotNodeIDs := []string{got.Nodes[0].ID, got.Nodes[1].ID}
	if !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Errorf("node IDs: got %v, want %v", gotNodeIDs, wantNodeIDs)
	}
}

func TestFilterGraph_CombinedFiltersApplyInDocumentedOrder(t *testing.T) {
	// Arrange
	g := &Graph{
		Nodes: []Node{
			{ID: "pkg/seed", Package: "pkg/seed"},
			{ID: "pkg/near", Package: "pkg/near"},
			{ID: "pkg/low", Package: "pkg/low"},
			{ID: "pkg/drop", Package: "pkg/drop"},
		},
		Edges: []Edge{
			{Source: "pkg/seed", Target: "pkg/near", CallCount: 3},
			{Source: "pkg/seed", Target: "pkg/low", CallCount: 1},
			{Source: "pkg/seed", Target: "pkg/drop", CallCount: 5},
		},
	}

	// Act
	got := FilterGraph(g, GraphOptions{
		Exclude:  "drop",
		Focus:    "seed",
		Depth:    1,
		MinCalls: 2,
	})

	// Assert
	wantNodeIDs := []string{"pkg/low", "pkg/near", "pkg/seed"}
	gotNodeIDs := []string{}
	for _, node := range got.Nodes {
		gotNodeIDs = append(gotNodeIDs, node.ID)
	}
	if !reflect.DeepEqual(gotNodeIDs, wantNodeIDs) {
		t.Fatalf("node IDs: got %v, want %v", gotNodeIDs, wantNodeIDs)
	}
	if len(got.Edges) != 1 {
		t.Fatalf("expected 1 edge after min-calls, got %d: %#v", len(got.Edges), got.Edges)
	}
	if got.Edges[0].Source != "pkg/seed" || got.Edges[0].Target != "pkg/near" {
		t.Errorf("unexpected remaining edge: %#v", got.Edges[0])
	}
}
