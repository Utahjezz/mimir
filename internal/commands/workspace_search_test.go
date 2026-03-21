package commands

// workspace_search_test.go — integration tests for search --workspace and
// refs --workspace fan-out across multiple repositories.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

// makeIndexedWSRepo creates a temp dir with a single Go file declaring a
// uniquely-named function, indexes it, and returns the root path.
func makeIndexedWSRepo(t *testing.T, funcName string) string {
	t.Helper()
	root := t.TempDir()
	src := "package p\nfunc " + funcName + "() {}\n"
	if err := os.WriteFile(filepath.Join(root, "pkg.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	db, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("indexer.OpenIndex: %v", err)
	}
	defer db.Close()
	if _, err := indexer.Run(root, db); err != nil {
		t.Fatalf("indexer.Run: %v", err)
	}
	return root
}

// makeWorkspaceWithRepos creates a named workspace (using t.TempDir for XDG),
// registers two repos into it, and returns their paths.
func makeWorkspaceWithRepos(t *testing.T, wsName, fnA, fnB string) (repoA, repoB string) {
	t.Helper()
	repoA = makeIndexedWSRepo(t, fnA)
	repoB = makeIndexedWSRepo(t, fnB)

	wsDB, err := workspace.OpenWorkspace(wsName)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer wsDB.Close()

	if _, err := workspace.AddRepository(wsDB, repoA); err != nil {
		t.Fatalf("AddRepository A: %v", err)
	}
	if _, err := workspace.AddRepository(wsDB, repoB); err != nil {
		t.Fatalf("AddRepository B: %v", err)
	}
	return
}

// runSearchWsCmd is a test helper that invokes runSearch with workspace flag
// support. Uses nil for [root] args (workspace mode does not need a root).
func runSearchWsCmd(t *testing.T, wsFlag, nameFlag string, jsonFlag bool) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	// Save and restore package-level flag state.
	prevWS := searchWorkspace
	prevName := searchName
	prevJSON := searchJSON
	prevLike := searchLike
	prevFuzzy := searchFuzzy
	prevType := searchType
	prevFile := searchFile
	prevNoR := searchNoRefresh
	t.Cleanup(func() {
		searchWorkspace = prevWS
		searchName = prevName
		searchJSON = prevJSON
		searchLike = prevLike
		searchFuzzy = prevFuzzy
		searchType = prevType
		searchFile = prevFile
		searchNoRefresh = prevNoR
	})

	searchWorkspace = wsFlag
	searchName = nameFlag
	searchJSON = jsonFlag
	searchLike = ""
	searchFuzzy = ""
	searchType = ""
	searchFile = ""
	searchNoRefresh = true // avoid re-indexing in tests

	err := runSearch(cmd, nil)
	return out.String(), err
}

// runRefsWsCmd is a test helper that invokes runRefs with workspace flag support.
func runRefsWsCmd(t *testing.T, wsFlag, calleeFlag string, jsonFlag bool) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	prevWS := refsWorkspace
	prevCallee := refsCallee
	prevCaller := refsCaller
	prevFile := refsFile
	prevJSON := refsJSON
	prevHS := refsHotspot
	prevLim := refsLimit
	prevNoR := refsNoRefresh
	t.Cleanup(func() {
		refsWorkspace = prevWS
		refsCallee = prevCallee
		refsCaller = prevCaller
		refsFile = prevFile
		refsJSON = prevJSON
		refsHotspot = prevHS
		refsLimit = prevLim
		refsNoRefresh = prevNoR
	})

	refsWorkspace = wsFlag
	refsCallee = calleeFlag
	refsCaller = ""
	refsFile = ""
	refsJSON = jsonFlag
	refsHotspot = false
	refsNoRefresh = true

	err := runRefs(cmd, nil)
	return out.String(), err
}

// --------------------------------------------------------------------------
// search --workspace tests
// --------------------------------------------------------------------------

