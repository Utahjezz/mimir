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

// TestOpenWorkspace_SchemaMismatch verifies that opening an existing workspace
// whose stored version differs from the current binary version returns a
// SchemaMismatchError and that IsSchemaMismatch detects it correctly.
func TestOpenWorkspace_SchemaMismatch(t *testing.T) {
	// Arrange: create a valid workspace at the current version.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	db, err := OpenWorkspace("mismatch")
	if err != nil {
		t.Fatalf("first OpenWorkspace: %v", err)
	}

	// Overwrite the stored version with a stale value.
	_, err = db.Exec(`UPDATE meta SET value = '1' WHERE key = 'version'`)
	if err != nil {
		t.Fatalf("UPDATE meta version: %v", err)
	}
	db.Close()

	// Act: re-open the same workspace — the version guard must fire.
	_, err = OpenWorkspace("mismatch")

	// Assert
	if err == nil {
		t.Fatal("expected SchemaMismatchError, got nil")
	}
	if !IsSchemaMismatch(err) {
		t.Errorf("expected IsSchemaMismatch(err) == true, got false; err = %v", err)
	}
}

// TestOpenWorkspace_LinksTableExists verifies that the links and link_meta
// tables are present after OpenWorkspace.
func TestOpenWorkspace_LinksTableExists(t *testing.T) {
	// Arrange
	db := openTestWorkspace(t, "linkscheck")

	// Act: insert a dummy pair of repos then a link to confirm the schema works
	// end-to-end. We use raw SQL here because the higher-level helpers are not
	// yet wired; the goal is purely schema validation.
	_, err := db.Exec(`INSERT INTO repositories (id, path) VALUES ('repo-a', '/tmp/a'), ('repo-b', '/tmp/b')`)
	if err != nil {
		t.Fatalf("insert repos: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO links (src_repo_id, src_symbol, dst_repo_id, dst_symbol, note)
		VALUES ('repo-a', 'FuncA', 'repo-b', 'FuncB', 'test link')
	`)
	if err != nil {
		t.Fatalf("insert link: %v", err)
	}

	var linkID int64
	if err := db.QueryRow(`SELECT id FROM links LIMIT 1`).Scan(&linkID); err != nil {
		t.Fatalf("query link id: %v", err)
	}

	_, err = db.Exec(`INSERT INTO link_meta (link_id, key, value) VALUES (?, 'protocol', 'grpc')`, linkID)
	if err != nil {
		t.Fatalf("insert link_meta: %v", err)
	}

	// Assert: verify cascade — deleting the link also removes its metadata.
	if _, err := db.Exec(`DELETE FROM links WHERE id = ?`, linkID); err != nil {
		t.Fatalf("delete link: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM link_meta WHERE link_id = ?`, linkID).Scan(&count); err != nil {
		t.Fatalf("count link_meta: %v", err)
	}
	if count != 0 {
		t.Errorf("expected link_meta rows to be cascade-deleted, got %d", count)
	}
}
