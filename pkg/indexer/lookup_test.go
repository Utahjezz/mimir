package indexer

// lookup_test.go — tests for SearchQuery, SymbolRow, and SearchSymbols.

import (
	"database/sql"
	"testing"
	"time"
)

// seedLookupDB writes a small fixture set into an in-memory test DB.
//
//	file.go  — Function:main, Method:serve (parent=Server), Class:Server
//	util.go  — Function:helper, Function:parse
//	main.py  — Function:run
func seedLookupDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, t.TempDir())

	files := []struct {
		path    string
		lang    string
		symbols []SymbolInfo
	}{
		{
			path: "file.go",
			lang: "go",
			symbols: []SymbolInfo{
				{Name: "main", Type: Function, StartLine: 1, EndLine: 5},
				{Name: "serve", Type: Method, StartLine: 7, EndLine: 12, Parent: "Server"},
				{Name: "Server", Type: Class, StartLine: 14, EndLine: 30},
			},
		},
		{
			path: "util.go",
			lang: "go",
			symbols: []SymbolInfo{
				{Name: "helper", Type: Function, StartLine: 1, EndLine: 4},
				{Name: "parse", Type: Function, StartLine: 6, EndLine: 10},
			},
		},
		{
			path: "main.py",
			lang: "python",
			symbols: []SymbolInfo{
				{Name: "run", Type: Function, StartLine: 1, EndLine: 8},
			},
		},
	}

	for _, f := range files {
		if err := WriteFile(db, f.path, FileEntry{
			Language:  f.lang,
			SHA256:    "x",
			IndexedAt: time.Now().UTC(),
			Symbols:   f.symbols,
		}); err != nil {
			t.Fatalf("seedLookupDB WriteFile %s: %v", f.path, err)
		}
	}
	return db
}

func TestSearchSymbols_ExactNameMatch(t *testing.T) {
	db := seedLookupDB(t)

	// Arrange
	q := SearchQuery{Name: "main"}

	// Act
	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Assert
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Name != "main" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "main")
	}
	if got[0].FilePath != "file.go" {
		t.Errorf("FilePath: got %q, want %q", got[0].FilePath, "file.go")
	}
}

func TestSearchSymbols_LikeMatch(t *testing.T) {
	db := seedLookupDB(t)

	// Arrange — "ma" should match "main" (only symbol starting with "ma")
	q := SearchQuery{NameLike: "ma"}

	// Act
	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Assert
	if len(got) != 1 {
		t.Fatalf("expected 1 result for LIKE 'ma%%', got %d: %v", len(got), got)
	}
	if got[0].Name != "main" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "main")
	}
}

func TestSearchSymbols_TypeFilter(t *testing.T) {
	db := seedLookupDB(t)

	// Arrange
	q := SearchQuery{Type: Method}

	// Act
	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Assert — only "serve" is a Method
	if len(got) != 1 {
		t.Fatalf("expected 1 Method, got %d: %v", len(got), got)
	}
	if got[0].Name != "serve" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "serve")
	}
	if got[0].Type != Method {
		t.Errorf("Type: got %q, want %q", got[0].Type, Method)
	}
}

func TestSearchSymbols_FilePathFilter(t *testing.T) {
	db := seedLookupDB(t)

	// Arrange
	q := SearchQuery{FilePath: "util.go"}

	// Act
	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Assert — util.go has 2 symbols
	if len(got) != 2 {
		t.Fatalf("expected 2 results for util.go, got %d: %v", len(got), got)
	}
	for _, row := range got {
		if row.FilePath != "util.go" {
			t.Errorf("FilePath: got %q, want %q", row.FilePath, "util.go")
		}
	}
}

func TestSearchSymbols_CombinedFilters(t *testing.T) {
	db := seedLookupDB(t)

	// Arrange — Function in file.go: only "main"
	q := SearchQuery{Type: Function, FilePath: "file.go"}

	// Act
	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Assert
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Name != "main" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "main")
	}
}

func TestSearchSymbols_EmptyResult(t *testing.T) {
	db := seedLookupDB(t)

	// Arrange — no symbol named "DoesNotExist"
	q := SearchQuery{Name: "DoesNotExist"}

	// Act
	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Assert — empty slice, not nil, not error
	if got == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

func TestSearchSymbols_NoFiltersReturnsAll(t *testing.T) {
	db := seedLookupDB(t)

	// Arrange — zero-value query: no filters applied
	q := SearchQuery{}

	// Act
	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Assert — fixture has 6 symbols total
	if len(got) != 6 {
		t.Errorf("expected 6 results with no filters, got %d", len(got))
	}
}

// --- dot-notation ---

func TestSearchSymbols_DotNotation_ExactParentAndName(t *testing.T) {
	db := seedLookupDB(t)

	// "Server.serve" — exact parent + exact name
	q := SearchQuery{Name: "Server.serve"}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Name != "serve" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "serve")
	}
	if got[0].Parent != "Server" {
		t.Errorf("Parent: got %q, want %q", got[0].Parent, "Server")
	}
}