// TestRunSearch_WorkspaceFanOut verifies that search --workspace fans out to
// all member repos and returns results from each, annotated with repo_id.
func TestRunSearch_WorkspaceFanOut(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	makeWorkspaceWithRepos(t, "searchfanoutws", "AlphaFunc", "BetaFunc")

	// Act: no name filter — get everything
	out, err := runSearchWsCmd(t, "searchfanoutws", "", false)

	// Assert
	if err != nil {
		t.Fatalf("runSearch --workspace: %v", err)
	}
	if !strings.Contains(out, "AlphaFunc") {
		t.Errorf("expected AlphaFunc in output, got: %q", out)
	}
	if !strings.Contains(out, "BetaFunc") {
		t.Errorf("expected BetaFunc in output, got: %q", out)
	}
}

// TestRunSearch_WorkspaceFanOut_RepoIDAnnotated verifies the text output
// includes a repo_id column prefix on each result row.
func TestRunSearch_WorkspaceFanOut_RepoIDAnnotated(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	repoA, _ := makeWorkspaceWithRepos(t, "searchannows", "GammaFunc", "DeltaFunc")
	repoAID := indexer.RepoID(repoA)

	// Act
	out, err := runSearchWsCmd(t, "searchannows", "GammaFunc", false)

	// Assert
	if err != nil {
		t.Fatalf("runSearch --workspace --name: %v", err)
	}
	if !strings.Contains(out, repoAID) {
		t.Errorf("expected repo_id %q in output, got: %q", repoAID, out)
	}
	if !strings.Contains(out, "GammaFunc") {
		t.Errorf("expected GammaFunc in output, got: %q", out)
	}
}

// TestRunSearch_WorkspaceFanOut_JSON verifies that --json produces valid JSON
// with repo_id fields on each element.
func TestRunSearch_WorkspaceFanOut_JSON(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	makeWorkspaceWithRepos(t, "searchjsonws", "EpsilonFunc", "ZetaFunc")

	// Act
	out, err := runSearchWsCmd(t, "searchjsonws", "", true)

	// Assert
	if err != nil {
		t.Fatalf("runSearch --workspace --json: %v", err)
	}

	var rows []WorkspaceSymbolRow
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rows); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %q", err, out)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r.RepoID == "" {
			t.Errorf("expected non-empty repo_id on row %+v", r)
		}
	}

	// Verify both symbols appear somewhere.
	var names []string
	for _, r := range rows {
		names = append(names, r.Name)
	}
	namesStr := strings.Join(names, ",")
	if !strings.Contains(namesStr, "EpsilonFunc") {
		t.Errorf("expected EpsilonFunc in JSON results, got: %v", names)
	}
	if !strings.Contains(namesStr, "ZetaFunc") {
		t.Errorf("expected ZetaFunc in JSON results, got: %v", names)
	}
}

// TestRunSearch_WorkspaceFanOut_Empty verifies that a workspace with no repos
// returns a "no symbols found" message rather than an error.
func TestRunSearch_WorkspaceFanOut_Empty(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	wsDB, err := workspace.OpenWorkspace("emptyws")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	wsDB.Close()

	// Act
	out, err := runSearchWsCmd(t, "emptyws", "", false)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error for empty workspace: %v", err)
	}
	if !strings.Contains(out, "no symbols found") {
		t.Errorf("expected 'no symbols found', got: %q", out)
	}
}

// TestRunSearch_NoArgs_NoWorkspace verifies that omitting both root and
// --workspace returns a clear error.
func TestRunSearch_NoArgs_NoWorkspace(t *testing.T) {
	_, err := runSearchWsCmd(t, "", "", false)
	if err == nil {
		t.Fatal("expected error when root and --workspace are both absent")
	}
}

// --------------------------------------------------------------------------
// refs --workspace tests
// --------------------------------------------------------------------------

