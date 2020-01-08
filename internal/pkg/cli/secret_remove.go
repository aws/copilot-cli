package cli

import (
	"github.com/spf13/cobra"
)

// BuildSecretRemoveCmd removes a secret.
func BuildSecretRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Removes a secret.",
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