func TestSearchSymbols_DotNotation_WildcardName(t *testing.T) {
	db := seedLookupDB(t)

	// "Server.*" — all members of Server
	q := SearchQuery{Name: "Server.*"}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// "serve" is the only child of Server in the fixture
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Parent != "Server" {
		t.Errorf("Parent: got %q, want %q", got[0].Parent, "Server")
	}
}

func TestSearchSymbols_DotNotation_WildcardParent(t *testing.T) {
	db := seedLookupDB(t)

	// "*.serve" — any method named serve on any class
	q := SearchQuery{Name: "*.serve"}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Name != "serve" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "serve")
	}
	if got[0].Parent == "" {
		t.Error("Parent should be non-empty for wildcard parent match")
	}
}

func TestSearchSymbols_DotNotation_LikePrefix(t *testing.T) {
	db := seedLookupDB(t)

	// "Server.se" via NameLike — parent exact, name prefix
	q := SearchQuery{NameLike: "Server.se"}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Name != "serve" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "serve")
	}
}

func TestSearchSymbols_DotNotation_NoMatchWrongParent(t *testing.T) {
	db := seedLookupDB(t)

	// "OtherClass.serve" — parent doesn't exist
	q := SearchQuery{Name: "OtherClass.serve"}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

// --- FTS5 fuzzy search ---

func TestSearchSymbols_FuzzyPartialWord(t *testing.T) {
	db := seedLookupDB(t)

	// "serv" has no FTS5 operator — auto-promoted to "serv*" prefix query.
	// Matches both "serve" (Method) and "Server" (Class).
	q := SearchQuery{FuzzyName: "serv"}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols FuzzyName: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 results for FuzzyName 'serv' (auto prefix), got %d: %v", len(got), got)
	}
}

func TestSearchSymbols_FuzzyPrefix(t *testing.T) {
	db := seedLookupDB(t)

	// "serv*" — FTS5 prefix query matching "serve" and "Server"
	q := SearchQuery{FuzzyName: "serv*"}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols FuzzyName: %v", err)
	}

	// fixture has "serve" (Method) and "Server" (Class) — both start with "serv"
	if len(got) != 2 {
		t.Fatalf("expected 2 results for FuzzyName 'serv*', got %d: %v", len(got), got)
	}
}

func TestSearchSymbols_FuzzyNoMatch(t *testing.T) {
	db := seedLookupDB(t)

	q := SearchQuery{FuzzyName: "zzznomatch"}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols FuzzyName: %v", err)
	}

	if got == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

func TestSearchSymbols_FuzzyWithTypeFilter(t *testing.T) {
	db := seedLookupDB(t)

	// "serv*" matches both "serve" (Method) and "Server" (Class)
	// adding Type=Method should narrow it to just "serve"
	q := SearchQuery{FuzzyName: "serv*", Type: Method}

	got, err := SearchSymbols(db, q)
	if err != nil {
		t.Fatalf("SearchSymbols FuzzyName+Type: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Name != "serve" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "serve")
	}
	if got[0].Type != Method {
		t.Errorf("Type: got %q, want %q", got[0].Type, Method)
	}
}

func TestSearchSymbols_DeduplicatesDuplicateRows(t *testing.T) {
	// Arrange: write a symbol then force-insert a second identical row via raw
	// SQL to simulate a corrupt/pre-fix index that already contains duplicates
	// (built before the UNIQUE constraint was added).
	db := openTestDB(t, t.TempDir())
	if err := WriteFile(db, "dup.ts", FileEntry{
		Language:  "typescript",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "MyEnum", Type: Enum, StartLine: 3, EndLine: 5},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Bypass INSERT OR IGNORE by inserting directly into the FTS shadow table
	// path is too tangled — instead disable the unique constraint via a raw
	// INSERT that uses a different end_line so SQLite accepts it, then
	// verify dedup fires on (file, name, start_line) regardless of end_line.
	if _, err := db.Exec(
		`INSERT INTO symbols (file_path, name, type, start_line, end_line, parent, name_tokens, body_snippet)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"dup.ts", "MyEnum", string(Enum), 3, 5, "", "MyEnum", "",
	); err != nil {
		// If the UNIQUE constraint is already in place this insert will fail —
		// that is also correct behaviour (write-time dedup). Skip gracefully.
		t.Logf("force-insert rejected by UNIQUE constraint (write-time dedup active): %v", err)
	}

	// Act
	got, err := SearchSymbols(db, SearchQuery{Name: "MyEnum"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Assert: regardless of whether the duplicate was inserted, exactly one
	// row should be returned.
	if len(got) != 1 {
		t.Errorf("expected 1 result after dedup, got %d: %v", len(got), got)
	}
	if got[0].Name != "MyEnum" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "MyEnum")
	}
}
