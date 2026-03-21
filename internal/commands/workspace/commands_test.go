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
	// Arrange: fresh temp dir with no current workspace set
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

// --------------------------------------------------------------------------
// Test helpers for link commands
// --------------------------------------------------------------------------

// runLinkCmd invokes runWorkspaceLink after setting flag globals. Restores all
// flag globals via t.Cleanup so tests are hermetic.
func runLinkCmd(t *testing.T, args []string, srcFile, dstFile, note string, meta []string) (string, error) {
	t.Helper()
	defer func() {
		workspaceLinkSrcFile = ""
		workspaceLinkDstFile = ""
		workspaceLinkNote = ""
		workspaceLinkMeta = nil
	}()
	workspaceLinkSrcFile = srcFile
	workspaceLinkDstFile = dstFile
	workspaceLinkNote = note
	workspaceLinkMeta = meta

	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	err := runWorkspaceLink(cmd, args)
	return out.String(), err
}

// runLinksCmd invokes runWorkspaceLinks after setting flag globals.
func runLinksCmd(t *testing.T, args []string, from string, asJSON bool) (string, error) {
	t.Helper()
	return runLinksCmdFull(t, args, from, "", "", asJSON)
}

// runLinksCmdFull is the full helper that exposes --src-symbol and --dst-symbol.
func runLinksCmdFull(t *testing.T, args []string, from, srcSymbol, dstSymbol string, asJSON bool) (string, error) {
	t.Helper()
	defer func() {
		workspaceLinksFrom = ""
		workspaceLinksJSON = false
		workspaceLinksSrcSymbol = ""
		workspaceLinksDstSymbol = ""
	}()
	workspaceLinksFrom = from
	workspaceLinksJSON = asJSON
	workspaceLinksSrcSymbol = srcSymbol
	workspaceLinksDstSymbol = dstSymbol

	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	err := runWorkspaceLinks(cmd, args)
	return out.String(), err
}

// runUnlinkCmd invokes runWorkspaceUnlink.
func runUnlinkCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	err := runWorkspaceUnlink(cmd, args)
	return out.String(), err
}

// makeAmbiguousRepo creates a temp repo with two files both exporting a symbol
// named "Shared", so symbol resolution returns an ambiguity error.
func makeAmbiguousRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"a/a.go": "package a\nfunc Shared() {}\n",
		"b/b.go": "package b\nfunc Shared() {}\n",
	}
	for rel, src := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
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

// setupLinkedWorkspace creates a workspace with two indexed repos already
// registered, and returns (workspaceName, srcRepoID, dstRepoID).
func setupLinkedWorkspace(t *testing.T, wsName string) (srcID, dstID string) {
	t.Helper()
	src := makeIndexedRepoForCmds(t)
	dst := makeIndexedRepoForCmds(t)
	if err := runCreateCmd(t, wsName); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := runAddCmd(t, src, wsName); err != nil {
		t.Fatalf("add src repo: %v", err)
	}
	if err := runAddCmd(t, dst, wsName); err != nil {
		t.Fatalf("add dst repo: %v", err)
	}
	return indexer.RepoID(src), indexer.RepoID(dst)
}

// --------------------------------------------------------------------------
// workspace link
// --------------------------------------------------------------------------

// TestRunWorkspaceLink_HappyPath verifies that a valid link is created and the
// confirmation message contains the link ID and both symbol names.
func TestRunWorkspaceLink_HappyPath(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "linkws")

	// Act
	out, err := runLinkCmd(t, []string{src, "F", dst, "F", "linkws"}, "", "", "a note", nil)

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceLink: %v", err)
	}
	if !strings.Contains(out, "Link #") {
		t.Errorf("expected output to contain 'Link #', got: %q", out)
	}
	if !strings.Contains(out, "F") {
		t.Errorf("expected output to contain symbol name 'F', got: %q", out)
	}
}

