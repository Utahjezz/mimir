package indexer

// report_test.go — tests for ReportIndex.

import (
	"database/sql"
	"testing"
	"time"
)

// seedReportDB writes a fixture set covering two languages and multiple symbol types.
//
//	a.go  — go   — Function:main, Method:serve (parent=Server), Class:Server
//	b.go  — go   — Function:helper
//	c.ts  — typescript — Function:run, Interface:Runner
func seedReportDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, t.TempDir())

	files := []struct {
		path    string
		lang    string
		symbols []SymbolInfo
	}{
		{
			path: "a.go",
			lang: "go",
			symbols: []SymbolInfo{
				{Name: "main", Type: Function, StartLine: 1, EndLine: 5},
				{Name: "serve", Type: Method, StartLine: 7, EndLine: 12, Parent: "Server"},
				{Name: "Server", Type: Class, StartLine: 14, EndLine: 30},
			},
		},
		{
			path: "b.go",
			lang: "go",
			symbols: []SymbolInfo{
				{Name: "helper", Type: Function, StartLine: 1, EndLine: 4},
			},
		},
		{
			path: "c.ts",
			lang: "typescript",
			symbols: []SymbolInfo{
				{Name: "run", Type: Function, StartLine: 1, EndLine: 3},
				{Name: "Runner", Type: Interface, StartLine: 5, EndLine: 8},
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
			t.Fatalf("seedReportDB WriteFile %s: %v", f.path, err)
		}
	}
	return db
}

func TestReportIndex_FilesAndSymbolCounts(t *testing.T) {
	db := seedReportDB(t)

	report, err := ReportIndex(db)
	if err != nil {
		t.Fatalf("ReportIndex: %v", err)
	}

	if report.FileCount != 3 {
		t.Errorf("FileCount: got %d, want 3", report.FileCount)
	}
	if report.SymbolCount != 6 {
		t.Errorf("SymbolCount: got %d, want 6", report.SymbolCount)
	}
}

func TestReportIndex_LanguageBreakdown(t *testing.T) {
	db := seedReportDB(t)

	report, err := ReportIndex(db)
	if err != nil {
		t.Fatalf("ReportIndex: %v", err)
	}

	if len(report.Languages) != 2 {
		t.Fatalf("Languages: got %d entries, want 2", len(report.Languages))
	}

	byLang := make(map[string]LanguageStat)
	for _, l := range report.Languages {
		byLang[l.Language] = l
	}

	go_ := byLang["go"]
	if go_.FileCount != 2 {
		t.Errorf("go FileCount: got %d, want 2", go_.FileCount)
	}
	if go_.SymbolCount != 4 {
		t.Errorf("go SymbolCount: got %d, want 4", go_.SymbolCount)
	}

	ts := byLang["typescript"]
	if ts.FileCount != 1 {
		t.Errorf("typescript FileCount: got %d, want 1", ts.FileCount)
	}
	if ts.SymbolCount != 2 {
		t.Errorf("typescript SymbolCount: got %d, want 2", ts.SymbolCount)
	}
}

func TestReportIndex_SymbolTypeBreakdown(t *testing.T) {
	db := seedReportDB(t)

	report, err := ReportIndex(db)
	if err != nil {
		t.Fatalf("ReportIndex: %v", err)
	}

	byType := make(map[string]int)
	for _, s := range report.SymbolTypes {
		byType[s.Type] = s.Count
	}

	if byType["function"] != 3 {
		t.Errorf("function count: got %d, want 3", byType["function"])
	}
	if byType["method"] != 1 {
		t.Errorf("method count: got %d, want 1", byType["method"])
	}
	if byType["class"] != 1 {
		t.Errorf("class count: got %d, want 1", byType["class"])
	}
	if byType["interface"] != 1 {
		t.Errorf("interface count: got %d, want 1", byType["interface"])
	}
}

func TestReportIndex_MetaFields(t *testing.T) {
	root := t.TempDir()
	db := openTestDB(t, root)

	report, err := ReportIndex(db)
	if err != nil {
		t.Fatalf("ReportIndex: %v", err)
	}

	if report.RepoID == "" {
		t.Error("RepoID should not be empty")
	}
	if report.Root == "" {
		t.Error("Root should not be empty")
	}
}

func TestReportIndex_EmptyDB(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	report, err := ReportIndex(db)
	if err != nil {
		t.Fatalf("ReportIndex on empty DB: %v", err)
	}

	if report.FileCount != 0 {
		t.Errorf("FileCount: got %d, want 0", report.FileCount)
	}
	if report.SymbolCount != 0 {
		t.Errorf("SymbolCount: got %d, want 0", report.SymbolCount)
	}
	if len(report.Languages) != 0 {
		t.Errorf("Languages: got %d entries, want 0", len(report.Languages))
	}
	if len(report.SymbolTypes) != 0 {
		t.Errorf("SymbolTypes: got %d entries, want 0", len(report.SymbolTypes))
	}
}
