package indexer

// store_test.go — tests for RepoID, OpenIndex, GetFileHash, WriteFile,
// PruneFiles, IndexedPaths, and git_head meta.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// openTestDB opens a fresh in-memory SQLite index for a fake root path.
// The returned *sql.DB is closed automatically via t.Cleanup.
func openTestDB(t *testing.T, root string) *sql.DB {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	db, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex(%q): %v", root, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// --- RepoID ---

func TestRepoID_ContainsBasename(t *testing.T) {
	id := RepoID("/home/user/projects/mimir")
	if !strings.HasPrefix(id, "mimir-") {
		t.Errorf("RepoID should start with basename 'mimir-', got %q", id)
	}
}

func TestRepoID_SuffixIsEightHexChars(t *testing.T) {
	id := RepoID("/home/user/projects/mimir")
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 'basename-hash' format, got %q", id)
	}
	suffix := parts[1]
	if len(suffix) != 8 {
		t.Errorf("hash suffix should be 8 chars, got %d in %q", len(suffix), suffix)
	}
	for _, c := range suffix {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("hash suffix contains non-hex char %q in %q", c, suffix)
		}
	}
}

func TestRepoID_StableForSamePath(t *testing.T) {
	id1 := RepoID("/home/user/projects/mimir")
	id2 := RepoID("/home/user/projects/mimir")
	if id1 != id2 {
		t.Errorf("RepoID not stable: %q != %q", id1, id2)
	}
}

func TestRepoID_DifferentForDifferentPaths(t *testing.T) {
	id1 := RepoID("/home/user/projects/mimir")
	id2 := RepoID("/home/user/projects/other")
	if id1 == id2 {
		t.Errorf("RepoID should differ for different paths, both got %q", id1)
	}
}

// --- OpenIndex ---

func TestOpenIndex_CreatesDBFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	root := t.TempDir()
	db, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer db.Close()

	// A simple ping confirms the connection is live.
	if err := db.Ping(); err != nil {
		t.Errorf("db.Ping after OpenIndex: %v", err)
	}
}

func TestOpenIndex_SetsMetaVersion(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	var version string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'version'`).Scan(&version); err != nil {
		t.Fatalf("querying meta version: %v", err)
	}
	if version == "" {
		t.Error("meta version should not be empty")
	}
}

func TestOpenIndex_SetsMetaRoot(t *testing.T) {
	root := t.TempDir()
	db := openTestDB(t, root)

	var stored string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'root'`).Scan(&stored); err != nil {
		t.Fatalf("querying meta root: %v", err)
	}
	if stored == "" {
		t.Error("meta root should not be empty")
	}
}

