package rust

// CallQueries captures direct function and method call sites in Rust.
// Matches:
//   - plain calls: foo()
//   - method calls: obj.method()
//   - associated function calls: Type::method() (captured as "method")
//   - macro invocations: println!(), vec![] (captured as "println", "vec")
const CallQueries = `
(call_expression
  function: (identifier) @callee) @call

(call_expression
  function: (field_expression
    field: (field_identifier) @callee)) @call

(call_expression
  function: (scoped_identifier
    name: (identifier) @callee)) @call

(macro_invocation
  macro: (identifier) @callee) @call

(macro_invocation
  macro: (scoped_identifier
    name: (identifier) @callee)) @call
`

// RefQueries captures identifiers used as values (not called directly):
//   - let bindings: let f = my_func;
//   - assignments: f = my_func;
const RefQueries = `
(let_declaration
  value: (identifier) @ref)

(assignment_expression
  right: (identifier) @ref)
`

// Queries contains tree-sitter query patterns for Rust symbol extraction.
// Covers:
//   - function declarations (fn foo)
//   - struct declarations (struct Foo)
//   - enum declarations (enum Foo)
//   - trait declarations (trait Foo)
//   - impl blocks — methods extracted via impl body
//   - type aliases (type Foo = Bar)
//   - const declarations (const FOO: T = v)
//   - static declarations (static FOO: T = v)
//   - mod declarations (mod foo)
//   - macro definitions (macro_rules! foo)
const Queries = `
(struct_item
  name: (type_identifier) @name) @class

(enum_item
  name: (type_identifier) @name) @enum

(trait_item
  name: (type_identifier) @name) @interface

(impl_item
  body: (declaration_list
    (function_item
      name: (identifier) @name) @method))

(trait_item
  body: (declaration_list
    (function_item
      name: (identifier) @name) @method))

(type_item
  name: (type_identifier) @name) @type

(const_item
  name: (identifier) @name) @variable

(static_item
  name: (identifier) @name) @variable

(mod_item
  name: (identifier) @name) @namespace

(mod_item
  body: (declaration_list
    (function_item
      name: (identifier) @name) @function))

(macro_definition
  name: (identifier) @name) @function

(source_file
  (function_item
    name: (identifier) @name) @function)
`
