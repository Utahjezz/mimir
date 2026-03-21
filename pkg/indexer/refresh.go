package indexer

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// autoRefreshMutex serializes re-index operations process-wide; each run holds the lock for the duration of Run().
var autoRefreshMutex sync.Mutex

// ShouldRefresh reports whether the index is stale and needs a re-walk.
// It returns true when:
//   - last_indexed_at is absent (fresh or legacy index — treat as stale), or
//   - time elapsed since the last Run() exceeds threshold.
//
// The check is a single SQLite point lookup with no filesystem I/O, making it
// safe to call before every query command.
func ShouldRefresh(db *sql.DB, threshold time.Duration) (bool, error) {
	// negative or zero threshold means "always refresh" — this is useful
	// for testing and can be set by users who want to disable the freshness check.
	if threshold <= 0 {
		threshold = 0
	}

	last, err := GetLastIndexedAt(db)
	if err != nil {
		return false, err
	}
	// Zero value means the key was missing — always refresh.
	if last.IsZero() {
		return true, nil
	}
	return time.Since(last) > threshold, nil
}

// AutoRefresh transparently re-indexes root when the index is stale.
// It calls ShouldRefresh first — if the index is younger than threshold it
// returns immediately with zero-value IndexStats and nil error, so callers pay
// only one SQLite point lookup. If the index is stale it delegates to Run()
// and returns its stats unchanged.
//
// This is the single entry point all query commands should use instead of
// calling Run() directly.
func AutoRefresh(root string, db *sql.DB, threshold time.Duration) (IndexStats, error) {
	autoRefreshMutex.Lock()
	defer autoRefreshMutex.Unlock()

	stale, err := ShouldRefresh(db, threshold)
	if err != nil {
		return IndexStats{}, fmt.Errorf("auto-refresh: %w", err)
	}
	if !stale {
		return IndexStats{}, nil
	}
	return Run(root, db)
}
