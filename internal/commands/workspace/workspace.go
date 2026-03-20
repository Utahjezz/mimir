package commands

import (
	"errors"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var WorkspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
	Long:  `A workspace is a named collection of repositories. Use this command to create or switch to a workspace.`,
}

func init() {
	Register(WorkspaceCmd)
}

// resolveWorkspaceName returns args[argIdx] when present, otherwise falls back
// to the current active workspace from the global config.
func resolveWorkspaceName(args []string, argIdx int) (string, error) {
	if len(args) > argIdx {
		return args[argIdx], nil
	}

	name, err := workspace.GetCurrentWorkspace()
	if errors.Is(err, workspace.ErrNoCurrentWorkspace) {
		return "", fmt.Errorf("no workspace specified and %w", err)
	}
	return name, err
}
