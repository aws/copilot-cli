package cli

import (
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/store/ssm"
	"github.com/spf13/cobra"
)

// InitProjectOpts contains the fields to collect for creating a project
type InitProjectOpts struct {
	ProjectName string `survey:"project"`
	Prompt      terminal.Stdio
	manager     archer.ProjectCreator
}

// CreateProject calls the manager to create a project
func (opts *InitProjectOpts) CreateProject() error {
	return opts.manager.CreateProject(&archer.Project{
		Name:    opts.ProjectName,
		Version: "1.0",
	})
}

// BuildProjectInitCommand creates a command which gets the fields for creating a project
func BuildProjectInitCommand() *cobra.Command {
	opts := InitProjectOpts{
		Prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
	}

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Creates a new, empty project",
		Example: `
  Create a new project named test
  $ archer project init test`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ssmStore, err := ssm.NewStore()
			if err != nil {
				return err
			}
			opts.manager = ssmStore
			opts.ProjectName = args[0]
			return opts.CreateProject()
		},
	}
	return cmd
}