// TestRunWorkspaceLink_WithMeta verifies that --meta pairs are stored and
// visible via workspace.ListLinks.
func TestRunWorkspaceLink_WithMeta(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "metaws")

	// Act
	_, err := runLinkCmd(t, []string{src, "F", dst, "F", "metaws"}, "", "", "", []string{"protocol=grpc", "transport=kafka"})

	// Assert: link created without error
	if err != nil {
		t.Fatalf("runWorkspaceLink with meta: %v", err)
	}

	// Verify meta is persisted
	db, err := workspace.OpenWorkspace("metaws")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer db.Close()
	links, err := workspace.ListLinks(db, workspace.LinkQuery{})
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Meta["protocol"] != "grpc" {
		t.Errorf("Meta[protocol]: got %q, want %q", links[0].Meta["protocol"], "grpc")
	}
	if links[0].Meta["transport"] != "kafka" {
		t.Errorf("Meta[transport]: got %q, want %q", links[0].Meta["transport"], "kafka")
	}
}

// TestRunWorkspaceLink_SrcRepoNotInWorkspace verifies that using a repo ID
// that is not registered in the workspace returns a clear error.
func TestRunWorkspaceLink_SrcRepoNotInWorkspace(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dst := makeIndexedRepoForCmds(t)
	unregistered := makeIndexedRepoForCmds(t)
	if err := runCreateCmd(t, "unregs"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := runAddCmd(t, dst, "unregs"); err != nil {
		t.Fatalf("add dst: %v", err)
	}

	dstID := indexer.RepoID(dst)
	unregisteredID := indexer.RepoID(unregistered)

	// Act: src is not registered
	_, err := runLinkCmd(t, []string{unregisteredID, "F", dstID, "F", "unregs"}, "", "", "", nil)

	// Assert
	if err == nil {
		t.Fatal("expected error for unregistered src repo, got nil")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error should mention 'not registered', got: %v", err)
	}
}

// TestRunWorkspaceLink_SymbolNotFound verifies that referencing a symbol that
// does not exist in the src repo returns a clear error.
func TestRunWorkspaceLink_SymbolNotFound(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "notfoundlinkws")

	// Act
	_, err := runLinkCmd(t, []string{src, "NoSuchSymbol", dst, "F", "notfoundlinkws"}, "", "", "", nil)

	// Assert
	if err == nil {
		t.Fatal("expected error for missing symbol, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// TestRunWorkspaceLink_AmbiguousSymbol verifies that when a symbol name matches
// multiple files the error message lists the candidates and suggests --src-file.
func TestRunWorkspaceLink_AmbiguousSymbol(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src := makeAmbiguousRepo(t)
	dst := makeIndexedRepoForCmds(t)
	if err := runCreateCmd(t, "ambigws"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := runAddCmd(t, src, "ambigws"); err != nil {
		t.Fatalf("add src: %v", err)
	}
	if err := runAddCmd(t, dst, "ambigws"); err != nil {
		t.Fatalf("add dst: %v", err)
	}

	srcID := indexer.RepoID(src)
	dstID := indexer.RepoID(dst)

	// Act: "Shared" exists in both a/a.go and b/b.go
	_, err := runLinkCmd(t, []string{srcID, "Shared", dstID, "F", "ambigws"}, "", "", "", nil)

	// Assert
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ambiguous") {
		t.Errorf("error should mention 'ambiguous', got: %v", msg)
	}
	if !strings.Contains(msg, "--src-file") {
		t.Errorf("error should suggest '--src-file', got: %v", msg)
	}
}

// TestRunWorkspaceLink_AmbiguousSymbol_ResolvedWithSrcFile verifies that
// providing --src-file disambiguates correctly and the link is created.
func TestRunWorkspaceLink_AmbiguousSymbol_ResolvedWithSrcFile(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src := makeAmbiguousRepo(t)
	dst := makeIndexedRepoForCmds(t)
	if err := runCreateCmd(t, "disambigws"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := runAddCmd(t, src, "disambigws"); err != nil {
		t.Fatalf("add src: %v", err)
	}
	if err := runAddCmd(t, dst, "disambigws"); err != nil {
		t.Fatalf("add dst: %v", err)
	}

	srcID := indexer.RepoID(src)
	dstID := indexer.RepoID(dst)

	// Act: disambiguate by pointing to a/a.go
	out, err := runLinkCmd(t, []string{srcID, "Shared", dstID, "F", "disambigws"}, "a/a.go", "", "", nil)

	// Assert
	if err != nil {
		t.Fatalf("expected link to succeed with --src-file, got: %v", err)
	}
	if !strings.Contains(out, "Link #") {
		t.Errorf("expected confirmation output, got: %q", out)
	}
}

// TestRunWorkspaceLink_InvalidMeta verifies that a malformed --meta value
// returns a user-friendly error before any DB writes occur.
func TestRunWorkspaceLink_InvalidMeta(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "invalidmetaws")

	// Act: "protocol" has no "=" separator
	_, err := runLinkCmd(t, []string{src, "F", dst, "F", "invalidmetaws"}, "", "", "", []string{"protocol"})

	// Assert
	if err == nil {
		t.Fatal("expected error for invalid --meta format, got nil")
	}
	if !strings.Contains(err.Error(), "key=value") {
		t.Errorf("error should mention 'key=value', got: %v", err)
	}
}

// --------------------------------------------------------------------------
// workspace links
// --------------------------------------------------------------------------

// TestRunWorkspaceLinks_Empty verifies that the command prints a "no links"
// message (not an error) on a workspace with no links.
func TestRunWorkspaceLinks_Empty(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := runCreateCmd(t, "emptylinksws"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Act
	out, err := runLinksCmd(t, []string{"emptylinksws"}, "", false)

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceLinks on empty workspace: %v", err)
	}
	if !strings.Contains(out, "No links") {
		t.Errorf("expected 'No links' message, got: %q", out)
	}
}

// TestRunWorkspaceLinks_ListsAll verifies that all links appear in table output.
func TestRunWorkspaceLinks_ListsAll(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "listallws")
	if _, err := runLinkCmd(t, []string{src, "F", dst, "F", "listallws"}, "", "", "my note", nil); err != nil {
		t.Fatalf("link: %v", err)
	}

	// Act: list all with explicit workspace, no --from filter
	out, err := runLinksCmd(t, []string{"listallws"}, "", false)

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceLinks: %v", err)
	}
	if !strings.Contains(out, "#1") {
		t.Errorf("expected link #1 in output, got: %q", out)
	}
	if !strings.Contains(out, "my note") {
		t.Errorf("expected note in output, got: %q", out)
	}
}

// TestRunWorkspaceLinks_FilterFrom verifies that --from filters to only links
// whose src_repo_id matches the given path.
func TestRunWorkspaceLinks_FilterFrom(t *testing.T) {
	// Arrange: create two links from different source repos.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	r1 := makeIndexedRepoForCmds(t)
	r2 := makeIndexedRepoForCmds(t)
	r3 := makeIndexedRepoForCmds(t)
	if err := runCreateCmd(t, "filterws"); err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, p := range []string{r1, r2, r3} {
		if err := runAddCmd(t, p, "filterws"); err != nil {
			t.Fatalf("add %s: %v", p, err)
		}
	}
	// link r1→r2 and r3→r2
	if _, err := runLinkCmd(t, []string{indexer.RepoID(r1), "F", indexer.RepoID(r2), "F", "filterws"}, "", "", "", nil); err != nil {
		t.Fatalf("link r1: %v", err)
	}
	if _, err := runLinkCmd(t, []string{indexer.RepoID(r3), "F", indexer.RepoID(r2), "F", "filterws"}, "", "", "", nil); err != nil {
		t.Fatalf("link r3: %v", err)
	}

	r1ID := indexer.RepoID(r1)
	r3ID := indexer.RepoID(r3)

	// Act: filter to r1 only
	out, err := runLinksCmd(t, []string{"filterws"}, r1, false)

	// Assert: only r1's link appears
	if err != nil {
		t.Fatalf("runWorkspaceLinks --from r1: %v", err)
	}
	if !strings.Contains(out, r1ID) {
		t.Errorf("expected r1 repoID %q in output, got: %q", r1ID, out)
	}
	if strings.Contains(out, r3ID) {
		t.Errorf("r3 repoID %q should not appear when filtering by r1, got: %q", r3ID, out)
	}
}

// TestRunWorkspaceLinks_FromNotInWorkspace verifies that when --from points to
// a path not registered in the workspace, all links are listed (silent fallback).
func TestRunWorkspaceLinks_FromNotInWorkspace(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "fallbackws")
	if _, err := runLinkCmd(t, []string{src, "F", dst, "F", "fallbackws"}, "", "", "", nil); err != nil {
		t.Fatalf("link: %v", err)
	}

	// Act: --from points to a path not in the workspace
	out, err := runLinksCmd(t, []string{"fallbackws"}, "/not/a/registered/repo", false)

	// Assert: no error, all links still shown
	if err != nil {
		t.Fatalf("runWorkspaceLinks unregistered --from: %v", err)
	}
	if !strings.Contains(out, "#1") {
		t.Errorf("expected all links listed as fallback, got: %q", out)
	}
}

