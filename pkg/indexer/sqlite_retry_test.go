package indexer

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	sqlite3 "modernc.org/sqlite/lib"
)

type fakeSQLiteError struct {
	msg  string
	code int
}

func (e *fakeSQLiteError) Error() string { return e.msg }

func (e *fakeSQLiteError) Code() int { return e.code }

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

func TestRun_RetriesLastIndexedAtUntilLockReleased(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)
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

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	<-released

	if stats.Added != 1 {
		t.Fatalf("Added = %d, want 1", stats.Added)
	}

	got, err := GetLastIndexedAt(db)
	if err != nil {
		t.Fatalf("GetLastIndexedAt: %v", err)
	}
	if got.IsZero() {
		t.Fatal("expected last_indexed_at after Run")
	}
}

func TestUpdateLastIndexedAt_DoesNotRetryNonBusyError(t *testing.T) {
	db := openTestDB(t, t.TempDir())
	attempts := 0
	setTestLastIndexedAtWriter(t, func(_ *sql.DB, _ time.Time) error {
		attempts++
		return &fakeSQLiteError{msg: "disk I/O error", code: sqlite3.SQLITE_IOERR}
	})

	err := updateLastIndexedAt(db, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestIsRetryableSQLiteBusy_MatchesExtendedBusyCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{name: "busy recovery", code: sqlite3.SQLITE_BUSY_RECOVERY},
		{name: "busy snapshot", code: sqlite3.SQLITE_BUSY_SNAPSHOT},
		{name: "busy timeout", code: sqlite3.SQLITE_BUSY_TIMEOUT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &fakeSQLiteError{msg: "busy", code: tt.code}
			if !isRetryableSQLiteBusy(err) {
				t.Fatalf("expected code %d to be retryable", tt.code)
			}
		})
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

func setTestLastIndexedAtWriter(t *testing.T, writer func(db *sql.DB, indexedAt time.Time) error) {
	t.Helper()
	prev := writeLastIndexedAt
	writeLastIndexedAt = writer
	t.Cleanup(func() {
		writeLastIndexedAt = prev
	})
}
