package commands

import (
	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var workspaceCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new workspace with the given name, or switch to it if it already exists",
	Long:  `A workspace is a named collection of repositories. Use this command to create or switch to a workspace.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceCreate,
}

func runWorkspaceCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	db, err := workspace.OpenWorkspace(name)
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		return err
	}

	cmd.Printf("Workspace created %q\n", name)
	cmd.Printf("Next, set it as current: mimir workspace use %s\n", name)

	return nil
}
