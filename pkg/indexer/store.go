package indexer

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // register "sqlite" driver
)

// SchemaMismatchError is returned by OpenIndex when the on-disk index was built
// with a different schema version than the current binary expects. The caller
// should instruct the user to run `mimir index --rebuild <path>`.
type SchemaMismatchError struct {
	Stored  int // version recorded in the existing index
	Current int // version expected by this binary
}

func (e *SchemaMismatchError) Error() string {
	return fmt.Sprintf("index schema mismatch: stored v%d, current v%d", e.Stored, e.Current)
}

// IsSchemaMismatch reports whether err (or any error in its chain) is a
// *SchemaMismatchError.
func IsSchemaMismatch(err error) bool {
	var target *SchemaMismatchError
	return errors.As(err, &target)
}

const schema = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    path       TEXT PRIMARY KEY,
    language   TEXT NOT NULL,
    sha256     TEXT NOT NULL,
    mtime      TEXT NOT NULL DEFAULT '',
    size       INTEGER NOT NULL DEFAULT 0,
    indexed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS symbols (
    id            INTEGER PRIMARY KEY,
    file_path     TEXT    NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    name          TEXT    NOT NULL,
    type          TEXT    NOT NULL,
    start_line    INTEGER NOT NULL,
    end_line      INTEGER NOT NULL,
    parent        TEXT    NOT NULL DEFAULT '',
    name_tokens   TEXT    NOT NULL DEFAULT '',
    body_snippet  TEXT    NOT NULL DEFAULT '',
    UNIQUE (file_path, name, type, start_line)
);

CREATE INDEX IF NOT EXISTS idx_symbols_file   ON symbols(file_path);
CREATE INDEX IF NOT EXISTS idx_symbols_name   ON symbols(name);
CREATE INDEX IF NOT EXISTS idx_symbols_parent ON symbols(parent);

CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(
    name, type, parent, file_path, name_tokens, body_snippet,
    content='symbols',
    content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS symbols_fts_ai AFTER INSERT ON symbols BEGIN
    INSERT INTO symbols_fts(rowid, name, type, parent, file_path, name_tokens, body_snippet)
    VALUES (new.id, new.name, new.type, new.parent, new.file_path, new.name_tokens, new.body_snippet);
END;

CREATE TRIGGER IF NOT EXISTS symbols_fts_ad AFTER DELETE ON symbols BEGIN
    INSERT INTO symbols_fts(symbols_fts, rowid, name, type, parent, file_path, name_tokens, body_snippet)
    VALUES ('delete', old.id, old.name, old.type, old.parent, old.file_path, old.name_tokens, old.body_snippet);
END;

CREATE TRIGGER IF NOT EXISTS symbols_fts_au AFTER UPDATE ON symbols BEGIN
    INSERT INTO symbols_fts(symbols_fts, rowid, name, type, parent, file_path, name_tokens, body_snippet)
    VALUES ('delete', old.id, old.name, old.type, old.parent, old.file_path, old.name_tokens, old.body_snippet);
    INSERT INTO symbols_fts(rowid, name, type, parent, file_path, name_tokens, body_snippet)
    VALUES (new.id, new.name, new.type, new.parent, new.file_path, new.name_tokens, new.body_snippet);
END;

CREATE TABLE IF NOT EXISTS refs (
    id          INTEGER PRIMARY KEY,
    caller_file TEXT    NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    caller_name TEXT    NOT NULL DEFAULT '',
    callee_name TEXT    NOT NULL,
    line        INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_refs_caller_file ON refs(caller_file);
CREATE INDEX IF NOT EXISTS idx_refs_caller_name ON refs(caller_name);
CREATE INDEX IF NOT EXISTS idx_refs_callee_name ON refs(callee_name);

CREATE TABLE IF NOT EXISTS imports (
    id          INTEGER PRIMARY KEY,
    file_path   TEXT    NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    import_path TEXT    NOT NULL,
    alias       TEXT    NOT NULL DEFAULT '',
    line        INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_imports_file_path   ON imports(file_path);
CREATE INDEX IF NOT EXISTS idx_imports_import_path ON imports(import_path);
`

// OpenIndex opens (or creates) the SQLite index for root, applies the schema,
// and writes repo metadata. The caller is responsible for closing the DB.
func OpenIndex(root string) (*sql.DB, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve root path: %w", err)
	}

	dbPath, err := dbPath(absRoot)
	if err != nil {
		return nil, err
	}

	// 0700: index may contain source snippets; restrict to owner only
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("cannot create index directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open index db: %w", err)
	}

	// WAL mode allows concurrent readers even when a writer is active.
	// busy_timeout tells SQLite to retry for up to 5s before returning
	// SQLITE_BUSY — covers any residual write contention during indexing.
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot set busy timeout: %w", err)
	}

	// Bootstrap the meta table alone so we can read the stored schema version
	// before applying the full schema. This is a no-op on an already-initialised DB.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot bootstrap meta table: %w", err)
	}

	// Check stored schema version before applying any DDL.
	// If the index was built with a different version, refuse to open it so
	// the caller can surface a clear "run --rebuild" message.
	var storedVersionStr string
	err = db.QueryRow(`SELECT value FROM meta WHERE key = 'version'`).Scan(&storedVersionStr)
	if err != nil && err != sql.ErrNoRows {
		db.Close()
		return nil, fmt.Errorf("cannot read meta version: %w", err)
	}
	if storedVersionStr != "" {
		stored, convErr := strconv.Atoi(storedVersionStr)
		if convErr != nil {
			db.Close()
			return nil, fmt.Errorf("cannot parse meta version %q: %w", storedVersionStr, convErr)
		}
		if stored != indexVersion {
			db.Close()
			return nil, &SchemaMismatchError{Stored: stored, Current: indexVersion}
		}
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot apply schema: %w", err)
	}

	// Persist repo metadata.
	if err := setMeta(db, absRoot); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// GetLastIndexedAt returns the UTC time at which the index was last written by
// Run(). If the key is absent (index has never been run, or was built by an
// older binary), it returns a zero time.Time and nil error — callers should
// treat a zero value as "always stale".
func GetLastIndexedAt(db *sql.DB) (time.Time, error) {
	var raw string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = 'last_indexed_at'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("GetLastIndexedAt: %w", err)
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("GetLastIndexedAt: malformed timestamp %q: %w", raw, err)
	}
	return t.UTC(), nil
}

// GetFileMeta returns the stored hash, mtime, and size for rel, or a zero-value
// FileMeta if the file is not in the index. Safe to call from multiple goroutines
// concurrently.
func GetFileMeta(db *sql.DB, rel string) (FileMeta, error) {
	var meta FileMeta
	err := db.QueryRow(
		`SELECT sha256, mtime, size FROM files WHERE path = ?`, rel,
	).Scan(&meta.Hash, &meta.Mtime, &meta.Size)
	if err == sql.ErrNoRows {
		return FileMeta{}, nil
	}
	if err != nil {
		return FileMeta{}, fmt.Errorf("GetFileMeta %s: %w", rel, err)
	}
	return meta, nil
}

// GetFileHash returns the stored sha256 for rel, or "" if the file is not
// in the index. Safe to call from multiple goroutines concurrently.
func GetFileHash(db *sql.DB, rel string) (string, error) {
	meta, err := GetFileMeta(db, rel)
	return meta.Hash, err
}

// WriteFile upserts a file entry and its symbols inside a single transaction.
// Existing symbols for the file are removed via FK cascade before re-inserting.
func WriteFile(db *sql.DB, rel string, entry FileEntry) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("WriteFile begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Enable FK enforcement inside this connection/transaction.
	if _, err := tx.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("WriteFile pragma: %w", err)
	}

	// Delete the file row — FK cascade removes its symbols automatically.
	if _, err := tx.Exec(`DELETE FROM files WHERE path = ?`, rel); err != nil {
		return fmt.Errorf("WriteFile delete file: %w", err)
	}

	// Insert the new file row.
	if _, err := tx.Exec(
		`INSERT INTO files (path, language, sha256, mtime, size, indexed_at) VALUES (?, ?, ?, ?, ?, ?)`,
		rel, entry.Language, entry.SHA256, entry.Mtime, entry.Size, entry.IndexedAt.UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("WriteFile insert file: %w", err)
	}

	// Batch-insert symbols.
	stmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO symbols (file_path, name, type, start_line, end_line, parent, name_tokens, body_snippet)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("WriteFile prepare symbols: %w", err)
	}
	defer stmt.Close()

	for _, s := range entry.Symbols {
		nameTokens := strings.Join(splitIdentifier(s.Name), " ")
		if _, err := stmt.Exec(rel, s.Name, string(s.Type), s.StartLine, s.EndLine, s.Parent, nameTokens, s.BodySnippet); err != nil {
			return fmt.Errorf("WriteFile insert symbol %q: %w", s.Name, err)
		}
	}

	// Batch-insert call-site refs (if any).
	if len(entry.Calls) > 0 {
		refStmt, err := tx.Prepare(
			`INSERT INTO refs (caller_file, caller_name, callee_name, line)
			 VALUES (?, ?, ?, ?)`,
		)
		if err != nil {
			return fmt.Errorf("WriteFile prepare refs: %w", err)
		}
		defer refStmt.Close()

		for _, c := range entry.Calls {
			callerName := resolveCallerName(entry.Symbols, c.Line)
			if _, err := refStmt.Exec(rel, callerName, c.CalleeName, c.Line); err != nil {
				return fmt.Errorf("WriteFile insert ref callee=%q line=%d: %w", c.CalleeName, c.Line, err)
			}
		}
	}

	// Batch-insert imports (if any).
	if len(entry.Imports) > 0 {
		impStmt, err := tx.Prepare(
			`INSERT INTO imports (file_path, import_path, alias, line)
			 VALUES (?, ?, ?, ?)`,
		)
		if err != nil {
			return fmt.Errorf("WriteFile prepare imports: %w", err)
		}
		defer impStmt.Close()

		for _, imp := range entry.Imports {
			if _, err := impStmt.Exec(rel, imp.ImportPath, imp.Alias, imp.Line); err != nil {
				return fmt.Errorf("WriteFile insert import path=%q line=%d: %w", imp.ImportPath, imp.Line, err)
			}
		}
	}

	return tx.Commit()
}

// PruneFiles removes index entries for files that no longer exist on disk.
// deleted is the set of relative paths to remove.
func PruneFiles(db *sql.DB, deleted []string) error {
	if len(deleted) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("PruneFiles begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("PruneFiles pragma: %w", err)
	}

	stmt, err := tx.Prepare(`DELETE FROM files WHERE path = ?`)
	if err != nil {
		return fmt.Errorf("PruneFiles prepare: %w", err)
	}
	defer stmt.Close()

	for _, rel := range deleted {
		if _, err := stmt.Exec(rel); err != nil {
			return fmt.Errorf("PruneFiles delete %s: %w", rel, err)
		}
	}

	return tx.Commit()
}

// IndexedPaths returns all file paths currently stored in the index.
// Used by Run to detect deleted files.
func IndexedPaths(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT path FROM files`)
	if err != nil {
		return nil, fmt.Errorf("IndexedPaths query: %w", err)
	}
	defer rows.Close()

	paths := make(map[string]bool)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("IndexedPaths scan: %w", err)
		}
		paths[p] = true
	}
	return paths, rows.Err()
}

