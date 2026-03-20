package workspace

// link_test.go — unit tests for CreateLink, SetLinkMeta, ListLinks, DeleteLink.

import (
	"testing"

	"github.com/Utahjezz/mimir/pkg/indexer"
)

// makeLinkedWorkspace returns a fresh workspace DB with two indexed repos
// already registered, ready for link tests.
func makeLinkedWorkspace(t *testing.T) (db interface{ Close() error }, srcID, dstID string) {
	t.Helper()
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)

	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)

	srcRepoID, err := AddRepository(wsDB, src)
	if err != nil {
		t.Fatalf("AddRepository src: %v", err)
	}
	dstRepoID, err := AddRepository(wsDB, dst)
	if err != nil {
		t.Fatalf("AddRepository dst: %v", err)
	}
	return wsDB, srcRepoID, dstRepoID
}

// TestCreateLink_ReturnsID verifies that CreateLink inserts a row and returns
// a positive link ID.
func TestCreateLink_ReturnsID(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)

	// Act
	id, err := CreateLink(wsDB, srcID, "FuncA", "", dstID, "FuncB", "", "test")

	// Assert
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive link ID, got %d", id)
	}
}

// TestCreateLink_RoundTrip verifies that a created link is retrievable via
// ListLinks with all fields intact.
func TestCreateLink_RoundTrip(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)

	// Act
	_, err := CreateLink(wsDB, srcID, "FuncA", "pkg/a/foo.go", dstID, "FuncB", "pkg/b/bar.go", "a note")
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	links, err := ListLinks(wsDB, "")
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}

	// Assert
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if l.SrcRepoID != srcID {
		t.Errorf("SrcRepoID: got %q, want %q", l.SrcRepoID, srcID)
	}
	if l.SrcSymbol != "FuncA" {
		t.Errorf("SrcSymbol: got %q, want %q", l.SrcSymbol, "FuncA")
	}
	if l.SrcFile != "pkg/a/foo.go" {
		t.Errorf("SrcFile: got %q, want %q", l.SrcFile, "pkg/a/foo.go")
	}
	if l.DstRepoID != dstID {
		t.Errorf("DstRepoID: got %q, want %q", l.DstRepoID, dstID)
	}
	if l.DstSymbol != "FuncB" {
		t.Errorf("DstSymbol: got %q, want %q", l.DstSymbol, "FuncB")
	}
	if l.Note != "a note" {
		t.Errorf("Note: got %q, want %q", l.Note, "a note")
	}
}

// TestSetLinkMeta_Upsert verifies that SetLinkMeta stores a value and that
// calling it again with the same key overwrites rather than duplicates.
func TestSetLinkMeta_Upsert(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)
	id, _ := CreateLink(wsDB, srcID, "FuncA", "", dstID, "FuncB", "", "")

	// Act: first write
	if err := SetLinkMeta(wsDB, id, "protocol", "grpc"); err != nil {
		t.Fatalf("SetLinkMeta first: %v", err)
	}
	// Act: overwrite same key
	if err := SetLinkMeta(wsDB, id, "protocol", "http"); err != nil {
		t.Fatalf("SetLinkMeta overwrite: %v", err)
	}

	// Assert: only one row, updated value
	links, err := ListLinks(wsDB, "")
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Meta["protocol"] != "http" {
		t.Errorf("Meta[protocol]: got %q, want %q", links[0].Meta["protocol"], "http")
	}
	if len(links[0].Meta) != 1 {
		t.Errorf("expected 1 meta entry, got %d", len(links[0].Meta))
	}
}

// TestListLinks_All verifies that ListLinks with an empty srcRepoID returns
// links from all repos.
func TestListLinks_All(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	r1 := makeIndexedRepo(t)
	r2 := makeIndexedRepo(t)
	r3 := makeIndexedRepo(t)
	id1, _ := AddRepository(wsDB, r1)
	id2, _ := AddRepository(wsDB, r2)
	id3, _ := AddRepository(wsDB, r3)

	CreateLink(wsDB, id1, "A", "", id2, "B", "", "")
	CreateLink(wsDB, id3, "C", "", id2, "D", "", "")

	// Act
	links, err := ListLinks(wsDB, "")

	// Assert
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}
}

