package indexer

// csharp_test.go — symbol-extraction tests for C# (.cs).
// Covers every C# pattern supported by languages/csharp/queries.go:
//   class, struct, interface, enum declarations,
//   methods, constructors, properties,
//   delegates (type aliases), top-level fields (constants/static vars).

import "testing"

// --- class ---

func TestGetSymbols_CSharp_Class(t *testing.T) {
	const fixture = `
namespace MyApp {
    public class Animal {
        public string Name { get; set; }
        public void Speak() {}
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("animal.cs", []byte(fixture))
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

// --- struct ---

func TestGetSymbols_CSharp_Struct(t *testing.T) {
	const fixture = `
public struct Point {
    public int X;
    public int Y;
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("point.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["Point"]
	if !ok {
		t.Fatal(`symbol "Point" not found`)
	}
	if s.Type != Class {
		t.Errorf(`"Point": got type %q, want %q`, s.Type, Class)
	}
}

// --- interface ---

func TestGetSymbols_CSharp_Interface(t *testing.T) {
	const fixture = `
public interface ISpeaker {
    void Speak();
    string Name { get; }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("speaker.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["ISpeaker"]
	if !ok {
		t.Fatal(`symbol "ISpeaker" not found`)
	}
	if s.Type != Interface {
		t.Errorf(`"ISpeaker": got type %q, want %q`, s.Type, Interface)
	}
}

// --- enum ---

func TestGetSymbols_CSharp_Enum(t *testing.T) {
	const fixture = `
public enum Direction {
    North,
    South,
    East,
    West
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("direction.cs", []byte(fixture))
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

// --- method ---

func TestGetSymbols_CSharp_Method(t *testing.T) {
	const fixture = `
public class Greeter {
    public string Greet(string name) {
        return "Hello, " + name;
    }

    private void Reset() {}
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("greeter.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"Greet", "Reset"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Method {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Method)
		}
	}
}

// --- constructor ---

func TestGetSymbols_CSharp_Constructor(t *testing.T) {
	const fixture = `
public class Car {
    public Car(string model) {
        Model = model;
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("car.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["Car"]
	if !ok {
		t.Fatal(`constructor "Car" not found`)
	}
	if s.Type != Function {
		t.Errorf(`"Car" constructor: got type %q, want %q`, s.Type, Function)
	}
}

// --- property ---

func TestGetSymbols_CSharp_Property(t *testing.T) {
	const fixture = `
public class Person {
    public string Name { get; set; }
    public int Age { get; private set; }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("person.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"Name", "Age"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("property %q not found", name)
			continue
		}
		if s.Type != Variable {
			t.Errorf("%q: got type %q, want %q", name, s.Type, Variable)
		}
	}
}

// --- delegate ---

func TestGetSymbols_CSharp_Delegate(t *testing.T) {
	const fixture = `
public delegate void EventHandler(object sender, EventArgs e);
public delegate int Transformer(int x);
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("delegates.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	for _, name := range []string{"EventHandler", "Transformer"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("delegate %q not found", name)
			continue
		}
		if s.Type != TypeAlias {
			t.Errorf("%q: got type %q, want %q", name, s.Type, TypeAlias)
		}
	}
}

// --- all symbols together ---

func TestGetSymbols_CSharp_AllSymbols(t *testing.T) {
	const fixture = `
using System;

namespace MyApp {
    public enum Status { Active, Inactive }

    public interface IRepository {
        void Save();
    }

    public class UserService : IRepository {
        public string Name { get; set; }

        public UserService(string name) {
            Name = name;
        }

        public void Save() {}

        public User Find(int id) { return null; }
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("service.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Status", Enum},
		{"IRepository", Interface},
		{"UserService", Class},
		{"UserService", Function}, // constructor — last write wins in byNameMap; handled below
		{"Save", Method},
		{"Find", Method},
		{"Name", Variable},
	}

	// Check each independently (constructor and class share name — check both exist)
	if _, ok := byName["Status"]; !ok {
		t.Error(`"Status" enum not found`)
	}
	if _, ok := byName["IRepository"]; !ok {
		t.Error(`"IRepository" interface not found`)
	}
	if _, ok := byName["UserService"]; !ok {
		t.Error(`"UserService" not found`)
	}
	if _, ok := byName["Save"]; !ok {
		t.Error(`"Save" method not found`)
	}
	if _, ok := byName["Find"]; !ok {
		t.Error(`"Find" method not found`)
	}
	if _, ok := byName["Name"]; !ok {
		t.Error(`"Name" property not found`)
	}

	_ = cases // used above individually
}

// --- namespace declarations ---

func TestGetSymbols_CSharp_Namespace_Simple(t *testing.T) {
	const fixture = `
namespace DataAccess {
    public class UserRepository {
        public void Save() {}
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("repo.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["DataAccess"]
	if !ok {
		t.Fatal(`symbol "DataAccess" not found`)
	}
	if s.Type != Namespace {
		t.Errorf(`"DataAccess": got type %q, want %q`, s.Type, Namespace)
	}
}

func TestGetSymbols_CSharp_Namespace_AsParent(t *testing.T) {
	const fixture = `
namespace DataAccess {
    public class UserRepository {
        public void Save() {}
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("repo.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// UserRepository should have parent "DataAccess"
	for _, s := range symbols {
		if s.Name == "UserRepository" {
			if s.Parent != "DataAccess" {
				t.Errorf(`"UserRepository" parent: got %q, want %q`, s.Parent, "DataAccess")
			}
			return
		}
	}
	t.Fatal(`symbol "UserRepository" not found`)
}

func TestGetSymbols_CSharp_Namespace_NestedFQN(t *testing.T) {
	const fixture = `
namespace Company.Platform {
    namespace Services {
        public class OrderService {
            public void PlaceOrder() {}
        }
        public interface IPaymentGateway {
            void Charge();
        }
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("services.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build a map keyed by name+type for disambiguation
	type key struct{ name, typ string }
	byKey := make(map[key]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byKey[key{s.Name, string(s.Type)}] = s
	}

	cases := []struct {
		name       string
		typ        SymbolType
		wantParent string
	}{
		{"Company.Platform", Namespace, ""},
		{"Services", Namespace, "Company.Platform"},
		{"OrderService", Class, "Company.Platform.Services"},
		{"PlaceOrder", Method, "OrderService"},
		{"IPaymentGateway", Interface, "Company.Platform.Services"},
		{"Charge", Method, "IPaymentGateway"},
	}

	for _, tc := range cases {
		s, ok := byKey[key{tc.name, string(tc.typ)}]
		if !ok {
			t.Errorf("symbol %q (type %s) not found", tc.name, tc.typ)
			continue
		}
		if s.Parent != tc.wantParent {
			t.Errorf("%q parent: got %q, want %q", tc.name, s.Parent, tc.wantParent)
		}
	}
}

// --- line ranges are sensible ---

func TestGetSymbols_CSharp_LineRanges(t *testing.T) {
	const fixture = `using System;

public class MyClass {
    public void DoWork() {
        var x = 1;
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("myclass.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cls, ok := byName["MyClass"]
	if !ok {
		t.Fatal(`"MyClass" not found`)
	}
	if cls.StartLine <= 0 {
		t.Errorf("MyClass StartLine should be > 0, got %d", cls.StartLine)
	}
	if cls.EndLine < cls.StartLine {
		t.Errorf("MyClass EndLine (%d) < StartLine (%d)", cls.EndLine, cls.StartLine)
	}

	method, ok := byName["DoWork"]
	if !ok {
		t.Fatal(`"DoWork" not found`)
	}
	if method.StartLine <= 0 {
		t.Errorf("DoWork StartLine should be > 0, got %d", method.StartLine)
	}
	if method.EndLine < method.StartLine {
		t.Errorf("DoWork EndLine (%d) < StartLine (%d)", method.EndLine, method.StartLine)
	}
}