// TestRunWorkspaceLinks_JSON verifies that --json emits a valid JSON array
// containing the link's fields.
func TestRunWorkspaceLinks_JSON(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "jsonlinksws")
	if _, err := runLinkCmd(t, []string{src, "F", dst, "F", "jsonlinksws"}, "", "", "", nil); err != nil {
		t.Fatalf("link: %v", err)
	}

	// Act
	out, err := runLinksCmd(t, []string{"jsonlinksws"}, "", true)

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceLinks --json: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("expected JSON array output, got: %q", out)
	}
	if !strings.Contains(out, `"src_symbol"`) {
		t.Errorf("expected 'src_symbol' key in JSON, got: %q", out)
	}
}

// TestRunWorkspaceLinks_FilterSrcSymbol verifies that --src-symbol shows only
// links whose src_symbol matches the given name.
func TestRunWorkspaceLinks_FilterSrcSymbol(t *testing.T) {
	// Arrange: workspace with two links that differ by src symbol.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "srcsymws")
	if _, err := runLinkCmd(t, []string{src, "F", dst, "F", "srcsymws"}, "", "", "", nil); err != nil {
		t.Fatalf("link F: %v", err)
	}

	// Act: filter by src symbol "F" — must find it
	out, err := runLinksCmdFull(t, []string{"srcsymws"}, "", "F", "", false)

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceLinks --src-symbol: %v", err)
	}
	if !strings.Contains(out, "#1") {
		t.Errorf("expected link in output when src-symbol matches, got: %q", out)
	}
}

