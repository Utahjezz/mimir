package javascript

// CallQueries captures direct function and method call sites in JavaScript.
// Matches:
//   - plain calls: foo()
//   - method/selector calls: obj.method()
const CallQueries = `
(call_expression
  function: (identifier) @callee) @call

(call_expression
  function: (member_expression
    property: (property_identifier) @callee)) @call
`

// JSXCallQueries extends CallQueries with JSX element patterns for JSX files.
// Matches everything CallQueries matches, plus:
//   - JSX opening elements: <MyComponent prop={x}>
//   - JSX self-closing elements: <MyComponent />
//
// Note: lowercase-initial JSX identifiers (native HTML tags such as <div>,
// <span>, <input>) are filtered out at runtime in runCallQuery so that only
// user-defined component names (PascalCase / uppercase-initial) are recorded
// as call references.
//
// These patterns are intentionally separated from CallQueries because
// jsx_opening_element and jsx_self_closing_element are only valid node types
// in the JSX-enabled JavaScript grammar — compiling them against the plain
// JavaScript grammar may produce a query compilation error depending on the
// grammar version.
const JSXCallQueries = CallQueries + `
(jsx_opening_element
  name: (identifier) @callee) @call

(jsx_self_closing_element
  name: (identifier) @callee) @call
`

// ImportQueries contains tree-sitter query patterns for JavaScript import extraction.
// Matches import_statement nodes:
//   - side-effect import:    import 'mod'               → path="mod", alias=""
//   - default import:        import X from 'mod'        → path="mod", alias=""
//   - named imports:         import { A, B } from 'mod' → path="mod", alias=""
//   - namespace import:      import * as ns from 'mod'  → path="mod", alias=""
//
// Each match yields a @path capture (the string_fragment of the module specifier).
// Alias resolution is handled at extraction time from the import_clause subtree.
const ImportQueries = `
(import_statement
  source: (string
    (string_fragment) @path)) @import
`

// RefQueries captures identifiers used as values (not called directly) in:
//   - object/array literal properties: { handler: myFn }
//   - variable declarators: const f = myFn  /  let f = myFn  /  var f = myFn
//   - assignment expressions: f = myFn
//
// Combined with CallQueries in ExtractCalls so functions passed as values
// are recorded as "used" and do not appear as dead code.
const RefQueries = `
(pair
  value: (identifier) @ref)

(variable_declarator
  value: (identifier) @ref)

(assignment_expression
  right: (identifier) @ref)
`

// Queries contains tree-sitter query patterns for JavaScript symbol extraction.
// Covers: function declarations, generator functions (function* foo), var/let/const
// assignments, class declarations, methods (including static/async), object shorthand
// methods, export default function/class, and module.exports.x = function() assignments.
const Queries = `
(function_declaration
  name: (identifier) @name) @function

(generator_function_declaration
  name: (identifier) @name) @function

(lexical_declaration
  (variable_declarator
    name: (identifier) @name
    value: [(arrow_function) (function_expression)])) @function

(variable_declaration
  (variable_declarator
    name: (identifier) @name
    value: [(arrow_function) (function_expression)])) @function

(class_declaration
  name: (identifier) @name) @class

(method_definition
  name: (property_identifier) @name) @method

(expression_statement
  (assignment_expression
    left: (member_expression
      property: (property_identifier) @name)
    right: [(function_expression) (arrow_function)])) @function
`
