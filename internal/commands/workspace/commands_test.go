package commands

// commands_test.go — unit tests for the workspace sub-commands.
// Follows the same helper pattern as internal/commands/index_test.go.

import (
	"bytes"
	"errors"
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

func newCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

func runCreateCmd(t *testing.T, name string) error {
	t.Helper()
	return runWorkspaceCreate(newCmd(), []string{name})
}

func runUseCmd(t *testing.T, name string) error {
	t.Helper()
	return runWorkspaceUse(newCmd(), []string{name})
}

func runShowCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	defer func() { workspaceShowJSON = false }()
	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	err := runWorkspaceShow(cmd, args)
	return out.String(), err
}

func runAddCmd(t *testing.T, args ...string) error {
	t.Helper()
	return runWorkspaceAdd(newCmd(), args)
}

func runRemoveCmd(t *testing.T, args ...string) error {
	t.Helper()
	return runWorkspaceRemove(newCmd(), args)
}

// makeIndexedRepoForCmds creates a temp dir, writes a .go file, and runs the
// indexer so that AddRepository (which calls indexer.OpenIndex internally) works.
func makeIndexedRepoForCmds(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	goFile := filepath.Join(root, "pkg.go")
	if err := os.WriteFile(goFile, []byte("package p\nfunc F() {}\n"), 0o644); err != nil {
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

// --------------------------------------------------------------------------
// workspace create
// --------------------------------------------------------------------------

// TestRunWorkspaceCreate_CreatesWorkspace verifies that after create the
// workspace DB file exists on disk.
func TestRunWorkspaceCreate_CreatesWorkspace(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Act
	if err := runCreateCmd(t, "newws"); err != nil {
		t.Fatalf("runWorkspaceCreate: %v", err)
	}

	// Assert: the DB file must exist under XDG_CONFIG_HOME/mimir/workspaces/
	dbPath := filepath.Join(tmp, "mimir", "workspaces", "newws.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("expected workspace DB file at %q, but it does not exist", dbPath)
	}
}

// --------------------------------------------------------------------------
// workspace use
// --------------------------------------------------------------------------

// TestRunWorkspaceUse_SetsCurrentWorkspace verifies that after use,
// GetCurrentWorkspace returns the name that was set.
func TestRunWorkspaceUse_SetsCurrentWorkspace(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act
	if err := runUseCmd(t, "myws"); err != nil {
		t.Fatalf("runWorkspaceUse: %v", err)
	}

	// Assert
	got, err := workspace.GetCurrentWorkspace()
	if err != nil {
		t.Fatalf("GetCurrentWorkspace: %v", err)
	}
	if got != "myws" {
		t.Errorf("GetCurrentWorkspace: got %q, want %q", got, "myws")
	}
}

// --------------------------------------------------------------------------
// workspace show
// --------------------------------------------------------------------------

// TestRunWorkspaceShow_NoArgs_UsesCurrentWorkspace verifies that show with no
// args uses the current workspace (previously set via use).
func TestRunWorkspaceShow_NoArgs_UsesCurrentWorkspace(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := runCreateCmd(t, "showws"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := runUseCmd(t, "showws"); err != nil {
		t.Fatalf("use: %v", err)
	}

	// Act: show with no explicit args
	out, err := runShowCmd(t)

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceShow: %v", err)
	}
	if !strings.Contains(out, "showws") {
		t.Errorf("expected output to contain workspace name %q, got: %q", "showws", out)
	}
}

// TestRunWorkspaceShow_ExplicitArg verifies that show with an explicit workspace
// name works independently of the current workspace.
func TestRunWorkspaceShow_ExplicitArg(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := runCreateCmd(t, "explicit"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Act: pass the name explicitly
	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	err := runWorkspaceShow(cmd, []string{"explicit"})

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceShow explicit: %v", err)
	}
	if !strings.Contains(out.String(), "explicit") {
		t.Errorf("expected output to contain %q, got: %q", "explicit", out.String())
	}
}

// TestRunWorkspaceShow_NoCurrentWorkspace verifies that show with no args and
// no current workspace set returns an error.
func TestRunWorkspaceShow_NoCurrentWorkspace(t *testing.T) {
	// Arrange: fresh temp dir with no current workspace set
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act
	_, err := runShowCmd(t)

	// Assert
	if err == nil {
		t.Fatal("expected error when no current workspace is set, got nil")
	}
	// The error should mention the missing workspace in some way.
	msg := err.Error()
	if !strings.Contains(msg, "workspace") {
		t.Errorf("error message should mention 'workspace', got: %q", msg)
	}
}

// --------------------------------------------------------------------------
// workspace add
// --------------------------------------------------------------------------

// TestRunWorkspaceAdd_AddsRepo verifies that after add, show lists the repo.
func TestRunWorkspaceAdd_AddsRepo(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repoPath := makeIndexedRepoForCmds(t)

	if err := runCreateCmd(t, "addws"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Act
	if err := runAddCmd(t, repoPath, "addws"); err != nil {
		t.Fatalf("runWorkspaceAdd: %v", err)
	}

	// Assert: show should list the repo by its repoID (basename-hash format)
	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	if err := runWorkspaceShow(cmd, []string{"addws"}); err != nil {
		t.Fatalf("runWorkspaceShow after add: %v", err)
	}
	repoID := indexer.RepoID(repoPath)
	if !strings.Contains(out.String(), repoID) {
		t.Errorf("expected show output to contain repoID %q, got: %q", repoID, out.String())
	}
}

// --------------------------------------------------------------------------
// workspace remove
// --------------------------------------------------------------------------

// TestRunWorkspaceRemove_RemovesRepo verifies that after add+remove the repo
// no longer appears in show output.
func TestRunWorkspaceRemove_RemovesRepo(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repoPath := makeIndexedRepoForCmds(t)

	if err := runCreateCmd(t, "removews"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := runAddCmd(t, repoPath, "removews"); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Act
	if err := runRemoveCmd(t, repoPath, "removews"); err != nil {
		t.Fatalf("runWorkspaceRemove: %v", err)
	}

	// Assert: repoID must not appear in show output
	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	if err := runWorkspaceShow(cmd, []string{"removews"}); err != nil {
		t.Fatalf("runWorkspaceShow after remove: %v", err)
	}
	repoID := indexer.RepoID(repoPath)
	if strings.Contains(out.String(), repoID) {
		t.Errorf("expected show output NOT to contain repoID %q after remove, got: %q", repoID, out.String())
	}
}

// TestRunWorkspaceRemove_NotFound verifies that removing an unknown path returns
// an error wrapping ErrRepositoryNotFound.
func TestRunWorkspaceRemove_NotFound(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := runCreateCmd(t, "notfoundws"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Act: remove a path that was never added
	err := runRemoveCmd(t, "/nonexistent/repo/path", "notfoundws")

	// Assert
	if err == nil {
		t.Fatal("expected error when removing unknown repo, got nil")
	}
	if !errors.Is(err, workspace.ErrRepositoryNotFound) {
		t.Errorf("expected ErrRepositoryNotFound, got: %v", err)
	}
}