// TestRunWorkspaceLinks_FilterSrcSymbol_NoMatch verifies that --src-symbol with
// a name that matches no link prints "No links found."
func TestRunWorkspaceLinks_FilterSrcSymbol_NoMatch(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "srcsymnomatch")
	if _, err := runLinkCmd(t, []string{src, "F", dst, "F", "srcsymnomatch"}, "", "", "", nil); err != nil {
		t.Fatalf("link: %v", err)
	}

	// Act
	out, err := runLinksCmdFull(t, []string{"srcsymnomatch"}, "", "DoesNotExist", "", false)

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceLinks --src-symbol no match: %v", err)
	}
	if !strings.Contains(out, "No links") {
		t.Errorf("expected 'No links' message, got: %q", out)
	}
}

// TestRunWorkspaceLinks_FilterDstSymbol verifies that --dst-symbol shows only
// links whose dst_symbol matches the given name.
func TestRunWorkspaceLinks_FilterDstSymbol(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "dstsymws")
	if _, err := runLinkCmd(t, []string{src, "F", dst, "F", "dstsymws"}, "", "", "", nil); err != nil {
		t.Fatalf("link F: %v", err)
	}

	// Act
	out, err := runLinksCmdFull(t, []string{"dstsymws"}, "", "", "F", false)

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceLinks --dst-symbol: %v", err)
	}
	if !strings.Contains(out, "#1") {
		t.Errorf("expected link in output when dst-symbol matches, got: %q", out)
	}
}

