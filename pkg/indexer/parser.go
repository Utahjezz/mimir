package indexer

import (
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

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
			Name:      name,
			Type:      symType,
			StartLine: int((*node).StartPosition().Row) + 1,
			EndLine:   int((*node).EndPosition().Row) + 1,
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

// runCallQuery executes a pre-compiled call-site query and returns a CallSite
// per @callee capture. The query is shared across goroutines; each call uses
// its own cursor so concurrent use is safe.
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
				calls = append(calls, CallSite{
					CalleeName: capture.Node.Utf8Text(code),
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
