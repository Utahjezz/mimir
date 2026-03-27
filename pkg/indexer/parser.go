package indexer

import (
	"fmt"
	"slices"
	"unicode"
	"unicode/utf8"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// isUserDefinedComponent reports whether name looks like a user-defined JSX
// component rather than a native HTML tag. The JSX convention is that
// user-defined components start with an uppercase letter (e.g. MyComponent,
// Button) while native HTML tags are all lowercase (div, span, input).
//
// This filter is applied to @callee captures from JSX query patterns so that
// <div> and <span> do not produce spurious call references.
func isUserDefinedComponent(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// parentTypes are the symbol types that can own child symbols.
var parentTypes = map[SymbolType]bool{
	Class:     true,
	Interface: true,
	Enum:      true,
}

// childTypes are the symbol types that receive a Parent when nested inside a
// parentType. Function is included for constructors (C#/JS) but the assignment
// only fires when the function's start line falls inside the parent's range.
var childTypes = map[SymbolType]bool{
	Method:   true,
	Variable: true, // properties
}

// assignParents post-processes a flat symbol slice (as returned by tree-sitter
// queries) and fills in the Parent field for any child symbol whose line range
// falls entirely within a preceding parent symbol.
//
// Tree-sitter queries return symbols in document order, so a simple linear scan
// is sufficient: we keep a stack of open containers and pop them when a symbol
// starts after their end line.
func assignParents(symbols []SymbolInfo) []SymbolInfo {
	type frame struct {
		name    string
		endLine int
	}

	var stack []frame

	for i, s := range symbols {
		// Pop containers that have ended before this symbol starts.
		for len(stack) > 0 && s.StartLine > stack[len(stack)-1].endLine {
			stack = stack[:len(stack)-1]
		}

		// If this symbol is a child type and there is an open container, assign parent.
		if childTypes[s.Type] && len(stack) > 0 {
			symbols[i].Parent = stack[len(stack)-1].name
		}

		// If this symbol is itself a container, push it.
		if parentTypes[s.Type] {
			stack = append(stack, frame{name: s.Name, endLine: s.EndLine})
		}
	}

	return symbols
}

var captureToSymbolType = map[string]SymbolType{
	"function":  Function,
	"class":     Class,
	"method":    Method,
	"interface": Interface,
	"type":      TypeAlias,
	"enum":      Enum,
	"namespace": Namespace,
	"variable":  Variable,
}

// runQuery executes the pre-compiled tree-sitter query from entry against code
// and returns all matched symbols. It is language-agnostic — behaviour is
// driven entirely by the compiled query and language carried in the langEntry.
// The compiled query is shared across goroutines; each call creates its own
// cursor so concurrent use is safe.
func runQuery(entry langEntry, code []byte) ([]SymbolInfo, error) {
	parser := tree_sitter.NewParser()
	parser.SetLanguage(entry.language)

	tree := parser.Parse(code, nil)
	defer tree.Close()

	query := entry.compiledQuery // pre-compiled at startup; do NOT close here

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(query, tree.RootNode(), code)

	var symbols []SymbolInfo

	for {
		match := matches.Next()
		if match == nil {
			break
		}

		var symType SymbolType
		var name string
		var node *tree_sitter.Node

		for _, capture := range match.Captures {
			captureName := query.CaptureNames()[capture.Index]

			switch captureName {
			case "name":
				name = capture.Node.Utf8Text(code)
			default:
				if t, ok := captureToSymbolType[captureName]; ok {
					symType = t
					n := capture.Node
					node = &n
				}
			}
		}

		if name == "" || symType == "" || node == nil {
			continue
		}

		symbols = append(symbols, SymbolInfo{
			Name:        name,
			Type:        symType,
			StartLine:   int((*node).StartPosition().Row) + 1,
			EndLine:     int((*node).EndPosition().Row) + 1,
			BodySnippet: bodySnippetFromNode(node, code),
		})
	}

	return assignParents(symbols), nil
}

// ExtractCalls runs the call-site query for the given language name against
// code and returns every outgoing call site found.
//
// For languages that also have a refQuery (identifier references used as
// values, e.g. RunE: runIndex), those are appended as additional CallSite
// entries so that functions passed as values are recorded as "used" in the
// refs table and do not appear as dead code.
//
// Languages that do not have a registered call query (e.g. Python, JS) return
// an empty slice without error — callers need not special-case them.
func ExtractCalls(lang string, code []byte) ([]CallSite, error) {
	def, ok := callLangMap[lang]
	if !ok {
		return nil, nil
	}

	parser := tree_sitter.NewParser()
	parser.SetLanguage(def.language)

	tree := parser.Parse(code, nil)
	defer tree.Close()

	calls, err := runCallQuery(def.compiledCallQuery, tree, code)
	if err != nil {
		return nil, fmt.Errorf("ExtractCalls runCallQuery: %w", err)
	}

	if def.compiledRefQuery != nil {
		refs, err := runRefQuery(def.compiledRefQuery, tree, code)
		if err != nil {
			return nil, fmt.Errorf("ExtractCalls runRefQuery: %w", err)
		}
		calls = append(calls, refs...)
	}

	return calls, nil
}

// ExtractImports runs the import query for the given language name against
// code and returns every import statement found.
//
// Languages that do not have a registered import query return nil, nil —
// callers need not special-case them.
// The function is goroutine-safe: compiled queries are shared read-only;
// each call creates its own parser and cursor.
func ExtractImports(lang string, code []byte) ([]ImportSite, error) {
	def, ok := importLangMap[lang]
	if !ok {
		return nil, nil
	}

	parser := tree_sitter.NewParser()
	parser.SetLanguage(def.language)

	tree := parser.Parse(code, nil)
	defer tree.Close()

	return runImportQuery(def.compiledImportQuery, tree, code)
}

// runImportQuery executes a pre-compiled import query and returns one
// ImportSite per matched @import capture. Within each match it reads the
// @path capture (required) and the optional @alias capture.
//
// Because some languages emit both a plain pattern and an alias pattern for
// the same node (e.g. Go import_spec with and without alias), we deduplicate
// by line: when two matches produce the same line, the one that also carries
// an @alias wins.
//
// The query is shared across goroutines; each call uses its own cursor.
func runImportQuery(query *tree_sitter.Query, tree *tree_sitter.Tree, code []byte) ([]ImportSite, error) {
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(query, tree.RootNode(), code)

	// Keyed by line so that an alias-bearing match overwrites a plain match.
	seen := make(map[int]ImportSite)

	for {
		match := matches.Next()
		if match == nil {
			break
		}

		var path, alias string
		var line int

		for _, capture := range match.Captures {
			capName := query.CaptureNames()[capture.Index]
			switch capName {
			case "path":
				path = capture.Node.Utf8Text(code)
				line = int(capture.Node.StartPosition().Row) + 1
			case "alias":
				alias = capture.Node.Utf8Text(code)
			}
		}

		if path == "" {
			continue
		}

		// Overwrite only if this match adds new information (has an alias
		// where the previous entry for this line did not).
		if prev, exists := seen[line]; exists && prev.Alias != "" {
			continue
		}

		seen[line] = ImportSite{
			ImportPath: path,
			Alias:      alias,
			Line:       line,
		}
	}

	if len(seen) == 0 {
		return nil, nil
	}

	imports := make([]ImportSite, 0, len(seen))
	for _, imp := range seen {
		imports = append(imports, imp)
	}

	// Sort by line for deterministic output.
	slices.SortFunc(imports, func(a, b ImportSite) int {
		return a.Line - b.Line
	})

	return imports, nil
}

// runCallQuery executes a pre-compiled call-site query and returns a CallSite
// per @callee capture. The query is shared across goroutines; each call uses
// its own cursor so concurrent use is safe.
//
// JSX element captures (<MyComponent />, <MyComponent>) use the same @callee
// capture name as function calls. To avoid recording native HTML tags (div,
// span, input, …) as refs, any callee whose parent node is a JSX element and
// whose name starts with a lowercase letter is silently skipped.
func runCallQuery(query *tree_sitter.Query, tree *tree_sitter.Tree, code []byte) ([]CallSite, error) {
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(query, tree.RootNode(), code)

	var calls []CallSite
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		for _, capture := range match.Captures {
			if query.CaptureNames()[capture.Index] == "callee" {
				name := capture.Node.Utf8Text(code)
				// Filter out native HTML tags emitted by JSX element patterns.
				parentKind := capture.Node.Parent().Kind()
				if (parentKind == "jsx_opening_element" || parentKind == "jsx_self_closing_element") &&
					!isUserDefinedComponent(name) {
					continue
				}
				calls = append(calls, CallSite{
					CalleeName: name,
					Line:       int(capture.Node.StartPosition().Row) + 1,
				})
			}
		}
	}
	return calls, nil
}

// runRefQuery executes a pre-compiled reference query and returns a CallSite
// per @ref capture. A @ref is an identifier used as a value (not a direct
// call), such as a function assigned to a struct field or a variable.
// Recording it as a CallSite ensures it is not flagged as dead code.
// The query is shared across goroutines; each call uses its own cursor.
func runRefQuery(query *tree_sitter.Query, tree *tree_sitter.Tree, code []byte) ([]CallSite, error) {
	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(query, tree.RootNode(), code)

	var refs []CallSite
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		for _, capture := range match.Captures {
			if query.CaptureNames()[capture.Index] == "ref" {
				refs = append(refs, CallSite{
					CalleeName: capture.Node.Utf8Text(code),
					Line:       int(capture.Node.StartPosition().Row) + 1,
				})
			}
		}
	}
	return refs, nil
}
