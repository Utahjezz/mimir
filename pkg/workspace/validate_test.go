package workspace

// validate_test.go — unit tests for ValidateLink.

import (
	"testing"
)

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
	if !result.SrcValid {
		t.Errorf("SrcValid: got false, want true")
	}
	if !result.DstValid {
		t.Errorf("DstValid: got false, want true")
	}
	if !result.SrcFileValid {
		t.Errorf("SrcFileValid: got false, want true (src file %q)", result.SrcActualFile)
	}
	if !result.DstFileValid {
		t.Errorf("DstFileValid: got false, want true (dst file %q)", result.DstActualFile)
	}
	if result.SrcError != "" {
		t.Errorf("SrcError: got %q, want empty", result.SrcError)
	}
	if result.DstError != "" {
		t.Errorf("DstError: got %q, want empty", result.DstError)
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
	if result.SrcValid {
		t.Errorf("SrcValid: got true, want false")
	}
	if result.SrcError == "" {
		t.Errorf("SrcError: got empty, want non-empty error about missing symbol")
	}
	if result.SrcFileValid {
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
	if result.DstValid {
		t.Errorf("DstValid: got true, want false")
	}
	if result.DstError == "" {
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
	if result.SrcError == "" {
		t.Errorf("SrcError: got empty, want error about repo not found")
	}
}
