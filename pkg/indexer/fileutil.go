package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// readFile reads the entire contents of path.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// fileSHA256 returns the hex-encoded SHA-256 digest of data.
func fileSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// bodySnippetFromNode extracts a deduplicated, space-joined bag of semantically
// meaningful tokens from the subtree rooted at node. Only leaf nodes whose
// type contains "identifier", "string", or "comment" are collected — control
// flow keywords, operators, and punctuation are naturally excluded because the
// tree-sitter grammar tags them as "keyword", "operator", etc., never as
// identifiers or strings.
//
// Collecting from the full subtree (not just the first N lines) means even
// large functions are fully represented in the FTS5 index.
func bodySnippetFromNode(node *tree_sitter.Node, src []byte) string {
	seen := make(map[string]struct{})
	collectSemanticTokens(node, src, seen)

	tokens := make([]string, 0, len(seen))
	for tok := range seen {
		tokens = append(tokens, tok)
	}
	return strings.Join(tokens, " ")
}

// collectSemanticTokens recursively walks the AST subtree and adds the text
// of semantically meaningful leaf nodes to seen.
func collectSemanticTokens(node *tree_sitter.Node, src []byte, seen map[string]struct{}) {
	if node == nil {
		return
	}

	kind := node.Kind()
	isLeaf := node.ChildCount() == 0

	if isLeaf {
		if strings.Contains(kind, "identifier") ||
			strings.Contains(kind, "string") ||
			strings.Contains(kind, "comment") {
			tok := strings.TrimSpace(node.Utf8Text(src))
			if tok != "" {
				seen[tok] = struct{}{}
			}
		}
		return
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		collectSemanticTokens(node.Child(i), src, seen)
	}
}
