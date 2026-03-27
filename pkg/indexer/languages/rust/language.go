package rust

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

// Extensions lists file extensions handled by the Rust grammar.
var Extensions = []string{".rs"}

// Language is the single shared tree-sitter language instance for Rust.
var Language = tree_sitter.NewLanguage(tree_sitter_rust.Language())
