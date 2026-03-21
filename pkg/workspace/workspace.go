package workspace

import (
	"fmt"
	"os"
	"strings"
)

// ListWorkspaces returns the names of all workspaces found on disk by scanning
// $XDG_CONFIG_HOME/mimir/workspaces/ for *.db files.
// Returns an empty slice (not an error) when the directory does not exist yet.
func ListWorkspaces() ([]string, error) {
	dir, err := configDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine workspace directory: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read workspace directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".db") {
			names = append(names, strings.TrimSuffix(e.Name(), ".db"))
		}
	}
	return names, nil
}

// ErrWorkspaceNotFound is returned when a workspace DB file does not exist.
var ErrWorkspaceNotFound = fmt.Errorf("workspace not found")

// DeleteWorkspace removes the workspace DB file for the named workspace.
// Returns ErrWorkspaceNotFound if the workspace does not exist.
func DeleteWorkspace(name string) error {
	path, err := dbPath(name)
	if err != nil {
		return fmt.Errorf("cannot determine workspace path: %w", err)
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrWorkspaceNotFound
		}
		return fmt.Errorf("cannot delete workspace: %w", err)
	}
	return nil
}

// WorkspaceID returns the stable identifier for a workspace.
// Workspace names are user-chosen unique strings, so the name itself
// is the identifier — no hash disambiguation is needed (contrast with
// indexer.RepoID, which hashes filesystem paths to avoid basename collisions).
func WorkspaceID(name string) string {
	return name
}

// WorkspaceDBPath returns the absolute path to the SQLite database file for
// the named workspace: $XDG_CONFIG_HOME/mimir/workspaces/<name>.db
// (or $HOME/.config/mimir/workspaces/<name>.db when XDG_CONFIG_HOME is unset).
func WorkspaceDBPath(name string) (string, error) {
	return dbPath(name)
}
