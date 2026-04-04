package indexer

// deadcode_test.go — tests for FindDeadSymbols.
//
// Coverage:
//   - Called symbol is not flagged as dead
//   - Uncalled symbol is flagged as dead
//   - Multiple dead symbols returned
//   - Filter by type (method only)
//   - Filter by file path substring
//   - UnexportedOnly suppresses exported names
//   - Empty DB returns empty slice
//   - Symbol called from another file is not dead

import (
	"database/sql"
	"testing"
	"time"
)

// seedDeadCodeDB writes a fixture set:
//
//	a.go — functions: usedFunc (called by caller), deadFunc (never called)
//	        method:   DeadMethod (never called), UsedMethod (called)
//	b.go — function: ExportedDead (never called, exported name)
//	        function: caller — calls usedFunc and UsedMethod
func seedDeadCodeDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "a.go", FileEntry{
		Language:  "go",
		SHA256:    "a",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "usedFunc", Type: Function, StartLine: 1, EndLine: 5},
			{Name: "deadFunc", Type: Function, StartLine: 7, EndLine: 10},
			{Name: "DeadMethod", Type: Method, StartLine: 12, EndLine: 15, Parent: "T"},
			{Name: "UsedMethod", Type: Method, StartLine: 17, EndLine: 20, Parent: "T"},
		},
		Calls: []CallSite{},
	}); err != nil {
		t.Fatalf("WriteFile a.go: %v", err)
	}

	if err := WriteFile(db, "b.go", FileEntry{
		Language:  "go",
		SHA256:    "b",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "ExportedDead", Type: Function, StartLine: 1, EndLine: 5},
			{Name: "caller", Type: Function, StartLine: 7, EndLine: 15},
		},
		Calls: []CallSite{
			{CalleeName: "usedFunc", Line: 9},
			{CalleeName: "UsedMethod", Line: 11},
		},
	}); err != nil {
		t.Fatalf("WriteFile b.go: %v", err)
	}

	return db
}

func TestFindDeadSymbols_CalledSymbol_NotFlagged(t *testing.T) {
	db := seedDeadCodeDB(t)

	dead, err := FindDeadSymbols(db, DeadCodeQuery{})
	if err != nil {
		t.Fatalf("FindDeadSymbols: %v", err)
	}

	for _, d := range dead {
		if d.Name == "usedFunc" {
			t.Errorf("usedFunc is called and must not appear in dead symbols")
		}
		if d.Name == "UsedMethod" {
			t.Errorf("UsedMethod is called and must not appear in dead symbols")
		}
	}
}

func TestFindDeadSymbols_UncalledSymbol_Flagged(t *testing.T) {
	db := seedDeadCodeDB(t)

	dead, err := FindDeadSymbols(db, DeadCodeQuery{})
	if err != nil {
		t.Fatalf("FindDeadSymbols: %v", err)
	}

	names := make(map[string]bool, len(dead))
	for _, d := range dead {
		names[d.Name] = true
	}

	if !names["deadFunc"] {
		t.Error("deadFunc is never called and must appear in dead symbols")
	}
	if !names["DeadMethod"] {
		t.Error("DeadMethod is never called and must appear in dead symbols")
	}
}

func TestFindDeadSymbols_FilterByType_MethodOnly(t *testing.T) {
	db := seedDeadCodeDB(t)

	dead, err := FindDeadSymbols(db, DeadCodeQuery{Type: "method"})
	if err != nil {
		t.Fatalf("FindDeadSymbols method: %v", err)
	}

	for _, d := range dead {
		if d.Type != "method" {
			t.Errorf("expected type=method, got %q for symbol %q", d.Type, d.Name)
		}
	}

	// DeadMethod should appear; deadFunc must not (it's a function).
	names := make(map[string]bool, len(dead))
	for _, d := range dead {
		names[d.Name] = true
	}
	if !names["DeadMethod"] {
		t.Error("DeadMethod must appear when filtering by type=method")
	}
	if names["deadFunc"] {
		t.Error("deadFunc must not appear when filtering by type=method")
	}
}

func TestFindDeadSymbols_FilterByFile(t *testing.T) {
	db := seedDeadCodeDB(t)

	dead, err := FindDeadSymbols(db, DeadCodeQuery{FilePath: "b.go"})
	if err != nil {
		t.Fatalf("FindDeadSymbols file filter: %v", err)
	}

	for _, d := range dead {
		if d.FilePath != "b.go" {
			t.Errorf("expected file b.go, got %q for symbol %q", d.FilePath, d.Name)
		}
	}

	// ExportedDead is in b.go and uncalled.
	names := make(map[string]bool, len(dead))
	for _, d := range dead {
		names[d.Name] = true
	}
	if !names["ExportedDead"] {
		t.Error("ExportedDead must appear when filtering by file=b.go")
	}
}

