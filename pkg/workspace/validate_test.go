package workspace

// validate_test.go — unit tests for ValidateLink.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Utahjezz/mimir/pkg/indexer"
)

// makeIndexedRepoWithFiles creates a temporary repo with one Go file per entry
// in files (map of filename → source). All files are indexed before returning
// the repo root path.
func makeIndexedRepoWithFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, src := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	db, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("indexer.OpenIndex: %v", err)
	}
	defer db.Close()
	if _, err := indexer.Run(root, db); err != nil {
		t.Fatalf("indexer.Run: %v", err)
	}
	return root
}

// boolVal safely dereferences a *bool for test assertions.
func boolVal(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// strVal safely dereferences a *string for test assertions.
func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// TestValidateLink_Valid verifies that a link where both symbols exist
// at the recorded paths returns valid results with no errors.
func TestValidateLink_Valid(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)

	// The indexed repos contain Hello() in hello.go.
	// CreateLink stores the file path from the index.
	linkID, _ := CreateLink(wsDB, srcID, "Hello", "hello.go", dstID, "Hello", "hello.go", "test")
	links, _ := ListLinks(wsDB, LinkQuery{})
	link := &links[linkID-1] // linkIDs start at 1

	// Act
	result, err := ValidateLink(wsDB, link)

	// Assert
	if err != nil {
		t.Fatalf("ValidateLink: %v", err)
	}
	if result.Link.SrcValid == nil || !*result.Link.SrcValid {
		t.Errorf("SrcValid: got %v, want true", result.Link.SrcValid)
	}
	if result.Link.DstValid == nil || !*result.Link.DstValid {
		t.Errorf("DstValid: got %v, want true", result.Link.DstValid)
	}
	if result.Link.SrcFileValid == nil || !*result.Link.SrcFileValid {
		t.Errorf("SrcFileValid: got %v, want true (src file %q)", result.Link.SrcFileValid, strVal(result.Link.SrcActualFile))
	}
	if result.Link.DstFileValid == nil || !*result.Link.DstFileValid {
		t.Errorf("DstFileValid: got %v, want true (dst file %q)", result.Link.DstFileValid, strVal(result.Link.DstActualFile))
	}
	if s := strVal(result.Link.SrcError); s != "" {
		t.Errorf("SrcError: got %q, want empty", s)
	}
	if s := strVal(result.Link.DstError); s != "" {
		t.Errorf("DstError: got %q, want empty", s)
	}
}

// TestValidateLink_SymbolNotFound verifies that when a symbol does not exist
// in the source repo, SrcValid is false and SrcError is populated.
func TestValidateLink_SymbolNotFound(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)

	// Link to a symbol that does not exist.
	linkID, _ := CreateLink(wsDB, srcID, "NonExistent", "hello.go", dstID, "Hello", "hello.go", "test")
	links, _ := ListLinks(wsDB, LinkQuery{})
	link := &links[linkID-1]

	// Act
	result, err := ValidateLink(wsDB, link)

	// Assert
	if err != nil {
		t.Fatalf("ValidateLink: %v", err)
	}
	if boolVal(result.Link.SrcValid) {
		t.Errorf("SrcValid: got true, want false")
	}
	if strVal(result.Link.SrcError) == "" {
		t.Errorf("SrcError: got empty, want non-empty error about missing symbol")
	}
	if boolVal(result.Link.SrcFileValid) {
		t.Errorf("SrcFileValid: got true, want false (cannot check path when symbol missing)")
	}
}

// TestValidateLink_DstSymbolNotFound verifies that when a symbol does not exist
// in the destination repo, DstValid is false and DstError is populated.
func TestValidateLink_DstSymbolNotFound(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)

	// Link where dst symbol does not exist.
	linkID, _ := CreateLink(wsDB, srcID, "Hello", "hello.go", dstID, "NonExistent", "hello.go", "test")
	links, _ := ListLinks(wsDB, LinkQuery{})
	link := &links[linkID-1]

	// Act
	result, err := ValidateLink(wsDB, link)

	// Assert
	if err != nil {
		t.Fatalf("ValidateLink: %v", err)
	}
	if boolVal(result.Link.DstValid) {
		t.Errorf("DstValid: got true, want false")
	}
	if strVal(result.Link.DstError) == "" {
		t.Errorf("DstError: got empty, want non-empty error about missing symbol")
	}
}