// DropIndex removes the SQLite index file for root.
// It is a no-op if the index does not exist yet (os.ErrNotExist is ignored).
// The caller is responsible for ensuring no open DB handle points to the file.
func DropIndex(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("cannot resolve root path: %w", err)
	}
	path, err := dbPath(absRoot)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove index: %w", err)
	}
	return nil
}

// --- helpers ---

// dbPath returns the absolute path to the SQLite file for the given
// (already-absolute) root directory.
func dbPath(absRoot string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, RepoID(absRoot), "index.db"), nil
}

// configDir returns the base directory for all mimir indexes.
// Respects $XDG_CONFIG_HOME; falls back to $HOME/.config.
func configDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "mimir", "indexes"), nil
}

// RepoID derives a stable, human-readable identifier for a repository root.
// Format: <basename>-<8-hex-chars> where the suffix is the first 8 characters
// of the SHA-256 of the absolute root path.
func RepoID(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Base(abs) + "-" + hex.EncodeToString(sum[:])[:8]
}

// setMeta writes version, root, repo_id, and git_head into the meta table.
// All 4 inserts are wrapped in a single transaction for atomicity.
// If the process dies mid-loop, meta is now either fully updated or fully rolled back.
func setMeta(db *sql.DB, absRoot string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("setMeta begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	pairs := []struct{ k, v string }{
		{"version", fmt.Sprintf("%d", indexVersion)},
		{"root", absRoot},
		{"repo_id", RepoID(absRoot)},
		{"git_head", gitHead(absRoot)},
	}
	for _, p := range pairs {
		if _, err := tx.Exec(
			`INSERT INTO meta (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			p.k, p.v,
		); err != nil {
			return fmt.Errorf("setMeta %s: %w", p.k, err)
		}
	}
	return tx.Commit()
}

// resolveCallerName returns the name of the innermost symbol in syms whose
// [StartLine, EndLine] range contains line. If no symbol contains the line,
// an empty string is returned (the call is at package/file scope).
func resolveCallerName(syms []SymbolInfo, line int) string {
	best := ""
	bestSpan := -1
	for _, s := range syms {
		if line >= s.StartLine && line <= s.EndLine {
			span := s.EndLine - s.StartLine
			if bestSpan < 0 || span < bestSpan {
				best = s.Name
				bestSpan = span
			}
		}
	}
	return best
}

// gitHead returns the current git HEAD commit hash for the repository rooted
// at dir. Returns an empty string if dir is not inside a git repository or if
// git is not available — callers should treat "" as "unknown".
func gitHead(dir string) string {
	// Guard against paths that could be interpreted as git flags.
	if strings.HasPrefix(dir, "-") {
		return ""
	}

	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
