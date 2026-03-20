package workspace

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

const schema = `
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS repositories (
	id   TEXT PRIMARY KEY,
	added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	last_indexed_at TIMESTAMP
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

	// Bootstrap the meta table alone so we can read the stored schema version
	// before applying the full schema. This is a no-op on an already-initialised DB.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot bootstrap meta table: %w", err)
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
