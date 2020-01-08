package cli

import (
	"github.com/spf13/cobra"
)

// BuildSecretAddCmd adds a secret.
func BuildSecretAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Adds a secret.",
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
