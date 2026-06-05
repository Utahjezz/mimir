package graph

// builder_test.go — tests for BuildGraph and shared graph test helpers.

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Utahjezz/mimir/pkg/indexer"
)

func openGraphTestDB(t *testing.T, root string) *sql.DB {
	t.Helper()

	db, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("indexer.OpenIndex(%q): %v", root, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func writeGraphTestFile(t *testing.T, db *sql.DB, rel string, symbols []indexer.SymbolInfo) {
	t.Helper()

	now := time.Now().UTC()
	err := indexer.WriteFile(db, rel, indexer.FileEntry{
		Language:  "go",
		SHA256:    "x",
		Mtime:     now.Format(time.RFC3339),
		Size:      1,
		IndexedAt: now,
		Symbols:   symbols,
	})
	if err != nil {
		t.Fatalf("indexer.WriteFile(%q): %v", rel, err)
	}
}

func TestBuildGraph_EmptyRefsReturnsEmptyGraph(t *testing.T) {
	// Arrange
	opts := BuildOptions{}

	// Act
	g, err := BuildGraph(opts)

	// Assert
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(g.Nodes) != 0 {
		t.Errorf("Nodes length: got %d, want 0", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("Edges length: got %d, want 0", len(g.Edges))
	}
}

func TestBuildGraph_AggregatesPackagesAndDeduplicatesNodes(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	db := openGraphTestDB(t, root)
	writeGraphTestFile(t, db, "pkg/lib/lib.go", []indexer.SymbolInfo{
		{Name: "Helper", Type: indexer.Function, StartLine: 1, EndLine: 3},
		{Name: "Shared", Type: indexer.Function, StartLine: 5, EndLine: 7},
	})

	callerFile := filepath.Join(root, "pkg/api/api.go")
	refs := []indexer.RefRow{
		{CallerFile: callerFile, CallerName: "Serve", CalleeName: "Helper", Line: 10},
		{CallerFile: callerFile, CallerName: "Serve", CalleeName: "Helper", Line: 11},
		{CallerFile: callerFile, CallerName: "Serve", CalleeName: "Shared", Line: 12},
	}

	// Act
	g, err := BuildGraph(BuildOptions{
		Root:    root,
		IndexDB: db,
		Refs:    refs,
		Options: GraphOptions{Scope: ScopePackage},
	})

	// Assert
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %#v", len(g.Nodes), g.Nodes)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %#v", len(g.Edges), g.Edges)
	}

	nodes := map[string]Node{}
	for _, node := range g.Nodes {
		nodes[node.ID] = node
	}
	if nodes["pkg/api"].CallCount != 3 {
		t.Errorf("pkg/api CallCount: got %d, want 3", nodes["pkg/api"].CallCount)
	}
	if nodes["pkg/lib"].CallCount != 3 {
		t.Errorf("pkg/lib CallCount: got %d, want 3", nodes["pkg/lib"].CallCount)
	}

	gotEdge := g.Edges[0]
	if gotEdge.Source != "pkg/api" {
		t.Errorf("edge Source: got %q, want %q", gotEdge.Source, "pkg/api")
	}
	if gotEdge.Target != "pkg/lib" {
		t.Errorf("edge Target: got %q, want %q", gotEdge.Target, "pkg/lib")
	}
	if gotEdge.CallCount != 3 {
		t.Errorf("edge CallCount: got %d, want 3", gotEdge.CallCount)
	}
	wantFunctions := []string{"Helper", "Serve", "Shared"}
	if !reflect.DeepEqual(gotEdge.Functions, wantFunctions) {
		t.Errorf("edge Functions: got %v, want %v", gotEdge.Functions, wantFunctions)
	}
}

func TestBuildGraph_IntraPackageCallsIncrementNodeWithoutEdge(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	db := openGraphTestDB(t, root)
	writeGraphTestFile(t, db, "pkg/api/helpers.go", []indexer.SymbolInfo{
		{Name: "Helper", Type: indexer.Function, StartLine: 1, EndLine: 3},
	})

	refs := []indexer.RefRow{{
		CallerFile: filepath.Join(root, "pkg/api/api.go"),
		CallerName: "Serve",
		CalleeName: "Helper",
		Line:       10,
	}}

	// Act
	g, err := BuildGraph(BuildOptions{
		Root:    root,
		IndexDB: db,
		Refs:    refs,
		Options: GraphOptions{Scope: ScopePackage},
	})

	// Assert
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d: %#v", len(g.Nodes), g.Nodes)
	}
	if len(g.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d: %#v", len(g.Edges), g.Edges)
	}
	if g.Nodes[0].ID != "pkg/api" {
		t.Errorf("node ID: got %q, want %q", g.Nodes[0].ID, "pkg/api")
	}
	if g.Nodes[0].CallCount != 1 {
		t.Errorf("node CallCount: got %d, want 1", g.Nodes[0].CallCount)
	}
}
