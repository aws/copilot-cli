package cli

import (
	"github.com/spf13/cobra"
)

// BuildStorageRdsCmd adds an RDS database.
func BuildStorageRdsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rds",
		Short: "Creates an RDS database.",
		Example: `
  Delete the "test" environment.
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