// makeIndexedWSRepoWithRefs creates a temp dir with a Go file that has a
// caller (outer) calling an inner function, indexes it, and returns the path.
func makeIndexedWSRepoWithRefs(t *testing.T, outer, inner string) string {
	t.Helper()
	root := t.TempDir()
	src := "package p\nfunc " + inner + "() {}\nfunc " + outer + "() { " + inner + "() }\n"
	if err := os.WriteFile(filepath.Join(root, "pkg.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	db, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("indexer.OpenIndex: %v", err)
	}
	defer db.Close()
	if _, err := indexer.Run(root, db); err != nil {
		t.Fatalf("indexer.Run: %v", err)
	}
	return root
}

// TestRunRefs_WorkspaceFanOut verifies that refs --workspace fans out across
// all member repos and returns results annotated with repo_id.
func TestRunRefs_WorkspaceFanOut(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	repoA := makeIndexedWSRepoWithRefs(t, "OuterA", "InnerA")
	repoB := makeIndexedWSRepoWithRefs(t, "OuterB", "InnerB")

	wsDB, err := workspace.OpenWorkspace("refsfanoutws")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer wsDB.Close()

	if _, err := workspace.AddRepository(wsDB, repoA); err != nil {
		t.Fatalf("AddRepository A: %v", err)
	}
	if _, err := workspace.AddRepository(wsDB, repoB); err != nil {
		t.Fatalf("AddRepository B: %v", err)
	}

	// Act: no callee filter — get all refs
	out, err := runRefsWsCmd(t, "refsfanoutws", "", false)

	// Assert
	if err != nil {
		t.Fatalf("runRefs --workspace: %v", err)
	}
	// Both repos should appear (each has a call site).
	repoAID := indexer.RepoID(repoA)
	repoBID := indexer.RepoID(repoB)
	if !strings.Contains(out, repoAID) {
		t.Errorf("expected repo A (%s) in output, got: %q", repoAID, out)
	}
	if !strings.Contains(out, repoBID) {
		t.Errorf("expected repo B (%s) in output, got: %q", repoBID, out)
	}
}

// TestRunRefs_WorkspaceFanOut_JSON verifies --json output has repo_id on each row.
func TestRunRefs_WorkspaceFanOut_JSON(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	repoA := makeIndexedWSRepoWithRefs(t, "OuterC", "InnerC")
	repoB := makeIndexedWSRepoWithRefs(t, "OuterD", "InnerD")

	wsDB, err := workspace.OpenWorkspace("refsjsonws")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer wsDB.Close()

	if _, err := workspace.AddRepository(wsDB, repoA); err != nil {
		t.Fatalf("AddRepository A: %v", err)
	}
	if _, err := workspace.AddRepository(wsDB, repoB); err != nil {
		t.Fatalf("AddRepository B: %v", err)
	}

	// Act
	out, err := runRefsWsCmd(t, "refsjsonws", "", true)

	// Assert
	if err != nil {
		t.Fatalf("runRefs --workspace --json: %v", err)
	}

	var rows []WorkspaceRefRow
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rows); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %q", err, out)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows (one per repo), got %d", len(rows))
	}
	for _, r := range rows {
		if r.RepoID == "" {
			t.Errorf("expected non-empty repo_id on row %+v", r)
		}
	}
}

// TestRunRefs_WorkspaceFanOut_Empty verifies that a workspace with no repos
// returns "no refs found" rather than an error.
func TestRunRefs_WorkspaceFanOut_Empty(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	wsDB, err := workspace.OpenWorkspace("refsemptyws")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	wsDB.Close()

	// Act
	out, err := runRefsWsCmd(t, "refsemptyws", "", false)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error for empty workspace: %v", err)
	}
	if !strings.Contains(out, "no refs found") {
		t.Errorf("expected 'no refs found', got: %q", out)
	}
}

// TestRunRefs_NoArgs_NoWorkspace verifies that omitting both root and
// --workspace returns a clear error.
func TestRunRefs_NoArgs_NoWorkspace(t *testing.T) {
	_, err := runRefsWsCmd(t, "", "", false)
	if err == nil {
		t.Fatal("expected error when root and --workspace are both absent")
	}
}
