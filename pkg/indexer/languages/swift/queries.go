package swift

// CallQueries captures direct function and method call sites in Swift.
// Matches:
//   - plain calls: foo()
//   - method calls: obj.method() — navigation_expression followed by call_suffix
const CallQueries = `
(call_expression
  (simple_identifier) @callee) @call

(call_expression
  (navigation_expression
    suffix: (navigation_suffix
      suffix: (simple_identifier) @callee))) @call
`

// RefQueries captures identifiers used as values (not called directly):
//   - variable bindings: let f = myFunc
//   - assignments: f = myFunc
const RefQueries = `
(property_declaration
  value: (simple_identifier) @ref)

(assignment
  result: (simple_identifier) @ref)
`

// Queries contains tree-sitter query patterns for Swift symbol extraction.
// Covers:
//   - top-level function declarations (func foo)
//   - class declarations (class Foo)
//   - struct declarations (struct Foo) — mapped to @class
//   - actor declarations (actor Foo) — mapped to @class
//   - enum declarations (enum Foo) — mapped to @enum
//   - extension declarations (extension Foo) — mapped to @namespace
//   - protocol declarations (protocol Foo) — mapped to @interface
//   - typealias declarations (typealias Foo = Bar)
//   - top-level property declarations (var/let)
//   - methods inside class/struct/actor/extension bodies
//   - init declarations inside type bodies
const Queries = `
(class_declaration
  declaration_kind: "class"
  name: (type_identifier) @name) @class

(class_declaration
  declaration_kind: "struct"
  name: (type_identifier) @name) @class

(class_declaration
  declaration_kind: "actor"
  name: (type_identifier) @name) @class

(class_declaration
  declaration_kind: "enum"
  name: (type_identifier) @name) @enum

(class_declaration
  declaration_kind: "extension"
  name: (user_type
    (type_identifier) @name)) @namespace

(protocol_declaration
  name: (type_identifier) @name) @interface

(typealias_declaration
  name: (type_identifier) @name) @type

(source_file
  (function_declaration
    name: (simple_identifier) @name) @function)

(source_file
  (property_declaration
    name: (pattern
      (simple_identifier) @name)) @variable)

(class_body
  (function_declaration
    name: (simple_identifier) @name) @method)

(class_body
  (init_declaration
    "init" @name) @method)

(enum_class_body
  (function_declaration
    name: (simple_identifier) @name) @method)

(enum_class_body
  (init_declaration
    "init" @name) @method)

(protocol_body
  (protocol_function_declaration
    name: (simple_identifier) @name) @method)
`

// ImportQueries captures Swift import declarations.
// Matches:
//   - import Foundation
//   - import UIKit
//   - import struct Foundation.URL
//
// The @path capture targets the identifier node directly (not its
// simple_identifier children) so that qualified imports such as
// "import Foundation.URL" are recorded as the full dotted path
// ("Foundation.URL") rather than just the last segment ("URL").
const ImportQueries = `
(import_declaration
  (identifier) @path) @import
`
