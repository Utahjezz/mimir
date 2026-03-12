package indexer

// filetree.go — GetFileTree: builds a directory tree from the indexed files table.

import (
	"database/sql"
	"fmt"
	"path"
	"sort"
	"strings"
)

// DirNode represents a single directory in the file tree.
// Children are nested DirNodes; Files lists the immediate files in this dir.
type DirNode struct {
	Path        string          `json:"path"`                // relative directory path, e.g. "pkg/indexer"
	FileCount   int             `json:"file_count"`          // files directly in this dir
	SymbolCount int             `json:"symbol_count"`        // symbols across all files in this dir
	Languages   map[string]int  `json:"languages,omitempty"` // language → file count for this dir
	Children    []*DirNode      `json:"children,omitempty"`  // sorted subdirectories
	Files       []FileTreeEntry `json:"files,omitempty"`     // immediate files in this dir
}

// FileTreeEntry is a single file row inside a DirNode.
type FileTreeEntry struct {
	Path        string `json:"path"`
	Language    string `json:"language"`
	SymbolCount int    `json:"symbol_count"`
}

// GetFileTree returns the root DirNode of a tree built from the index.
// The root node's Path is ".".
func GetFileTree(db *sql.DB) (*DirNode, error) {
	// Load every file with its language and symbol count in one query.
	rows, err := db.Query(`
		SELECT f.path, f.language, COUNT(s.id)
		FROM files f
		LEFT JOIN symbols s ON s.file_path = f.path
		GROUP BY f.path, f.language
		ORDER BY f.path`)
	if err != nil {
		return nil, fmt.Errorf("GetFileTree query: %w", err)
	}
	defer rows.Close()

	var entries []FileTreeEntry
	for rows.Next() {
		var e FileTreeEntry
		if err := rows.Scan(&e.Path, &e.Language, &e.SymbolCount); err != nil {
			return nil, fmt.Errorf("GetFileTree scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetFileTree rows: %w", err)
	}

	return buildTree(entries), nil
}

// buildTree constructs the DirNode tree from a flat list of file entries.
func buildTree(entries []FileTreeEntry) *DirNode {
	// nodeMap holds one DirNode per unique directory path seen.
	nodeMap := map[string]*DirNode{}

	getOrCreate := func(dirPath string) *DirNode {
		if n, ok := nodeMap[dirPath]; ok {
			return n
		}
		n := &DirNode{
			Path:      dirPath,
			Languages: map[string]int{},
		}
		nodeMap[dirPath] = n
		return n
	}

	root := getOrCreate(".")

	for _, e := range entries {
		dir := path.Dir(e.Path)
		if dir == "." || dir == "" {
			dir = "."
		}

		// Ensure all ancestor nodes exist and are linked.
		ensureAncestors(dir, nodeMap, getOrCreate)

		node := getOrCreate(dir)
		node.FileCount++
		node.SymbolCount += e.SymbolCount
		node.Languages[e.Language]++
		node.Files = append(node.Files, e)
	}

	// Bubble symbol counts and language stats up to ancestors.
	bubbleUp(root, nodeMap)

	// Sort children and files for deterministic output.
	sortTree(root)

	return root
}

// ensureAncestors walks from dir up to "." and creates+links any missing nodes.
func ensureAncestors(dir string, nodeMap map[string]*DirNode, getOrCreate func(string) *DirNode) {
	for dir != "." {
		node := getOrCreate(dir)
		parent := path.Dir(dir)
		if parent == dir {
			break
		}
		if parent == "" {
			parent = "."
		}
		parentNode := getOrCreate(parent)

		// Add node as child of parent if not already present.
		found := false
		for _, c := range parentNode.Children {
			if c.Path == node.Path {
				found = true
				break
			}
		}
		if !found {
			parentNode.Children = append(parentNode.Children, node)
		}

		dir = parent
	}
}

// bubbleUp aggregates symbol counts and language stats from children into
// each ancestor. We do a post-order traversal via a simple DFS.
func bubbleUp(node *DirNode, _ map[string]*DirNode) {
	for _, child := range node.Children {
		bubbleUp(child, nil)
		node.SymbolCount += child.SymbolCount
		node.FileCount += child.FileCount
		for lang, count := range child.Languages {
			node.Languages[lang] += count
		}
	}
}

// sortTree recursively sorts children by path and files by path.
func sortTree(node *DirNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Path < node.Children[j].Path
	})
	sort.Slice(node.Files, func(i, j int) bool {
		return node.Files[i].Path < node.Files[j].Path
	})
	for _, child := range node.Children {
		sortTree(child)
	}
}

// FlattenTree returns all DirNodes in the tree as a flat sorted slice,
// useful for printing a compact directory listing.
func FlattenTree(root *DirNode) []*DirNode {
	var result []*DirNode
	var walk func(*DirNode)
	walk = func(n *DirNode) {
		result = append(result, n)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	// Sort by path for clean output.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

// indentFor returns a visual indent prefix for a directory path depth.
func indentFor(p string) string {
	if p == "." {
		return ""
	}
	depth := strings.Count(p, "/") + 1
	return strings.Repeat("  ", depth)
}