// TestValidateLink_RepoNotFound verifies that when the source repo ID does not
// match any registered repo, SrcError reports the issue.
func TestValidateLink_RepoNotFound(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)

	// Create a link, then replace its srcRepoID with a non-existent one.
	linkID, _ := CreateLink(wsDB, srcID, "Hello", "hello.go", dstID, "Hello", "hello.go", "test")
	links, _ := ListLinks(wsDB, LinkQuery{})
	link := &links[linkID-1]
	link.SrcRepoID = "nonexistent-repo-id"

	// Act
	result, err := ValidateLink(wsDB, link)

	// Assert
	if err != nil {
		t.Fatalf("ValidateLink: %v", err)
	}
	if strVal(result.Link.SrcError) == "" {
		t.Errorf("SrcError: got empty, want error about repo not found")
	}
}

// TestValidateLink_ExactMatchPreferred verifies that when a symbol exists in
// multiple files, the recorded file is preferred and the link is reported valid.
func TestValidateLink_ExactMatchPreferred(t *testing.T) {
	// Arrange: repo with the same symbol name in two files.
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepoWithFiles(t, map[string]string{
		"foo.go": "package main\nfunc Hello() {}\n",
		"bar.go": "package main\nfunc Hello() {}\n",
	})
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)

	// Record the link pointing at bar.go specifically.
	linkID, _ := CreateLink(wsDB, srcID, "Hello", "bar.go", dstID, "Hello", "hello.go", "test")
	links, _ := ListLinks(wsDB, LinkQuery{})
	link := &links[linkID-1]

	// Act
	result, err := ValidateLink(wsDB, link)

	// Assert
	if err != nil {
		t.Fatalf("ValidateLink: %v", err)
	}
	// Symbol still lives in bar.go — should be valid, not reported as moved.
	if !boolVal(result.Link.SrcValid) {
		t.Errorf("SrcValid: got false, want true")
	}
	if !boolVal(result.Link.SrcFileValid) {
		t.Errorf("SrcFileValid: got false, want true — exact match should be preferred (actual: %q)", strVal(result.Link.SrcActualFile))
	}
	if s := strVal(result.Link.SrcError); s != "" {
		t.Errorf("SrcError: got %q, want empty", s)
	}
}

// TestValidateLink_AmbiguousSymbol verifies that when a symbol matches multiple
// files after suffix filtering, an ambiguous error is returned.
func TestValidateLink_AmbiguousSymbol(t *testing.T) {
	// Arrange: repo with the same symbol name in two files that both match the
	// recorded suffix.
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepoWithFiles(t, map[string]string{
		"pkg/a/run.go": "package a\nfunc Run() {}\n",
		"pkg/b/run.go": "package b\nfunc Run() {}\n",
	})
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)

	// Record a suffix that matches both files ("run.go").
	linkID, _ := CreateLink(wsDB, srcID, "Run", "run.go", dstID, "Hello", "hello.go", "test")
	links, _ := ListLinks(wsDB, LinkQuery{})
	link := &links[linkID-1]

	// Act
	result, err := ValidateLink(wsDB, link)

	// Assert
	if err != nil {
		t.Fatalf("ValidateLink: %v", err)
	}
	if boolVal(result.Link.SrcValid) {
		t.Errorf("SrcValid: got true, want false (symbol is ambiguous)")
	}
	if errMsg := strVal(result.Link.SrcError); !strings.Contains(errMsg, "ambiguous") {
		t.Errorf("SrcError: got %q, want message containing 'ambiguous'", errMsg)
	}
}
