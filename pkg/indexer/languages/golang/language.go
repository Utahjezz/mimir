package golang

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// Extensions lists file extensions handled by the Go grammar.
var Extensions = []string{".go"}

// Language is the single shared tree-sitter language instance for Go.
var Language = tree_sitter.NewLanguage(tree_sitter_go.Language())
