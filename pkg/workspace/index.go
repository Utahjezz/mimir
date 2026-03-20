package workspace

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/Utahjezz/mimir/pkg/indexer"
)

// RepoResult holds the outcome of indexing a single repository.
type RepoResult struct {
	Repo  Repository
	Stats indexer.IndexStats
	Err   error
}

// IndexWorkspace indexes all repositories in the workspace concurrently,
// running at most concurrency repos at the same time.
// Results are streamed to the returned channel in completion order.
// The channel is closed once all repos have been processed.
func IndexWorkspace(db *sql.DB, concurrency int, rebuild bool) (<-chan RepoResult, error) {
	repos, err := ListRepositories(db)
	if err != nil {
		return nil, fmt.Errorf("cannot list repositories: %w", err)
	}

	results := make(chan RepoResult, len(repos))

	go func() {
		defer close(results)

		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup

		for _, repo := range repos {
			repo := repo // capture
			wg.Add(1)
			sem <- struct{}{} // acquire slot

			go func() {
				defer wg.Done()
				defer func() { <-sem }() // release slot

				results <- indexRepo(db, repo, rebuild)
			}()
		}

		wg.Wait()
	}()

	return results, nil
}

// indexRepo runs indexer.Run on a single repository and updates last_indexed_at
// in the workspace DB on success.
func indexRepo(db *sql.DB, repo Repository, rebuild bool) RepoResult {
	if rebuild {
		if err := indexer.DropIndex(repo.Path); err != nil {
			return RepoResult{Repo: repo, Err: fmt.Errorf("cannot drop index: %w", err)}
		}
	}

	repoDB, err := indexer.OpenIndex(repo.Path)
	if err != nil {
		return RepoResult{Repo: repo, Err: fmt.Errorf("cannot open index: %w", err)}
	}
	defer repoDB.Close()

	stats, err := indexer.Run(repo.Path, repoDB)
	if err != nil {
		return RepoResult{Repo: repo, Err: fmt.Errorf("indexing failed: %w", err)}
	}

	// Update last_indexed_at in the workspace DB.
	_, _ = db.Exec(
		`UPDATE repositories SET last_indexed_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), repo.ID,
	)

	return RepoResult{Repo: repo, Stats: stats}
}
