package python

// CallQueries captures direct function and method call sites in Python.
// Matches:
//   - plain calls: foo()
//   - attribute/method calls: obj.method()
const CallQueries = `
(call
  function: (identifier) @callee) @call

(call
  function: (attribute
    attribute: (identifier) @callee)) @call
`

// RefQueries captures identifiers used as values (not called directly) in:
//   - assignment statements: f = my_func  (right-hand side plain identifier)
//   - dict literal values: {"handler": my_func}
//   - keyword arguments: handler=my_func
//
// Combined with CallQueries in ExtractCalls so functions passed as values
// are recorded as "used" and do not appear as dead code.
const RefQueries = `
(assignment
  right: (identifier) @ref)

(pair
  value: (identifier) @ref)

(keyword_argument
  value: (identifier) @ref)
`

// ImportQueries contains tree-sitter query patterns for Python import extraction.
// Matches import_statement and import_from_statement nodes:
//   - plain import:          import os                  → path="os", alias=""
//   - aliased import:        import os as op_sys        → path="os", alias="op_sys"
//   - from import:           from os import path        → path="os", alias=""
//   - from relative import:  from . import something    → path=".", alias=""
//
// Each match yields a @path capture (dotted_name or relative_import text) and
// an optional @alias capture (the identifier after 'as').
const ImportQueries = `
(import_statement
  (dotted_name) @path) @import

(import_statement
  (aliased_import
    (dotted_name) @path
    (identifier) @alias)) @import

(import_from_statement
  module_name: (dotted_name) @path) @import

(import_from_statement
  module_name: (relative_import) @path) @import
`

// Queries contains tree-sitter query patterns for Python symbol extraction.
// Covers:
//   - top-level function definitions (def foo — direct children of module)
//   - decorated top-level function definitions (@decorator def foo)
//   - class definitions (class Foo)
//   - decorated class definitions (@decorator class Foo)
//   - method definitions (def foo inside a class body)
//   - decorated method definitions (@decorator def foo inside a class body)
//
// The @function patterns are anchored to module-level to avoid matching methods
// that are already captured by the @method patterns.
//
// In Python's tree-sitter grammar, decorated definitions are wrapped in a
// decorated_definition node, so we need separate patterns for each case.
const Queries = `
(module
  (function_definition
    name: (identifier) @name) @function)

(module
  (decorated_definition
    definition: (function_definition
      name: (identifier) @name)) @function)

(module
  (class_definition
    name: (identifier) @name) @class)

(module
  (decorated_definition
    definition: (class_definition
      name: (identifier) @name)) @class)

(class_definition
  body: (block
    (function_definition
      name: (identifier) @name) @method))

(class_definition
  body: (block
    (decorated_definition
      definition: (function_definition
        name: (identifier) @name)) @method))
`
