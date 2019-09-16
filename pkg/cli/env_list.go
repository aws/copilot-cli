package cli

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/store/ssm"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// ListEnvOpts contains the fields to collect for listing an environment
type ListEnvOpts struct {
	ProjectName string `survey:"project"`
	Prompt      terminal.Stdio
	manager     archer.EnvironmentLister
}

// ListEnvironments does the actual work of listing environments
func (opts *ListEnvOpts) ListEnvironments() error {
	envs, err := opts.manager.ListEnvironments(opts.ProjectName)
	if err != nil {
		fmt.Fprintf(opts.Prompt.Err, "%v\n", err)
		return err
	}

	prodColor := color.New(color.FgYellow, color.Bold).SprintFunc()
	for _, env := range envs {
		if env.Prod {
			fmt.Fprintf(opts.Prompt.Out, "%s (prod)\n", prodColor(env.Name))
		} else {
			fmt.Fprintln(opts.Prompt.Out, env.Name)
		}
	}

	return nil
}

// BuildEnvListCmd lists environments for a particular project
func BuildEnvListCmd() *cobra.Command {
	opts := ListEnvOpts{
		Prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the environments in a particular project",
		Example: `
  Lists all the environments for the test project
  $ archer env ls --project test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ssmStore, err := ssm.NewStore()
			if err != nil {
				return err
			}
			opts.manager = ssmStore
			return opts.ListEnvironments()
		},
	}
	cmd.Flags().StringVar(&opts.ProjectName, "project", "", "Name of the project (required).")
	return cmd
}
