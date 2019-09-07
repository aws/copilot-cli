package cli

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/store/ssm"
	"github.com/spf13/cobra"
)

// ListProjectOpts contains the fields to collect for creating a project
type ListProjectOpts struct {
	Prompt  terminal.Stdio
	manager archer.ProjectLister
}

// ListProjects calls the manager to create a project
func (opts *ListProjectOpts) ListProjects() error {
	projects, err := opts.manager.ListProjects()
	if err != nil {
		return err
	}

	for _, proj := range projects {
		fmt.Fprintln(opts.Prompt.Out, proj.Name)
	}

	return nil
}

// BuildProjectListCommand creates a command which lists projects
func BuildProjectListCommand() *cobra.Command {
	opts := ListProjectOpts{
		Prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all projects in your account",
		Example: `
  List all the projects in your account and region
  $ archer project ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ssmStore, err := ssm.NewStore()
			if err != nil {
				return err
			}
			opts.manager = ssmStore
			return opts.ListProjects()
		},
	}
	return cmd
}
