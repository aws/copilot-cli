package cli

import (
	"github.com/spf13/cobra"
)

// BuildEndpointDeleteCmd adds a secret.
func BuildEndpointDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-prod-url",
		Short: "Deletes the CNAME for the prod app.",
		Example: `/code $ ecs-preview endpoint delete-prod-url`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}
	return cmd
}
