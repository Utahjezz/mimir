package indexer

type SymbolType string

const (
	Function  SymbolType = "function"
	Class     SymbolType = "class"
	Method    SymbolType = "method"
	Interface SymbolType = "interface"
	TypeAlias SymbolType = "type_alias"
	Enum      SymbolType = "enum"
	Namespace SymbolType = "namespace"
	Variable  SymbolType = "variable"
)

type SymbolInfo struct {
	Name      string     `json:"name"`
	Type      SymbolType `json:"type"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`
	// Parent holds the name of the enclosing class/struct/interface for methods,
	// constructors, and properties. Empty for top-level symbols.
	Parent string `json:"parent,omitempty"`
}
