package cli

import (
	"github.com/spf13/cobra"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/cli/template"
)

// RootCmd is top-level entry point to all subcommands.
var RootCmd *cobra.Command

func init() {
	RootCmd = buildRootCmd()
}

func buildRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archer",
		Short: "Launch and manage applications on Amazon ECS and AWS Fargate 🚀",
		Example: `
  Display the help menu for the init command
  $ archer init --help`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// If we don't set a Run() function the help menu doesn't show up.
			// See https://github.com/spf13/cobra/issues/790
		},
		SilenceUsage: true,
	}

	cmd.AddCommand(buildInitCmd())
	cmd.AddCommand(buildEnvCmd())
	cmd.AddCommand(buildProjCmd())
	cmd.SetUsageTemplate(template.RootUsage)
	return cmd
}
