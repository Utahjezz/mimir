package indexer

// swift_test.go — symbol-extraction tests for Swift (.swift).
// Covers every Swift pattern supported by languages/swift/queries.go:
//   function, class, struct, actor, enum, protocol, extension,
//   typealias, property, init, methods.

import "testing"

// --- function ---

func TestGetSymbols_Swift_Function(t *testing.T) {
	const fixture = `
func greet(name: String) -> String {
    return "Hello, \(name)"
}

public func add(_ a: Int, _ b: Int) -> Int {
    return a + b
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.swift", []byte(fixture))
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

// --- class ---

func TestGetSymbols_Swift_Class(t *testing.T) {
	const fixture = `
class Animal {
    var name: String

    init(name: String) {
        self.name = name
    }
}

public class Vehicle {
    var speed: Double = 0.0
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("models.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"Animal", "Vehicle"} {
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

// --- struct ---

func TestGetSymbols_Swift_Struct(t *testing.T) {
	const fixture = `
struct Point {
    var x: Double
    var y: Double
}

public struct Config {
    let host: String
    let port: Int
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("types.swift", []byte(fixture))
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

// --- actor ---

func TestGetSymbols_Swift_Actor(t *testing.T) {
	const fixture = `
actor Counter {
    private var count = 0

    func increment() {
        count += 1
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("counter.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["Counter"]
	if !ok {
		t.Fatal(`symbol "Counter" not found`)
	}
	if s.Type != Class {
		t.Errorf(`"Counter": got type %q, want %q`, s.Type, Class)
	}
}

// --- enum ---

func TestGetSymbols_Swift_Enum(t *testing.T) {
	const fixture = `
enum Direction {
    case north
    case south
    case east
    case west
}

public enum Status: String {
    case active = "active"
    case inactive = "inactive"
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("enums.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"Direction", "Status"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Enum {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Enum)
		}
	}
}

// --- protocol ---

func TestGetSymbols_Swift_Protocol(t *testing.T) {
	const fixture = `
protocol Drawable {
    func draw()
    var color: String { get }
}

public protocol Configurable {
    func configure(with settings: [String: Any])
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("protocols.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"Drawable", "Configurable"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Interface {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Interface)
		}
	}
}

// --- extension ---

func TestGetSymbols_Swift_Extension(t *testing.T) {
	const fixture = `
extension String {
    func reversed() -> String {
        return String(self.reversed())
    }
}

extension Int {
    var isEven: Bool { return self % 2 == 0 }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("extensions.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"String", "Int"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("extension %q not found", name)
			continue
		}
		if s.Type != Namespace {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Namespace)
		}
	}
}

// --- typealias ---

func TestGetSymbols_Swift_TypeAlias(t *testing.T) {
	const fixture = `
typealias Callback = (String) -> Void
typealias StringDict = [String: String]
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("aliases.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"Callback", "StringDict"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("typealias %q not found", name)
			continue
		}
		if s.Type != TypeAlias {
			t.Errorf("%q: got type %q, want %q", name, s.Type, TypeAlias)
		}
	}
}

// --- methods ---

func TestGetSymbols_Swift_Methods(t *testing.T) {
	const fixture = `
class Circle {
    var radius: Double

    init(radius: Double) {
        self.radius = radius
    }

    func area() -> Double {
        return Double.pi * radius * radius
    }

    func scale(by factor: Double) {
        radius *= factor
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("circle.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	// Circle should be found as a class
	s, ok := byName["Circle"]
	if !ok {
		t.Fatal(`symbol "Circle" not found`)
	}
	if s.Type != Class {
		t.Errorf(`"Circle": got type %q, want %q`, s.Type, Class)
	}

	// Methods and init should be extracted
	for _, name := range []string{"init", "area", "scale"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("method %q not found", name)
			continue
		}
		if s.Type != Method {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Method)
		}
	}
}

// --- top-level property ---

func TestGetSymbols_Swift_Property(t *testing.T) {
	const fixture = `
let maxRetries = 3
var defaultTimeout: Double = 30.0
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("constants.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"maxRetries", "defaultTimeout"} {
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

// --- all symbols together ---

func TestGetSymbols_Swift_AllSymbols(t *testing.T) {
	const fixture = `
import Foundation

let version = "1.0"

struct Server {
    var port: Int

    init(port: Int) {
        self.port = port
    }

    func run() {
        print("Running on port \(port)")
    }
}

enum Status {
    case running
    case stopped
}

protocol Service {
    func start()
}

typealias ServerResult = Result<Server, Error>

extension String {
    func shout() -> String {
        return self.uppercased()
    }
}

func main() {
    let s = Server(port: 8080)
    s.run()
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.swift", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	expected := map[string]SymbolType{
		"version":      Variable,
		"Server":       Class,
		"Status":       Enum,
		"Service":      Interface,
		"ServerResult": TypeAlias,
		"String":       Namespace,
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

	// methods — check at least one
	if _, ok := byName["run"]; !ok {
		t.Error(`method "run" not found`)
	}
	if _, ok := byName["init"]; !ok {
		t.Error(`method "init" not found`)
	}
}

// --- line ranges ---

func TestGetSymbols_Swift_LineRanges(t *testing.T) {
	const fixture = `func hello() {
    print("hello")
}

struct Foo {
    var x: Int
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("test.swift", []byte(fixture))
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
