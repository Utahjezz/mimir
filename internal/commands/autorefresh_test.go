package commands

// autorefresh_test.go — integration tests for the auto-refresh behaviour wired
// into query commands (runSearch) and the --no-refresh flag.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/spf13/cobra"
)

// runSearchCmd is a test helper that calls runSearch with configurable flags.
func runSearchCmd(t *testing.T, root, name string, noRefresh bool) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	// Save and restore flag state.
	prevName := searchName
	prevNoRefresh := searchNoRefresh
	prevThreshold := RefreshThreshold
	prevJSON := searchJSON
	prevLike := searchLike
	prevFuzzy := searchFuzzy
	prevType := searchType
	prevFile := searchFile
	t.Cleanup(func() {
		searchName = prevName
		searchNoRefresh = prevNoRefresh
		RefreshThreshold = prevThreshold
		searchJSON = prevJSON
		searchLike = prevLike
		searchFuzzy = prevFuzzy
		searchType = prevType
		searchFile = prevFile
	})

	searchName = name
	searchNoRefresh = noRefresh
	searchJSON = false
	searchLike = ""
	searchFuzzy = ""
	searchType = ""
	searchFile = ""

	err := runSearch(cmd, []string{root})
	return out.String(), err
}

// seedFile writes a minimal Go source file with one exported function to root.
func seedFile(t *testing.T, root, filename, funcName string) {
	t.Helper()
	src := "package main\nfunc " + funcName + "() {}\n"
	if err := os.WriteFile(filepath.Join(root, filename), []byte(src), 0o644); err != nil {
		t.Fatalf("seedFile %s: %v", filename, err)
	}
}

// buildFreshIndex creates a fresh, fully indexed root using runIndexCmd.
func buildFreshIndex(t *testing.T, root string) {
	t.Helper()
	if err := runIndexCmd(t, root, false); err != nil {
		t.Fatalf("initial index: %v", err)
	}
}

// backdate forcibly sets last_indexed_at to a time in the past so that
// ShouldRefresh will report the index as stale without requiring a sleep.
func backdate(t *testing.T, root string, age time.Duration) {
	t.Helper()
	db, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("backdate OpenIndex: %v", err)
	}
	defer db.Close()

	past := time.Now().Add(-age).UTC().Format(time.RFC3339)
	_, execErr := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('last_indexed_at', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		past,
	)
	if execErr != nil {
		t.Fatalf("backdate: %v", execErr)
	}
}

// --- End-to-end auto-refresh ---

// TestAutoRefresh_EndToEnd_PicksUpNewSymbol verifies that a query command
// transparently re-indexes a stale index and returns a symbol that was added
// after the initial index run, without an explicit `mimir index` call.
func TestAutoRefresh_EndToEnd_PicksUpNewSymbol(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	// Index an empty-ish root (one file so the DB is created).
	seedFile(t, root, "original.go", "Original")
	buildFreshIndex(t, root)

	// Simulate the index becoming stale by backdating it.
	backdate(t, root, 30*time.Second)
	// Set threshold below the backdated age so auto-refresh fires.
	RefreshThreshold = 10 * time.Second

	// Add a new symbol AFTER indexing — auto-refresh must pick it up.
	seedFile(t, root, "new.go", "BrandNewSymbol")

	out, err := runSearchCmd(t, root, "BrandNewSymbol", false)
	if err != nil {
		t.Fatalf("runSearch: %v", err)
	}
	if out == "" || bytes.Equal([]byte(out), []byte("no symbols found\n")) {
		t.Errorf("expected BrandNewSymbol in output, got: %q", out)
	}
}

// TestAutoRefresh_EndToEnd_FreshIndexSkipsRun verifies that a query does NOT
// re-index when the index is younger than the threshold (single SQLite lookup
// path, no filesystem walk).
func TestAutoRefresh_EndToEnd_FreshIndexSkipsRun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	seedFile(t, root, "base.go", "BaseSymbol")
	buildFreshIndex(t, root)

	// Threshold is large — index just created, so it's definitely fresh.
	RefreshThreshold = 10 * time.Minute

	// Add a file that would only appear if a re-walk happened.
	seedFile(t, root, "ghost.go", "GhostSymbol")

	out, err := runSearchCmd(t, root, "GhostSymbol", false)
	if err != nil {
		t.Fatalf("runSearch: %v", err)
	}
	// GhostSymbol should NOT appear — the index is fresh, so the auto-refresh
	// must not run and the result should be deterministic.
	if out != "no symbols found\n" {
		t.Errorf("expected no symbols for GhostSymbol with fresh index, got: %q", out)
	}
}

// --- --no-refresh flag ---

// TestNoRefresh_SkipsWalkOnStaleIndex verifies that passing --no-refresh
// prevents auto-refresh even when the index is older than the threshold.
// A new symbol added after indexing should NOT appear in the results.
func TestNoRefresh_SkipsWalkOnStaleIndex(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	seedFile(t, root, "stable.go", "StableSymbol")
	buildFreshIndex(t, root)

	// Make the index appear stale.
	backdate(t, root, 60*time.Second)
	RefreshThreshold = 10 * time.Second

	// Add a new symbol — with --no-refresh it must NOT be discovered.
	seedFile(t, root, "hidden.go", "HiddenSymbol")

	out, err := runSearchCmd(t, root, "HiddenSymbol", true /* noRefresh */)
	if err != nil {
		t.Fatalf("runSearch --no-refresh: %v", err)
	}
	if out != "" && out != "no symbols found\n" {
		t.Errorf("--no-refresh should have suppressed walk; HiddenSymbol should not appear, got: %q", out)
	}
}

// TestNoRefresh_PreservesStaleResultsExactly verifies that --no-refresh returns
// exactly the symbols present at last-index time, not updated ones.
func TestNoRefresh_PreservesStaleResultsExactly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	// Index with one known symbol.
	seedFile(t, root, "known.go", "KnownSymbol")
	buildFreshIndex(t, root)

	// Make stale and add more symbols.
	backdate(t, root, 60*time.Second)
	RefreshThreshold = 10 * time.Second
	seedFile(t, root, "extra.go", "ExtraSymbol")

	// --no-refresh: KnownSymbol must still be found (it was indexed).
	out, err := runSearchCmd(t, root, "KnownSymbol", true /* noRefresh */)
	if err != nil {
		t.Fatalf("runSearch --no-refresh KnownSymbol: %v", err)
	}
	if out == "" || out == "no symbols found\n" {
		t.Errorf("KnownSymbol should still be findable with --no-refresh, got: %q", out)
	}
}

// --- RefreshThreshold zero ---

// TestAutoRefresh_ZeroThreshold_AlwaysRefreshes verifies that --refresh-threshold=0
// forces a re-walk on every query, regardless of how recently the index was built.
func TestAutoRefresh_ZeroThreshold_AlwaysRefreshes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	seedFile(t, root, "first.go", "FirstSymbol")
	buildFreshIndex(t, root)

	// Zero threshold: every query triggers a re-walk.
	RefreshThreshold = 0

	// Add a symbol immediately after indexing (no backdate needed).
	seedFile(t, root, "second.go", "SecondSymbol")

	out, err := runSearchCmd(t, root, "SecondSymbol", false)
	if err != nil {
		t.Fatalf("runSearch with zero threshold: %v", err)
	}
	if out == "" || out == "no symbols found\n" {
		t.Errorf("zero threshold should always re-walk; SecondSymbol should appear, got: %q", out)
	}
}
