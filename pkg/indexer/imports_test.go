package indexer

// imports_test.go — tests for import-site extraction (ExtractImports) across
// all five supported languages, and for SearchImports query filtering.

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"
)

// --- ExtractImports: Go ---

const goImportFixture = `package main

import "fmt"
import myfmt "fmt"
import . "pkg/dot"
import _ "pkg/blank"
`

func TestExtractImports_Go_Plain(t *testing.T) {
	imports, err := ExtractImports("go", []byte(goImportFixture))
	if err != nil {
		t.Fatalf("ExtractImports Go: %v", err)
	}
	if !importPathSet(imports)["fmt"] {
		t.Errorf("expected plain import 'fmt', got %v", imports)
	}
}

func TestExtractImports_Go_Aliased(t *testing.T) {
	imports, err := ExtractImports("go", []byte(goImportFixture))
	if err != nil {
		t.Fatalf("ExtractImports Go aliased: %v", err)
	}
	for _, imp := range imports {
		if imp.ImportPath == "fmt" && imp.Alias == "myfmt" {
			return
		}
	}
	t.Errorf("expected aliased import myfmt='fmt', got %v", imports)
}

func TestExtractImports_Go_DotImport(t *testing.T) {
	imports, err := ExtractImports("go", []byte(goImportFixture))
	if err != nil {
		t.Fatalf("ExtractImports Go dot: %v", err)
	}
	for _, imp := range imports {
		if imp.ImportPath == "pkg/dot" && imp.Alias == "." {
			return
		}
	}
	t.Errorf("expected dot import .='pkg/dot', got %v", imports)
}

func TestExtractImports_Go_BlankImport(t *testing.T) {
	imports, err := ExtractImports("go", []byte(goImportFixture))
	if err != nil {
		t.Fatalf("ExtractImports Go blank: %v", err)
	}
	for _, imp := range imports {
		if imp.ImportPath == "pkg/blank" && imp.Alias == "_" {
			return
		}
	}
	t.Errorf("expected blank import _='pkg/blank', got %v", imports)
}

// --- ExtractImports: TypeScript ---

const tsImportFixture = `import { A, B } from 'mod';
import X from 'mod-default';
import * as ns from 'mod-ns';
import 'mod-side-effect';
`

func TestExtractImports_TS_Named(t *testing.T) {
	assertImportPath(t, "typescript", tsImportFixture, "mod")
}

func TestExtractImports_TS_Default(t *testing.T) {
	assertImportPath(t, "typescript", tsImportFixture, "mod-default")
}

func TestExtractImports_TS_Namespace(t *testing.T) {
	assertImportPath(t, "typescript", tsImportFixture, "mod-ns")
}

func TestExtractImports_TS_SideEffect(t *testing.T) {
	assertImportPath(t, "typescript", tsImportFixture, "mod-side-effect")
}

// --- ExtractImports: JavaScript ---

const jsImportFixture = `import { A, B } from 'mod';
import X from 'mod-default';
import * as ns from 'mod-ns';
import 'mod-side-effect';
`

func TestExtractImports_JS_Named(t *testing.T) {
	assertImportPath(t, "javascript", jsImportFixture, "mod")
}

func TestExtractImports_JS_Default(t *testing.T) {
	assertImportPath(t, "javascript", jsImportFixture, "mod-default")
}

func TestExtractImports_JS_Namespace(t *testing.T) {
	assertImportPath(t, "javascript", jsImportFixture, "mod-ns")
}

func TestExtractImports_JS_SideEffect(t *testing.T) {
	assertImportPath(t, "javascript", jsImportFixture, "mod-side-effect")
}

// --- ExtractImports: Python ---

const pyImportFixture = `import os
import os as operating_system
from os import path
from os import path as p
`

func TestExtractImports_Python_Plain(t *testing.T) {
	assertImportPath(t, "python", pyImportFixture, "os")
}

