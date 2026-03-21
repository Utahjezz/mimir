package commands

import (
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceRemoveCmd = &cobra.Command{
	Use:   "remove <path> [workspace_name]",
	Short: "Remove a repository from a workspace",
	Long:  `Remove a repository from a workspace. If workspace_name is omitted the current active workspace is used.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runWorkspaceRemove,
}

func runWorkspaceRemove(cmd *cobra.Command, args []string) error {
	path := args[0]

	workspaceName, err := resolveWorkspaceName(args, 1)
	if err != nil {
		return err
	}

	db, err := workspace.OpenWorkspace(workspaceName)
	if err != nil {
		return fmt.Errorf("cannot open workspace: %w", err)
	}
	defer db.Close()

	if err := workspace.RemoveRepository(db, path); err != nil {
		return err
	}

	cmd.Printf("Repository %q removed from workspace %q\n", path, workspaceName)

	return nil
}
