package workspace

// index_test.go — unit tests for IndexWorkspace.

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/Utahjezz/mimir/pkg/indexer"
	_ "modernc.org/sqlite"
)

// openWorkspaceForIndex opens a fresh workspace with all I/O directed to a
// temp XDG_CONFIG_HOME. The DB handle is registered for cleanup.
func openWorkspaceForIndex(t *testing.T) *sql.DB {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	db, err := OpenWorkspace("idxtest")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// makeAndRegisterRepo creates an indexed repo (a real temp dir with a .go
// file indexed by indexer.Run) and adds it to the provided workspace DB.
func makeAndRegisterRepo(t *testing.T, db *sql.DB) string {
	t.Helper()
	root := t.TempDir()
	goFile := filepath.Join(root, "fn.go")
	if err := os.WriteFile(goFile, []byte("package pkg\nfunc Fn() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	repoDB, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("indexer.OpenIndex: %v", err)
	}
	defer repoDB.Close()
	if _, err := indexer.Run(root, repoDB); err != nil {
		t.Fatalf("indexer.Run: %v", err)
	}
	if _, err := AddRepository(db, root); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	return root
}

// drainResults collects all results from the channel into a slice.
func drainResults(ch <-chan RepoResult) []RepoResult {
	var results []RepoResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

// TestIndexWorkspace_IndexesAllRepos verifies that with 2 repos added to the
// workspace, IndexWorkspace delivers exactly 2 results and none have errors.
func TestIndexWorkspace_IndexesAllRepos(t *testing.T) {
	// Arrange
	db := openWorkspaceForIndex(t)
	makeAndRegisterRepo(t, db)
	makeAndRegisterRepo(t, db)

	// Act
	ch, err := IndexWorkspace(db, 2, false)
	if err != nil {
		t.Fatalf("IndexWorkspace: %v", err)
	}
	results := drainResults(ch)

	// Assert
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error for repo %q: %v", r.Repo.Path, r.Err)
		}
	}
}

// TestIndexWorkspace_ContinuesOnFailure verifies that when one repo path does
// not exist, its result carries a non-nil Err, but the other repo still
// succeeds. The channel must still close normally.
func TestIndexWorkspace_ContinuesOnFailure(t *testing.T) {
	// Arrange
	db := openWorkspaceForIndex(t)

	// Add a real, properly indexed repo.
	goodPath := makeAndRegisterRepo(t, db)
	_ = goodPath

	// Manually insert a fake repo row for a path that does not exist so that
	// indexRepo will fail when it tries to walk the directory. Supply a
	// non-NULL last_indexed_at so that ListRepositories can scan it into
	// time.Time without error.
	const zeroTS = "2000-01-01T00:00:00Z"
	_, err := db.Exec(
		`INSERT OR IGNORE INTO repositories (id, path, last_indexed_at) VALUES (?, ?, ?)`,
		"ghost-00000000", "/tmp/nonexistent-mimir-test-repo-xyz", zeroTS,
	)
	if err != nil {
		t.Fatalf("inserting ghost repo: %v", err)
	}

	// Act
	ch, err := IndexWorkspace(db, 2, false)
	if err != nil {
		t.Fatalf("IndexWorkspace: %v", err)
	}
	results := drainResults(ch)

	// Assert: exactly 2 results
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	var errCount, okCount int
	for _, r := range results {
		if r.Err != nil {
			errCount++
		} else {
			okCount++
		}
	}
	if errCount != 1 {
		t.Errorf("expected 1 error result, got %d", errCount)
	}
	if okCount != 1 {
		t.Errorf("expected 1 success result, got %d", okCount)
	}
}

// TestIndexWorkspace_RebuildFlag is a smoke test: calling IndexWorkspace with
// rebuild=true on an already-indexed repo must succeed without error.
func TestIndexWorkspace_RebuildFlag(t *testing.T) {
	// Arrange
	db := openWorkspaceForIndex(t)
	makeAndRegisterRepo(t, db)

	// Act
	ch, err := IndexWorkspace(db, 1, true)
	if err != nil {
		t.Fatalf("IndexWorkspace rebuild: %v", err)
	}
	results := drainResults(ch)

	// Assert
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("rebuild result should not have error, got: %v", results[0].Err)
	}
}

// TestIndexWorkspace_EmptyWorkspace verifies that on a workspace with no repos
// the returned channel closes immediately without producing any results.
func TestIndexWorkspace_EmptyWorkspace(t *testing.T) {
	// Arrange
	db := openWorkspaceForIndex(t)

	// Act
	ch, err := IndexWorkspace(db, 2, false)
	if err != nil {
		t.Fatalf("IndexWorkspace on empty workspace: %v", err)
	}
	results := drainResults(ch)

	// Assert
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty workspace, got %d", len(results))
	}
}
