package commands

import (
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceAddCmd = &cobra.Command{
	Use:   "add <path> [workspace_name]",
	Short: "Add a repository to a workspace",
	Long:  `Add a repository to a workspace. If workspace_name is omitted the current active workspace is used.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runWorkspaceAdd,
}

func runWorkspaceAdd(cmd *cobra.Command, args []string) error {
	path := args[0]

	workspaceName, err := resolveWorkspaceName(args, 1)
	if err != nil {
		return err
	}

	workspaceDb, err := workspace.OpenWorkspace(workspaceName)
	if err != nil {
		return fmt.Errorf("cannot open workspace: %w", err)
	}
	defer workspaceDb.Close()

	repoID, err := workspace.AddRepository(workspaceDb, path)
	if err != nil {
		return err
	}

	cmd.Printf("Repository %q added to workspace %q\n", repoID, workspaceName)

	return nil
}
