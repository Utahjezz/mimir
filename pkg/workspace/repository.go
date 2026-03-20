package workspace

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Utahjezz/mimir/pkg/indexer"
)

// ErrRepositoryNotFound is returned when the requested repository is not in the workspace.
var ErrRepositoryNotFound = errors.New("repository not found in workspace")

func AddRepository(db *sql.DB, path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}

	repoDB, err := indexer.OpenIndex(absPath)
	if err != nil {
		return "", err
	}
	defer repoDB.Close()

	last_indexed_at, err := indexer.GetLastIndexedAt(repoDB)
	if err != nil {
		return "", err
	}

	repoID := indexer.RepoID(absPath)

	_, err = db.Exec(
		`INSERT OR IGNORE INTO repositories (id, path, last_indexed_at) VALUES (?, ?, ?)`,
		repoID, absPath, last_indexed_at,
	)
	return repoID, err
}

func ListRepositories(db *sql.DB) ([]Repository, error) {
	rows, err := db.Query(`SELECT id, path, added_at, last_indexed_at FROM repositories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.ID, &repo.Path, &repo.AddedAt, &repo.LastIndexed); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

// RemoveRepository removes a repository from the workspace by its path.
// Returns ErrRepositoryNotFound if the repository is not in the workspace.
func RemoveRepository(db *sql.DB, path string) error {
	repoID := indexer.RepoID(path)

	result, err := db.Exec(`DELETE FROM repositories WHERE id = ?`, repoID)
	if err != nil {
		return fmt.Errorf("cannot remove repository: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("cannot check rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", ErrRepositoryNotFound, repoID)
	}

	return nil
}
