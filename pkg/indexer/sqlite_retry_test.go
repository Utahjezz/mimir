package indexer

import (
	"fmt"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestUpdateLastIndexedAt_RetriesUntilLockReleased(t *testing.T) {
	root := t.TempDir()
	db := openTestDB(t, root)
	db.SetMaxOpenConns(1)
	lockerDB, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex locker: %v", err)
	}
	lockerDB.SetMaxOpenConns(1)
	t.Cleanup(func() { lockerDB.Close() })

	setTestBusyTimeout(t, db, 1)
	setTestBusyTimeout(t, lockerDB, 1)
	setTestRetryPolicy(t, 6, 5*time.Millisecond)

	tx, err := lockerDB.Begin()
	if err != nil {
		t.Fatalf("Begin locker tx: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES ('lock_probe', '1')
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		t.Fatalf("lock probe write: %v", err)
	}

	released := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = tx.Commit()
		close(released)
	}()

	stamp := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	if err := updateLastIndexedAt(db, stamp); err != nil {
		t.Fatalf("updateLastIndexedAt: %v", err)
	}
	<-released

	got, err := GetLastIndexedAt(db)
	if err != nil {
		t.Fatalf("GetLastIndexedAt: %v", err)
	}
	if !got.Equal(stamp) {
		t.Fatalf("last_indexed_at = %v, want %v", got, stamp)
	}
}

func TestUpdateLastIndexedAt_ReturnsErrorAfterRetriesExhausted(t *testing.T) {
	root := t.TempDir()
	db := openTestDB(t, root)
	db.SetMaxOpenConns(1)
	lockerDB, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex locker: %v", err)
	}
	lockerDB.SetMaxOpenConns(1)
	t.Cleanup(func() { lockerDB.Close() })

	setTestBusyTimeout(t, db, 1)
	setTestBusyTimeout(t, lockerDB, 1)
	setTestRetryPolicy(t, 2, 1*time.Millisecond)

	tx, err := lockerDB.Begin()
	if err != nil {
		t.Fatalf("Begin locker tx: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES ('lock_probe', '1')
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		t.Fatalf("lock probe write: %v", err)
	}

	err = updateLastIndexedAt(db, time.Now().UTC())
	if err == nil {
		t.Fatal("expected busy error, got nil")
	}
	if !strings.Contains(err.Error(), "database is locked") {
		t.Fatalf("expected locked error, got %v", err)
	}

	got, getErr := GetLastIndexedAt(db)
	if getErr != nil {
		t.Fatalf("GetLastIndexedAt: %v", getErr)
	}
	if !got.IsZero() {
		t.Fatalf("expected last_indexed_at to remain unset, got %v", got)
	}
}

func setTestBusyTimeout(t *testing.T, db *sql.DB, timeoutMS int) {
	t.Helper()
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA busy_timeout = %d`, timeoutMS)); err != nil {
		t.Fatalf("set busy_timeout: %v", err)
	}
}

func setTestRetryPolicy(t *testing.T, attempts int, backoff time.Duration) {
	t.Helper()
	prevAttempts := lastIndexedAtRetryMaxAttempts
	prevBackoff := lastIndexedAtRetryInitialBackoff
	lastIndexedAtRetryMaxAttempts = attempts
	lastIndexedAtRetryInitialBackoff = backoff
	t.Cleanup(func() {
		lastIndexedAtRetryMaxAttempts = prevAttempts
		lastIndexedAtRetryInitialBackoff = prevBackoff
	})
}
