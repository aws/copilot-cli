package cli

import (
	"github.com/spf13/cobra"
)

// BuildDatabaseCreateCmd adds an RDS database.
func BuildDatabaseCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a serverless Aurora database.",
		Example: `
/code $ ecs-preview env delete --name test --profile default`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}
	return cmd
}
