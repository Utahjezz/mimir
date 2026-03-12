package python

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// Extensions lists file extensions handled by the Python grammar.
var Extensions = []string{".py", ".pyw"}

// Language is the single shared tree-sitter language instance for Python.
var Language = tree_sitter.NewLanguage(tree_sitter_python.Language())
