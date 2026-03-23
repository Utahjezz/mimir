package indexer

// python_decorated_regression_test.go — regression test proving that decorated
// Python definitions (functions, classes, methods) are captured by the indexer.
//
// Before the fix (main branch): these tests FAIL because the tree-sitter
// queries did not include patterns for decorated_definition nodes.
//
// After the fix (fix/python-decorated-definitions branch): these tests PASS.

import "testing"

// decoratedFixture exercises all three decorated definition categories at once:
//   - decorated top-level function  (@router.post def ...)
//   - decorated class               (@dataclass class ...)
//   - decorated method               (@staticmethod def ...)
//
// Each category also includes a plain (non-decorated) counterpart so we can
// verify the fix doesn't regress existing behaviour.
const decoratedFixture = `from dataclasses import dataclass
from fastapi import APIRouter

router = APIRouter()

@router.post("/items")
def create_item(data: dict):
    pass

def plain_function():
    pass

@dataclass
class Item:
    name: str

class PlainClass:
    pass

class Service:
    def regular(self):
        pass

    @staticmethod
    def helper():
        pass
`

func TestRegression_DecoratedDefinitions(t *testing.T) {
	m := newTestMuncher()
	symbols, err := m.GetSymbols("app.py", []byte(decoratedFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)

	cases := []struct {
		name     string
		wantType SymbolType
	}{
		// Decorated definitions — these were MISSED before the fix
		{"create_item", Function},
		{"Item", Class},
		{"helper", Method},

		// Plain definitions — must still work (no regression)
		{"plain_function", Function},
		{"PlainClass", Class},
		{"regular", Method},
		{"Service", Class},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := byName[tc.name]
			if !ok {
				t.Fatalf("symbol %q not found — decorated definition not captured", tc.name)
			}
			if s.Type != tc.wantType {
				t.Errorf("symbol %q: got type %q, want %q", tc.name, s.Type, tc.wantType)
			}
		})
	}
}
