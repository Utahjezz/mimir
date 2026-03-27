package indexer

// rust_test.go — symbol-extraction tests for Rust (.rs).
// Covers every Rust pattern supported by languages/rust/queries.go:
//   function, struct, enum, trait, impl methods, type aliases,
//   const, static, mod, macro_rules.

import "testing"

// --- function ---

func TestGetSymbols_Rust_Function(t *testing.T) {
	const fixture = `
fn greet(name: &str) -> String {
    format!("Hello, {}", name)
}

pub fn add(a: i32, b: i32) -> i32 {
    a + b
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("lib.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"greet", "add"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Function {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Function)
		}
	}
}

// --- struct ---

func TestGetSymbols_Rust_Struct(t *testing.T) {
	const fixture = `
pub struct Point {
    pub x: f64,
    pub y: f64,
}

struct Config {
    name: String,
    value: i32,
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"Point", "Config"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Class {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Class)
		}
	}
}

// --- enum ---

func TestGetSymbols_Rust_Enum(t *testing.T) {
	const fixture = `
pub enum Direction {
    North,
    South,
    East,
    West,
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("direction.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["Direction"]
	if !ok {
		t.Fatal(`symbol "Direction" not found`)
	}
	if s.Type != Enum {
		t.Errorf(`"Direction": got type %q, want %q`, s.Type, Enum)
	}
}

// --- trait ---

func TestGetSymbols_Rust_Trait(t *testing.T) {
	const fixture = `
pub trait Drawable {
    fn draw(&self);
    fn area(&self) -> f64;
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("drawable.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["Drawable"]
	if !ok {
		t.Fatal(`symbol "Drawable" not found`)
	}
	if s.Type != Interface {
		t.Errorf(`"Drawable": got type %q, want %q`, s.Type, Interface)
	}
}

// --- impl methods ---

func TestGetSymbols_Rust_ImplMethods(t *testing.T) {
	const fixture = `
struct Circle {
    radius: f64,
}

impl Circle {
    pub fn new(radius: f64) -> Self {
        Circle { radius }
    }

    pub fn area(&self) -> f64 {
        std::f64::consts::PI * self.radius * self.radius
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("circle.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"new", "area"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("impl method %q not found", name)
			continue
		}
		if s.Type != Method {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Method)
		}
	}

	// Circle should also be found as a struct
	s, ok := byName["Circle"]
	if !ok {
		t.Fatal(`symbol "Circle" not found`)
	}
	if s.Type != Class {
		t.Errorf(`"Circle": got type %q, want %q`, s.Type, Class)
	}
}

// --- type alias ---

func TestGetSymbols_Rust_TypeAlias(t *testing.T) {
	const fixture = `
type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;
type Callback = fn(i32) -> i32;
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("aliases.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"Result", "Callback"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("type alias %q not found", name)
			continue
		}
		if s.Type != TypeAlias {
			t.Errorf("%q: got type %q, want %q", name, s.Type, TypeAlias)
		}
	}
}

// --- const and static ---

func TestGetSymbols_Rust_ConstStatic(t *testing.T) {
	const fixture = `
const MAX_SIZE: usize = 1024;
static COUNTER: std::sync::atomic::AtomicU64 = std::sync::atomic::AtomicU64::new(0);
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("constants.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"MAX_SIZE", "COUNTER"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Variable {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Variable)
		}
	}
}

// --- mod ---

func TestGetSymbols_Rust_Mod(t *testing.T) {
	const fixture = `
mod utils {
    pub fn helper() {}
}

mod tests {
    use super::*;
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("lib.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"utils", "tests"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("mod %q not found", name)
			continue
		}
		if s.Type != Namespace {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Namespace)
		}
	}
}

// --- macro_rules ---

func TestGetSymbols_Rust_Macro(t *testing.T) {
	const fixture = `
macro_rules! my_vec {
    ($($x:expr),*) => {
        {
            let mut v = Vec::new();
            $(v.push($x);)*
            v
        }
    };
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("macros.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["my_vec"]
	if !ok {
		t.Fatal(`symbol "my_vec" not found`)
	}
	if s.Type != Function {
		t.Errorf(`"my_vec": got type %q, want %q`, s.Type, Function)
	}
}

// --- all symbols together ---

func TestGetSymbols_Rust_AllSymbols(t *testing.T) {
	const fixture = `
use std::fmt;

const VERSION: &str = "1.0";

pub struct Server {
    port: u16,
}

pub enum Status {
    Running,
    Stopped,
}

pub trait Service {
    fn start(&self);
}

impl Server {
    pub fn new(port: u16) -> Self {
        Server { port }
    }

    pub fn run(&self) {
        println!("Running on port {}", self.port);
    }
}

type ServerResult = Result<Server, String>;

mod config {
    pub fn load() -> String {
        String::from("default")
    }
}

fn main() {
    let s = Server::new(8080);
    s.run();
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	expected := map[string]SymbolType{
		"VERSION":      Variable,
		"Server":       Class,
		"Status":       Enum,
		"Service":      Interface,
		"ServerResult": TypeAlias,
		"config":       Namespace,
		"main":         Function,
	}

	for name, wantType := range expected {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != wantType {
			t.Errorf("%q: got type %q, want %q", name, s.Type, wantType)
		}
	}

	// impl methods — check at least one
	if _, ok := byName["new"]; !ok {
		t.Error(`impl method "new" not found`)
	}
	if _, ok := byName["run"]; !ok {
		t.Error(`impl method "run" not found`)
	}
}

// --- line ranges ---

func TestGetSymbols_Rust_LineRanges(t *testing.T) {
	const fixture = `fn hello() {
    println!("hello");
}

struct Foo {
    x: i32,
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("test.rs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	fn, ok := byName["hello"]
	if !ok {
		t.Fatal(`"hello" not found`)
	}
	if fn.StartLine <= 0 {
		t.Errorf("hello StartLine should be > 0, got %d", fn.StartLine)
	}
	if fn.EndLine < fn.StartLine {
		t.Errorf("hello EndLine (%d) < StartLine (%d)", fn.EndLine, fn.StartLine)
	}

	st, ok := byName["Foo"]
	if !ok {
		t.Fatal(`"Foo" not found`)
	}
	if st.StartLine <= 0 {
		t.Errorf("Foo StartLine should be > 0, got %d", st.StartLine)
	}
}
