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

// S3AddOpts contains the fields to collect to create the s3 environment variables.
type S3AddOpts struct {
	appName string

	manifestPath string

	storeReader storeReader

	ws archer.Workspace

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *S3AddOpts) Validate() error {
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
func (o *S3AddOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askAppName()
}

// Execute adds the environment variables.
func (o *S3AddOpts) Execute() error {
	project := o.GlobalOpts.ProjectName()
	o.manifestPath = o.ws.AppManifestFileName(o.appName)

	mft, err := o.readManifest()
	if err != nil {
		return err
	}
	lbmft := mft.(*manifest.LBFargateManifest)
	if lbmft.Variables == nil {
		lbmft.Variables = make(map[string]string)
	}
	if lbmft.Environments == nil {
		lbmft.Environments = make(map[string]manifest.LBFargateConfig)
	}

	for _, envName := range []string{"dev", "prod"} {
		if _, ok := lbmft.Environments[envName]; !ok {
			lbmft.Environments[envName] = manifest.LBFargateConfig{}
		}
		if lbmft.Environments[envName].Variables == nil {
			envConf := lbmft.Environments[envName]
			envConf.Variables = make(map[string]string)
			lbmft.Environments[envName] = envConf
		}
		lbmft.Environments[envName].Variables["S3_BUCKET"] = fmt.Sprintf("%s-%s-storage", project, envName)
		lbmft.Environments[envName].Variables["S3_PREFIX"] = fmt.Sprintf("/apps/%s", o.appName)
	}
	if err = o.writeManifest(lbmft); err != nil {
		return err
	}

	log.Successf("Added the S3 environment variables to the manifest.\n")
	return nil
}

func (o *S3AddOpts) readManifest() (archer.Manifest, error) {
	raw, err := o.ws.ReadFile(o.manifestPath)
	if err != nil {
		return nil, err
	}
	return manifest.UnmarshalApp(raw)
}

func (o *S3AddOpts) writeManifest(manifest *manifest.LBFargateManifest) error {
	manifestBytes, err := yaml.Marshal(manifest)
	_, err = o.ws.WriteFile(manifestBytes, o.manifestPath)
	return err
}

func (o *S3AddOpts) askProject() error {
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

func (o *S3AddOpts) askAppName() error {
	if o.appName != "" {
		return nil
	}
	appNames, err := o.workspaceAppNames()
	if err != nil {
		return err
	}
	if len(appNames) == 0 {
		log.Infof("No applications found in project '%s'\n.", o.ProjectName())
		return nil
	}
	if len(appNames) == 1 {
		o.appName = appNames[0]
		log.Infof("Found the app: %s\n", color.HighlightUserInput(o.appName))
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

func (o *S3AddOpts) retrieveProjects() ([]string, error) {
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

func (o *S3AddOpts) workspaceAppNames() ([]string, error) {
	apps, err := o.ws.Apps()
	if err != nil {
		return nil, fmt.Errorf("get applications in the workspace: %w", err)
	}
	var names []string
	for _, app := range apps {
		names = append(names, app.AppName())
	}
	return names, nil
}

// BuildS3CreateCmd adds the environment variables to access S3 storage.
func BuildS3AddCmd() *cobra.Command {
	opts := S3AddOpts{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"create"},
		Short:   "Adds the environment variables to access S3 storage.",
		Example: `
  /code $ dw_run.sh s3 create`,
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
