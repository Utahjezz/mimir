package indexer

// go_test.go — symbol-extraction tests for Go (.go).
// Covers every Go pattern supported by languages/golang/queries.go:
//   function declarations, method declarations,
//   struct types, interface types, type aliases/other types,
//   iota const blocks (Go enums),
//   true type aliases (type Foo = Bar),
//   package-level var declarations (single and block).

import "testing"

// goFixture is a representative Go file covering one of each symbol type.
const goFixture = `package main

import "fmt"

func Greet(name string) string {
	return fmt.Sprintf("Hello, %s", name)
}

type Animal struct {
	Name string
}

func (a *Animal) Speak() string {
	return a.Name + " makes a noise."
}

type Speaker interface {
	Speak() string
}

type StringSlice []string
`

// --- function declarations ---

func TestGetSymbols_Go_FunctionDeclaration(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(goFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["Greet"]
	if !ok {
		t.Fatal(`symbol "Greet" not found`)
	}
	if s.Type != Function {
		t.Errorf(`"Greet": got type %q, want %q`, s.Type, Function)
	}
}

// --- method declarations ---

func TestGetSymbols_Go_MethodDeclaration(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(goFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["Speak"]
	if !ok {
		t.Fatal(`symbol "Speak" not found`)
	}
	if s.Type != Method {
		t.Errorf(`"Speak": got type %q, want %q`, s.Type, Method)
	}
}

// --- struct types ---

func TestGetSymbols_Go_StructType(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(goFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["Animal"]
	if !ok {
		t.Fatal(`symbol "Animal" not found`)
	}
	if s.Type != Class {
		t.Errorf(`"Animal": got type %q, want %q`, s.Type, Class)
	}
}

// --- interface types ---

func TestGetSymbols_Go_InterfaceType(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(goFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["Speaker"]
	if !ok {
		t.Fatal(`symbol "Speaker" not found`)
	}
	if s.Type != Interface {
		t.Errorf(`"Speaker": got type %q, want %q`, s.Type, Interface)
	}
}

// --- type aliases / other types ---

func TestGetSymbols_Go_TypeAlias(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(goFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["StringSlice"]
	if !ok {
		t.Fatal(`symbol "StringSlice" not found`)
	}
	if s.Type != TypeAlias {
		t.Errorf(`"StringSlice": got type %q, want %q`, s.Type, TypeAlias)
	}
}

// --- all symbols together ---

func TestGetSymbols_Go_AllSymbols(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(goFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Greet", Function},
		{"Animal", Class},
		{"Speak", Method},
		{"Speaker", Interface},
		{"StringSlice", TypeAlias},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
		if s.StartLine <= 0 {
			t.Errorf("symbol %q: StartLine should be > 0, got %d", tc.name, s.StartLine)
		}
		if s.EndLine < s.StartLine {
			t.Errorf("symbol %q: EndLine (%d) < StartLine (%d)", tc.name, s.EndLine, s.StartLine)
		}
	}
}

// --- multiple functions ---

func TestGetSymbols_Go_MultipleFunctions(t *testing.T) {
	const fixture = `package utils

func Add(a, b int) int { return a + b }
func Sub(a, b int) int { return a - b }
func Mul(a, b int) int { return a * b }
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("utils.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"Add", "Sub", "Mul"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Function {
			t.Errorf("symbol %q: got type %q, want %q", name, s.Type, Function)
		}
	}
}

// --- multiple methods on the same receiver ---

func TestGetSymbols_Go_MultipleMethods(t *testing.T) {
	const fixture = `package repo

type UserRepo struct{ db DB }

func (r *UserRepo) Find(id int) User    { return r.db.Find(id) }
func (r *UserRepo) Save(u User) error   { return r.db.Save(u) }
func (r *UserRepo) Delete(id int) error { return r.db.Delete(id) }
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("repo.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"UserRepo", Class},
		{"Find", Method},
		{"Save", Method},
		{"Delete", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

// --- multiple interfaces ---

func TestGetSymbols_Go_MultipleInterfaces(t *testing.T) {
	const fixture = `package io

type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
type ReadWriter interface {
	Reader
	Writer
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("io.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"Reader", "Writer", "ReadWriter"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Interface {
			t.Errorf("symbol %q: got type %q, want %q", name, s.Type, Interface)
		}
	}
}

// --- iota enum blocks ---

func TestGetSymbols_Go_IotaEnum(t *testing.T) {
	const fixture = `package main

type Direction int

const (
	North Direction = iota
	South
	East
	West
)
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	// North is the representative name — the first const with = iota
	s, ok := byName["North"]
	if !ok {
		t.Fatal(`symbol "North" not found (iota enum)`)
	}
	if s.Type != Enum {
		t.Errorf(`"North": got type %q, want %q`, s.Type, Enum)
	}

	// South/East/West have no iota — should NOT be captured
	for _, name := range []string{"South", "East", "West"} {
		if _, ok := byName[name]; ok {
			t.Errorf("symbol %q should not be captured (no iota)", name)
		}
	}
}

func TestGetSymbols_Go_MultipleIotaBlocks(t *testing.T) {
	const fixture = `package main

type Color int
type Size int

const (
	Red Color = iota
	Green
	Blue
)

const (
	Small Size = iota
	Medium
	Large
)
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Red", Enum},
		{"Small", Enum},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found", tc.name)
			continue
		}
		if s.Type != tc.wantType {
			t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
		}
	}
}

func TestGetSymbols_Go_PlainConstNotCaptured(t *testing.T) {
	const fixture = `package main

const StatusActive = "active"
const MaxRetries = 3
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	// Plain consts with no iota should not appear
	for _, name := range []string{"StatusActive", "MaxRetries"} {
		if _, ok := byName[name]; ok {
			t.Errorf("plain const %q should not be captured", name)
		}
	}
}

// --- true type alias (type Foo = Bar) ---

func TestGetSymbols_Go_TrueTypeAlias(t *testing.T) {
	const fixture = `package main

import "io"

type ReadCloser = io.ReadCloser
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["ReadCloser"]
	if !ok {
		t.Fatal(`symbol "ReadCloser" not found (true type alias)`)
	}
	if s.Type != TypeAlias {
		t.Errorf(`"ReadCloser": got type %q, want %q`, s.Type, TypeAlias)
	}
}

// --- single package-level var ---

func TestGetSymbols_Go_SingleVar(t *testing.T) {
	const fixture = `package main

import "errors"

var ErrNotFound = errors.New("not found")
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["ErrNotFound"]
	if !ok {
		t.Fatal(`symbol "ErrNotFound" not found (single var)`)
	}
	if s.Type != Variable {
		t.Errorf(`"ErrNotFound": got type %q, want %q`, s.Type, Variable)
	}
}

// --- local var must NOT be captured ---

func TestGetSymbols_Go_VarLineRange(t *testing.T) {
	const fixture = `package main

import "errors"

var ErrNotFound = errors.New("not found")
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["ErrNotFound"]
	if !ok {
		t.Fatal(`symbol "ErrNotFound" not found`)
	}
	// var declaration is on line 5 — line range must not span the whole file
	if s.StartLine != 5 {
		t.Errorf("StartLine: got %d, want 5", s.StartLine)
	}
	if s.EndLine != 5 {
		t.Errorf("EndLine: got %d, want 5", s.EndLine)
	}
}

func TestGetSymbols_Go_LocalVarNotCaptured(t *testing.T) {
	const fixture = `package main

func Run() {
	var wg sync.WaitGroup
	var stats IndexStats
	var deleted []string
	wg.Wait()
	_ = stats
	_ = deleted
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"wg", "stats", "deleted"} {
		if _, ok := byName[name]; ok {
			t.Errorf("local var %q should not be captured", name)
		}
	}
}

func TestGetSymbols_Go_PackageLevelVarStillCaptured(t *testing.T) {
	const fixture = `package main

import "errors"

var ErrNotFound = errors.New("not found")

var (
	ErrTimeout  = errors.New("timeout")
	ErrCanceled = errors.New("canceled")
)

func Run() {
	var localVar int
	_ = localVar
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	// Package-level vars must still be captured.
	for _, name := range []string{"ErrNotFound", "ErrTimeout", "ErrCanceled"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("package-level var %q should be captured", name)
		}
	}

	// Local var inside Run must not be captured.
	if _, ok := byName["localVar"]; ok {
		t.Error("local var \"localVar\" should not be captured")
	}
}

// --- block var declaration ---

func TestGetSymbols_Go_BlockVar(t *testing.T) {
	const fixture = `package main

import "errors"

var (
	ErrTimeout   = errors.New("timeout")
	ErrCanceled  = errors.New("canceled")
)
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.go", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"ErrTimeout", "ErrCanceled"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found (block var)", name)
			continue
		}
		if s.Type != Variable {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Variable)
		}
	}
}