func TestExtractImports_Python_Aliased(t *testing.T) {
	imports, err := ExtractImports("python", []byte(pyImportFixture))
	if err != nil {
		t.Fatalf("ExtractImports Python aliased: %v", err)
	}
	for _, imp := range imports {
		if imp.ImportPath == "os" && imp.Alias == "operating_system" {
			return
		}
	}
	t.Errorf("expected aliased import operating_system='os', got %v", imports)
}

func TestExtractImports_Python_FromModule(t *testing.T) {
	// "from os import path" — the module being tracked is "os".
	assertImportPath(t, "python", pyImportFixture, "os")
}

func TestExtractImports_Python_NonNilResult(t *testing.T) {
	imports, err := ExtractImports("python", []byte(pyImportFixture))
	if err != nil {
		t.Fatalf("ExtractImports Python: %v", err)
	}
	if imports == nil {
		t.Error("expected non-nil imports for Python fixture")
	}
}

// --- ExtractImports: C# ---

const csImportFixture = `using System;
using Alias = System.Collections;
`

func TestExtractImports_CSharp_Plain(t *testing.T) {
	assertImportPath(t, "csharp", csImportFixture, "System")
}

func TestExtractImports_CSharp_Aliased(t *testing.T) {
	imports, err := ExtractImports("csharp", []byte(csImportFixture))
	if err != nil {
		t.Fatalf("ExtractImports C# aliased: %v", err)
	}
	for _, imp := range imports {
		if imp.ImportPath == "System.Collections" && imp.Alias == "Alias" {
			return
		}
	}
	t.Errorf("expected aliased import Alias='System.Collections', got %v", imports)
}

// --- ExtractImports: unsupported language ---

func TestExtractImports_UnsupportedLanguage_ReturnsNil(t *testing.T) {
	imports, err := ExtractImports("ruby", []byte("require 'json'"))
	if err != nil {
		t.Fatalf("ExtractImports on unsupported lang should not error: %v", err)
	}
	if imports != nil {
		t.Errorf("expected nil imports for unsupported language, got %v", imports)
	}
}

// --- SearchImports ---

// seedImportsDB writes a two-file fixture into db with known imports:
//
//	main.go  — imports "fmt" (plain) and "fmt" aliased as "myfmt"
//	other.go — imports "os" (plain)
func seedImportsDB(t *testing.T, db *sql.DB) {
	t.Helper()

	mainEntry := FileEntry{
		Language:  "go",
		SHA256:    "hash1",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "main", Type: Function, StartLine: 1, EndLine: 5}},
		Imports: []ImportSite{
			{ImportPath: "fmt", Line: 3},
			{ImportPath: "fmt", Alias: "myfmt", Line: 4},
		},
	}

	otherEntry := FileEntry{
		Language:  "go",
		SHA256:    "hash2",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "run", Type: Function, StartLine: 1, EndLine: 3}},
		Imports: []ImportSite{
			{ImportPath: "os", Line: 3},
		},
	}

	if err := WriteFile(db, "main.go", mainEntry); err != nil {
		t.Fatalf("WriteFile main.go: %v", err)
	}
	if err := WriteFile(db, "other.go", otherEntry); err != nil {
		t.Fatalf("WriteFile other.go: %v", err)
	}
}

func TestSearchImports_ByFilePath(t *testing.T) {
	db := openTestDB(t, t.TempDir())
	seedImportsDB(t, db)

	rows, err := SearchImports(db, ImportQuery{FilePath: "main.go"})
	if err != nil {
		t.Fatalf("SearchImports by file_path: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 imports for main.go, got %d: %v", len(rows), rows)
	}
	for _, r := range rows {
		if r.FilePath != "main.go" {
			t.Errorf("expected file_path=main.go, got %q", r.FilePath)
		}
	}
}

func TestSearchImports_ByImportPath(t *testing.T) {
	db := openTestDB(t, t.TempDir())
	seedImportsDB(t, db)

	rows, err := SearchImports(db, ImportQuery{ImportPath: "fmt"})
	if err != nil {
		t.Fatalf("SearchImports by import_path: %v", err)
	}
	// fmt is imported twice in main.go (plain + aliased).
	if len(rows) != 2 {
		t.Fatalf("expected 2 imports of 'fmt', got %d: %v", len(rows), rows)
	}
	for _, r := range rows {
		if r.ImportPath != "fmt" {
			t.Errorf("expected import_path=fmt, got %q", r.ImportPath)
		}
	}
}

