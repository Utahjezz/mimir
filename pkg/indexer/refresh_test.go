package indexer

// refresh_test.go — tests for GetLastIndexedAt, ShouldRefresh, and AutoRefresh.

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- GetLastIndexedAt ---

func TestGetLastIndexedAt_ZeroWhenKeyAbsent(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	got, err := GetLastIndexedAt(db)
	if err != nil {
		t.Fatalf("GetLastIndexedAt: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time for absent key, got %v", got)
	}
}

func TestGetLastIndexedAt_ReturnsParsedTimestamp(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	want := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	if _, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('last_indexed_at', ?)`,
		want.Format(time.RFC3339),
	); err != nil {
		t.Fatalf("inserting last_indexed_at: %v", err)
	}

	got, err := GetLastIndexedAt(db)
	if err != nil {
		t.Fatalf("GetLastIndexedAt: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGetLastIndexedAt_WrittenByRun(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	before := time.Now().UTC().Add(-time.Second)

	if _, err := Run(root, db); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := GetLastIndexedAt(db)
	if err != nil {
		t.Fatalf("GetLastIndexedAt after Run: %v", err)
	}
	if got.IsZero() {
		t.Fatal("expected non-zero last_indexed_at after Run()")
	}
	if got.Before(before) {
		t.Errorf("last_indexed_at %v is before Run() started at %v", got, before)
	}
}

// --- ShouldRefresh ---

func TestShouldRefresh_TrueWhenKeyAbsent(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	stale, err := ShouldRefresh(db, 10*time.Second)
	if err != nil {
		t.Fatalf("ShouldRefresh: %v", err)
	}
	if !stale {
		t.Error("expected true (stale) when last_indexed_at is absent")
	}
}

func TestShouldRefresh_FalseWhenIndexIsFresh(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Stamp now — index is brand new.
	if _, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('last_indexed_at', ?)`,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatalf("inserting last_indexed_at: %v", err)
	}

	stale, err := ShouldRefresh(db, 10*time.Second)
	if err != nil {
		t.Fatalf("ShouldRefresh: %v", err)
	}
	if stale {
		t.Error("expected false (not stale) for a just-indexed DB")
	}
}

func TestShouldRefresh_TrueWhenIndexIsOlderThanThreshold(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Stamp a time well in the past.
	past := time.Now().UTC().Add(-1 * time.Minute)
	if _, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('last_indexed_at', ?)`,
		past.Format(time.RFC3339),
	); err != nil {
		t.Fatalf("inserting last_indexed_at: %v", err)
	}

	stale, err := ShouldRefresh(db, 10*time.Second)
	if err != nil {
		t.Fatalf("ShouldRefresh: %v", err)
	}
	if !stale {
		t.Error("expected true (stale) when index is 1m old with 10s threshold")
	}
}

func TestShouldRefresh_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		age       time.Duration // how long ago the index was stamped; -1 = absent
		threshold time.Duration
		want      bool
	}{
		{"absent key always stale", -1, 10 * time.Second, true},
		{"just indexed fresh", 1 * time.Second, 10 * time.Second, false},
		{"exactly at threshold not stale", 9 * time.Second, 10 * time.Second, false},
		{"one second past threshold stale", 11 * time.Second, 10 * time.Second, true},
		{"zero threshold always stale", 1 * time.Second, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t, t.TempDir())

			if tc.age >= 0 {
				stamp := time.Now().UTC().Add(-tc.age)
				if _, err := db.Exec(
					`INSERT INTO meta (key, value) VALUES ('last_indexed_at', ?)`,
					stamp.Format(time.RFC3339),
				); err != nil {
					t.Fatalf("inserting last_indexed_at: %v", err)
				}
			}

			got, err := ShouldRefresh(db, tc.threshold)
			if err != nil {
				t.Fatalf("ShouldRefresh: %v", err)
			}
			if got != tc.want {
				t.Errorf("ShouldRefresh = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- AutoRefresh ---

func TestAutoRefresh_SkipsRunWhenFresh(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	// Initial index.
	if _, err := Run(root, db); err != nil {
		t.Fatalf("initial Run: %v", err)
	}

	// Add a new file — but the threshold is very long so AutoRefresh should skip.
	if err := os.WriteFile(filepath.Join(root, "new.go"),
		[]byte("package main\nfunc New() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stats, err := AutoRefresh(root, db, 1*time.Hour)
	if err != nil {
		t.Fatalf("AutoRefresh: %v", err)
	}

	// Zero stats means Run() was not called.
	if stats.Added != 0 || stats.Updated != 0 || stats.Unchanged != 0 {
		t.Errorf("expected zero stats (skipped), got %+v", stats)
	}

	// Confirm new.go was NOT indexed.
	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	if paths["new.go"] {
		t.Error("new.go should not be indexed when AutoRefresh skips the walk")
	}
}

func TestAutoRefresh_RunsWhenStale(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	// Initial index.
	if _, err := Run(root, db); err != nil {
		t.Fatalf("initial Run: %v", err)
	}

	// Add a new file.
	if err := os.WriteFile(filepath.Join(root, "extra.go"),
		[]byte("package main\nfunc Extra() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Use a zero threshold — index is always considered stale.
	stats, err := AutoRefresh(root, db, 0)
	if err != nil {
		t.Fatalf("AutoRefresh: %v", err)
	}

	if stats.Added != 1 {
		t.Errorf("expected 1 added file, got %+v", stats)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	if !paths["extra.go"] {
		t.Error("extra.go should be indexed after AutoRefresh with zero threshold")
	}
}

func TestAutoRefresh_PicksUpMutatedSymbol(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	if _, err := Run(root, db); err != nil {
		t.Fatalf("initial Run: %v", err)
	}

	// Mutate main.go — add a new symbol.
	if err := os.WriteFile(filepath.Join(root, "main.go"),
		[]byte("package main\nfunc Hello() {}\nfunc World() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// AutoRefresh with zero threshold must pick up the change.
	if _, err := AutoRefresh(root, db, 0); err != nil {
		t.Fatalf("AutoRefresh: %v", err)
	}

	syms := symbolsForFile(t, db, "main.go")
	if !syms["World"] {
		t.Error(`symbol "World" should appear after AutoRefresh detects mutation`)
	}
}
