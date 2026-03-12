package csharp

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
)

// Extensions lists file extensions handled by the C# grammar.
var Extensions = []string{".cs"}

// Language is the single shared tree-sitter language instance for C#.
var Language = tree_sitter.NewLanguage(tree_sitter_csharp.Language())
