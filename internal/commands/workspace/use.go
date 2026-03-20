package commands

import (
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active workspace",
	Long:  `Set the active workspace. Subsequent commands that require a workspace will use this one by default.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceUse,
}

func runWorkspaceUse(cmd *cobra.Command, args []string) error {
	name := args[0]

	if err := workspace.SetCurrentWorkspace(name); err != nil {
		return fmt.Errorf("cannot set current workspace: %w", err)
	}

	cmd.Printf("Switched to workspace %q\n", name)
	return nil
}
