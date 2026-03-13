package commands

// index_test.go — tests for the index command and its --rebuild flag.

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"

	_ "modernc.org/sqlite"
)

// runIndexCmd is a test helper that invokes runIndex via a fresh cobra.Command
// with the given flags and returns any error.
func runIndexCmd(t *testing.T, root string, rebuild bool) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	// Reflect the flag state into the package-level vars used by runIndex.
	indexRebuild = rebuild
	defer func() { indexRebuild = false }()

	return runIndex(cmd, []string{root})
}

// --- --rebuild flag ---

func TestRunIndex_RebuildDropsExistingData(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	// Write a sentinel Go file so the indexer picks up at least one symbol.
	goFile := filepath.Join(root, "sentinel.go")
	if err := os.WriteFile(goFile, []byte("package main\nfunc Sentinel() {}\n"), 0o644); err != nil {
		t.Fatalf("writing sentinel file: %v", err)
	}

	// First index run — populates the DB.
	if err := runIndexCmd(t, root, false); err != nil {
		t.Fatalf("first index run: %v", err)
	}

	// Verify data was written.
	db, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex after first run: %v", err)
	}
	paths, err := indexer.IndexedPaths(db)
	db.Close()
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one indexed path after first run")
	}

	// Remove the source file so the --rebuild run produces an empty index.
	if err := os.Remove(goFile); err != nil {
		t.Fatalf("removing sentinel file: %v", err)
	}

	// Second run with --rebuild — must start from a clean slate.
	if err := runIndexCmd(t, root, true); err != nil {
		t.Fatalf("rebuild index run: %v", err)
	}

	db2, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex after rebuild: %v", err)
	}
	defer db2.Close()

	paths2, err := indexer.IndexedPaths(db2)
	if err != nil {
		t.Fatalf("IndexedPaths after rebuild: %v", err)
	}
	if len(paths2) != 0 {
		t.Errorf("expected empty index after --rebuild with no source files, got %d path(s)", len(paths2))
	}
}

func TestRunIndex_RebuildOnAbsentIndex(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	// --rebuild on a never-indexed root must not error.
	if err := runIndexCmd(t, root, true); err != nil {
		t.Errorf("rebuild with no prior index: unexpected error: %v", err)
	}
}

func TestRunIndex_WithoutRebuildPreservesData(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	// Write and index a file.
	goFile := filepath.Join(root, "keep.go")
	if err := os.WriteFile(goFile, []byte("package main\nfunc Keep() {}\n"), 0o644); err != nil {
		t.Fatalf("writing keep file: %v", err)
	}
	if err := runIndexCmd(t, root, false); err != nil {
		t.Fatalf("first index run: %v", err)
	}

	// A second normal run (no --rebuild) must keep the existing data in place.
	if err := runIndexCmd(t, root, false); err != nil {
		t.Fatalf("second index run: %v", err)
	}

	db, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer db.Close()

	paths, err := indexer.IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	found := false
	for p := range paths {
		if filepath.Base(p) == "keep.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("keep.go should still be in the index after a non-rebuild run")
	}
}

// TestRunIndex_JSONOutput verifies the --json flag produces valid JSON stats.
func TestRunIndex_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	jsonOutput = true
	indexRebuild = false
	defer func() {
		jsonOutput = false
		indexRebuild = false
	}()

	if err := runIndex(cmd, []string{root}); err != nil {
		t.Fatalf("runIndex with --json: %v", err)
	}

	// Output must be non-empty and start with '{' (JSON object).
	got := bytes.TrimSpace(out.Bytes())
	if len(got) == 0 {
		t.Fatal("expected non-empty JSON output")
	}
	if got[0] != '{' {
		t.Errorf("expected JSON object, got: %q", string(got))
	}
}

// --- schema mismatch warning ---

// plantStaleIndex creates a real SQLite DB file at the XDG path for root and
// writes a meta table with a version one below the current indexVersion. This
// simulates an index that was built with an older binary.
func plantStaleIndex(t *testing.T, root string) {
	t.Helper()

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	sum := sha256.Sum256([]byte(absRoot))
	repoID := filepath.Base(absRoot) + "-" + hex.EncodeToString(sum[:])[:8]

	xdg := os.Getenv("XDG_CONFIG_HOME")
	dbFile := filepath.Join(xdg, "mimir", "indexes", repoID, "index.db")
	if err := os.MkdirAll(filepath.Dir(dbFile), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		t.Fatalf("sql.Open stale db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create meta: %v", err)
	}
	staleVersion := 1 // always older than current indexVersion (≥ 3)
	if _, err := db.Exec(`INSERT INTO meta (key, value) VALUES ('version', ?)`, fmt.Sprintf("%d", staleVersion)); err != nil {
		t.Fatalf("insert stale version: %v", err)
	}
}

func TestRunIndex_SchemaMismatch_ReturnsActionableMessage(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	plantStaleIndex(t, root)

	err := runIndexCmd(t, root, false)
	if err == nil {
		t.Fatal("expected error for stale schema, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "--rebuild") {
		t.Errorf("error message should mention --rebuild, got: %q", msg)
	}
	if !strings.Contains(msg, "schema") {
		t.Errorf("error message should mention 'schema', got: %q", msg)
	}
}

func TestRunIndex_RebuildClearsSchemaMismatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	plantStaleIndex(t, root)

	// --rebuild must drop the stale index and succeed.
	if err := runIndexCmd(t, root, true); err != nil {
		t.Errorf("--rebuild after schema mismatch: unexpected error: %v", err)
	}
}

// Ensure the imported packages are used (compiler guard).
var _ = indexer.IsSchemaMismatch
