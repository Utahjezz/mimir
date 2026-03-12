package indexer

// filetree_test.go — tests for GetFileTree, buildTree, FlattenTree, and indentFor.
//
// Coverage:
//   - Empty DB returns root-only node (".")
//   - Files in root directory (path has no slash → dir = ".")
//   - Files in a single subdirectory
//   - Nested directories: file counts and symbol counts bubble up
//   - Multiple languages in one directory
//   - FlattenTree returns all nodes sorted by path, root first
//   - buildTree (pure) with synthetic FileTreeEntry slices
//   - indentFor depth calculation

import (
	"testing"
	"time"
)

// --- GetFileTree (DB-backed) ---

func TestGetFileTree_EmptyDB_ReturnsRootOnly(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	root, err := GetFileTree(db)
	if err != nil {
		t.Fatalf("GetFileTree on empty DB: %v", err)
	}

	if root == nil {
		t.Fatal("expected non-nil root node")
	}
	if root.Path != "." {
		t.Errorf("root.Path: got %q, want %q", root.Path, ".")
	}
	if root.FileCount != 0 {
		t.Errorf("root.FileCount: got %d, want 0", root.FileCount)
	}
	if root.SymbolCount != 0 {
		t.Errorf("root.SymbolCount: got %d, want 0", root.SymbolCount)
	}
	if len(root.Children) != 0 {
		t.Errorf("root.Children: got %d, want 0", len(root.Children))
	}
}

func TestGetFileTree_FlatFiles_CountsInRoot(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Two root-level files with 2 symbols each.
	for _, rel := range []string{"main.go", "util.go"} {
		if err := WriteFile(db, rel, FileEntry{
			Language:  "go",
			SHA256:    "x",
			IndexedAt: time.Now().UTC(),
			Symbols: []SymbolInfo{
				{Name: "A", Type: Function, StartLine: 1, EndLine: 3},
				{Name: "B", Type: Function, StartLine: 5, EndLine: 7},
			},
		}); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}

	root, err := GetFileTree(db)
	if err != nil {
		t.Fatalf("GetFileTree: %v", err)
	}

	if root.FileCount != 2 {
		t.Errorf("root.FileCount: got %d, want 2", root.FileCount)
	}
	if root.SymbolCount != 4 {
		t.Errorf("root.SymbolCount: got %d, want 4", root.SymbolCount)
	}
	if len(root.Children) != 0 {
		t.Errorf("root.Children: got %d, want 0", len(root.Children))
	}
	if root.Languages["go"] != 2 {
		t.Errorf("root.Languages[go]: got %d, want 2", root.Languages["go"])
	}
}

func TestGetFileTree_Subdirectory_FilesInChild(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	if err := WriteFile(db, "pkg/foo.go", FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "Foo", Type: Function, StartLine: 1, EndLine: 5},
		},
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	root, err := GetFileTree(db)
	if err != nil {
		t.Fatalf("GetFileTree: %v", err)
	}

	// Root should have 1 child ("pkg"), 0 direct files.
	if root.FileCount != 1 {
		t.Errorf("root.FileCount (bubbled up): got %d, want 1", root.FileCount)
	}
	if root.SymbolCount != 1 {
		t.Errorf("root.SymbolCount (bubbled up): got %d, want 1", root.SymbolCount)
	}
	if len(root.Children) != 1 {
		t.Fatalf("root.Children: got %d, want 1", len(root.Children))
	}

	pkg := root.Children[0]
	if pkg.Path != "pkg" {
		t.Errorf("child.Path: got %q, want %q", pkg.Path, "pkg")
	}
	if pkg.FileCount != 1 {
		t.Errorf("pkg.FileCount: got %d, want 1", pkg.FileCount)
	}
	if pkg.SymbolCount != 1 {
		t.Errorf("pkg.SymbolCount: got %d, want 1", pkg.SymbolCount)
	}
}

