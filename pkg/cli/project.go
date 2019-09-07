package cli

import (
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	"github.com/spf13/cobra"
)

// BuildProjCmd builds the top level project command and related subcommands
func BuildProjCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project commands",
		Long: `Command for working with projects.
A Project represents all of your deployment environments.`,
	}
	cmd.AddCommand(BuildProjectInitCommand())
	cmd.AddCommand(BuildProjectListCommand())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": "Develop 🔧",
	}
	return cmd
}