func TestOpenIndex_SetsMetaRepoID(t *testing.T) {
	root := t.TempDir()
	db := openTestDB(t, root)

	var repoID string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'repo_id'`).Scan(&repoID); err != nil {
		t.Fatalf("querying meta repo_id: %v", err)
	}
	if repoID == "" {
		t.Error("meta repo_id should not be empty")
	}
}

func TestOpenIndex_IsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	db1, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("first OpenIndex: %v", err)
	}
	db1.Close()

	// Opening again must not fail (CREATE TABLE IF NOT EXISTS is idempotent).
	db2, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("second OpenIndex: %v", err)
	}
	db2.Close()
}

// --- GetFileHash ---

func TestGetFileHash_ReturnsEmptyForUnknownFile(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	hash, err := GetFileHash(db, "nonexistent.go")
	if err != nil {
		t.Fatalf("GetFileHash: %v", err)
	}
	if hash != "" {
		t.Errorf("expected empty hash for unknown file, got %q", hash)
	}
}

func TestGetFileHash_ReturnsStoredHash(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	entry := FileEntry{
		Language:  "go",
		SHA256:    "deadbeef",
		IndexedAt: time.Now().UTC(),
		Symbols:   nil,
	}
	if err := WriteFile(db, "main.go", entry); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	hash, err := GetFileHash(db, "main.go")
	if err != nil {
		t.Fatalf("GetFileHash: %v", err)
	}
	if hash != "deadbeef" {
		t.Errorf("GetFileHash: got %q, want %q", hash, "deadbeef")
	}
}

// --- WriteFile ---

func TestWriteFile_StoresFileAndSymbols(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	entry := FileEntry{
		Language:  "go",
		SHA256:    "abc123",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "main", Type: Function, StartLine: 1, EndLine: 5},
			{Name: "helper", Type: Function, StartLine: 7, EndLine: 12},
		},
	}
	if err := WriteFile(db, "main.go", entry); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Verify file row.
	var lang, sha string
	if err := db.QueryRow(`SELECT language, sha256 FROM files WHERE path = 'main.go'`).Scan(&lang, &sha); err != nil {
		t.Fatalf("querying file row: %v", err)
	}
	if lang != "go" {
		t.Errorf("language: got %q, want %q", lang, "go")
	}
	if sha != "abc123" {
		t.Errorf("sha256: got %q, want %q", sha, "abc123")
	}

	// Verify symbol rows.
	rows, err := db.Query(`SELECT name FROM symbols WHERE file_path = 'main.go' ORDER BY name`)
	if err != nil {
		t.Fatalf("querying symbols: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan symbol: %v", err)
		}
		names = append(names, n)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 symbols, got %d: %v", len(names), names)
	}
}

func TestWriteFile_OverwriteRemovesOldSymbols(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	first := FileEntry{
		Language:  "go",
		SHA256:    "v1",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "OldFunc", Type: Function, StartLine: 1, EndLine: 3},
		},
	}
	if err := WriteFile(db, "file.go", first); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}

	second := FileEntry{
		Language:  "go",
		SHA256:    "v2",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "NewFunc", Type: Function, StartLine: 1, EndLine: 3},
		},
	}
	if err := WriteFile(db, "file.go", second); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}

	// Only the new symbol should remain.
	rows, err := db.Query(`SELECT name FROM symbols WHERE file_path = 'file.go'`)
	if err != nil {
		t.Fatalf("querying symbols: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, n)
	}

	if len(names) != 1 {
		t.Fatalf("expected 1 symbol after overwrite, got %d: %v", len(names), names)
	}
	if names[0] != "NewFunc" {
		t.Errorf("expected symbol %q, got %q", "NewFunc", names[0])
	}
}

func TestWriteFile_UpdatesHash(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "f.go", FileEntry{Language: "go", SHA256: "old", IndexedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}
	if err := WriteFile(db, "f.go", FileEntry{Language: "go", SHA256: "new", IndexedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}

	hash, err := GetFileHash(db, "f.go")
	if err != nil {
		t.Fatalf("GetFileHash: %v", err)
	}
	if hash != "new" {
		t.Errorf("hash after update: got %q, want %q", hash, "new")
	}
}

// --- PruneFiles ---

func TestPruneFiles_RemovesSpecifiedPaths(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	for _, rel := range []string{"a.go", "b.go", "c.go"} {
		if err := WriteFile(db, rel, FileEntry{Language: "go", SHA256: "x", IndexedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}

	if err := PruneFiles(db, []string{"a.go", "c.go"}); err != nil {
		t.Fatalf("PruneFiles: %v", err)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}

	if paths["a.go"] {
		t.Error("a.go should have been pruned")
	}
	if paths["c.go"] {
		t.Error("c.go should have been pruned")
	}
	if !paths["b.go"] {
		t.Error("b.go should still be present")
	}
}

func TestPruneFiles_CascadesSymbols(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	entry := FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "Foo", Type: Function, StartLine: 1, EndLine: 2}},
	}
	if err := WriteFile(db, "foo.go", entry); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := PruneFiles(db, []string{"foo.go"}); err != nil {
		t.Fatalf("PruneFiles: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM symbols WHERE file_path = 'foo.go'`).Scan(&count); err != nil {
		t.Fatalf("counting symbols: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 symbols after prune, got %d", count)
	}
}

func TestPruneFiles_NoOpOnEmpty(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Should not error on empty slice.
	if err := PruneFiles(db, nil); err != nil {
		t.Errorf("PruneFiles(nil): unexpected error: %v", err)
	}
	if err := PruneFiles(db, []string{}); err != nil {
		t.Errorf("PruneFiles([]): unexpected error: %v", err)
	}
}

// --- IndexedPaths ---

