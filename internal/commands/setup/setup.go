package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	skill "github.com/Utahjezz/mimir/skills/mimir"
	tool "github.com/Utahjezz/mimir/tools/opencode"
	"github.com/spf13/cobra"
)

// errCancelled is returned when the user signals EOF on an empty line. It is
// handled by runSetup to exit cleanly without printing a cobra error message.
var errCancelled = errors.New("cancelled")

// SetupCmd is the cobra command for `mimir setup`.
var SetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure mimir integrations with agentic coding tools",
	Long:  `Interactive wizard to install the mimir skill and/or tool plugin for OpenCode and Claude Code.`,
	RunE:  runSetup,
}

// prompt prints question to w, reads a line from r, and returns an integer in
// [min..max]. On invalid input it re-prompts until the user provides a valid choice.
// If stdin is closed (EOF) with a partial line, that line is used as input.
// If stdin is closed on an empty line, errCancelled is returned.
func prompt(w io.Writer, r *bufio.Reader, question string, min, max int) (int, error) {
	for {
		fmt.Fprint(w, question)
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				line = strings.TrimSpace(line)
				if line == "" {
					return 0, errCancelled
				}
				// fall through: treat partial line as input
			} else {
				return 0, fmt.Errorf("reading input: %w", err)
			}
		} else {
			line = strings.TrimSpace(line)
		}
		n, convErr := strconv.Atoi(line)
		if convErr == nil && n >= min && n <= max {
			return n, nil
		}
		fmt.Fprintf(w, "  Invalid choice %q — enter a number between %d and %d.\n", line, min, max)
	}
}

// displayPath replaces the user's home directory prefix with "~" for cleaner output.
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// writeEmbedded reads a single file from the given FS and writes it to dest,
// creating parent directories as needed.
func writeEmbedded(fsys fs.FS, name, dest string) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("reading embedded %s: %w", name, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", name, err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

// installSkill copies all skill files from SkillFS into targetDir.
func installSkill(w io.Writer, targetDir string) error {
	skillFiles := []string{
		"SKILL.md",
		"references/commands.md",
		"references/workspaces.md",
	}
	for _, name := range skillFiles {
		dest := filepath.Join(targetDir, filepath.FromSlash(name))
		if err := writeEmbedded(skill.SkillFS, name, dest); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "  ✓  %s\n", displayPath(targetDir)+"/")
	return nil
}

// installTool copies mimir.ts from ToolFS to targetPath.
func installTool(w io.Writer, targetPath string) error {
	if err := writeEmbedded(tool.ToolFS, "mimir.ts", targetPath); err != nil {
		return err
	}
	fmt.Fprintf(w, "  ✓  %s\n", displayPath(targetPath))
	return nil
}

// skillTargetDir resolves the skill installation directory based on scope and app.
// scope: 1=global, 2=project. app (only used when scope=2): 1=OpenCode, 2=Claude Code.
func skillTargetDir(scope, app int) (string, error) {
	if scope == 1 {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		return filepath.Join(home, ".agents", "skills", "mimir"), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}
	if app == 1 {
		return filepath.Join(cwd, ".opencode", "skills", "mimir"), nil
	}
	return filepath.Join(cwd, ".claude", "skills", "mimir"), nil
}

// toolTargetPath resolves the tool installation path based on scope.
// scope: 1=global, 2=project.
func toolTargetPath(scope int) (string, error) {
	if scope == 1 {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		return filepath.Join(home, ".config", "opencode", "tools", "mimir.ts"), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}
	return filepath.Join(cwd, ".opencode", "tools", "mimir.ts"), nil
}

// runSkillWizard handles the skill installation sub-flow.
func runSkillWizard(w io.Writer, r *bufio.Reader) error {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Install scope?")
	fmt.Fprintln(w, "  [1] Global  (~/.agents/skills/mimir/)")
	fmt.Fprintln(w, "  [2] Project-local")
	scope, err := prompt(w, r, "▶ ", 1, 2)
	if err != nil {
		return err
	}

	app := 0
	if scope == 2 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Install for which tool?")
		fmt.Fprintln(w, "  [1] OpenCode    (.opencode/skills/mimir/)")
		fmt.Fprintln(w, "  [2] Claude Code (.claude/skills/mimir/)")
		app, err = prompt(w, r, "▶ ", 1, 2)
		if err != nil {
			return err
		}
	}

	targetDir, err := skillTargetDir(scope, app)
	if err != nil {
		return err
	}

	fmt.Fprintln(w)
	fmt.Fprint(w, "Installing skill...  ")
	return installSkill(w, targetDir)
}

// runToolWizard handles the tool installation sub-flow.
func runToolWizard(w io.Writer, r *bufio.Reader) error {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Note: The tool plugin is currently only supported by OpenCode.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Install scope?")
	fmt.Fprintln(w, "  [1] Global  (~/.config/opencode/tools/mimir.ts)")
	fmt.Fprintln(w, "  [2] Project (.opencode/tools/mimir.ts)")
	scope, err := prompt(w, r, "▶ ", 1, 2)
	if err != nil {
		return err
	}

	targetPath, err := toolTargetPath(scope)
	if err != nil {
		return err
	}

	fmt.Fprintln(w)
	fmt.Fprint(w, "Installing tool...  ")
	return installTool(w, targetPath)
}

// runSetup drives the interactive setup wizard.
func runSetup(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	r := bufio.NewReader(os.Stdin)

	fmt.Fprintln(w, "Mimir Setup — integrate with your agentic coding tools")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "What do you want to install?")
	fmt.Fprintln(w, "  [1] Skill   (works with OpenCode and Claude Code)")
	fmt.Fprintln(w, "  [2] Tool    (OpenCode only — exposes mimir as native tool calls)")
	fmt.Fprintln(w, "  [0] Cancel")

	choice, err := prompt(w, r, "▶ ", 0, 2)
	if err != nil {
		if errors.Is(err, errCancelled) {
			fmt.Fprintln(w, "Cancelled.")
			return nil
		}
		return err
	}
	if choice == 0 {
		fmt.Fprintln(w, "Cancelled.")
		return nil
	}

	if choice == 1 {
		err = runSkillWizard(w, r)
	} else {
		err = runToolWizard(w, r)
	}
	if err != nil {
		if errors.Is(err, errCancelled) {
			fmt.Fprintln(w, "Cancelled.")
			return nil
		}
		return err
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Done. Restart your agentic tool to pick up the changes.")
	return nil
}
