package indexer

// js_test.go — symbol-extraction tests for JavaScript (.js, .jsx, .mjs, .cjs).
// Covers every JS pattern supported by languages/javascript/queries.go:
//   function declarations, arrow/const functions, var functions,
//   classes, methods, object shorthand, export default, static/async,
//   module.exports patterns, CJS direct assignment exports.

import "testing"

// --- core JS patterns ---

func TestGetSymbols_JS_JSXExtension(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("component.jsx", []byte(jsxFixture))
	if err != nil {
		t.Fatalf("unexpected error for .jsx: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected symbols from .jsx file, got none")
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Greeting", Function},
		{"Button", Function},
		{"Counter", Class},
		{"render", Method},
		{"increment", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in .jsx results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

func TestGetSymbols_JS_VarFunction(t *testing.T) {
	const fixture = `var foo = function() { return 1; }
var bar = (x) => x * 2;
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("file.js", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"foo", Function},
		{"bar", Function},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in var declaration results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

func TestGetSymbols_JS_ObjectMethodShorthand(t *testing.T) {
	const fixture = `const obj = {
  greet() { return "hi"; },
  farewell() { return "bye"; }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("file.js", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"greet", Method},
		{"farewell", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in object shorthand results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

func TestGetSymbols_JS_StaticAndAsyncMethods(t *testing.T) {
	const fixture = `class Outer {
  static create() { return new Outer(); }
  async fetchData() { return await db.get(); }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("file.js", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Outer", Class},
		{"create", Method},
		{"fetchData", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in static/async method results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

// --- export default ---

func TestGetSymbols_JS_ExportDefaultFunction(t *testing.T) {
	const fixture = `export default function handler(req, res) {
  res.send("ok");
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("file.mjs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["handler"]
	if !ok {
		t.Fatal(`symbol "handler" not found in export default function results`)
	}
	if s.Type != Function {
		t.Errorf(`"handler": got type %q, want %q`, s.Type, Function)
	}
}

func TestGetSymbols_JS_ExportDefaultClass(t *testing.T) {
	const fixture = `export default class MyService {
  run() {}
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("file.mjs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["MyService"]
	if !ok {
		t.Fatal(`symbol "MyService" not found in export default class results`)
	}
	if s.Type != Class {
		t.Errorf(`"MyService": got type %q, want %q`, s.Type, Class)
	}
}

// --- MJS (.mjs) ---

func TestGetSymbols_JS_MJSExtension(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("module.mjs", []byte(jsFixture))
	if err != nil {
		t.Fatalf("unexpected error for .mjs: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected symbols from .mjs file, got none")
	}
}

func TestGetSymbols_JS_MJS_ExportedSymbols(t *testing.T) {
	const fixture = `export function fetchUser(id) {
  return fetch('/users/' + id);
}

export const formatDate = (date) => date.toISOString();

export default class UserService {
  find(id) {
    return db.find(id);
  }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("service.mjs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"fetchUser", Function},
		{"formatDate", Function},
		{"UserService", Class},
		{"find", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("exported symbol %q not found in .mjs results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

// --- CJS (.cjs) ---

func TestGetSymbols_JS_CJSExtension(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("module.cjs", []byte(jsFixture))
	if err != nil {
		t.Fatalf("unexpected error for .cjs: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected symbols from .cjs file, got none")
	}
}

func TestGetSymbols_JS_CJS_ModuleExports(t *testing.T) {
	const fixture = `function createUser(name) {
  return { name };
}

const deleteUser = (id) => db.delete(id);

class UserRepository {
  save(user) {
    return db.save(user);
  }
}

module.exports = { createUser, deleteUser, UserRepository };
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("repo.cjs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"createUser", Function},
		{"deleteUser", Function},
		{"UserRepository", Class},
		{"save", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in .cjs results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

func TestGetSymbols_JS_CJS_DirectAssignmentExports(t *testing.T) {
	const fixture = `module.exports.create = function(data) {
  return db.insert(data);
}

module.exports.destroy = function(id) {
  return db.delete(id);
}

module.exports.update = function(id, data) {
  return db.update(id, data);
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("repo.cjs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"create", Function},
		{"destroy", Function},
		{"update", Function},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in direct assignment .cjs results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}
