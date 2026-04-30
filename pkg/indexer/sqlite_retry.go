package indexer

import (
	"database/sql"
	"errors"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var lastIndexedAtRetryMaxAttempts = 5

var lastIndexedAtRetryInitialBackoff = 10 * time.Millisecond

func updateLastIndexedAt(db *sql.DB, indexedAt time.Time) error {
	return retrySQLiteBusy(func() error {
		_, err := db.Exec(
			`INSERT INTO meta (key, value) VALUES ('last_indexed_at', ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			indexedAt.UTC().Format(time.RFC3339),
		)
		return err
	})
}

func retrySQLiteBusy(run func() error) error {
	backoff := lastIndexedAtRetryInitialBackoff

	for attempt := 1; attempt <= lastIndexedAtRetryMaxAttempts; attempt++ {
		err := run()
		if err == nil {
			return nil
		}
		if !isRetryableSQLiteBusy(err) || attempt == lastIndexedAtRetryMaxAttempts {
			return err
		}
		time.Sleep(backoff)
		backoff *= 2
	}

	return nil
}

func isRetryableSQLiteBusy(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}

	return sqliteErr.Code()&0xff == sqlite3.SQLITE_BUSY
}
