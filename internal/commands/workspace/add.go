package commands

import (
	"fmt"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceAddCmd = &cobra.Command{
	Use:   "add <path> [workspace_name]",
	Short: "Add a repository to a workspace",
	Long: `Add a repository to a workspace. The first argument is always the repository path.
If workspace_name is omitted, the repository is added to the current active workspace.

Note: Argument order is <path> first, then [workspace_name]. This allows adding to the
current workspace without specifying a name when called with only one argument.

Examples:
  mimir workspace add .                              # Add current directory to active workspace
  mimir workspace add /path/to/repo                  # Add /path/to/repo to active workspace
  mimir workspace add . my-workspace                 # Add current directory to "my-workspace"
  mimir workspace add /path/to/repo my-workspace     # Add /path/to/repo to "my-workspace"`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runWorkspaceAdd,
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