func TestGetFileTree_NestedDirs_CountsBubbleUp(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	// Deep file: 3 symbols.
	if err := WriteFile(db, "a/b/c/deep.go", FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "X", Type: Function, StartLine: 1, EndLine: 2},
			{Name: "Y", Type: Function, StartLine: 3, EndLine: 4},
			{Name: "Z", Type: Function, StartLine: 5, EndLine: 6},
		},
	}); err != nil {
		t.Fatalf("WriteFile deep: %v", err)
	}

	// Shallow file in "a": 1 symbol.
	if err := WriteFile(db, "a/top.go", FileEntry{
		Language:  "go",
		SHA256:    "y",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "T", Type: Function, StartLine: 1, EndLine: 2},
		},
	}); err != nil {
		t.Fatalf("WriteFile top: %v", err)
	}

	root, err := GetFileTree(db)
	if err != nil {
		t.Fatalf("GetFileTree: %v", err)
	}

	// Root must aggregate everything: 2 files, 4 symbols.
	if root.FileCount != 2 {
		t.Errorf("root.FileCount: got %d, want 2", root.FileCount)
	}
	if root.SymbolCount != 4 {
		t.Errorf("root.SymbolCount: got %d, want 4", root.SymbolCount)
	}

	// "a" must aggregate its own file + descendant.
	if len(root.Children) != 1 {
		t.Fatalf("root.Children: got %d, want 1", len(root.Children))
	}
	nodeA := root.Children[0]
	if nodeA.Path != "a" {
		t.Errorf("root child: got %q, want %q", nodeA.Path, "a")
	}
	if nodeA.FileCount != 2 {
		t.Errorf("a.FileCount: got %d, want 2", nodeA.FileCount)
	}
	if nodeA.SymbolCount != 4 {
		t.Errorf("a.SymbolCount: got %d, want 4", nodeA.SymbolCount)
	}
}

func TestGetFileTree_MultiLanguage_LanguageCountsAggregated(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	for _, f := range []struct {
		path string
		lang string
	}{
		{"src/a.go", "go"},
		{"src/b.go", "go"},
		{"src/c.ts", "typescript"},
	} {
		if err := WriteFile(db, f.path, FileEntry{
			Language:  f.lang,
			SHA256:    "x",
			IndexedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("WriteFile %s: %v", f.path, err)
		}
	}

	root, err := GetFileTree(db)
	if err != nil {
		t.Fatalf("GetFileTree: %v", err)
	}

	// Root aggregates "src" counts.
	if root.Languages["go"] != 2 {
		t.Errorf("root.Languages[go]: got %d, want 2", root.Languages["go"])
	}
	if root.Languages["typescript"] != 1 {
		t.Errorf("root.Languages[typescript]: got %d, want 1", root.Languages["typescript"])
	}

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child (src), got %d", len(root.Children))
	}
	src := root.Children[0]
	if src.Languages["go"] != 2 {
		t.Errorf("src.Languages[go]: got %d, want 2", src.Languages["go"])
	}
	if src.Languages["typescript"] != 1 {
		t.Errorf("src.Languages[typescript]: got %d, want 1", src.Languages["typescript"])
	}
}

// --- buildTree (pure, no DB) ---

func TestBuildTree_Empty_ReturnsRootOnly(t *testing.T) {
	root := buildTree(nil)

	if root.Path != "." {
		t.Errorf("root.Path: got %q, want %q", root.Path, ".")
	}
	if root.FileCount != 0 {
		t.Errorf("root.FileCount: got %d, want 0", root.FileCount)
	}
	if len(root.Children) != 0 {
		t.Errorf("root.Children: got %d, want 0", len(root.Children))
	}
}

func TestBuildTree_SiblingDirectories_AreSortedByPath(t *testing.T) {
	entries := []FileTreeEntry{
		{Path: "z/file.go", Language: "go"},
		{Path: "a/file.go", Language: "go"},
		{Path: "m/file.go", Language: "go"},
	}

	root := buildTree(entries)

	if len(root.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(root.Children))
	}
	wantOrder := []string{"a", "m", "z"}
	for i, want := range wantOrder {
		if root.Children[i].Path != want {
			t.Errorf("Children[%d].Path: got %q, want %q", i, root.Children[i].Path, want)
		}
	}
}

