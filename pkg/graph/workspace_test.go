package graph

// workspace_test.go — tests for EnrichWithWorkspaceLinks.

import (
	"reflect"
	"testing"
	"time"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
)

func TestEnrichWithWorkspaceLinks_AddsCrossRepoEdge(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	indexDB := openGraphTestDB(t, root)
	writeGraphTestFile(t, indexDB, "pkg/local/local.go", []indexer.SymbolInfo{
		{Name: "LocalFunc", Type: indexer.Function, StartLine: 1, EndLine: 3},
	})

	wsDB, err := workspace.OpenWorkspace("graph-test")
	if err != nil {
		t.Fatalf("workspace.OpenWorkspace: %v", err)
	}
	defer wsDB.Close()

	rootRepoID := indexer.RepoID(root)
	otherRoot := t.TempDir()
	otherRepoID := indexer.RepoID(otherRoot)
	otherIndexDB := openGraphTestDB(t, otherRoot)
	writeGraphTestFile(t, otherIndexDB, "pkg/remote/remote.go", []indexer.SymbolInfo{
		{Name: "RemoteFunc", Type: indexer.Function, StartLine: 1, EndLine: 3},
	})

	if _, err := workspace.AddRepository(wsDB, root); err != nil {
		t.Fatalf("workspace.AddRepository(root): %v", err)
	}
	if _, err := workspace.AddRepository(wsDB, otherRoot); err != nil {
		t.Fatalf("workspace.AddRepository(otherRoot): %v", err)
	}

	linkID, err := workspace.CreateLink(wsDB, rootRepoID, "LocalFunc", "", otherRepoID, "RemoteFunc", "", "test link")
	if err != nil {
		t.Fatalf("workspace.CreateLink: %v", err)
	}
	if err := workspace.SetLinkMeta(wsDB, linkID, "protocol", "grpc"); err != nil {
		t.Fatalf("workspace.SetLinkMeta: %v", err)
	}

	g := &Graph{
		Nodes: []Node{{
			ID:      "pkg/local",
			Label:   "pkg/local",
			Package: "pkg/local",
		}},
		Edges: []Edge{},
	}

	// Act
	err = EnrichWithWorkspaceLinks(g, wsDB, root)

	// Assert
	if err != nil {
		t.Fatalf("EnrichWithWorkspaceLinks: %v", err)
	}
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %#v", len(g.Nodes), g.Nodes)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %#v", len(g.Edges), g.Edges)
	}

	gotEdge := g.Edges[0]
	if !gotEdge.IsCrossRepo {
		t.Error("expected cross-repo edge")
	}
	if gotEdge.CallCount != 1 {
		t.Errorf("CallCount: got %d, want 1", gotEdge.CallCount)
	}
	if gotEdge.Source != "pkg/local" {
		t.Errorf("Source: got %q, want %q", gotEdge.Source, "pkg/local")
	}
	if gotEdge.Target != otherRepoID+"/RemoteFunc" {
		t.Errorf("Target: got %q, want %q", gotEdge.Target, otherRepoID+"/RemoteFunc")
	}
	wantFunctions := []string{"LocalFunc", "RemoteFunc"}
	if !reflect.DeepEqual(gotEdge.Functions, wantFunctions) {
		t.Errorf("Functions: got %v, want %v", gotEdge.Functions, wantFunctions)
	}
	if gotEdge.RepoID != otherRepoID {
		t.Errorf("RepoID: got %q, want %q", gotEdge.RepoID, otherRepoID)
	}
	if gotEdge.LinkMeta["protocol"] != "grpc" {
		t.Errorf("LinkMeta[protocol]: got %q, want %q", gotEdge.LinkMeta["protocol"], "grpc")
	}

	var gotExternal *Node
	for i := range g.Nodes {
		if g.Nodes[i].RepoID == otherRepoID {
			gotExternal = &g.Nodes[i]
			break
		}
	}
	if gotExternal == nil {
		t.Fatalf("expected external node for repo %q, got %#v", otherRepoID, g.Nodes)
	}
	if gotExternal.ID != otherRepoID+"/RemoteFunc" {
		t.Errorf("external node ID: got %q, want %q", gotExternal.ID, otherRepoID+"/RemoteFunc")
	}
	if gotExternal.RepoID != otherRepoID {
		t.Errorf("external node RepoID: got %q, want %q", gotExternal.RepoID, otherRepoID)
	}
}

func TestBuildLinkFunctions_SortsAndDeduplicatesNames(t *testing.T) {
	// Arrange
	src := "Zed"
	dst := "Alpha"

	// Act
	got := buildLinkFunctions(src, dst)

	// Assert
	want := []string{"Alpha", "Zed"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildLinkFunctions: got %v, want %v", got, want)
	}
}

func TestCloneMeta_ReturnsIndependentCopy(t *testing.T) {
	// Arrange
	in := map[string]string{"protocol": "grpc", "transport": "kafka"}

	// Act
	got := cloneMeta(in)
	got["protocol"] = "http"

	// Assert
	if in["protocol"] != "grpc" {
		t.Errorf("expected original map to stay unchanged, got %q", in["protocol"])
	}
	if got["protocol"] != "http" {
		t.Errorf("expected copied map mutation, got %q", got["protocol"])
	}
	if len(got) != 2 {
		t.Errorf("copied map length: got %d, want 2", len(got))
	}
}

var _ = time.Time{}
