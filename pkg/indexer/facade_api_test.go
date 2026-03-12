package indexer

// facade_api_test.go — tests for the three public MuncherFacade methods:
//   GetSymbols, GetSymbol, GetSymbolContent
//
// These tests are language-agnostic: they verify the API contract and error
// handling using the shared jsFixture, not JS-specific symbol patterns.

import (
	"os"
	"testing"
)

// --- GetSymbols ---

func TestGetSymbols_ReturnsAllSymbols(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("file.js", []byte(jsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected symbols, got none")
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		{"greet", Function},
		{"add", Function},
		{"Animal", Class},
		{"speak", Method},
	}

	for _, tc := range cases {
		s, ok := byName[tc.name]
		if !ok {
			t.Errorf("symbol %q not found in results", tc.name)
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

func TestGetSymbols_UnsupportedExtension(t *testing.T) {
	m := newTestMuncher()
	_, err := m.GetSymbols("file.rb", []byte("def foo; end"))
	if err == nil {
		t.Fatal("expected error for unsupported extension, got nil")
	}
}

// --- GetSymbol ---

func TestGetSymbol_Found(t *testing.T) {
	m := newTestMuncher()
	sym, err := m.GetSymbol("file.js", []byte(jsFixture), "greet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sym.Name != "greet" {
		t.Errorf("got name %q, want %q", sym.Name, "greet")
	}
	if sym.Type != Function {
		t.Errorf("got type %q, want %q", sym.Type, Function)
	}
}

func TestGetSymbol_NotFound(t *testing.T) {
	m := newTestMuncher()
	_, err := m.GetSymbol("file.js", []byte(jsFixture), "nonExistent")
	if err == nil {
		t.Fatal("expected error for missing symbol, got nil")
	}
}

// --- GetSymbolContent ---

func TestGetSymbolContent_ReturnsCorrectLines(t *testing.T) {
	tmp, err := os.CreateTemp("", "mimir-test-*.js")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(jsFixture); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	tmp.Close()

	m := newTestMuncher()

	// greet spans lines 1-3 in jsFixture
	content, err := m.GetSymbolContent(tmp.Name(), 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "function greet(name) {\n  return \"Hello, \" + name;\n}\n"
	if content != want {
		t.Errorf("got content:\n%q\nwant:\n%q", content, want)
	}
}

func TestGetSymbolContent_InvalidFile(t *testing.T) {
	m := newTestMuncher()
	_, err := m.GetSymbolContent("/nonexistent/path/file.js", 1, 3)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
