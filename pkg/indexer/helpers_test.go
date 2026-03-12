package indexer

// helpers_test.go — shared test infrastructure: constructor helper and fixtures
// reused across facade_api_test.go, js_test.go, and typescript_test.go.

// jsFixture is a small JavaScript file covering one of each core symbol type.
const jsFixture = `function greet(name) {
  return "Hello, " + name;
}

const add = (a, b) => a + b;

class Animal {
  constructor(name) {
    this.name = name;
  }

  speak() {
    return this.name + " makes a noise.";
  }
}
`

// jsxFixture is a React component file with real JSX syntax.
// Exercises function components, arrow function components, class components, and methods.
const jsxFixture = `import React from 'react';

function Greeting({ name }) {
  return <h1>Hello, {name}</h1>;
}

const Button = ({ onClick, children }) => (
  <button onClick={onClick}>{children}</button>
);

class Counter extends React.Component {
  increment() {
    this.setState({ count: this.state.count + 1 });
  }

  render() {
    return <div>{this.state.count}</div>;
  }
}
`

// newTestMuncher returns a fresh MuncherFacade for use in tests.
func newTestMuncher() *MuncherFacade {
	return NewMuncher()
}

// byNameMap is a convenience helper that indexes a symbol slice by name.
func byNameMap(symbols []SymbolInfo) map[string]SymbolInfo {
	m := make(map[string]SymbolInfo, len(symbols))
	for _, s := range symbols {
		m[s.Name] = s
	}
	return m
}
