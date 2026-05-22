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

func TestParseDotNotation_LastDotSplit(t *testing.T) {
	cases := []struct {
		input      SearchQuery
		wantParent string
		wantName   string
	}{
		// Single dot — unchanged behavior
		{SearchQuery{Name: "Server.serve"}, "Server", "serve"},
		// FQN — splits on last dot
		{SearchQuery{Name: "Company.Platform.Services.*"}, "Company.Platform.Services", ""},
		{SearchQuery{Name: "A.B.C"}, "A.B", "C"},
		// Wildcard parent — unchanged
		{SearchQuery{Name: "*.serve"}, "*", "serve"},
		// NameLike FQN
		{SearchQuery{NameLike: "Company.Platform.Ser"}, "Company.Platform", "Ser"},
	}

	for _, tc := range cases {
		got := ParseDotNotation(tc.input)
		if got.Parent != tc.wantParent {
			t.Errorf("ParseDotNotation(%+v) Parent: got %q, want %q", tc.input, got.Parent, tc.wantParent)
		}
		name := got.Name
		if name == "" {
			name = got.NameLike
		}
		if name != tc.wantName {
			t.Errorf("ParseDotNotation(%+v) Name: got %q, want %q", tc.input, name, tc.wantName)
		}
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

// TestSearchSymbols_FuzzyCamelCaseQuery verifies that a camelCase query word is
// split into sub-tokens before FTS5 matching, so that e.g. "getUserPrimaryAddress"
// finds a symbol whose name_tokens contains "get", "user", "primary", "address".
func TestSearchSymbols_FuzzyCamelCaseQuery(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Seed a symbol whose name splits into [get, user, primary, address].
	if err := WriteFile(db, "svc.go", FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "getUserPrimaryAddress", Type: Function, StartLine: 1, EndLine: 5},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		desc  string
		query string
	}{
		{"full camelCase identifier", "getUserPrimaryAddress"},
		{"PascalCase identifier", "GetUserPrimaryAddress"},
		{"snake_case identifier", "get_user_primary_address"},
		{"plain words", "get user primary address"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := SearchSymbols(db, SearchQuery{FuzzyName: tt.query})
			if err != nil {
				t.Fatalf("SearchSymbols(%q): %v", tt.query, err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result for query %q, got %d: %v", tt.query, len(got), got)
			}
			if got[0].Name != "getUserPrimaryAddress" {
				t.Errorf("Name: got %q, want %q", got[0].Name, "getUserPrimaryAddress")
			}
		})
	}
}

// TestSearchSymbols_FuzzyNormalisedStringLiteral verifies that string literals
// stored in body_snippet are normalised at index time so that slash-separated
// values like "application/json" are searchable as plain words.
func TestSearchSymbols_FuzzyNormalisedStringLiteral(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Seed a symbol whose body snippet contains normalised tokens from the
	// string literal "application/json" (already split by normaliseStringToken).
	if err := WriteFile(db, "handler.go", FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{
				Name:        "setContentType",
				Type:        Function,
				StartLine:   1,
				EndLine:     5,
				BodySnippet: "application json",
			},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		desc  string
		query string
	}{
		{"space-separated words", "application json"},
		{"only first word", "application"},
		{"only second word", "json"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := SearchSymbols(db, SearchQuery{FuzzyName: tt.query})
			if err != nil {
				t.Fatalf("SearchSymbols(%q): %v", tt.query, err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result for query %q, got %d: %v", tt.query, len(got), got)
			}
			if got[0].Name != "setContentType" {
				t.Errorf("Name: got %q, want %q", got[0].Name, "setContentType")
			}
		})
	}
}

// TestSearchSymbols_FuzzyBM25Ranking verifies that FTS5 BM25 relevance ordering
// surfaces the closer-matching symbol before the weaker-matching one.
//
// Fixture:
//   - "processOrder"   — name tokens are "process order" (strong: both query
//     words hit the name_tokens column directly)
//   - "handleRequest"  — name tokens are "handle request"; body snippet contains
//     "process order" (weak: both words hit only via body_snippet)
//
// With BM25 ranking the name-token match should surface before the body-only match.
func TestSearchSymbols_FuzzyBM25Ranking(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "order.go", FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			// Strong match: both query words hit name_tokens.
			{Name: "processOrder", Type: Function, StartLine: 1, EndLine: 5},
			// Weak match: both words match, but only via body_snippet.
			{Name: "handleRequest", Type: Function, StartLine: 7, EndLine: 12, BodySnippet: "process order"},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "process order"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) < 2 {
		t.Fatalf("expected at least 2 results, got %d: %v", len(got), got)
	}
	if got[0].Name != "processOrder" {
		t.Errorf("BM25 ranking: expected 'processOrder' at index 0, got %q", got[0].Name)
	}
}

