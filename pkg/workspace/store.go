package workspace

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const schema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS repositories (
	id   TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	last_indexed_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS links (
    id          INTEGER PRIMARY KEY,
    src_repo_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    src_symbol  TEXT NOT NULL,
    src_file    TEXT NOT NULL DEFAULT '',
    dst_repo_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    dst_symbol  TEXT NOT NULL,
    dst_file    TEXT NOT NULL DEFAULT '',
    note        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS link_meta (
    link_id INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    key     TEXT    NOT NULL,
    value   TEXT    NOT NULL,
    PRIMARY KEY (link_id, key)
);
`

func OpenWorkspace(name string) (*sql.DB, error) {
	dbPath, err := dbPath(name)
	if err != nil {
		return nil, fmt.Errorf("cannot determine workspace path: %w", err)
	}

	// if not exists, create new workspace
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create workspace directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open index db: %w", err)
	}

	// WAL mode allows concurrent readers even when a writer is active.
	// busy_timeout tells SQLite to retry for up to 5s before returning
	// SQLITE_BUSY — covers write contention during concurrent workspace access.
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
	// If the workspace was created with a different version, refuse to open it
	// so the caller can surface a clear "recreate the workspace" message.
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
		if stored != workspaceVersion {
			db.Close()
			return nil, &SchemaMismatchError{Stored: stored, Current: workspaceVersion}
		}
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot apply schema: %w", err)
	}

	// Persist workspace metadata.
	if err := setMeta(db, name); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil

}

func dbPath(name string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%s.db", name)), nil
}

func configDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "mimir", "workspaces"), nil
}

func setMeta(db *sql.DB, name string) error {
	pairs := []struct{ k, v string }{
		{"version", fmt.Sprintf("%d", workspaceVersion)},
		{"workspace", name},
	}
	for _, p := range pairs {
		if _, err := db.Exec(
			`INSERT INTO meta (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			p.k, p.v,
		); err != nil {
			return fmt.Errorf("setMeta %s: %w", p.k, err)
		}
	}
	return nil
}

func GetMeta(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("cannot get meta %q: %w", key, err)
	}
	return value, nil
}
