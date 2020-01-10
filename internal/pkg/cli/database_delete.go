package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// DatabaseDeleteOpts contains the fields to collect to delete a database.
type DatabaseDeleteOpts struct {
	appName string

	manifestPath string

	dbManager   archer.DatabaseManager
	storeReader storeReader

	ws archer.Workspace

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *DatabaseDeleteOpts) Validate() error {
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
func (o *DatabaseDeleteOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askAppName()
}

// Execute creates the cluster.
func (o *DatabaseDeleteOpts) Execute() error {
	o.manifestPath = o.ws.AppManifestFileName(o.appName)

	mft, err := o.readManifest()
	if err != nil {
		return err
	}
	lbmft := mft.(*manifest.LBFargateManifest)

	clusterID := fmt.Sprintf("%s-%s-%s", o.GlobalOpts.ProjectName(),
		o.appName, lbmft.Variables["DB_NAME"])

	if err := o.dbManager.DeleteDatabase(clusterID); err != nil {
		return err
	}

	log.Successf("Deleted the database %s in %s under project %s. Final snapshot: %s.\n",
		color.HighlightUserInput(lbmft.Variables["DB_NAME"]), color.HighlightResource(o.appName),
		color.HighlightResource(o.GlobalOpts.ProjectName()), color.HighlightResource(clusterID))

	// remove the db details from the manifest
	delete(lbmft.Variables, "DB_HOST")
	delete(lbmft.Variables, "DB_PORT")
	delete(lbmft.Variables, "DB_NAME")
	delete(lbmft.Variables, "DB_USERNAME")
	delete(lbmft.Variables, "DB_PASSWORD")
	lbmft.Database = manifest.DatabaseConfig{}

	if err = o.writeManifest(lbmft); err != nil {
		return err
	}

	log.Successf("Removed the parameters of the database from the manifest.\n")
	return nil
}

func (o *DatabaseDeleteOpts) readManifest() (archer.Manifest, error) {
	raw, err := o.ws.ReadFile(o.manifestPath)
	if err != nil {
		return nil, err
	}
	return manifest.UnmarshalApp(raw)
}

func (o *DatabaseDeleteOpts) writeManifest(manifest *manifest.LBFargateManifest) error {
	manifestBytes, err := yaml.Marshal(manifest)
	_, err = o.ws.WriteFile(manifestBytes, o.manifestPath)
	return err
}

func (o *DatabaseDeleteOpts) askProject() error {
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

func (o *DatabaseDeleteOpts) askAppName() error {
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

func (o *DatabaseDeleteOpts) retrieveProjects() ([]string, error) {
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

func (o *DatabaseDeleteOpts) retrieveApplications() ([]string, error) {
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

// BuildDatabaseDeleteCmd deletes a serverless Aurora cluster.
func BuildDatabaseDeleteCmd() *cobra.Command {
	opts := DatabaseDeleteOpts{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes a serverless Aurora database.",
		Example: `
/code $ ecs-preview env delete --name test --profile default`,
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
			opts.dbManager = store

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
	cmd.Flags().StringP(projectFlag, projectFlagShort, "" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))

	return cmd
}
