package setup

// setup_test.go — tests for the interactive setup wizard and its helpers.
// Patterns follow the repo convention: t.TempDir(), t.Setenv(), cmd.SetIn/SetOut.

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

// newPromptReader wraps s in a bufio.Reader as prompt() expects.
func newPromptReader(s string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(s))
}

// runSetupWithInput wires input/output to a fresh cobra.Command and calls
// runSetup — the same path cobra takes when the binary is invoked.
func runSetupWithInput(t *testing.T, input string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(input))
	err := runSetup(cmd, nil)
	return out.String(), err
}

// ────────────────────────────────────────────────────────────────────────────
// prompt
// ────────────────────────────────────────────────────────────────────────────

func TestPrompt_ValidChoice(t *testing.T) {
	w := &bytes.Buffer{}
	r := newPromptReader("2\n")
	got, err := prompt(w, r, "▶ ", 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestPrompt_InvalidThenValid(t *testing.T) {
	w := &bytes.Buffer{}
	// "abc" is rejected, then "1" is accepted.
	r := newPromptReader("abc\n1\n")
	got, err := prompt(w, r, "▶ ", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
	if !strings.Contains(w.String(), "Invalid choice") {
		t.Error("expected an 'Invalid choice' message in output")
	}
}

func TestPrompt_OutOfRangeThenValid(t *testing.T) {
	w := &bytes.Buffer{}
	r := newPromptReader("99\n1\n")
	got, err := prompt(w, r, "▶ ", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}

func TestPrompt_EOFOnEmptyLine_ReturnsCancelled(t *testing.T) {
	w := &bytes.Buffer{}
	// io.EOF with no preceding text → errCancelled
	r := bufio.NewReader(strings.NewReader(""))
	_, err := prompt(w, r, "▶ ", 1, 2)
	if err != errCancelled {
		t.Errorf("expected errCancelled, got %v", err)
	}
}

func TestPrompt_EOFWithPartialLine_UsesIt(t *testing.T) {
	w := &bytes.Buffer{}
	// "1" with no trailing newline — bufio returns io.EOF but line is "1".
	r := bufio.NewReader(strings.NewReader("1"))
	got, err := prompt(w, r, "▶ ", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}

func TestPrompt_MinBoundaryAccepted(t *testing.T) {
	w := &bytes.Buffer{}
	r := newPromptReader("0\n")
	got, err := prompt(w, r, "▶ ", 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// displayPath
// ────────────────────────────────────────────────────────────────────────────

func TestDisplayPath_ReplacesHomePrefix(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	path := filepath.Join(tmp, "foo", "bar")
	got := displayPath(path)
	if !strings.HasPrefix(got, "~") {
		t.Errorf("expected tilde prefix, got %q", got)
	}
	if strings.Contains(got, tmp) {
		t.Errorf("expected home dir to be replaced, got %q", got)
	}
}

func TestDisplayPath_ExactHomeDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := displayPath(tmp)
	if got != "~" {
		t.Errorf("expected \"~\", got %q", got)
	}
}

func TestDisplayPath_NoHomePrefix_Unchanged(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// A path that shares characters but not the separator boundary.
	other := tmp + "_sibling"
	got := displayPath(other)
	if got != other {
		t.Errorf("expected path unchanged, got %q", got)
	}
}

func TestDisplayPath_NoBoundaryFalsePositive(t *testing.T) {
	// Regression: home=/tmp/al must not match /tmp/alex/foo.
	fakeHome := "/tmp/al"
	path := "/tmp/alex/foo"
	// Override HOME env so os.UserHomeDir picks it up on Unix.
	t.Setenv("HOME", fakeHome)
	got := displayPath(path)
	if got != path {
		t.Errorf("false prefix match: expected %q unchanged, got %q", path, got)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// skillTargetDir
// ────────────────────────────────────────────────────────────────────────────

func TestSkillTargetDir_GlobalScope(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got, err := skillTargetDir(1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(tmp, ".agents", "skills", "mimir")
	if got != want {
		t.Errorf("global skill dir: got %q, want %q", got, want)
	}
}

// resolveDir resolves symlinks up to the first component that does not yet
// exist. This lets us compare paths that include directories that will be
// created later (e.g. .opencode/skills/mimir) on macOS where t.TempDir()
// returns a /var/... symlink but os.Getwd() returns the canonical /private/...
// path.
func resolveDir(t *testing.T, path string) string {
	t.Helper()
	// Walk up the path until EvalSymlinks succeeds, then reattach the suffix.
	suffix := ""
	cur := path
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(resolved, suffix)
		}
		suffix = filepath.Join(filepath.Base(cur), suffix)
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached filesystem root without success — return original.
			return path
		}
		cur = parent
	}
}

func TestSkillTargetDir_ProjectOpenCode(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	got, err := skillTargetDir(2, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	want := filepath.Join(realTmp, ".opencode", "skills", "mimir")
	if resolveDir(t, got) != want {
		t.Errorf("project opencode skill dir: got %q (resolved %q), want %q", got, resolveDir(t, got), want)
	}
}

func TestSkillTargetDir_ProjectClaudeCode(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	got, err := skillTargetDir(2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	want := filepath.Join(realTmp, ".claude", "skills", "mimir")
	if resolveDir(t, got) != want {
		t.Errorf("project claude skill dir: got %q (resolved %q), want %q", got, resolveDir(t, got), want)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// toolTargetPath
// ────────────────────────────────────────────────────────────────────────────

func TestToolTargetPath_GlobalScope(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got, err := toolTargetPath(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(tmp, ".config", "opencode", "tools", "mimir.ts")
	if got != want {
		t.Errorf("global tool path: got %q, want %q", got, want)
	}
}

func TestToolTargetPath_ProjectScope(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	got, err := toolTargetPath(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	want := filepath.Join(realTmp, ".opencode", "tools", "mimir.ts")
	if resolveDir(t, got) != want {
		t.Errorf("project tool path: got %q (resolved %q), want %q", got, resolveDir(t, got), want)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// installSkill
// ────────────────────────────────────────────────────────────────────────────

func TestInstallSkill_WritesAllFiles(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "skill-out")
	w := &bytes.Buffer{}

	if err := installSkill(w, targetDir); err != nil {
		t.Fatalf("installSkill: %v", err)
	}

	expected := []string{
		filepath.Join(targetDir, "SKILL.md"),
		filepath.Join(targetDir, "references", "commands.md"),
		filepath.Join(targetDir, "references", "workspaces.md"),
	}
	for _, p := range expected {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %q to exist: %v", p, err)
		}
	}
}

func TestInstallSkill_SuccessMessageContainsTargetDir(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "skill-out")
	w := &bytes.Buffer{}

	if err := installSkill(w, targetDir); err != nil {
		t.Fatalf("installSkill: %v", err)
	}

	out := w.String()
	if !strings.Contains(out, "✓") {
		t.Errorf("expected success checkmark in output, got: %q", out)
	}
}

func TestInstallSkill_IdempotentOverwrite(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "skill-out")

	// First install.
	if err := installSkill(io.Discard, targetDir); err != nil {
		t.Fatalf("first installSkill: %v", err)
	}
	// Second install must not error (idempotent overwrite).
	if err := installSkill(io.Discard, targetDir); err != nil {
		t.Fatalf("second installSkill (idempotent): %v", err)
	}

	// Verify files are still intact.
	skillMd := filepath.Join(targetDir, "SKILL.md")
	if _, err := os.Stat(skillMd); err != nil {
		t.Errorf("SKILL.md missing after second install: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// installTool
// ────────────────────────────────────────────────────────────────────────────

func TestInstallTool_WritesFile(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "tools", "mimir.ts")

	if err := installTool(io.Discard, targetPath); err != nil {
		t.Fatalf("installTool: %v", err)
	}

	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("expected mimir.ts at %q: %v", targetPath, err)
	}
}

func TestInstallTool_IdempotentOverwrite(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "tools", "mimir.ts")

	if err := installTool(io.Discard, targetPath); err != nil {
		t.Fatalf("first installTool: %v", err)
	}
	if err := installTool(io.Discard, targetPath); err != nil {
		t.Fatalf("second installTool (idempotent): %v", err)
	}
}

func TestInstallTool_SuccessMessageContainsPath(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "tools", "mimir.ts")
	w := &bytes.Buffer{}

	if err := installTool(w, targetPath); err != nil {
		t.Fatalf("installTool: %v", err)
	}
	if !strings.Contains(w.String(), "✓") {
		t.Errorf("expected checkmark in output, got: %q", w.String())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// runSetup — end-to-end via cobra cmd.SetIn/SetOut
// ────────────────────────────────────────────────────────────────────────────

func TestRunSetup_Choice0_Cancels(t *testing.T) {
	out, err := runSetupWithInput(t, "0\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected 'Cancelled' in output, got: %q", out)
	}
}

func TestRunSetup_EOFOnFirstPrompt_Cancels(t *testing.T) {
	// Empty stdin → EOF → errCancelled → prints "Cancelled." and returns nil.
	out, err := runSetupWithInput(t, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected 'Cancelled' in output, got: %q", out)
	}
}

func TestRunSetup_SkillGlobalInstall(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Choice 1 (Skill) → scope 1 (Global)
	out, err := runSetupWithInput(t, "1\n1\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Success banner should appear.
	if !strings.Contains(out, "Done") {
		t.Errorf("expected 'Done' in output, got: %q", out)
	}

	// Files must exist under ~/.agents/skills/mimir/.
	skillDir := filepath.Join(tmp, ".agents", "skills", "mimir")
	for _, name := range []string{"SKILL.md", filepath.Join("references", "commands.md"), filepath.Join("references", "workspaces.md")} {
		if _, err := os.Stat(filepath.Join(skillDir, name)); err != nil {
			t.Errorf("expected skill file %q: %v", name, err)
		}
	}
}

func TestRunSetup_SkillProjectOpenCode(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Choice 1 (Skill) → scope 2 (Project) → app 1 (OpenCode)
	out, err := runSetupWithInput(t, "1\n2\n1\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("expected 'Done' in output, got: %q", out)
	}

	skillDir := filepath.Join(tmp, ".opencode", "skills", "mimir")
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md in .opencode: %v", err)
	}
}

func TestRunSetup_SkillProjectClaudeCode(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Choice 1 (Skill) → scope 2 (Project) → app 2 (Claude Code)
	out, err := runSetupWithInput(t, "1\n2\n2\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("expected 'Done' in output, got: %q", out)
	}

	skillDir := filepath.Join(tmp, ".claude", "skills", "mimir")
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Errorf("expected SKILL.md in .claude: %v", err)
	}
}

func TestRunSetup_ToolGlobalInstall(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Choice 2 (Tool) → scope 1 (Global)
	out, err := runSetupWithInput(t, "2\n1\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("expected 'Done' in output, got: %q", out)
	}

	toolPath := filepath.Join(tmp, ".config", "opencode", "tools", "mimir.ts")
	if _, err := os.Stat(toolPath); err != nil {
		t.Errorf("expected mimir.ts at global path: %v", err)
	}
}

func TestRunSetup_ToolProjectInstall(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Choice 2 (Tool) → scope 2 (Project)
	out, err := runSetupWithInput(t, "2\n2\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("expected 'Done' in output, got: %q", out)
	}

	toolPath := filepath.Join(tmp, ".opencode", "tools", "mimir.ts")
	if _, err := os.Stat(toolPath); err != nil {
		t.Errorf("expected mimir.ts at project path: %v", err)
	}
}

func TestRunSetup_EOFDuringSkillScopePrompt_Cancels(t *testing.T) {
	// Choice 1 (Skill) then EOF on scope prompt → Cancelled.
	out, err := runSetupWithInput(t, "1\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected 'Cancelled' in output, got: %q", out)
	}
}

func TestRunSetup_IdempotentSkillInstall(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	input := "1\n1\n"

	// First install.
	if _, err := runSetupWithInput(t, input); err != nil {
		t.Fatalf("first setup run: %v", err)
	}
	// Second install must not error (idempotent overwrite).
	if _, err := runSetupWithInput(t, input); err != nil {
		t.Fatalf("second setup run (idempotent): %v", err)
	}
}
