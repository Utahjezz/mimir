package swift

import (
	tree_sitter_swift "github.com/Utahjezz/tree-sitter-swift/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Extensions lists file extensions handled by the Swift grammar.
var Extensions = []string{".swift"}

// Language is the single shared tree-sitter language instance for Swift.
var Language = tree_sitter.NewLanguage(tree_sitter_swift.Language())
