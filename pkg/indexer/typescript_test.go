package indexer

// TypeScript-specific tests.
// Each test follows the red→green TDD approach: write the assertion first,
// then confirm the implementation satisfies it.
//
// Extensions covered: .ts, .tsx, .mts, .cts

import "testing"

// --- fixtures ---

// tsFixture covers the core TypeScript-only constructs.
const tsFixture = `interface User {
  id: number;
  name: string;
}

type UserID = number;

enum Direction {
  Up,
  Down,
  Left,
  Right,
}

const enum Status {
  Active,
  Inactive,
}

namespace Utils {
  export function log(msg: string) {
    console.log(msg);
  }
}

abstract class Shape {
  abstract area(): number;

  toString(): string {
    return "Shape";
  }
}

class Circle extends Shape {
  constructor(private radius: number) {
    super();
  }

  area(): number {
    return Math.PI * this.radius * this.radius;
  }
}
`

// tsxFixture exercises TSX-specific patterns: React function/class components.
const tsxFixture = `import React from 'react';

interface Props {
  name: string;
}

const Greeting: React.FC<Props> = ({ name }) => (
  <h1>Hello, {name}</h1>
);

function Welcome({ name }: Props): JSX.Element {
  return <p>Welcome, {name}</p>;
}

class ErrorBoundary extends React.Component {
  componentDidCatch(error: Error) {
    console.error(error);
  }

  render() {
    return this.props.children;
  }
}
`

func TestGetSymbols_TS_Interface(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	s, ok := byName["User"]
	if !ok {
		t.Fatal(`symbol "User" not found`)
	}
	if s.Type != Interface {
		t.Errorf(`"User": got type %q, want %q`, s.Type, Interface)
	}
}

func TestGetSymbols_TS_TypeAlias(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	s, ok := byName["UserID"]
	if !ok {
		t.Fatal(`symbol "UserID" not found`)
	}
	if s.Type != TypeAlias {
		t.Errorf(`"UserID": got type %q, want %q`, s.Type, TypeAlias)
	}
}

func TestGetSymbols_TS_Enum(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	s, ok := byName["Direction"]
	if !ok {
		t.Fatal(`symbol "Direction" not found`)
	}
	if s.Type != Enum {
		t.Errorf(`"Direction": got type %q, want %q`, s.Type, Enum)
	}
}

func TestGetSymbols_TS_ConstEnum(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	s, ok := byName["Status"]
	if !ok {
		t.Fatal(`symbol "Status" not found (const enum)`)
	}
	if s.Type != Enum {
		t.Errorf(`"Status": got type %q, want %q`, s.Type, Enum)
	}
}

func TestGetSymbols_TS_Namespace(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	s, ok := byName["Utils"]
	if !ok {
		t.Fatal(`symbol "Utils" not found`)
	}
	if s.Type != Namespace {
		t.Errorf(`"Utils": got type %q, want %q`, s.Type, Namespace)
	}
}

func TestGetSymbols_TS_AbstractClass(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	s, ok := byName["Shape"]
	if !ok {
		t.Fatal(`symbol "Shape" not found`)
	}
	if s.Type != Class {
		t.Errorf(`"Shape": got type %q, want %q`, s.Type, Class)
	}
}

func TestGetSymbols_TS_AbstractMethod(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	s, ok := byName["area"]
	if !ok {
		t.Fatal(`symbol "area" not found (abstract method)`)
	}
	if s.Type != Method {
		t.Errorf(`"area": got type %q, want %q`, s.Type, Method)
	}
}

func TestGetSymbols_TS_ConcreteClass(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Circle", Class},
		{"area", Method},
		{"toString", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in TS concrete class results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

// --- extension coverage ---

func TestGetSymbols_TS_MTSExtension(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("module.mts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error for .mts: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected symbols from .mts file, got none")
	}
}

func TestGetSymbols_TS_CTSExtension(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("module.cts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error for .cts: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected symbols from .cts file, got none")
	}
}

// --- TSX ---

func TestGetSymbols_TSX_FunctionComponent(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("component.tsx", []byte(tsxFixture))
	if err != nil {
		t.Fatalf("unexpected error for .tsx: %v", err)
	}

	byName := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byName[s.Name] = s
	}

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Props", Interface},
		{"Greeting", Function},
		{"Welcome", Function},
		{"ErrorBoundary", Class},
		{"render", Method},
		{"componentDidCatch", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in .tsx results", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

// --- generator functions ---

func TestGetSymbols_TS_GeneratorFunction(t *testing.T) {
	const fixture = `
function* ids() {
  let i = 0;
  while (true) yield i++;
}

export function* stream(data: string[]) {
  for (const item of data) yield item;
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("gen.ts", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"ids", "stream"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("generator function %q not found", name)
			continue
		}
		if s.Type != Function {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Function)
		}
	}
}

// --- line number sanity ---

func TestGetSymbols_TS_LineNumbers(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.ts", []byte(tsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range symbols {
		if s.StartLine <= 0 {
			t.Errorf("symbol %q: StartLine should be > 0, got %d", s.Name, s.StartLine)
		}
		if s.EndLine < s.StartLine {
			t.Errorf("symbol %q: EndLine (%d) < StartLine (%d)", s.Name, s.EndLine, s.StartLine)
		}
	}
}
