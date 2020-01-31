package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// EndpointCreateOpts contains the fields to collect to create a secret.
type EndpointCreateOpts struct {
	appName string

	r53         archer.URLManager
	storeReader storeReader

	ws archer.Workspace

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *EndpointCreateOpts) Validate() error {
	if o.ProjectName() != "" {
		_, err := o.storeReader.GetProject(o.ProjectName())
		if err != nil {
			return err
		}
	}
	if o.appName != "" {
		_, err := o.storeReader.GetApplication(o.ProjectName(), o.appName)
		if err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *EndpointCreateOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askAppName()
}

// Execute creates a CNAME to {app-name}.dw.run.
func (o *EndpointCreateOpts) Execute() error {
	source := fmt.Sprintf("%s.prod.dw-run.dw.run", o.appName)
	target := fmt.Sprintf("%s.dw.run", o.appName)

	if err := o.r53.CreateCNAME(source, target); err != nil {
		return err
	}

	log.Successf("You can now access the app at %s\n", fmt.Sprintf("https://%s", target))

	return nil
}

func (o *EndpointCreateOpts) askProject() error {
	if o.ProjectName() != "" {
		return nil
	}
	projNames, err := o.retrieveProjects()
	if err != nil {
		return err
	}
	if len(projNames) == 0 {
		log.Infoln("There are no projects to select.")
	}
	proj, err := o.prompt.SelectOne(
		"Which project:",
		applicationShowProjectNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return fmt.Errorf("selecting projects: %w", err)
	}
	o.projectName = proj

	return nil
}

func (o *EndpointCreateOpts) askAppName() error {
	if o.appName != "" {
		return nil
	}
	appNames, err := o.retrieveApplications()
	if err != nil {
		return err
	}
	if len(appNames) == 0 {
		log.Infof("No applications found in project '%s'\n.", o.ProjectName())
		return nil
	}
	appName, err := o.prompt.SelectOne(
		fmt.Sprintf("Which app:"),
		"The app this secret belongs to.",
		appNames,
	)
	if err != nil {
		return fmt.Errorf("selecting applications for project %s: %w", o.ProjectName(), err)
	}
	o.appName = appName

	return nil
}

func (o *EndpointCreateOpts) retrieveProjects() ([]string, error) {
	projs, err := o.storeReader.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	projNames := make([]string, len(projs))
	for ind, proj := range projs {
		projNames[ind] = proj.Name
	}
	return projNames, nil
}

func (o *EndpointCreateOpts) retrieveApplications() ([]string, error) {
	apps, err := o.storeReader.ListApplications(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("listing applications for project %s: %w", o.ProjectName(), err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}
	return appNames, nil
}

// BuildEndpointCreateCmd adds a CNAME to Route53.
func BuildEndpointCreateCmd() *cobra.Command {
	opts := EndpointCreateOpts{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:     "create-prod-url",
		Aliases: []string{"create"},
		Short:   "Creates a short reference to the prod app; e.g. {app-name}.prod.dw-run.dw.run -> {app-name}.dw.run.",
		Example: `/code $ dw_run.sh endpoint create-prod-url`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			store, err := store.New()
			if err != nil {
				return fmt.Errorf("connect to environment datastore: %w", err)
			}
			ws, err := workspace.New()
			if err != nil {
				return fmt.Errorf("new workspace: %w", err)
			}
			opts.ws = ws
			opts.storeReader = store
			opts.r53 = store

			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}

	cmd.Flags().StringVarP(&opts.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().StringP(projectFlag, projectFlagShort, "dw-run" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))

	return cmd
}