func TestBuildTree_FilesInSameDir_AreSortedByPath(t *testing.T) {
	entries := []FileTreeEntry{
		{Path: "pkg/z.go", Language: "go"},
		{Path: "pkg/a.go", Language: "go"},
	}

	root := buildTree(entries)

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.Children))
	}
	pkg := root.Children[0]
	if len(pkg.Files) != 2 {
		t.Fatalf("pkg.Files: got %d, want 2", len(pkg.Files))
	}
	if pkg.Files[0].Path != "pkg/a.go" {
		t.Errorf("Files[0]: got %q, want %q", pkg.Files[0].Path, "pkg/a.go")
	}
	if pkg.Files[1].Path != "pkg/z.go" {
		t.Errorf("Files[1]: got %q, want %q", pkg.Files[1].Path, "pkg/z.go")
	}
}

// --- FlattenTree ---

func TestFlattenTree_SingleRoot_ReturnsOneNode(t *testing.T) {
	root := buildTree(nil)
	flat := FlattenTree(root)

	if len(flat) != 1 {
		t.Fatalf("expected 1 node, got %d", len(flat))
	}
	if flat[0].Path != "." {
		t.Errorf("flat[0].Path: got %q, want %q", flat[0].Path, ".")
	}
}

func TestFlattenTree_NestedTree_AllNodesPresent(t *testing.T) {
	entries := []FileTreeEntry{
		{Path: "a/b/c.go", Language: "go"},
		{Path: "a/d.go", Language: "go"},
		{Path: "e/f.go", Language: "go"},
	}
	root := buildTree(entries)
	flat := FlattenTree(root)

	// Expect: ".", "a", "a/b", "e" — 4 nodes total.
	if len(flat) != 4 {
		t.Fatalf("expected 4 nodes, got %d: %v", len(flat), pathsOf(flat))
	}

	// Must be sorted by path.
	wantPaths := []string{".", "a", "a/b", "e"}
	for i, want := range wantPaths {
		if flat[i].Path != want {
			t.Errorf("flat[%d].Path: got %q, want %q", i, flat[i].Path, want)
		}
	}
}

func TestFlattenTree_RootIsFirst(t *testing.T) {
	entries := []FileTreeEntry{
		{Path: "pkg/foo.go", Language: "go"},
	}
	root := buildTree(entries)
	flat := FlattenTree(root)

	if len(flat) == 0 {
		t.Fatal("FlattenTree returned empty slice")
	}
	if flat[0].Path != "." {
		t.Errorf("first node should be root \".\", got %q", flat[0].Path)
	}
}

// --- indentFor ---

func TestIndentFor_RootHasNoIndent(t *testing.T) {
	if got := indentFor("."); got != "" {
		t.Errorf("indentFor(%q): got %q, want empty string", ".", got)
	}
}

func TestIndentFor_TopLevelDirHasTwoSpaces(t *testing.T) {
	got := indentFor("pkg")
	want := "  "
	if got != want {
		t.Errorf("indentFor(%q): got %q, want %q", "pkg", got, want)
	}
}

func TestIndentFor_NestedDirHasFourSpaces(t *testing.T) {
	got := indentFor("pkg/indexer")
	want := "    "
	if got != want {
		t.Errorf("indentFor(%q): got %q, want %q", "pkg/indexer", got, want)
	}
}

func TestIndentFor_DeeplyNestedDirHasSixSpaces(t *testing.T) {
	got := indentFor("a/b/c")
	want := "      "
	if got != want {
		t.Errorf("indentFor(%q): got %q, want %q", "a/b/c", got, want)
	}
}

// --- helpers ---

// pathsOf extracts the Path field from a slice of DirNode pointers — used for
// informative failure messages.
func pathsOf(nodes []*DirNode) []string {
	paths := make([]string, len(nodes))
	for i, n := range nodes {
		paths[i] = n.Path
	}
	return paths
}
