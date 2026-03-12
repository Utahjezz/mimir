package typescript

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// Extensions lists file extensions handled by the TypeScript grammar.
var Extensions = []string{".ts", ".mts", ".cts"}

// TSXExtensions lists file extensions handled by the TSX grammar.
// TSX is a distinct grammar from TypeScript — it adds JSX syntax support.
var TSXExtensions = []string{".tsx"}

// Language is the single shared tree-sitter language instance for TypeScript.
var Language = tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())

// TSXLanguage is the single shared tree-sitter language instance for TSX.
var TSXLanguage = tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTSX())
