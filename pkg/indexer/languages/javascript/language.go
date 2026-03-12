package javascript

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

// Extensions lists all file extensions handled by the JavaScript grammar.
var Extensions = []string{".js", ".jsx", ".mjs", ".cjs"}

// Language is the single shared tree-sitter language instance for JavaScript.
// It is allocated once and reused across all extensions.
var Language = tree_sitter.NewLanguage(tree_sitter_javascript.Language())
