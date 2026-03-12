package indexer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

// MuncherFacade is the public entry point for symbol extraction.
// Use NewMuncher to create an instance.
type MuncherFacade struct {
	langs map[string]langEntry
}

// NewMuncher creates a MuncherFacade with all registered languages loaded.
func NewMuncher() *MuncherFacade {
	return &MuncherFacade{langs: buildLangMap()}
}

// GetSymbols parses the given source code using tree-sitter and returns all
// extracted symbols for the file identified by path.
func (m *MuncherFacade) GetSymbols(path string, code []byte) ([]SymbolInfo, error) {
	ext := filepath.Ext(path)
	entry, ok := m.langs[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", ext)
	}

	return runQuery(entry, code)
}

// GetSymbol returns the first symbol matching name from the parsed file.
func (m *MuncherFacade) GetSymbol(path string, code []byte, name string) (*SymbolInfo, error) {
	symbols, err := m.GetSymbols(path, code)
	if err != nil {
		return nil, err
	}

	for _, s := range symbols {
		if s.Name == name {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("symbol %q not found in %s", name, path)
}

// GetSymbolContent reads the source lines of a symbol from disk using its
// StartLine and EndLine. Lines are 1-indexed.
func (m *MuncherFacade) GetSymbolContent(path string, startLine, endLine int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open file %s: %w", path, err)
	}
	defer f.Close()

	var result []byte
	scanner := bufio.NewScanner(f)
	line := 1

	for scanner.Scan() {
		if line >= startLine {
			result = append(result, scanner.Bytes()...)
			result = append(result, '\n')
		}
		if line == endLine {
			break
		}
		line++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file %s: %w", path, err)
	}

	return string(result), nil
}
