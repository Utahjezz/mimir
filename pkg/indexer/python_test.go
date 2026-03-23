package indexer

// python_test.go — symbol-extraction tests for Python (.py).
// Covers every Python pattern supported by languages/python/queries.go:
//   function definitions, class definitions, methods,
//   module-level variable assignments.

import "testing"

// pyFixture is a representative Python file covering one of each symbol type.
const pyFixture = `class Animal:
    def __init__(self, name):
        self.name = name

    def speak(self):
        return self.name + " makes a noise."


def greet(name):
    return "Hello, " + name
`

// --- function definitions ---

func TestGetSymbols_Python_FunctionDefinition(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.py", []byte(pyFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["greet"]
	if !ok {
		t.Fatal(`symbol "greet" not found`)
	}
	if s.Type != Function {
		t.Errorf(`"greet": got type %q, want %q`, s.Type, Function)
	}
}

// --- class definitions ---

func TestGetSymbols_Python_ClassDefinition(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.py", []byte(pyFixture))
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

// --- method definitions ---

func TestGetSymbols_Python_MethodDefinition(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.py", []byte(pyFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["speak"]
	if !ok {
		t.Fatal(`symbol "speak" not found`)
	}
	if s.Type != Method {
		t.Errorf(`"speak": got type %q, want %q`, s.Type, Method)
	}
}

// --- all symbols together ---

func TestGetSymbols_Python_AllSymbols(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("main.py", []byte(pyFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Animal", Class},
		{"speak", Method},
		{"greet", Function},
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

// --- multiple top-level functions ---

func TestGetSymbols_Python_MultipleFunctions(t *testing.T) {
	const fixture = `def add(a, b):
    return a + b

def sub(a, b):
    return a - b

def mul(a, b):
    return a * b
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("utils.py", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"add", "sub", "mul"} {
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

// --- multiple methods on the same class ---

func TestGetSymbols_Python_MultipleMethods(t *testing.T) {
	const fixture = `class UserRepo:
    def find(self, user_id):
        pass

    def save(self, user):
        pass

    def delete(self, user_id):
        pass
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("repo.py", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"UserRepo", Class},
		{"find", Method},
		{"save", Method},
		{"delete", Method},
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

// --- __init__ is captured as a method ---

func TestGetSymbols_Python_InitMethod(t *testing.T) {
	const fixture = `class Foo:
    def __init__(self):
        pass
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("foo.py", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	s, ok := byName["__init__"]
	if !ok {
		t.Fatal(`symbol "__init__" not found`)
	}
	if s.Type != Method {
		t.Errorf(`"__init__": got type %q, want %q`, s.Type, Method)
	}
}

// --- decorated function definitions (FastAPI route handlers, etc.) ---

func TestGetSymbols_Python_DecoratedFunctions(t *testing.T) {
	const fixture = `from fastapi import APIRouter

router = APIRouter()

@router.post("/invoices")
async def create_invoice(data: dict):
    pass

@router.get("/invoices/{id}")
async def get_invoice(invoice_id: int):
    pass

def plain_helper():
    pass
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("routes.py", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"create_invoice", "get_invoice", "plain_helper"} {
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

// --- decorated class definitions ---

func TestGetSymbols_Python_DecoratedClass(t *testing.T) {
	const fixture = `from dataclasses import dataclass

@dataclass
class Item:
    name: str
    price: float

class PlainClass:
    pass
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("models.py", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	for _, name := range []string{"Item", "PlainClass"} {
		s, ok := byName[name]
		if !ok {
			t.Errorf("symbol %q not found", name)
			continue
		}
		if s.Type != Class {
			t.Errorf("symbol %q: got type %q, want %q", name, s.Type, Class)
		}
	}
}

// --- decorated method definitions (@staticmethod, @classmethod, etc.) ---

func TestGetSymbols_Python_DecoratedMethods(t *testing.T) {
	const fixture = `class Service:
    def regular_method(self):
        pass

    @staticmethod
    def static_helper():
        pass

    @classmethod
    def from_config(cls, config):
        pass
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("service.py", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"Service", Class},
		{"regular_method", Method},
		{"static_helper", Method},
		{"from_config", Method},
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

// --- unrecognised extension returns an error ---

func TestGetSymbols_Python_UnknownExtension(t *testing.T) {
	m := newTestMuncher()
	_, err := m.GetSymbols("script.rb", []byte("def foo; end"))
	if err == nil {
		t.Fatal("expected error for unsupported extension, got nil")
	}
}
