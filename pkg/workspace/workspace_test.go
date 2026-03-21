package workspace

import (
	"errors"
	"os"
	"testing"
)

// TestListWorkspaces_NoDirectory returns an empty slice (not an error) when
// the workspace directory does not exist yet.
func TestListWorkspaces_NoDirectory(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act
	names, err := ListWorkspaces()

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty slice, got: %v", names)
	}
}

// TestListWorkspaces_ReturnsAllWorkspaces verifies that creating two workspaces
// on disk causes both names to appear in the list.
func TestListWorkspaces_ReturnsAllWorkspaces(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	for _, name := range []string{"alpha", "beta"} {
		db, err := OpenWorkspace(name)
		if err != nil {
			t.Fatalf("OpenWorkspace(%q): %v", name, err)
		}
		db.Close()
	}

	// Act
	names, err := ListWorkspaces()

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 workspaces, got %d: %v", len(names), names)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	for _, want := range []string{"alpha", "beta"} {
		if !got[want] {
			t.Errorf("expected %q in list, got: %v", want, names)
		}
	}
}

// TestListWorkspaces_IgnoresNonDBFiles verifies that non-.db files in the
// workspaces directory are not returned.
func TestListWorkspaces_IgnoresNonDBFiles(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Create a real workspace so the directory is initialised.
	db, err := OpenWorkspace("real")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	db.Close()

	dir, err := configDir()
	if err != nil {
		t.Fatalf("configDir: %v", err)
	}
	if err := os.WriteFile(dir+"/stray.txt", []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Act
	names, err := ListWorkspaces()

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(names) != 1 || names[0] != "real" {
		t.Errorf("expected [real], got: %v", names)
	}
}

// TestDeleteWorkspace_DeletesDBFile verifies that the workspace DB file is
// removed from disk after a successful delete.
func TestDeleteWorkspace_DeletesDBFile(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	db, err := OpenWorkspace("todelete")
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	db.Close()

	// Act
	if err := DeleteWorkspace("todelete"); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}

	// Assert: DB file must no longer exist.
	names, err := ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	for _, n := range names {
		if n == "todelete" {
			t.Errorf("expected workspace %q to be gone, but it still appears in list", "todelete")
		}
	}
}

// TestDeleteWorkspace_NotFound verifies that deleting a non-existent workspace
// returns ErrWorkspaceNotFound.
func TestDeleteWorkspace_NotFound(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act
	err := DeleteWorkspace("ghost")

	// Assert
	if !errors.Is(err, ErrWorkspaceNotFound) {
		t.Errorf("expected ErrWorkspaceNotFound, got: %v", err)
	}
}
