package indexer

// lookup_filepath_regression_test.go — regression tests for substring-based
// file path filtering. Previously, SearchQuery.FilePath used exact match (=)
// in SQL, so directory prefixes and filename substrings returned no results.
// Now uses INSTR() for literal substring matching.

import (
	"database/sql"
	"testing"
	"time"
)

// seedDirectoryDB creates a fixture with files under nested directories,
// mimicking a real project layout (e.g. backend/app/models/user.py).
func seedDirectoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, t.TempDir())

	files := []struct {
		path    string
		lang    string
		symbols []SymbolInfo
	}{
		{
			path: "backend/app/models/user.py",
			lang: "python",
			symbols: []SymbolInfo{
				{Name: "User", Type: Class, StartLine: 1, EndLine: 10},
				{Name: "get_full_name", Type: Method, StartLine: 5, EndLine: 8, Parent: "User"},
			},
		},
		{
			path: "backend/app/models/product.py",
			lang: "python",
			symbols: []SymbolInfo{
				{Name: "Product", Type: Class, StartLine: 1, EndLine: 15},
				{Name: "discount", Type: Method, StartLine: 8, EndLine: 12, Parent: "Product"},
			},
		},
		{
			path: "backend/app/routes/invoices.py",
			lang: "python",
			symbols: []SymbolInfo{
				{Name: "create_invoice", Type: Function, StartLine: 1, EndLine: 5},
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
			t.Fatalf("seedDirectoryDB WriteFile %s: %v", f.path, err)
		}
	}
	return db
}

// TestSearchSymbols_FilePathSubstring_Alone verifies that --file with a
// directory prefix returns all symbols under that directory.
func TestSearchSymbols_FilePathSubstring_Alone(t *testing.T) {
	db := seedDirectoryDB(t)

	// "backend/app/models" should match both user.py and product.py
	got, err := SearchSymbols(db, SearchQuery{FilePath: "backend/app/models"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// 4 symbols: User, get_full_name, Product, discount
	if len(got) != 4 {
		t.Errorf("FilePath directory prefix alone: expected 4 results, got %d: %v", len(got), got)
	}
}

// TestSearchSymbols_FilePathSubstring_WithType verifies that --file with a
// directory prefix COMBINED with --type returns the correct subset.
//
// Regression: previously file_path used exact match (=) instead of
// substring match, so directory prefixes like this returned nothing.
func TestSearchSymbols_FilePathSubstring_WithType(t *testing.T) {
	db := seedDirectoryDB(t)

	// --type class --file backend/app/models → should return User and Product
	got, err := SearchSymbols(db, SearchQuery{
		Type:     Class,
		FilePath: "backend/app/models",
	})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("FilePath prefix + Type filter: expected 2 classes (User, Product), got %d: %v", len(got), got)
	}

	names := map[string]bool{}
	for _, r := range got {
		names[r.Name] = true
		if r.Type != Class {
			t.Errorf("expected type %q, got %q for %q", Class, r.Type, r.Name)
		}
	}
	if !names["User"] || !names["Product"] {
		t.Errorf("expected User and Product, got %v", names)
	}
}

// TestSearchSymbols_FilePathSubstring_SingleFile verifies that passing a
// filename substring (not full path) still works.
func TestSearchSymbols_FilePathSubstring_SingleFile(t *testing.T) {
	db := seedDirectoryDB(t)

	// "user.py" should match "backend/app/models/user.py"
	got, err := SearchSymbols(db, SearchQuery{FilePath: "user.py"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("FilePath filename substring: expected 2 results (User + get_full_name), got %d: %v", len(got), got)
	}
}

// TestSearchSymbols_FuzzyWithFilePathSubstring verifies that the FTS5 path
// (--fuzzy + --file) also supports directory prefix matching.
func TestSearchSymbols_FuzzyWithFilePathSubstring(t *testing.T) {
	db := seedDirectoryDB(t)

	// fuzzy "disc*" + --file "backend/app/models" → should match "discount" only
	got, err := SearchSymbols(db, SearchQuery{
		FuzzyName: "disc*",
		FilePath:  "backend/app/models",
	})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("FuzzyName + FilePath prefix: expected 1 result (discount), got %d: %v", len(got), got)
	}
	if got[0].Name != "discount" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "discount")
	}
}
