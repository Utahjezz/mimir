package csharp

// ImportQueries contains tree-sitter query patterns for C# using directive extraction.
// Matches using_directive nodes:
//   - plain using:           using System;                   → path="System", alias=""
//   - qualified using:       using System.Collections;       → path="System.Collections", alias=""
//   - alias using:           using Alias = System.Coll...;   → path="System.Coll...", alias="Alias"
//
// Each match yields a @path capture (identifier or qualified_name) and an
// optional @alias capture (the identifier before '=').
const ImportQueries = `
(using_directive
  (identifier) @path) @import

(using_directive
  (qualified_name) @path) @import

(using_directive
  (identifier) @alias
  (qualified_name) @path) @import
`

// CallQueries captures direct method and function call sites in C#.
// Matches:
//   - plain calls: Foo()  /  foo()
//   - member access calls: obj.Method()
const CallQueries = `
(invocation_expression
  function: (identifier) @callee) @call

(invocation_expression
  function: (member_access_expression
    name: (identifier) @callee)) @call
`

// RefQueries captures identifiers used as values (not called directly) in:
//   - local variable declarators: Action a = MyMethod;
//   - simple assignment: a = MyMethod;
//
// Combined with CallQueries in ExtractCalls so delegates/actions assigned as
// values are recorded as "used" and do not appear as dead code.
const RefQueries = `
(variable_declarator
  name: (identifier)
  (identifier) @ref)

(assignment_expression
  right: (identifier) @ref)
`

// Queries contains tree-sitter query patterns for C# symbol extraction.
// Covers:
//   - class declarations (class Foo)
//   - struct declarations (struct Foo) — mapped to @class
//   - record declarations (record Foo, record class Foo, record struct Foo) — mapped to @class
//   - interface declarations (interface IFoo) — mapped to @interface
//   - enum declarations (enum Foo) — mapped to @enum
//   - method declarations — mapped to @method
//   - constructor declarations — mapped to @function
//   - property declarations (auto-properties with accessors) — mapped to @variable
//   - field declarations (bare fields without accessors) — mapped to @variable
//   - delegate declarations — mapped to @type
const Queries = `
(class_declaration
  name: (identifier) @name) @class

(struct_declaration
  name: (identifier) @name) @class

(record_declaration
  name: (identifier) @name) @class

(interface_declaration
  name: (identifier) @name) @interface

(enum_declaration
  name: (identifier) @name) @enum

(method_declaration
  name: (identifier) @name) @method

(constructor_declaration
  name: (identifier) @name) @function

(property_declaration
  name: (identifier) @name) @variable

(field_declaration
  (variable_declaration
    (variable_declarator
      name: (identifier) @name))) @variable

(delegate_declaration
  name: (identifier) @name) @type
`
