package workspace

// config_test.go — unit tests for GetCurrentWorkspace and SetCurrentWorkspace.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestGetCurrentWorkspace_NoConfigFile verifies that ErrNoCurrentWorkspace is
// returned when the config file does not exist yet.
func TestGetCurrentWorkspace_NoConfigFile(t *testing.T) {
	// Arrange: redirect all file I/O to an empty temp directory.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act
	_, err := GetCurrentWorkspace()

	// Assert
	if !errors.Is(err, ErrNoCurrentWorkspace) {
		t.Errorf("expected ErrNoCurrentWorkspace, got: %v", err)
	}
}

// TestGetCurrentWorkspace_EmptyWorkspace verifies that ErrNoCurrentWorkspace is
// returned when the config file exists but current_workspace is "".
func TestGetCurrentWorkspace_EmptyWorkspace(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Write a config file with an empty workspace name.
	cfgDir := filepath.Join(tmp, "mimir")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	cfgFile := filepath.Join(cfgDir, "config.json")
	if err := os.WriteFile(cfgFile, []byte(`{"current_workspace":""}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Act
	_, err := GetCurrentWorkspace()

	// Assert
	if !errors.Is(err, ErrNoCurrentWorkspace) {
		t.Errorf("expected ErrNoCurrentWorkspace for empty field, got: %v", err)
	}
}

// TestSetAndGetCurrentWorkspace is a roundtrip test: set a name then get it
// back and verify the values match.
func TestSetAndGetCurrentWorkspace(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := "my-workspace"

	// Act
	if err := SetCurrentWorkspace(want); err != nil {
		t.Fatalf("SetCurrentWorkspace: %v", err)
	}
	got, err := GetCurrentWorkspace()

	// Assert
	if err != nil {
		t.Fatalf("GetCurrentWorkspace: %v", err)
	}
	if got != want {
		t.Errorf("GetCurrentWorkspace: got %q, want %q", got, want)
	}
}

// TestSetCurrentWorkspace_Overwrites verifies that calling SetCurrentWorkspace
// twice keeps only the most recent value.
func TestSetCurrentWorkspace_Overwrites(t *testing.T) {
	// Arrange
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Act: set twice
	if err := SetCurrentWorkspace("first"); err != nil {
		t.Fatalf("first SetCurrentWorkspace: %v", err)
	}
	if err := SetCurrentWorkspace("second"); err != nil {
		t.Fatalf("second SetCurrentWorkspace: %v", err)
	}
	got, err := GetCurrentWorkspace()

	// Assert
	if err != nil {
		t.Fatalf("GetCurrentWorkspace: %v", err)
	}
	if got != "second" {
		t.Errorf("expected %q after overwrite, got %q", "second", got)
	}
}

// TestGetCurrentWorkspace_MalformedJSON verifies that an error is returned when
// the config file contains invalid JSON.
func TestGetCurrentWorkspace_MalformedJSON(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfgDir := filepath.Join(tmp, "mimir")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	cfgFile := filepath.Join(cfgDir, "config.json")
	if err := os.WriteFile(cfgFile, []byte(`{not valid json`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Act
	_, err := GetCurrentWorkspace()

	// Assert: must be a non-nil error that is NOT ErrNoCurrentWorkspace
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if errors.Is(err, ErrNoCurrentWorkspace) {
		t.Errorf("malformed JSON should not produce ErrNoCurrentWorkspace; got: %v", err)
	}
}