func TestIndexedPaths_EmptyWhenNothingIndexed(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestIndexedPaths_ReturnsAllIndexedFiles(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	want := []string{"a.go", "b.py", "c.ts"}
	for _, rel := range want {
		if err := WriteFile(db, rel, FileEntry{Language: "go", SHA256: "x", IndexedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}

	if len(paths) != len(want) {
		t.Fatalf("expected %d paths, got %d", len(want), len(paths))
	}
	for _, rel := range want {
		if !paths[rel] {
			t.Errorf("expected path %q in IndexedPaths result", rel)
		}
	}
}

// --- git_head meta ---

// TestOpenIndex_GitRepo_StoresNonEmptyGitHead opens an index whose root is the
// mimir repository itself (a real git repo with at least one commit). The meta
// table must contain a non-empty git_head value. The test is skipped when the
// repo has no commits yet (e.g. a freshly initialised repository).
func TestOpenIndex_GitRepo_StoresNonEmptyGitHead(t *testing.T) {
	const gitRoot = "../.." // relative to pkg/indexer → repo root

	// Pre-check: skip if this repo has no commits (HEAD unresolvable).
	out, err := exec.Command("git", "-C", gitRoot, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Skip("skipping: repo has no commits (HEAD unresolvable)")
	}

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	db, openErr := OpenIndex(gitRoot)
	if openErr != nil {
		t.Fatalf("OpenIndex git root: %v", openErr)
	}
	defer db.Close()

	var head string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'git_head'`).Scan(&head); err != nil {
		t.Fatalf("querying git_head from meta: %v", err)
	}

	if head == "" {
		t.Error("git_head should be non-empty for a git repository")
	}
	// A valid SHA-1 is 40 hex characters.
	if len(head) != 40 {
		t.Errorf("git_head length: got %d, want 40 (SHA-1); value=%q", len(head), head)
	}
	for _, c := range head {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("git_head contains non-hex char %q; value=%q", c, head)
		}
	}
}

// TestOpenIndex_NonGitDir_StoresEmptyGitHead opens an index for a plain temp
// directory that is not a git repository. The meta git_head must be empty.
func TestOpenIndex_NonGitDir_StoresEmptyGitHead(t *testing.T) {
	root := t.TempDir() // plain dir, not a git repo
	db := openTestDB(t, root)

	var head string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'git_head'`).Scan(&head); err != nil {
		t.Fatalf("querying git_head from meta: %v", err)
	}

	if head != "" {
		t.Errorf("git_head should be empty for a non-git directory, got %q", head)
	}
}

// --- DropIndex ---

func TestDropIndex_RemovesDBFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	// Create the index first.
	db, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	db.Close()

	// Drop it.
	if err := DropIndex(root); err != nil {
		t.Fatalf("DropIndex: %v", err)
	}

	// Opening again must succeed — the file was removed and gets recreated cleanly.
	db2, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex after DropIndex: %v", err)
	}
	defer db2.Close()

	// The recreated index must be empty (no files).
	paths, err := IndexedPaths(db2)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected empty index after DropIndex, got %d paths", len(paths))
	}
}

func TestDropIndex_NoOpWhenIndexAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	// No index created — DropIndex must not return an error.
	if err := DropIndex(root); err != nil {
		t.Errorf("DropIndex on absent index: unexpected error: %v", err)
	}
}

func TestDropIndex_DiscardsExistingData(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	root := t.TempDir()

	// Create index and write a file into it.
	db, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	entry := FileEntry{
		Language:  "go",
		SHA256:    "abc123",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "Foo", Type: Function, StartLine: 1, EndLine: 5}},
	}
	if err := WriteFile(db, "foo.go", entry); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	db.Close()

	// Drop and reopen.
	if err := DropIndex(root); err != nil {
		t.Fatalf("DropIndex: %v", err)
	}
	db2, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex after DropIndex: %v", err)
	}
	defer db2.Close()

	// Previously written data must be gone.
	paths, err := IndexedPaths(db2)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	if paths["foo.go"] {
		t.Error("foo.go should not exist in freshly rebuilt index")
	}
}

// --- GetFileMeta ---

func TestGetFileMeta_ReturnsZeroForUnknownFile(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	meta, err := GetFileMeta(db, "nonexistent.go")
	if err != nil {
		t.Fatalf("GetFileMeta: %v", err)
	}
	if meta.Hash != "" {
		t.Errorf("expected empty hash for unknown file, got %q", meta.Hash)
	}
	if meta.Mtime != "" {
		t.Errorf("expected empty mtime for unknown file, got %q", meta.Mtime)
	}
	if meta.Size != 0 {
		t.Errorf("expected zero size for unknown file, got %d", meta.Size)
	}
}