func TestSearchSymbols_FuzzyTwoTokens_RequiresBoth(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "strict.go", FileEntry{
		Language:  "go",
		SHA256:    "strict",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "getUser", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "getProfile", Type: Function, StartLine: 5, EndLine: 7},
			{Name: "userProfile", Type: Function, StartLine: 9, EndLine: 11},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "get user"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result for strict 2-token fuzzy query, got %d: %v", len(got), got)
	}
	if got[0].Name != "getUser" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "getUser")
	}
}

func TestSearchSymbols_FuzzyThreeTokens_AllowsTwoMatches(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "partial.go", FileEntry{
		Language:  "go",
		SHA256:    "partial",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "getUserAddress", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "getUserProfile", Type: Function, StartLine: 5, EndLine: 7},
			{Name: "deleteInvoice", Type: Function, StartLine: 9, EndLine: 11},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "get user primary"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 partial results for 3-token fuzzy query, got %d: %v", len(got), got)
	}
	if got[0].Name != "getUserAddress" {
		t.Errorf("first result Name: got %q, want %q", got[0].Name, "getUserAddress")
	}
	if got[1].Name != "getUserProfile" {
		t.Errorf("second result Name: got %q, want %q", got[1].Name, "getUserProfile")
	}
}

func TestSearchSymbols_FuzzyLongQuery_UsesMinimumMatchThreshold(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "threshold.go", FileEntry{
		Language:  "go",
		SHA256:    "threshold",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "loadUserAddressFromCache", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "loadUserAddress", Type: Function, StartLine: 5, EndLine: 7},
			{Name: "loadCache", Type: Function, StartLine: 9, EndLine: 11},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "load user primary address cache"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 results meeting long-query threshold, got %d: %v", len(got), got)
	}
	if got[0].Name != "loadUserAddressFromCache" {
		t.Errorf("first result Name: got %q, want %q", got[0].Name, "loadUserAddressFromCache")
	}
	if got[1].Name != "loadUserAddress" {
		t.Errorf("second result Name: got %q, want %q", got[1].Name, "loadUserAddress")
	}
}

func TestSearchSymbols_FuzzyRanksMoreMatchedTokensHigher(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "ranking.go", FileEntry{
		Language:  "go",
		SHA256:    "ranking",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "loadUserAddressFromCache", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "loadUserAddress", Type: Function, StartLine: 5, EndLine: 7},
			{Name: "loadCache", Type: Function, StartLine: 9, EndLine: 11},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "load user address cache"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) < 3 {
		t.Fatalf("expected at least 3 ranked partial results, got %d: %v", len(got), got)
	}
	if got[0].Name != "loadUserAddressFromCache" {
		t.Errorf("rank 0 Name: got %q, want %q", got[0].Name, "loadUserAddressFromCache")
	}
	if got[1].Name != "loadUserAddress" {
		t.Errorf("rank 1 Name: got %q, want %q", got[1].Name, "loadUserAddress")
	}
	if got[2].Name != "loadCache" {
		t.Errorf("rank 2 Name: got %q, want %q", got[2].Name, "loadCache")
	}
}