// --------------------------------------------------------------------------
// workspace unlink
// --------------------------------------------------------------------------

// TestRunWorkspaceUnlink_HappyPath verifies that unlink removes an existing
// link and prints the confirmation message.
func TestRunWorkspaceUnlink_HappyPath(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src, dst := setupLinkedWorkspace(t, "unlinkws")
	if _, err := runLinkCmd(t, []string{src, "F", dst, "F", "unlinkws"}, "", "", "", nil); err != nil {
		t.Fatalf("link: %v", err)
	}

	// Act
	out, err := runUnlinkCmd(t, "1", "unlinkws")

	// Assert
	if err != nil {
		t.Fatalf("runWorkspaceUnlink: %v", err)
	}
	if !strings.Contains(out, "Link #1 removed") {
		t.Errorf("expected 'Link #1 removed', got: %q", out)
	}

	// Verify the link is gone
	db, err := workspace.OpenWorkspace("unlinkws")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	defer db.Close()
	links, _ := workspace.ListLinks(db, workspace.LinkQuery{})
	if len(links) != 0 {
		t.Errorf("expected 0 links after unlink, got %d", len(links))
	}
}

// TestRunWorkspaceUnlink_NotFound verifies that unlinking a non-existent ID
// returns an error mentioning the ID.
func TestRunWorkspaceUnlink_NotFound(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := runCreateCmd(t, "unlinknotfoundws"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Act
	_, err := runUnlinkCmd(t, "999", "unlinknotfoundws")

	// Assert
	if err == nil {
		t.Fatal("expected error for non-existent link ID, got nil")
	}
	if !strings.Contains(err.Error(), "999") {
		t.Errorf("error should mention ID 999, got: %v", err)
	}
}

// TestRunWorkspaceUnlink_InvalidID verifies that a non-numeric ID argument
// returns a user-friendly error before any DB access.
func TestRunWorkspaceUnlink_InvalidID(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := runCreateCmd(t, "invalididws"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Act
	_, err := runUnlinkCmd(t, "not-a-number", "invalididws")

	// Assert
	if err == nil {
		t.Fatal("expected error for non-numeric ID, got nil")
	}
	if !strings.Contains(err.Error(), "numeric") {
		t.Errorf("error should mention 'numeric', got: %v", err)
	}
}

// --------------------------------------------------------------------------
// workspace list
// --------------------------------------------------------------------------

func runListCmd(t *testing.T, jsonFlag bool) (string, error) {
	t.Helper()
	t.Cleanup(func() { workspaceListJSON = false })
	workspaceListJSON = jsonFlag
	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	err := runWorkspaceList(cmd, []string{})
	return out.String(), err
}

// TestRunWorkspaceList_Empty verifies that listing with no workspaces on disk
// produces no output and no error.
func TestRunWorkspaceList_Empty(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act
	out, err := runListCmd(t, false)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got: %q", out)
	}
}

// TestRunWorkspaceList_ListsAll verifies that all created workspace names
// appear in the output.
func TestRunWorkspaceList_ListsAll(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, name := range []string{"ws-one", "ws-two"} {
		if err := runCreateCmd(t, name); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}

	// Act
	out, err := runListCmd(t, false)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	for _, name := range []string{"ws-one", "ws-two"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected %q in output, got:\n%s", name, out)
		}
	}
}

