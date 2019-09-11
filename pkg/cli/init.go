package cli

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	"github.com/spf13/cobra"
)

// InitAppOpts represents an ECS Service or Task and any related AWS Infrastructure.
type InitAppOpts struct {
	Project string `survey:"project"` // namespace that this application belongs to.
	Name    string `survey:"name"`    // unique identifier to logically group AWS resources together.
	Type    string `survey:"Type"`    // The type of application you're trying to build (LoadBalanced, Backend, etc.)
	// Prompt holds the interfaces to receive and output app configuration data to the terminal.
	Prompt terminal.Stdio
}

// Ask prompts the user for the value of any required fields that are not already provided.
func (opts *InitAppOpts) Ask() error {
	var qs []*survey.Question
	if opts.Project == "" {
		qs = append(qs, &survey.Question{
			Name: "project",
			Prompt: &survey.Input{
				Message: "What is your project's name?",
				Help:    "Applications under the same project share the same VPC and ECS Cluster and are discoverable via service discovery.",
			},
			Validate: validateProjectName,
		})
	}
	if opts.Name == "" {
		qs = append(qs, &survey.Question{
			Name: "name",
			Prompt: &survey.Input{
				Message: "What is your application's name?",
				Help:    "Collection of AWS services to achieve a business capability. Must be unique within a project.",
			},
			Validate: validateApplicationName,
		})
	}
	return survey.Ask(qs, opts, survey.WithStdio(opts.Prompt.In, opts.Prompt.Out, opts.Prompt.Err))
}

// Validate returns an error if a command line flag provided value is invalid
func (opts *InitAppOpts) Validate() error {
	if err := validateProjectName(opts.Project); err != nil && err != errValueEmpty {
		return fmt.Errorf("project name invalid: %v", err)
	}

	if err := validateApplicationName(opts.Name); err != nil && err != errValueEmpty {
		return fmt.Errorf("application name invalid: %v", err)
	}

	return nil
}

// InitApp creates the project and application structure
func (a *InitAppOpts) InitApp() error {
	//TODO fill in
	return nil
}

// BuildInitCmd builds the command to build an application
func BuildInitCmd() *cobra.Command {
	opts := InitAppOpts{Prompt: terminal.Stdio{
		In:  os.Stdin,
		Out: os.Stderr,
		Err: os.Stderr,
	}}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new ECS application",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Ask()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.InitApp()
		},
	}
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Name of the project (required).")
	cmd.Flags().StringVarP(&opts.Name, "app", "a", "", "Name of the application (required).")
	cmd.Flags().StringVarP(&opts.Type, "type", "t", "", "Type of application to create.")
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": "Getting Started ✨",
	}
	return cmd
}