func TestSearchImports_NoFilter_ReturnsAll(t *testing.T) {
	db := openTestDB(t, t.TempDir())
	seedImportsDB(t, db)

	// main.go has 2 imports, other.go has 1 = 3 total.
	rows, err := SearchImports(db, ImportQuery{})
	if err != nil {
		t.Fatalf("SearchImports no filter: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 total imports, got %d: %v", len(rows), rows)
	}
}

func TestSearchImports_EmptyResult(t *testing.T) {
	db := openTestDB(t, t.TempDir())
	seedImportsDB(t, db)

	rows, err := SearchImports(db, ImportQuery{ImportPath: "doesNotExist"})
	if err != nil {
		t.Fatalf("SearchImports empty: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for unknown import, got %d", len(rows))
	}
}

func TestSearchImports_EmptyResult_JSONNotNull(t *testing.T) {
	db := openTestDB(t, t.TempDir())
	seedImportsDB(t, db)

	rows, err := SearchImports(db, ImportQuery{ImportPath: "doesNotExist"})
	if err != nil {
		t.Fatalf("SearchImports empty: %v", err)
	}

	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(data) == "null" {
		t.Error("SearchImports returned a nil slice: JSON output is null, want []")
	}
	if string(data) != "[]" {
		t.Errorf("expected JSON [], got %s", string(data))
	}
}

func TestWriteFile_ReplacesOldImports(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	first := FileEntry{
		Language:  "go",
		SHA256:    "v1",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "foo", Type: Function, StartLine: 1, EndLine: 5}},
		Imports:   []ImportSite{{ImportPath: "oldpkg", Line: 3}},
	}
	if err := WriteFile(db, "file.go", first); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}

	second := FileEntry{
		Language:  "go",
		SHA256:    "v2",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "foo", Type: Function, StartLine: 1, EndLine: 5}},
		Imports:   []ImportSite{{ImportPath: "newpkg", Line: 3}},
	}
	if err := WriteFile(db, "file.go", second); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}

	rows, err := SearchImports(db, ImportQuery{FilePath: "file.go"})
	if err != nil {
		t.Fatalf("SearchImports: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 import after re-write, got %d: %v", len(rows), rows)
	}
	if rows[0].ImportPath != "newpkg" {
		t.Errorf("expected import_path=newpkg, got %q", rows[0].ImportPath)
	}
}

func TestPruneFiles_CascadesToImports(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	entry := FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "foo", Type: Function, StartLine: 1, EndLine: 5}},
		Imports:   []ImportSite{{ImportPath: "fmt", Line: 3}},
	}
	if err := WriteFile(db, "foo.go", entry); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := PruneFiles(db, []string{"foo.go"}); err != nil {
		t.Fatalf("PruneFiles: %v", err)
	}

	rows, err := SearchImports(db, ImportQuery{FilePath: "foo.go"})
	if err != nil {
		t.Fatalf("SearchImports after prune: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 imports after prune, got %d", len(rows))
	}
}

// --- helpers ---

// importPathSet builds a set of ImportPath values from a slice of ImportSite.
func importPathSet(imports []ImportSite) map[string]bool {
	m := make(map[string]bool, len(imports))
	for _, imp := range imports {
		m[imp.ImportPath] = true
	}
	return m
}

// assertImportPath calls ExtractImports for lang and asserts wantPath is present.
func assertImportPath(t *testing.T, lang, src, wantPath string) {
	t.Helper()
	imports, err := ExtractImports(lang, []byte(src))
	if err != nil {
		t.Fatalf("ExtractImports %s: %v", lang, err)
	}
	if !importPathSet(imports)[wantPath] {
		t.Errorf("expected import path %q for lang %s, got %v", wantPath, lang, imports)
	}
}