// TestRunWorkspaceList_MarksCurrentWorkspace verifies that the active workspace
// is marked with * and others are not.
func TestRunWorkspaceList_MarksCurrentWorkspace(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, name := range []string{"ws-active", "ws-other"} {
		if err := runCreateCmd(t, name); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}
	if err := runUseCmd(t, "ws-active"); err != nil {
		t.Fatalf("use: %v", err)
	}

	// Act
	out, err := runListCmd(t, false)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(out, "* ws-active") {
		t.Errorf("expected active workspace marked with *, got:\n%s", out)
	}
	if strings.Contains(out, "* ws-other") {
		t.Errorf("expected ws-other NOT marked with *, got:\n%s", out)
	}
}

// TestRunWorkspaceList_JSON verifies that --json outputs a valid JSON array
// containing all workspace names.
func TestRunWorkspaceList_JSON(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, name := range []string{"alpha", "beta"} {
		if err := runCreateCmd(t, name); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}

	// Act
	out, err := runListCmd(t, true)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	for _, name := range []string{"alpha", "beta"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected %q in JSON output, got:\n%s", name, out)
		}
	}
	// Must start with '[' — a JSON array.
	trimmed := strings.TrimSpace(out)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		t.Errorf("expected JSON array output, got:\n%s", out)
	}
}

// --------------------------------------------------------------------------
// workspace delete
// --------------------------------------------------------------------------

func runDeleteCmd(t *testing.T, name string, confirm bool) (string, error) {
	t.Helper()
	t.Cleanup(func() { workspaceDeleteConfirm = false })
	workspaceDeleteConfirm = confirm
	out := &bytes.Buffer{}
	cmd := newCmd()
	cmd.SetOut(out)
	err := runWorkspaceDelete(cmd, []string{name})
	return out.String(), err
}

// TestRunWorkspaceDelete_RequiresConfirm verifies that omitting --confirm
// returns an error and does NOT delete the workspace.
func TestRunWorkspaceDelete_RequiresConfirm(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := runCreateCmd(t, "protected"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Act
	_, err := runDeleteCmd(t, "protected", false)

	// Assert: must error
	if err == nil {
		t.Fatal("expected error when --confirm is not passed, got nil")
	}
	if !strings.Contains(err.Error(), "--confirm") {
		t.Errorf("error should mention --confirm, got: %v", err)
	}

	// Workspace must still exist.
	names, _ := workspace.ListWorkspaces()
	found := false
	for _, n := range names {
		if n == "protected" {
			found = true
		}
	}
	if !found {
		t.Error("workspace was deleted despite missing --confirm")
	}
}

// TestRunWorkspaceDelete_HappyPath verifies that passing --confirm deletes
// the workspace DB and prints a confirmation message.
func TestRunWorkspaceDelete_HappyPath(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := runCreateCmd(t, "todelete"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Act
	out, err := runDeleteCmd(t, "todelete", true)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(out, "todelete") {
		t.Errorf("expected confirmation message mentioning workspace name, got: %q", out)
	}

	// Workspace must no longer appear in list.
	names, _ := workspace.ListWorkspaces()
	for _, n := range names {
		if n == "todelete" {
			t.Error("workspace still exists after delete")
		}
	}
}

// TestRunWorkspaceDelete_NotFound verifies that deleting a non-existent
// workspace returns a clear error.
func TestRunWorkspaceDelete_NotFound(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act
	_, err := runDeleteCmd(t, "ghost", true)

	// Assert
	if err == nil {
		t.Fatal("expected error for non-existent workspace, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// TestRunWorkspaceDelete_ClearsCurrentWorkspace verifies that deleting the
// active workspace clears it from the global config so subsequent commands
// do not fail with a stale reference.
func TestRunWorkspaceDelete_ClearsCurrentWorkspace(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := runCreateCmd(t, "active"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := runUseCmd(t, "active"); err != nil {
		t.Fatalf("use: %v", err)
	}

	// Act
	if _, err := runDeleteCmd(t, "active", true); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Assert: current workspace must now be unset.
	_, err := workspace.GetCurrentWorkspace()
	if !errors.Is(err, workspace.ErrNoCurrentWorkspace) {
		t.Errorf("expected ErrNoCurrentWorkspace after deleting active workspace, got: %v", err)
	}
}