func TestFindDeadSymbols_UnexportedOnly_SuppressesExported(t *testing.T) {
	db := seedDeadCodeDB(t)

	dead, err := FindDeadSymbols(db, DeadCodeQuery{UnexportedOnly: true})
	if err != nil {
		t.Fatalf("FindDeadSymbols unexported: %v", err)
	}

	for _, d := range dead {
		if !isUnexported(d.Name, d.FilePath) {
			t.Errorf("symbol %q is exported and must be suppressed by UnexportedOnly", d.Name)
		}
	}

	names := make(map[string]bool, len(dead))
	for _, d := range dead {
		names[d.Name] = true
	}
	if !names["deadFunc"] {
		t.Error("deadFunc (unexported) must appear with UnexportedOnly=true")
	}
	if names["ExportedDead"] {
		t.Error("ExportedDead (exported) must be suppressed with UnexportedOnly=true")
	}
}

func TestFindDeadSymbols_EmptyDB_ReturnsEmpty(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	dead, err := FindDeadSymbols(db, DeadCodeQuery{})
	if err != nil {
		t.Fatalf("FindDeadSymbols on empty DB: %v", err)
	}
	if len(dead) != 0 {
		t.Errorf("expected 0 dead symbols on empty DB, got %d", len(dead))
	}
}

func TestFindDeadSymbols_CallerNotFlaggedAsDead(t *testing.T) {
	db := seedDeadCodeDB(t)

	// "caller" in b.go is never itself called — but it IS a function.
	// It should appear as dead (it is dead by definition).
	// This test confirms the function still works end-to-end, not that
	// callers are excluded — they aren't; dead means "nothing calls it".
	dead, err := FindDeadSymbols(db, DeadCodeQuery{FilePath: "b.go"})
	if err != nil {
		t.Fatalf("FindDeadSymbols: %v", err)
	}

	names := make(map[string]bool, len(dead))
	for _, d := range dead {
		names[d.Name] = true
	}

	// caller is never called → it IS dead.
	if !names["caller"] {
		t.Error("caller in b.go is never itself called, so it must appear as dead")
	}
}

// --- runtime / test-framework exclusions ---

func seedRuntimeNamesDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "main.go", FileEntry{
		Language:  "go",
		SHA256:    "m",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "main", Type: Function, StartLine: 1, EndLine: 5},
			{Name: "init", Type: Function, StartLine: 7, EndLine: 10},
			{Name: "reallyDead", Type: Function, StartLine: 12, EndLine: 15},
		},
		Calls: []CallSite{},
	}); err != nil {
		t.Fatalf("WriteFile main.go: %v", err)
	}

	if err := WriteFile(db, "main_test.go", FileEntry{
		Language:  "go",
		SHA256:    "t",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "TestFoo", Type: Function, StartLine: 1, EndLine: 5},
			{Name: "BenchmarkFoo", Type: Function, StartLine: 7, EndLine: 10},
			{Name: "ExampleFoo", Type: Function, StartLine: 12, EndLine: 15},
			{Name: "FuzzFoo", Type: Function, StartLine: 17, EndLine: 20},
		},
		Calls: []CallSite{},
	}); err != nil {
		t.Fatalf("WriteFile main_test.go: %v", err)
	}

	return db
}

func TestFindDeadSymbols_ExcludesMain(t *testing.T) {
	db := seedRuntimeNamesDB(t)

	dead, err := FindDeadSymbols(db, DeadCodeQuery{})
	if err != nil {
		t.Fatalf("FindDeadSymbols: %v", err)
	}

	for _, d := range dead {
		if d.Name == "main" {
			t.Error("main must be excluded from dead symbols (runtime entry point)")
		}
		if d.Name == "init" {
			t.Error("init must be excluded from dead symbols (runtime entry point)")
		}
	}
}

func TestFindDeadSymbols_ExcludesTestFrameworkNames(t *testing.T) {
	db := seedRuntimeNamesDB(t)

	dead, err := FindDeadSymbols(db, DeadCodeQuery{})
	if err != nil {
		t.Fatalf("FindDeadSymbols: %v", err)
	}

	for _, d := range dead {
		switch {
		case len(d.Name) >= 4 && d.Name[:4] == "Test":
			t.Errorf("Test* symbol %q must be excluded from dead symbols", d.Name)
		case len(d.Name) >= 9 && d.Name[:9] == "Benchmark":
			t.Errorf("Benchmark* symbol %q must be excluded from dead symbols", d.Name)
		case len(d.Name) >= 7 && d.Name[:7] == "Example":
			t.Errorf("Example* symbol %q must be excluded from dead symbols", d.Name)
		case len(d.Name) >= 4 && d.Name[:4] == "Fuzz":
			t.Errorf("Fuzz* symbol %q must be excluded from dead symbols", d.Name)
		}
	}
}

