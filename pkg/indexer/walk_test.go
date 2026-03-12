package indexer

// walk_test.go — tests for Run (concurrent directory walker + smart re-indexer).

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeDir creates a temporary directory tree for walk tests.
// files is a map of relative path → content.
func makeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("makeDir MkdirAll: %v", err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("makeDir WriteFile %s: %v", rel, err)
		}
	}
	return root
}

// openWalkDB opens a fresh SQLite index scoped to root for walk tests.
func openWalkDB(t *testing.T, root string) *sql.DB {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	db, err := OpenIndex(root)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// symbolsForFile queries all symbol names stored for a relative path.
func symbolsForFile(t *testing.T, db *sql.DB, rel string) map[string]bool {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM symbols WHERE file_path = ?`, rel)
	if err != nil {
		t.Fatalf("symbolsForFile query %s: %v", rel, err)
	}
	defer rows.Close()
	m := make(map[string]bool)
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("symbolsForFile scan: %v", err)
		}
		m[n] = true
	}
	return m
}

// --- basic indexing ---

func TestRun_IndexesNewFiles(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if stats.Added != 1 {
		t.Errorf("Added: got %d, want 1", stats.Added)
	}
	if stats.Unchanged != 0 {
		t.Errorf("Unchanged: got %d, want 0", stats.Unchanged)
	}
	if stats.Errors != 0 {
		t.Errorf("Errors: got %d, want 0", stats.Errors)
	}

	hash, err := GetFileHash(db, "main.go")
	if err != nil {
		t.Fatalf("GetFileHash: %v", err)
	}
	if hash == "" {
		t.Error("SHA256 should not be empty after indexing")
	}

	var lang string
	if err := db.QueryRow(`SELECT language FROM files WHERE path = 'main.go'`).Scan(&lang); err != nil {
		t.Fatalf("querying language: %v", err)
	}
	if lang != "go" {
		t.Errorf("Language: got %q, want %q", lang, "go")
	}

	syms := symbolsForFile(t, db, "main.go")
	if !syms["Hello"] {
		t.Error(`symbol "Hello" not found in indexed file`)
	}
}

func TestRun_SkipsUnsupportedExtensions(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go":   "package main\nfunc Hello() {}\n",
		"README.md": "# readme",
		"data.json": `{"key": "value"}`,
	})
	db := openWalkDB(t, root)

	_, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}

	if paths["README.md"] {
		t.Error("README.md should not be indexed")
	}
	if paths["data.json"] {
		t.Error("data.json should not be indexed")
	}
	if !paths["main.go"] {
		t.Error("main.go should be indexed")
	}
}

func TestRun_SkipsSkipDirs(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go":                   "package main\nfunc Hello() {}\n",
		".git/config":               "[core]",
		"vendor/lib/lib.go":         "package lib\nfunc Lib() {}\n",
		"node_modules/pkg/index.js": "function noop() {}",
	})
	db := openWalkDB(t, root)

	_, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}

	for rel := range paths {
		for skip := range skipDirs {
			if filepath.HasPrefix(rel, skip) {
				t.Errorf("file inside %s should not be indexed: %s", skip, rel)
			}
		}
	}

	if !paths["main.go"] {
		t.Error("main.go should be indexed")
	}
}

// --- smart re-indexing ---

func TestRun_SkipsUnchangedFiles(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	if _, err := Run(root, db); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	if stats.Unchanged != 1 {
		t.Errorf("second run Unchanged: got %d, want 1", stats.Unchanged)
	}
	if stats.Updated != 0 {
		t.Errorf("second run Updated: got %d, want 0", stats.Updated)
	}
}

func TestRun_ReindexesChangedFiles(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	if _, err := Run(root, db); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "main.go"),
		[]byte("package main\nfunc Hello() {}\nfunc World() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	if stats.Updated != 1 {
		t.Errorf("Updated: got %d, want 1", stats.Updated)
	}

	syms := symbolsForFile(t, db, "main.go")
	if !syms["World"] {
		t.Error(`symbol "World" should appear after re-index`)
	}
}

func TestRun_PrunesDeletedFiles(t *testing.T) {
	root := makeDir(t, map[string]string{
		"a.go": "package main\nfunc A() {}\n",
		"b.go": "package main\nfunc B() {}\n",
	})
	db := openWalkDB(t, root)

	if _, err := Run(root, db); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	if err := os.Remove(filepath.Join(root, "b.go")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	if stats.Removed != 1 {
		t.Errorf("Removed: got %d, want 1", stats.Removed)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}

	if paths["b.go"] {
		t.Error("b.go should have been pruned from the index")
	}
	if !paths["a.go"] {
		t.Error("a.go should still be present")
	}
}

// --- multi-language ---

func TestRun_IndexesMultipleLanguages(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go":  "package main\nfunc GoFunc() {}\n",
		"app.py":   "def py_func(): pass\n",
		"index.js": "function jsFunc() {}\n",
		"types.ts": "function tsFunc() {}\n",
	})
	db := openWalkDB(t, root)

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if stats.Added != 4 {
		t.Errorf("Added: got %d, want 4", stats.Added)
	}

	wantLangs := map[string]string{
		"main.go":  "go",
		"app.py":   "python",
		"index.js": "javascript",
		"types.ts": "typescript",
	}
	for rel, wantLang := range wantLangs {
		var lang string
		err := db.QueryRow(`SELECT language FROM files WHERE path = ?`, rel).Scan(&lang)
		if err != nil {
			t.Errorf("querying language for %s: %v", rel, err)
			continue
		}
		if lang != wantLang {
			t.Errorf("%s: language got %q, want %q", rel, lang, wantLang)
		}
	}
}

// --- error collection ---

func TestRun_CollectsFileErrors(t *testing.T) {
	root := makeDir(t, map[string]string{
		"good.go": "package main\nfunc Good() {}\n",
		"bad.go":  "package main\nfunc Bad() {}\n",
	})
	db := openWalkDB(t, root)

	// Make bad.go unreadable.
	if err := os.Chmod(filepath.Join(root, "bad.go"), 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(filepath.Join(root, "bad.go"), 0o644)
	})

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run should not return a fatal error: %v", err)
	}

	if stats.Errors != 1 {
		t.Errorf("Errors: got %d, want 1", stats.Errors)
	}
	if len(stats.FileErrors) != 1 {
		t.Fatalf("FileErrors: got %d entries, want 1", len(stats.FileErrors))
	}
	if stats.FileErrors[0].Path != "bad.go" {
		t.Errorf("FileErrors[0].Path: got %q, want %q", stats.FileErrors[0].Path, "bad.go")
	}
	if stats.FileErrors[0].Err == nil {
		t.Error("FileErrors[0].Err should not be nil")
	}
}

func TestRun_ContinuesAfterFileError(t *testing.T) {
	root := makeDir(t, map[string]string{
		"good.go": "package main\nfunc Good() {}\n",
		"bad.go":  "package main\nfunc Bad() {}\n",
	})
	db := openWalkDB(t, root)

	if err := os.Chmod(filepath.Join(root, "bad.go"), 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(filepath.Join(root, "bad.go"), 0o644)
	})

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// good.go should still be indexed despite bad.go failing.
	if stats.Added != 1 {
		t.Errorf("Added: got %d, want 1 (good.go only)", stats.Added)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	if !paths["good.go"] {
		t.Error("good.go should be indexed even when bad.go fails")
	}
}

// --- concurrency ---

func TestRun_ConcurrentLargeRepo(t *testing.T) {
	// Create more files than the worker pool size to exercise the full pipeline.
	files := make(map[string]string, workerCount*3)
	for i := range workerCount * 3 {
		name := filepath.Join("pkg", "file"+string(rune('a'+i))+".go")
		files[name] = "package pkg\nfunc F" + string(rune('A'+i)) + "() {}\n"
	}

	root := makeDir(t, files)
	db := openWalkDB(t, root)

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := workerCount * 3
	if stats.Added != want {
		t.Errorf("Added: got %d, want %d", stats.Added, want)
	}
	if stats.Errors != 0 {
		t.Errorf("unexpected errors: %v", stats.FileErrors)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}
	if len(paths) != want {
		t.Errorf("indexed files: got %d, want %d", len(paths), want)
	}
}

func TestRun_ConcurrentReindexIsIdempotent(t *testing.T) {
	// Run twice with no changes — all files should be unchanged on second pass.
	files := make(map[string]string, workerCount*2)
	for i := range workerCount * 2 {
		name := "file" + string(rune('a'+i)) + ".go"
		files[name] = "package main\nfunc F" + string(rune('A'+i)) + "() {}\n"
	}

	root := makeDir(t, files)
	db := openWalkDB(t, root)

	if _, err := Run(root, db); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}

	want := workerCount * 2
	if stats.Unchanged != want {
		t.Errorf("second run Unchanged: got %d, want %d", stats.Unchanged, want)
	}
	if stats.Updated != 0 || stats.Added != 0 {
		t.Errorf("second run should have no updates or additions")
	}
}

func TestRun_SkipsDotPrefixedDirs(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go":                 "package main\nfunc Hello() {}\n",
		".github/workflows/ci.go": "package ci\nfunc Run() {}\n",
		".vscode/settings.go":     "package vscode\nfunc Settings() {}\n",
		".cache/data.go":          "package cache\nfunc Load() {}\n",
		".idea/workspace.go":      "package idea\nfunc Open() {}\n",
	})
	db := openWalkDB(t, root)

	_, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}

	dotDirs := []string{".github", ".vscode", ".cache", ".idea"}
	for rel := range paths {
		for _, d := range dotDirs {
			if len(rel) > len(d) && rel[:len(d)] == d {
				t.Errorf("file inside dot-prefixed dir %q should not be indexed: %s", d, rel)
			}
		}
	}

	if !paths["main.go"] {
		t.Error("main.go should still be indexed")
	}
}

func TestRun_SkipsOpenCodeDir(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go":                      "package main\nfunc Hello() {}\n",
		".opencode/skills/task-cli.ts": "function noop() {}",
		".opencode/context/core/standards/code.md": "# standards",
	})
	db := openWalkDB(t, root)

	_, err := Run(root, db)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	paths, err := IndexedPaths(db)
	if err != nil {
		t.Fatalf("IndexedPaths: %v", err)
	}

	for rel := range paths {
		if len(rel) >= len(".opencode") && rel[:len(".opencode")] == ".opencode" {
			t.Errorf("file inside .opencode should not be indexed: %s", rel)
		}
	}

	if !paths["main.go"] {
		t.Error("main.go should still be indexed")
	}
}

// --- mtime + size stat skip ---

func TestRun_SkipsUnchangedByMtime(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	// First pass: index the file.
	if _, err := Run(root, db); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Second pass: file is untouched — mtime+size match → must be skipped.
	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if stats.Unchanged != 1 {
		t.Errorf("Unchanged: got %d, want 1", stats.Unchanged)
	}
	if stats.Updated != 0 {
		t.Errorf("Updated: got %d, want 0", stats.Updated)
	}
}

func TestRun_ReindexesWhenMtimeChanges(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	if _, err := Run(root, db); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Rewrite with identical content but bump mtime via os.Chtimes.
	p := filepath.Join(root, "main.go")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	// Content hash is unchanged, so the file should still end up as Unchanged
	// (stat mtime differs → read+hash → hash matches → skip write).
	// The important thing: no error, and Updated+Unchanged covers the file.
	if stats.Updated+stats.Unchanged != 1 {
		t.Errorf("Updated+Unchanged: got %d, want 1", stats.Updated+stats.Unchanged)
	}
}

func TestRun_ReindexesWhenSizeChanges(t *testing.T) {
	root := makeDir(t, map[string]string{
		"main.go": "package main\nfunc Hello() {}\n",
	})
	db := openWalkDB(t, root)

	if _, err := Run(root, db); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Overwrite with different (larger) content → new size → must re-parse.
	newContent := "package main\nfunc Hello() {}\nfunc World() {}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"),
		[]byte(newContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stats, err := Run(root, db)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if stats.Updated != 1 {
		t.Errorf("Updated: got %d, want 1", stats.Updated)
	}
	syms := symbolsForFile(t, db, "main.go")
	if !syms["World"] {
		t.Error(`symbol "World" should appear after re-index`)
	}
}
