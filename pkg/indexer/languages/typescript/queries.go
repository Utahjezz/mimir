package typescript

// CallQueries captures direct function and method call sites in TypeScript/TSX.
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

// Queries contains tree-sitter query patterns for TypeScript symbol extraction.
// Covers all JS patterns plus TypeScript-specific constructs:
// interfaces, type aliases, enums (regular and const), namespaces,
// abstract classes, abstract methods, and decorated classes.
const Queries = `
(function_declaration
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
  name: (type_identifier) @name) @class

(abstract_class_declaration
  name: (type_identifier) @name) @class

(method_definition
  name: (property_identifier) @name) @method

(abstract_method_signature
  name: (property_identifier) @name) @method

(expression_statement
  (assignment_expression
    left: (member_expression
      property: (property_identifier) @name)
    right: [(function_expression) (arrow_function)])) @function

(interface_declaration
  name: (type_identifier) @name) @interface

(type_alias_declaration
  name: (type_identifier) @name) @type

(enum_declaration
  name: (identifier) @name) @enum

(internal_module
  name: (identifier) @name) @namespace
`
