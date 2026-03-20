package workspace

// repository_test.go — unit tests for AddRepository, ListRepositories,
// and RemoveRepository.

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Utahjezz/mimir/pkg/indexer"
	_ "modernc.org/sqlite"
)

// makeIndexedRepo creates a temporary directory, writes a minimal Go file into
// it, and runs the indexer so that indexer.OpenIndex succeeds on that path.
// It returns the absolute path to the directory.
func makeIndexedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	goFile := filepath.Join(root, "hello.go")
	if err := os.WriteFile(goFile, []byte("package main\nfunc Hello() {}\n"), 0o644); err != nil {
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

// openFreshWorkspace opens a fresh workspace DB with all I/O redirected to tmp.
func openFreshWorkspace(t *testing.T, tmp string) *sql.DB {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	db, err := OpenWorkspace("repotest")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestAddRepository_AddsRow verifies that after AddRepository the repo appears
// in the list returned by ListRepositories.
func TestAddRepository_AddsRow(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	db := openFreshWorkspace(t, tmp)
	repoPath := makeIndexedRepo(t)

	// Act
	_, err := AddRepository(db, repoPath)

	// Assert
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	repos, err := ListRepositories(db)
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repository, got %d", len(repos))
	}
}

// TestAddRepository_StoresPath verifies that the stored Repository.Path matches
// the path that was passed to AddRepository.
func TestAddRepository_StoresPath(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	db := openFreshWorkspace(t, tmp)
	repoPath := makeIndexedRepo(t)

	// Act
	_, err := AddRepository(db, repoPath)

	// Assert
	if err != nil {
		t.Fatalf("AddRepository: %v", err)
	}
	repos, err := ListRepositories(db)
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) == 0 {
		t.Fatal("expected at least one repository")
	}
	if repos[0].Path != repoPath {
		t.Errorf("stored path: got %q, want %q", repos[0].Path, repoPath)
	}
}

// TestAddRepository_Idempotent verifies that adding the same path twice does
// not error and does not create duplicate rows (INSERT OR IGNORE semantics).
func TestAddRepository_Idempotent(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	db := openFreshWorkspace(t, tmp)
	repoPath := makeIndexedRepo(t)

	// Act: add same path twice
	if _, err := AddRepository(db, repoPath); err != nil {
		t.Fatalf("first AddRepository: %v", err)
	}
	if _, err := AddRepository(db, repoPath); err != nil {
		t.Fatalf("second AddRepository: %v", err)
	}

	// Assert: exactly one row
	repos, err := ListRepositories(db)
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repo after double-add, got %d", len(repos))
	}
}

// TestListRepositories_Empty verifies that ListRepositories returns an empty
// (non-nil) slice — not an error — on a fresh workspace.
func TestListRepositories_Empty(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	db := openFreshWorkspace(t, tmp)

	// Act
	repos, err := ListRepositories(db)

	// Assert
	if err != nil {
		t.Fatalf("ListRepositories on empty workspace: %v", err)
	}
	// The result must be usable as a slice (nil is fine; length must be 0).
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

// TestRemoveRepository_RemovesRow verifies that after add+remove the repository
// no longer appears in ListRepositories.
func TestRemoveRepository_RemovesRow(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	db := openFreshWorkspace(t, tmp)
	repoPath := makeIndexedRepo(t)

	if _, err := AddRepository(db, repoPath); err != nil {
		t.Fatalf("AddRepository: %v", err)
	}

	// Act
	if err := RemoveRepository(db, repoPath); err != nil {
		t.Fatalf("RemoveRepository: %v", err)
	}

	// Assert
	repos, err := ListRepositories(db)
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	for _, r := range repos {
		if r.Path == repoPath {
			t.Errorf("repo %q should have been removed but is still listed", repoPath)
		}
	}
}

// TestRemoveRepository_NotFound verifies that ErrRepositoryNotFound is returned
// when trying to remove a path that was never added.
func TestRemoveRepository_NotFound(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	db := openFreshWorkspace(t, tmp)

	// Act
	err := RemoveRepository(db, "/nonexistent/path")

	// Assert
	if !errors.Is(err, ErrRepositoryNotFound) {
		t.Errorf("expected ErrRepositoryNotFound, got: %v", err)
	}
}
