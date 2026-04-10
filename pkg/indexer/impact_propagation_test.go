package indexer

import (
	"database/sql"
	"testing"
	"time"
)

func seedImpactPropagationPrimaryDB(t *testing.T, cfgHome string) string {
	t.Helper()
	root := t.TempDir()
	db := openImpactPropagationDB(t, root, cfgHome)

	entry := FileEntry{
		Language:  "go",
		SHA256:    "impact-primary",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "runImpact", Type: Function, StartLine: 1, EndLine: 5},
			{Name: "service", Type: Function, StartLine: 6, EndLine: 12},
			{Name: "controller", Type: Function, StartLine: 13, EndLine: 20},
		},
		Calls: []CallSite{
			{CalleeName: "runImpact", Line: 8},
			{CalleeName: "service", Line: 15},
		},
	}

	if err := WriteFile(db, "main.go", entry); err != nil {
		t.Fatalf("WriteFile main.go: %v", err)
	}
	if err := setMeta(db, root); err != nil {
		t.Fatalf("setMeta root: %v", err)
	}
	_ = db.Close()
	return root
}

func seedImpactPropagationSecondaryDB(t *testing.T, cfgHome string) (root string, repoID string) {
	t.Helper()
	root = t.TempDir()
	db := openImpactPropagationDB(t, root, cfgHome)

	entry := FileEntry{
		Language:  "go",
		SHA256:    "impact-secondary",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "remoteHandler", Type: Function, StartLine: 3, EndLine: 9},
		},
		Calls: []CallSite{},
	}

	if err := WriteFile(db, "remote.go", entry); err != nil {
		t.Fatalf("WriteFile remote.go: %v", err)
	}
	if err := setMeta(db, root); err != nil {
		t.Fatalf("setMeta root: %v", err)
	}
	repoID = RepoID(root)
	_ = db.Close()
	return root, repoID
}

func openImpactPropagationDB(t *testing.T, root, cfgHome string) *sql.DB {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	db, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex(%q): %v", root, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestPropagateImpact_InRepoCallChain(t *testing.T) {
	cfgHome := t.TempDir()
	root := seedImpactPropagationPrimaryDB(t, cfgHome)

	input := ImpactSimulationInput{
		Symbol:    "runImpact",
		Workspace: "default",
		Change: ImpactChange{
			Kind: ImpactChangeAddRequiredParam,
		},
		Options: &ImpactOptions{MaxDepth: 4},
	}

	result, err := PropagateImpact(root, input, ImpactPropagationOptions{MaxDepth: 4})
	if err != nil {
		t.Fatalf("PropagateImpact: %v", err)
	}

	if result.Start.Symbol != "runImpact" {
		t.Fatalf("start symbol = %q, want runImpact", result.Start.Symbol)
	}
	if len(result.Impacts) == 0 {
		t.Fatalf("expected non-empty impacts")
	}

	if !hasImpactSymbol(result.Impacts, "service") {
		t.Fatalf("expected propagated impact for service")
	}
	if !hasImpactSymbol(result.Impacts, "controller") {
		t.Fatalf("expected transitive impact for controller")
	}

	if result.Scope.DirectCallers < 1 {
		t.Fatalf("expected at least one direct caller, got %d", result.Scope.DirectCallers)
	}
	if result.Scope.IndirectCallers < 1 {
		t.Fatalf("expected at least one indirect caller, got %d", result.Scope.IndirectCallers)
	}
}

func TestPropagateImpact_MaxDepthRespected(t *testing.T) {
	cfgHome := t.TempDir()
	root := seedImpactPropagationPrimaryDB(t, cfgHome)

	input := ImpactSimulationInput{
		Symbol:    "runImpact",
		Workspace: "default",
		Change:    ImpactChange{Kind: ImpactChangeAddRequiredParam},
		Options:   &ImpactOptions{MaxDepth: 1},
	}

	result, err := PropagateImpact(root, input, ImpactPropagationOptions{MaxDepth: 1})
	if err != nil {
		t.Fatalf("PropagateImpact: %v", err)
	}

	if hasImpactSymbol(result.Impacts, "controller") {
		t.Fatalf("did not expect controller impact at max depth 1")
	}
	if !hasImpactSymbol(result.Impacts, "service") {
		t.Fatalf("expected direct caller impact at max depth 1")
	}
}

func TestPropagateImpact_CrossRepoWorkspaceLink(t *testing.T) {
	cfgHome := t.TempDir()
	primaryRoot := seedImpactPropagationPrimaryDB(t, cfgHome)
	secondaryRoot, secondaryRepoID := seedImpactPropagationSecondaryDB(t, cfgHome)

	input := ImpactSimulationInput{
		Symbol:    "runImpact",
		Workspace: "default",
		Change:    ImpactChange{Kind: ImpactChangeRenameSymbol},
		Options:   &ImpactOptions{MaxDepth: 4, IncludeCrossRepo: true},
	}

	result, err := PropagateImpact(primaryRoot, input, ImpactPropagationOptions{
		MaxDepth:         4,
		IncludeCrossRepo: true,
		RepoRoots: map[string]string{
			secondaryRepoID: secondaryRoot,
		},
		WorkspaceLinks: []ImpactWorkspaceLink{
			{
				ID:        1,
				SrcRepoID: RepoID(primaryRoot),
				SrcSymbol: "runImpact",
				DstRepoID: secondaryRepoID,
				DstSymbol: "remoteHandler",
			},
		},
	})
	if err != nil {
		t.Fatalf("PropagateImpact: %v", err)
	}

	if !hasImpactSymbol(result.Impacts, "remoteHandler") {
		t.Fatalf("expected cross-repo impact for remoteHandler")
	}
	if result.Scope.CrossBoundaryEdges < 1 {
		t.Fatalf("expected at least one cross boundary edge, got %d", result.Scope.CrossBoundaryEdges)
	}
	if result.Scope.AffectedRepos < 2 {
		t.Fatalf("expected at least two affected repos, got %d", result.Scope.AffectedRepos)
	}
}

func hasImpactSymbol(items []ImpactItem, symbol string) bool {
	for _, item := range items {
		if item.Target.Symbol == symbol {
			return true
		}
	}
	return false
}
