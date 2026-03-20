package workspace

// store_test.go — unit tests for OpenWorkspace and GetMeta.

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"
)

// openTestWorkspace is a test helper that opens a fresh workspace DB for the
// given name inside a temp XDG_CONFIG_HOME. The DB is closed via t.Cleanup.
func openTestWorkspace(t *testing.T, name string) *sql.DB {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	db, err := OpenWorkspace(name)
	if err != nil {
		t.Fatalf("OpenWorkspace(%q): %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestOpenWorkspace_CreatesDB verifies that OpenWorkspace succeeds and the
// returned connection is live (Ping succeeds).
func TestOpenWorkspace_CreatesDB(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act
	db, err := OpenWorkspace("myws")

	// Assert
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Errorf("db.Ping after OpenWorkspace: %v", err)
	}
}

// TestOpenWorkspace_IdempotentSchema verifies that calling OpenWorkspace twice
// on the same name does not error (CREATE TABLE IF NOT EXISTS is idempotent).
func TestOpenWorkspace_IdempotentSchema(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Act: first open
	db1, err := OpenWorkspace("idempotent")
	if err != nil {
		t.Fatalf("first OpenWorkspace: %v", err)
	}
	db1.Close()

	// Act: second open on the same name
	db2, err := OpenWorkspace("idempotent")

	// Assert
	if err != nil {
		t.Fatalf("second OpenWorkspace (idempotent): %v", err)
	}
	db2.Close()
}

// TestOpenWorkspace_SetsMetaVersion verifies that GetMeta returns the correct
// schema version string after a workspace is opened.
func TestOpenWorkspace_SetsMetaVersion(t *testing.T) {
	// Arrange
	db := openTestWorkspace(t, "versioned")

	// Act
	version, err := GetMeta(db, "version")

	// Assert
	if err != nil {
		t.Fatalf("GetMeta version: %v", err)
	}
	want := fmt.Sprintf("%d", workspaceVersion)
	if version != want {
		t.Errorf("meta version: got %q, want %q", version, want)
	}
}

// TestOpenWorkspace_SetsMetaName verifies that GetMeta returns the workspace
// name stored in the meta table.
func TestOpenWorkspace_SetsMetaName(t *testing.T) {
	// Arrange
	const name = "testworkspace"
	db := openTestWorkspace(t, name)

	// Act
	stored, err := GetMeta(db, "workspace")

	// Assert
	if err != nil {
		t.Fatalf("GetMeta workspace: %v", err)
	}
	if stored != name {
		t.Errorf("meta workspace: got %q, want %q", stored, name)
	}
}

// TestGetMeta_MissingKey verifies that GetMeta returns an error for a key that
// does not exist in the meta table.
func TestGetMeta_MissingKey(t *testing.T) {
	// Arrange
	db := openTestWorkspace(t, "keytest")

	// Act
	_, err := GetMeta(db, "nonexistent_key")

	// Assert
	if err == nil {
		t.Fatal("expected error for missing meta key, got nil")
	}
	// Must not be a nil-error sentinel — any non-nil error is acceptable.
	if errors.Is(err, sql.ErrNoRows) {
		// The implementation wraps sql.ErrNoRows — that is fine.
		return
	}
	// Otherwise ensure it is truly non-nil (already confirmed above).
}