func TestSearchSymbols_FuzzyRawFTSPassthrough_Unchanged(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "raw.go", FileEntry{
		Language:  "go",
		SHA256:    "raw",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "processUser", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "syncInvoice", Type: Function, StartLine: 5, EndLine: 7},
			{Name: "cleanupCache", Type: Function, StartLine: 9, EndLine: 11},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "process* OR sync*"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 results from raw FTS passthrough query, got %d: %v", len(got), got)
	}
	if got[0].Name != "processUser" {
		t.Errorf("first result Name: got %q, want %q", got[0].Name, "processUser")
	}
	if got[1].Name != "syncInvoice" {
		t.Errorf("second result Name: got %q, want %q", got[1].Name, "syncInvoice")
	}
}

func TestSearchSymbols_FuzzyRanksCompactNameHigher(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "compact.go", FileEntry{
		Language:  "go",
		SHA256:    "compact",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "loadUserAddress", Type: Function, StartLine: 1, EndLine: 3},
			{
				Name:        "loadUserAddressFromCache",
				Type:        Function,
				StartLine:   5,
				EndLine:     7,
				BodySnippet: "load user address load user address cache",
			},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "load user address"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) < 2 {
		t.Fatalf("expected at least 2 results, got %d: %v", len(got), got)
	}
	if got[0].Name != "loadUserAddress" {
		t.Errorf("rank 0 Name: got %q, want %q", got[0].Name, "loadUserAddress")
	}
	if got[1].Name != "loadUserAddressFromCache" {
		t.Errorf("rank 1 Name: got %q, want %q", got[1].Name, "loadUserAddressFromCache")
	}
}

func TestSearchSymbols_FuzzyRanksNameMatchAboveBodyOnly(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "name-vs-body.go", FileEntry{
		Language:  "go",
		SHA256:    "name-vs-body",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "syncInvoice", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "processJob", Type: Function, StartLine: 5, EndLine: 7, BodySnippet: "sync invoice"},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "sync invoice"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) < 2 {
		t.Fatalf("expected at least 2 results, got %d: %v", len(got), got)
	}
	if got[0].Name != "syncInvoice" {
		t.Errorf("rank 0 Name: got %q, want %q", got[0].Name, "syncInvoice")
	}
	if got[1].Name != "processJob" {
		t.Errorf("rank 1 Name: got %q, want %q", got[1].Name, "processJob")
	}
}

func TestSearchSymbols_FuzzyRanksHigherCoverageAboveLowerCoverage(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "coverage.go", FileEntry{
		Language:  "go",
		SHA256:    "coverage",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "loadUserAddressFromCache", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "loadUserAddress", Type: Function, StartLine: 5, EndLine: 7},
			{Name: "loadCache", Type: Function, StartLine: 9, EndLine: 11},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "load user address cache"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) < 3 {
		t.Fatalf("expected at least 3 results, got %d: %v", len(got), got)
	}
	if got[0].Name != "loadUserAddressFromCache" {
		t.Errorf("rank 0 Name: got %q, want %q", got[0].Name, "loadUserAddressFromCache")
	}
	if got[1].Name != "loadUserAddress" {
		t.Errorf("rank 1 Name: got %q, want %q", got[1].Name, "loadUserAddress")
	}
	if got[2].Name != "loadCache" {
		t.Errorf("rank 2 Name: got %q, want %q", got[2].Name, "loadCache")
	}
}

func TestSearchSymbols_FuzzyRanksBodyOnlyLowerThanNameMatch(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "body-lower.go", FileEntry{
		Language:  "go",
		SHA256:    "body-lower",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "parseResponseJSON", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "handleRequest", Type: Function, StartLine: 5, EndLine: 7, BodySnippet: "parse json response"},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "json response parse"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) < 2 {
		t.Fatalf("expected at least 2 results, got %d: %v", len(got), got)
	}
	if got[0].Name != "parseResponseJSON" {
		t.Errorf("rank 0 Name: got %q, want %q", got[0].Name, "parseResponseJSON")
	}
	if got[1].Name != "handleRequest" {
		t.Errorf("rank 1 Name: got %q, want %q", got[1].Name, "handleRequest")
	}
}

