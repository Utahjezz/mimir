package indexer

// callers_test.go — tests for FindCallers reverse call-site lookup.
//
// Coverage:
//   - Single caller of a symbol
//   - Multiple callers across multiple files
//   - Symbol called multiple times in the same function
//   - Symbol with no callers returns empty slice (not an error)
//   - Empty callee name returns an error

import (
	"database/sql"
	"testing"
	"time"
)

// seedCallersDB writes a three-file fixture:
//
//	a.go — foo calls bar (line 3) and baz (line 4); bar calls baz (line 8)
//	b.go — run calls bar (line 2) and foo (line 3)
//	c.go — init calls baz (line 2)
func seedCallersDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, t.TempDir())

	files := []struct {
		path  string
		syms  []SymbolInfo
		calls []CallSite
	}{
		{
			path: "a.go",
			syms: []SymbolInfo{
				{Name: "foo", Type: Function, StartLine: 1, EndLine: 6},
				{Name: "bar", Type: Function, StartLine: 7, EndLine: 10},
			},
			calls: []CallSite{
				{CalleeName: "bar", Line: 3},
				{CalleeName: "baz", Line: 4},
				{CalleeName: "baz", Line: 8},
			},
		},
		{
			path: "b.go",
			syms: []SymbolInfo{
				{Name: "run", Type: Function, StartLine: 1, EndLine: 5},
			},
			calls: []CallSite{
				{CalleeName: "bar", Line: 2},
				{CalleeName: "foo", Line: 3},
			},
		},
		{
			path: "c.go",
			syms: []SymbolInfo{
				{Name: "init", Type: Function, StartLine: 1, EndLine: 4},
			},
			calls: []CallSite{
				{CalleeName: "baz", Line: 2},
			},
		},
	}

	for _, f := range files {
		if err := WriteFile(db, f.path, FileEntry{
			Language:  "go",
			SHA256:    f.path,
			IndexedAt: time.Now().UTC(),
			Symbols:   f.syms,
			Calls:     f.calls,
		}); err != nil {
			t.Fatalf("seedCallersDB WriteFile %s: %v", f.path, err)
		}
	}

	return db
}

func TestFindCallers_SingleCaller(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "a.go", FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "caller", Type: Function, StartLine: 1, EndLine: 5}},
		Calls:     []CallSite{{CalleeName: "target", Line: 3}},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rows, err := FindCallers(db, "target")
	if err != nil {
		t.Fatalf("FindCallers: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 caller, got %d: %v", len(rows), rows)
	}
	if rows[0].CallerFile != "a.go" {
		t.Errorf("CallerFile: got %q, want %q", rows[0].CallerFile, "a.go")
	}
	if rows[0].CalleeName != "target" {
		t.Errorf("CalleeName: got %q, want %q", rows[0].CalleeName, "target")
	}
}

func TestFindCallers_MultipleCallersAcrossFiles(t *testing.T) {
	db := seedCallersDB(t)

	// bar is called from a.go (line 3) and b.go (line 2).
	rows, err := FindCallers(db, "bar")
	if err != nil {
		t.Fatalf("FindCallers bar: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 callers of bar, got %d: %v", len(rows), rows)
	}
	for _, r := range rows {
		if r.CalleeName != "bar" {
			t.Errorf("CalleeName: got %q, want %q", r.CalleeName, "bar")
		}
	}

	// Results ordered by file then line: a.go first, b.go second.
	if rows[0].CallerFile != "a.go" {
		t.Errorf("rows[0].CallerFile: got %q, want %q", rows[0].CallerFile, "a.go")
	}
	if rows[1].CallerFile != "b.go" {
		t.Errorf("rows[1].CallerFile: got %q, want %q", rows[1].CallerFile, "b.go")
	}
}

func TestFindCallers_CalledMultipleTimesInDifferentFunctions(t *testing.T) {
	db := seedCallersDB(t)

	// baz is called 3 times total: a.go line 4 (foo), a.go line 8 (bar), c.go line 2 (init).
	rows, err := FindCallers(db, "baz")
	if err != nil {
		t.Fatalf("FindCallers baz: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 callers of baz, got %d: %v", len(rows), rows)
	}
	for _, r := range rows {
		if r.CalleeName != "baz" {
			t.Errorf("CalleeName: got %q, want %q", r.CalleeName, "baz")
		}
	}
}

func TestFindCallers_NoCallers_ReturnsEmpty(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	rows, err := FindCallers(db, "orphan")
	if err != nil {
		t.Fatalf("FindCallers on unknown symbol: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 callers, got %d", len(rows))
	}
}

func TestFindCallers_EmptyCalleeName_ReturnsError(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	_, err := FindCallers(db, "")
	if err == nil {
		t.Error("expected error for empty callee name, got nil")
	}
}

func TestFindCallers_CallerNamePopulated(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Write a file where the call is inside a named function.
	if err := WriteFile(db, "main.go", FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "myFunc", Type: Function, StartLine: 1, EndLine: 10}},
		Calls:     []CallSite{{CalleeName: "helper", Line: 5}},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rows, err := FindCallers(db, "helper")
	if err != nil {
		t.Fatalf("FindCallers: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 caller, got %d", len(rows))
	}
	if rows[0].CallerName != "myFunc" {
		t.Errorf("CallerName: got %q, want %q", rows[0].CallerName, "myFunc")
	}
}