func TestGetFileMeta_ReturnsStoredMeta(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	entry := FileEntry{
		Language:  "go",
		SHA256:    "cafebabe",
		Mtime:     "2026-01-01T00:00:00Z",
		Size:      1234,
		IndexedAt: time.Now().UTC(),
		Symbols:   nil,
	}
	if err := WriteFile(db, "main.go", entry); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	meta, err := GetFileMeta(db, "main.go")
	if err != nil {
		t.Fatalf("GetFileMeta: %v", err)
	}
	if meta.Hash != "cafebabe" {
		t.Errorf("Hash: got %q, want %q", meta.Hash, "cafebabe")
	}
	if meta.Mtime != "2026-01-01T00:00:00Z" {
		t.Errorf("Mtime: got %q, want %q", meta.Mtime, "2026-01-01T00:00:00Z")
	}
	if meta.Size != 1234 {
		t.Errorf("Size: got %d, want 1234", meta.Size)
	}
}

// --- SchemaMismatchError ---

// openRawDB opens a bare SQLite connection at the XDG path for root without
// going through OpenIndex (so no version is written yet).
func openRawDB(t *testing.T, root string) *sql.DB {
	t.Helper()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	sum := sha256.Sum256([]byte(absRoot))
	repoID := filepath.Base(absRoot) + "-" + hex.EncodeToString(sum[:])[:8]
	path := filepath.Join(dir, "mimir", "indexes", repoID, "index.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open raw: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenIndex_SchemaMismatch_ReturnsError(t *testing.T) {
	root := t.TempDir()
	raw := openRawDB(t, root)

	// Bootstrap a minimal schema and write a stale version into meta.
	if _, err := raw.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create meta table: %v", err)
	}
	staleVersion := indexVersion - 1
	if staleVersion < 1 {
		staleVersion = 1
	}
	if _, err := raw.Exec(`INSERT INTO meta (key, value) VALUES ('version', ?)`, fmt.Sprintf("%d", staleVersion)); err != nil {
		t.Fatalf("insert stale version: %v", err)
	}
	raw.Close()

	// OpenIndex must detect the mismatch and return a SchemaMismatchError.
	db, err := OpenIndex(root)
	if err == nil {
		db.Close()
		t.Fatal("expected error for stale schema version, got nil")
	}

	var mismatch *SchemaMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected *SchemaMismatchError, got: %v", err)
	}
	if mismatch.Stored != staleVersion {
		t.Errorf("Stored: got %d, want %d", mismatch.Stored, staleVersion)
	}
	if mismatch.Current != indexVersion {
		t.Errorf("Current: got %d, want %d", mismatch.Current, indexVersion)
	}
}

func TestOpenIndex_MatchingVersion_Succeeds(t *testing.T) {
	root := t.TempDir()
	raw := openRawDB(t, root)

	// Bootstrap meta table with the current version — OpenIndex must succeed.
	if _, err := raw.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create meta table: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO meta (key, value) VALUES ('version', ?)`, fmt.Sprintf("%d", indexVersion)); err != nil {
		t.Fatalf("insert current version: %v", err)
	}
	raw.Close()

	db, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex with matching version: %v", err)
	}
	db.Close()
}

func TestOpenIndex_NoVersionRow_Succeeds(t *testing.T) {
	// A brand-new DB (no meta table, no version row) must open fine.
	root := t.TempDir()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	db, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex on fresh DB: %v", err)
	}
	db.Close()
}

func TestIsSchemaMismatch_TrueForWrappedError(t *testing.T) {
	inner := &SchemaMismatchError{Stored: 1, Current: 3}
	wrapped := fmt.Errorf("outer: %w", inner)
	if !IsSchemaMismatch(wrapped) {
		t.Error("IsSchemaMismatch should return true for wrapped SchemaMismatchError")
	}
}

func TestIsSchemaMismatch_FalseForOtherError(t *testing.T) {
	if IsSchemaMismatch(fmt.Errorf("some other error")) {
		t.Error("IsSchemaMismatch should return false for unrelated error")
	}
}