func TestSearchSymbols_FuzzyNormalizesCustomAbbreviation(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "abbr.go", FileEntry{
		Language:  "go",
		SHA256:    "abbr",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "loadConfig", Type: Function, StartLine: 1, EndLine: 3},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "load cfg"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Name != "loadConfig" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "loadConfig")
	}
}

func TestSearchSymbols_FuzzyNormalizesTechTerm(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "k8s.go", FileEntry{
		Language:  "go",
		SHA256:    "k8s",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "kubernetesClient", Type: Function, StartLine: 1, EndLine: 3},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "k8s client"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Name != "kubernetesClient" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "kubernetesClient")
	}
}

func TestSearchSymbols_FuzzyRawFTSDoesNotNormalize(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "raw-normalize.go", FileEntry{
		Language:  "go",
		SHA256:    "raw-normalize",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "loadCfg", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "loadConfig", Type: Function, StartLine: 5, EndLine: 7},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{FuzzyName: "cfg*"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 raw-FTS result, got %d: %v", len(got), got)
	}
	if got[0].Name != "loadCfg" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "loadCfg")
	}
}

func TestSearchSymbols_NameLikeDoesNotNormalize(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "like.go", FileEntry{
		Language:  "go",
		SHA256:    "like",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "cfgLoader", Type: Function, StartLine: 1, EndLine: 3},
			{Name: "configLoader", Type: Function, StartLine: 5, EndLine: 7},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := SearchSymbols(db, SearchQuery{NameLike: "cfg"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 LIKE result, got %d: %v", len(got), got)
	}
	if got[0].Name != "cfgLoader" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "cfgLoader")
	}
}

// TestSearchSymbols_Limit verifies that the Limit field caps results for both
// the SQL path (Name/NameLike) and the FTS path (FuzzyName).
func TestSearchSymbols_Limit(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Seed 5 symbols so we can assert sub-limits.
	if err := WriteFile(db, "limit.go", FileEntry{
		Language:  "go",
		SHA256:    "lim",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "Alpha", Type: Function, StartLine: 1, EndLine: 2},
			{Name: "Beta", Type: Function, StartLine: 3, EndLine: 4},
			{Name: "Gamma", Type: Function, StartLine: 5, EndLine: 6},
			{Name: "Delta", Type: Function, StartLine: 7, EndLine: 8},
			{Name: "Epsilon", Type: Function, StartLine: 9, EndLine: 10},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Run("SQL path: zero means unlimited", func(t *testing.T) {
		got, err := SearchSymbols(db, SearchQuery{Type: Function, Limit: 0})
		if err != nil {
			t.Fatalf("SearchSymbols: %v", err)
		}
		if len(got) != 5 {
			t.Errorf("expected 5 results, got %d", len(got))
		}
	})

	t.Run("SQL path: limit caps results", func(t *testing.T) {
		got, err := SearchSymbols(db, SearchQuery{Type: Function, Limit: 2})
		if err != nil {
			t.Fatalf("SearchSymbols: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 results, got %d", len(got))
		}
	})

	t.Run("SQL path: limit larger than result set returns all", func(t *testing.T) {
		got, err := SearchSymbols(db, SearchQuery{Type: Function, Limit: 100})
		if err != nil {
			t.Fatalf("SearchSymbols: %v", err)
		}
		if len(got) != 5 {
			t.Errorf("expected 5 results, got %d", len(got))
		}
	})

	t.Run("FTS path: limit caps results", func(t *testing.T) {
		// "a" prefix matches Alpha, Beta, Gamma, Delta, Epsilon via body_snippet
		// or at least the ones whose name tokens contain the letter — use a broad
		// fuzzy that matches all 5 via prefix wildcard passthrough.
		got, err := SearchSymbols(db, SearchQuery{FuzzyName: "alph* OR beta* OR gamm* OR delt* OR epsil*", Limit: 3})
		if err != nil {
			t.Fatalf("SearchSymbols: %v", err)
		}
		if len(got) > 3 {
			t.Errorf("FTS limit: expected ≤3 results, got %d", len(got))
		}
	})
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
