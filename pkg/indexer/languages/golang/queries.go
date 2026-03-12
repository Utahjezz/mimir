package golang

// Queries contains tree-sitter query patterns for Go symbol extraction.
// Covers:

// CallQueries contains tree-sitter query patterns for Go call-site extraction.
// Matches:
//   - plain function calls: foo()
//   - method/selector calls: obj.Method() or pkg.Func()
//
// Each match yields a @callee capture whose text is the function/method name.
const CallQueries = `
(call_expression
  function: (identifier) @callee) @call

(call_expression
  function: (selector_expression
    field: (field_identifier) @callee)) @call
`

// Queries contains tree-sitter query patterns for Go symbol extraction.
// Covers:
//   - function declarations (func Foo)
//   - method declarations (func (r *T) Foo)
//   - struct type declarations (type Foo struct)
//   - interface type declarations (type Foo interface)
//   - other type declarations / named types (type Foo Bar)
//   - true type aliases with = (type Foo = Bar)
//   - iota const blocks — idiomatic Go enums (captures the first const name)
//   - package-level var declarations (single and block) — anchored to source_file
const Queries = `
(function_declaration
  name: (identifier) @name) @function

(method_declaration
  name: (field_identifier) @name) @method

(type_spec
  name: (type_identifier) @name
  type: (struct_type)) @class

(type_spec
  name: (type_identifier) @name
  type: (interface_type)) @interface

(type_spec
  name: (type_identifier) @name
  type: [
    (type_identifier)
    (qualified_type)
    (pointer_type)
    (slice_type)
    (array_type)
    (map_type)
    (channel_type)
    (function_type)
  ]) @type

(type_alias
  name: (type_identifier) @name) @type

(const_declaration
  (const_spec
    name: (identifier) @name
    value: (expression_list (iota)))) @enum

(source_file
  (var_declaration
    (var_spec
      name: (identifier) @name)) @variable)

(source_file
  (var_declaration
    (var_spec_list
      (var_spec
        name: (identifier) @name))) @variable)
`

// RefQueries captures identifiers used as values (but not called) in:
//   - struct/composite literal fields: RunE: runIndex
//   - var declarations: var f = myFunc
//   - short var declarations: f := myFunc
//   - assignment statements: f = myFunc
//
// These are combined with CallQueries in ExtractCalls so that functions
// passed as values (e.g. cobra RunE) are recorded as "used" and do not
// appear as dead code.
const RefQueries = `
(keyed_element
  (literal_element (identifier))
  (literal_element (identifier) @ref))

(var_spec
  value: (expression_list (identifier) @ref))

(short_var_declaration
  right: (expression_list (identifier) @ref))

(assignment_statement
  right: (expression_list (identifier) @ref))
`