// TestListLinks_Filtered verifies that a non-empty srcRepoID filters correctly.
func TestListLinks_Filtered(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	r1 := makeIndexedRepo(t)
	r2 := makeIndexedRepo(t)
	r3 := makeIndexedRepo(t)
	id1, _ := AddRepository(wsDB, r1)
	id2, _ := AddRepository(wsDB, r2)
	id3, _ := AddRepository(wsDB, r3)

	CreateLink(wsDB, id1, "A", "", id2, "B", "", "")
	CreateLink(wsDB, id3, "C", "", id2, "D", "", "")

	// Act: filter to only id1 links
	links, err := ListLinks(wsDB, id1)

	// Assert
	if err != nil {
		t.Fatalf("ListLinks filtered: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link for src %q, got %d", id1, len(links))
	}
	if links[0].SrcRepoID != id1 {
		t.Errorf("SrcRepoID: got %q, want %q", links[0].SrcRepoID, id1)
	}
}

// TestDeleteLink_Found verifies that DeleteLink returns true and removes the row.
func TestDeleteLink_Found(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)
	linkID, _ := CreateLink(wsDB, srcID, "FuncA", "", dstID, "FuncB", "", "")

	// Act
	deleted, err := DeleteLink(wsDB, linkID)

	// Assert
	if err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if !deleted {
		t.Error("expected DeleteLink to return true for existing link")
	}
	links, _ := ListLinks(wsDB, "")
	if len(links) != 0 {
		t.Errorf("expected 0 links after delete, got %d", len(links))
	}
}

// TestDeleteLink_NotFound verifies that DeleteLink returns false (not an error)
// when no link with the given ID exists.
func TestDeleteLink_NotFound(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)

	// Act
	deleted, err := DeleteLink(wsDB, 9999)

	// Assert
	if err != nil {
		t.Fatalf("expected nil error for missing link, got: %v", err)
	}
	if deleted {
		t.Error("expected DeleteLink to return false for non-existent link")
	}
}

// TestDeleteLink_CascadesMeta verifies that deleting a link also removes its
// link_meta rows via ON DELETE CASCADE.
func TestDeleteLink_CascadesMeta(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)
	linkID, _ := CreateLink(wsDB, srcID, "FuncA", "", dstID, "FuncB", "", "")
	SetLinkMeta(wsDB, linkID, "protocol", "grpc")
	SetLinkMeta(wsDB, linkID, "transport", "kafka")

	// Act
	DeleteLink(wsDB, linkID)

	// Assert: no orphaned link_meta rows remain
	var count int
	if err := wsDB.QueryRow(`SELECT COUNT(*) FROM link_meta WHERE link_id = ?`, linkID).Scan(&count); err != nil {
		t.Fatalf("count link_meta: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 link_meta rows after cascade delete, got %d", count)
	}
}

// TestRemoveRepository_CascadesLinks verifies that removing a repo from the
// workspace also removes all links where it is the source (ON DELETE CASCADE).
func TestRemoveRepository_CascadesLinks(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)
	src := makeIndexedRepo(t)
	dst := makeIndexedRepo(t)
	srcID, _ := AddRepository(wsDB, src)
	dstID, _ := AddRepository(wsDB, dst)
	CreateLink(wsDB, srcID, "FuncA", "", dstID, "FuncB", "", "link to be cascaded")

	// Verify link exists before removal
	links, _ := ListLinks(wsDB, "")
	if len(links) != 1 {
		t.Fatalf("pre-condition: expected 1 link, got %d", len(links))
	}

	// Act: remove the source repo
	if err := RemoveRepository(wsDB, src); err != nil {
		t.Fatalf("RemoveRepository: %v", err)
	}

	// Assert: link is gone
	links, err := ListLinks(wsDB, "")
	if err != nil {
		t.Fatalf("ListLinks after repo removal: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected 0 links after src repo removal, got %d", len(links))
	}
}

// TestListLinks_Empty verifies that ListLinks returns a nil/empty slice (not
// an error) on a workspace with no links.
func TestListLinks_Empty(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	wsDB := openFreshWorkspace(t, tmp)

	// Act
	links, err := ListLinks(wsDB, "")

	// Assert
	if err != nil {
		t.Fatalf("ListLinks on empty workspace: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

// Compile-time check: ensure indexer package is available in test binary.
var _ = indexer.RepoID
