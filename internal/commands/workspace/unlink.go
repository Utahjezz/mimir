package commands

import (
	"fmt"
	"strconv"

	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceUnlinkCmd = &cobra.Command{
	Use:   "unlink <id> [workspace]",
	Short: "Remove a cross-repo symbol link by ID",
	Long:  `Remove a cross-repo symbol link from a workspace by its numeric ID.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runWorkspaceUnlink,
}

func runWorkspaceUnlink(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid link ID %q: expected a numeric value", args[0])
	}

	workspaceName, err := resolveWorkspaceName(args, 1)
	if err != nil {
		return err
	}

	db, err := workspace.OpenWorkspace(workspaceName)
	if err != nil {
		return fmt.Errorf("cannot open workspace: %w", err)
	}
	defer db.Close()

	if err := workspace.DeleteLink(db, id); err != nil {
		return fmt.Errorf("cannot delete link: %w", err)
	}

	cmd.Printf("Link #%d removed\n", id)
	return nil
}
