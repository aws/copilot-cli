package cli

import (
	"github.com/spf13/cobra"
)

// BuildEndpointCreateCmd adds a secret.
func BuildEndpointCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-prod-url",
		Short: "Creates a CNAME for the prod app.",
		Example: `/code $ ecs-preview endpoint create-prod-url`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}
	return cmd
}
