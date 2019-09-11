package cli

import (
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	"github.com/spf13/cobra"
)

// BuildEnvCmd is the top level command for environments
func BuildEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Environment commands",
		Long: `Command for working with environments.
An environment represents a deployment stage.`,
	}
	cmd.AddCommand(BuildEnvAddCmd())
	cmd.AddCommand(BuildEnvListCmd())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": "Develop 🔧",
	}
	return cmd
}