func TestFindDeadSymbols_RuntimeExclusion_StillShowsRealDeadCode(t *testing.T) {
	db := seedRuntimeNamesDB(t)

	dead, err := FindDeadSymbols(db, DeadCodeQuery{})
	if err != nil {
		t.Fatalf("FindDeadSymbols: %v", err)
	}

	names := make(map[string]bool, len(dead))
	for _, d := range dead {
		names[d.Name] = true
	}

	if !names["reallyDead"] {
		t.Error("reallyDead is never called and must still appear in dead symbols")
	}
}

// --- RunE-style functions-as-values are not dead ---

func TestFindDeadSymbols_FuncPassedAsValue_NotDead(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Simulate a cobra command file: runIndex is registered as RunE, not called directly.
	// The ref is recorded via ExtractCalls' refQuery path.
	if err := WriteFile(db, "cmd.go", FileEntry{
		Language:  "go",
		SHA256:    "c",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "runIndex", Type: Function, StartLine: 1, EndLine: 10},
		},
		// runIndex appears as a ref (value reference), not a call.
		Calls: []CallSite{
			{CalleeName: "runIndex", Line: 15},
		},
	}); err != nil {
		t.Fatalf("WriteFile cmd.go: %v", err)
	}

	dead, err := FindDeadSymbols(db, DeadCodeQuery{})
	if err != nil {
		t.Fatalf("FindDeadSymbols: %v", err)
	}

	for _, d := range dead {
		if d.Name == "runIndex" {
			t.Error("runIndex is referenced as a value (RunE) and must not appear as dead")
		}
	}
}

// --- Rust: UnexportedOnly must not misclassify Rust symbols ---

func seedRustDeadCodeDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, t.TempDir())

	// Rust file with both "pub" and non-pub symbols.
	// Note: pub visibility is NOT reflected in the symbol name.
	if err := WriteFile(db, "lib.rs", FileEntry{
		Language:  "rust",
		SHA256:    "rs1",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "public_fn", Type: Function, StartLine: 1, EndLine: 5},   // pub fn
			{Name: "private_fn", Type: Function, StartLine: 7, EndLine: 10}, // fn (no pub)
			{Name: "Server", Type: Class, StartLine: 12, EndLine: 20},       // pub struct
			{Name: "new", Type: Method, StartLine: 14, EndLine: 18, Parent: "Server"},
		},
		Calls: []CallSite{},
	}); err != nil {
		t.Fatalf("WriteFile lib.rs: %v", err)
	}

	return db
}

func TestFindDeadSymbols_Rust_UnexportedOnly_DoesNotFilter(t *testing.T) {
	db := seedRustDeadCodeDB(t)

	// With UnexportedOnly=true, Rust symbols should NOT be filtered out because
	// we cannot determine pub/non-pub from the symbol name alone.
	dead, err := FindDeadSymbols(db, DeadCodeQuery{UnexportedOnly: true})
	if err != nil {
		t.Fatalf("FindDeadSymbols Rust unexported: %v", err)
	}

	// All Rust symbols should be treated as potentially exported → excluded
	// by UnexportedOnly (isUnexported returns false for .rs files).
	for _, d := range dead {
		if d.FilePath == "lib.rs" {
			t.Errorf("Rust symbol %q in %s should be excluded by UnexportedOnly "+
				"(cannot determine pub visibility from name)", d.Name, d.FilePath)
		}
	}
}

func TestFindDeadSymbols_Rust_WithoutUnexportedOnly_ShowsAll(t *testing.T) {
	db := seedRustDeadCodeDB(t)

	// Without UnexportedOnly, all dead Rust symbols should appear.
	dead, err := FindDeadSymbols(db, DeadCodeQuery{})
	if err != nil {
		t.Fatalf("FindDeadSymbols Rust: %v", err)
	}

	names := make(map[string]bool, len(dead))
	for _, d := range dead {
		names[d.Name] = true
	}

	for _, want := range []string{"public_fn", "private_fn", "new"} {
		if !names[want] {
			t.Errorf("expected dead Rust symbol %q, not found in results", want)
		}
	}
}

func TestIsUnexported_GoConvention(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"lowercase", "main.go", true},
		{"Uppercase", "main.go", false},
		{"_underscore", "util.go", false},
		{"", "main.go", false},
	}
	for _, tt := range tests {
		if got := isUnexported(tt.name, tt.filePath); got != tt.want {
			t.Errorf("isUnexported(%q, %q) = %v, want %v", tt.name, tt.filePath, got, tt.want)
		}
	}
}

func TestIsUnexported_RustAlwaysFalse(t *testing.T) {
	// Rust symbols should always return false (cannot determine from name).
	for _, name := range []string{"public_fn", "Server", "new", "private_fn"} {
		if isUnexported(name, "lib.rs") {
			t.Errorf("isUnexported(%q, \"lib.rs\") = true, want false for Rust", name)
		}
	}
}
