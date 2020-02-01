package cli

import (
	"fmt"
	"reflect"

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

// VariableDeleteOpts contains the fields to collect to delete an environment variable.
type VariableDeleteOpts struct {
	appName string
	envName string
	name    string

	manifestPath string
	manifest     *manifest.LBFargateManifest

	storeReader storeReader

	ws archer.Workspace

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *VariableDeleteOpts) Validate() error {
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
	if o.envName != "" {
		if _, err := o.storeReader.GetEnvironment(o.ProjectName(), o.envName); err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *VariableDeleteOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	if err := o.askAppName(); err != nil {
		return err
	}

	return o.askEnvVarName()
}

// Execute deletes the environment variable.
func (o *VariableDeleteOpts) Execute() error {
	if o.envName == "" {
		delete(o.manifest.Variables, o.name)
	} else {
		delete(o.manifest.Environments[o.envName].Variables, o.name)
	}

	if err := o.writeManifest(o.manifest); err != nil {
		return err
	}

	log.Successf("Removed the environment variable %s from the manifest.\n",
		color.HighlightUserInput(o.name))
	return nil
}

func (o *VariableDeleteOpts) readManifest() (archer.Manifest, error) {
	raw, err := o.ws.ReadFile(o.manifestPath)
	if err != nil {
		return nil, err
	}
	return manifest.UnmarshalApp(raw)
}

func (o *VariableDeleteOpts) writeManifest(mft *manifest.LBFargateManifest) error {
	if len(o.manifest.Environments[o.envName].Variables) == 0 {
		envConf := o.manifest.Environments[o.envName]
		envConf.Variables = nil
		o.manifest.Environments[o.envName] = envConf
	}
	if reflect.DeepEqual(o.manifest.Environments[o.envName], manifest.LBFargateConfig{}) {
		delete(o.manifest.Environments, o.envName)
	}
	manifestBytes, err := yaml.Marshal(mft)
	_, err = o.ws.WriteFile(manifestBytes, o.manifestPath)
	return err
}

func (o *VariableDeleteOpts) askProject() error {
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

func (o *VariableDeleteOpts) askAppName() error {
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
		"The app this environment variable belongs to.",
		appNames,
	)
	if err != nil {
		return fmt.Errorf("selecting applications for project %s: %w", o.ProjectName(), err)
	}
	o.appName = appName

	return nil
}

func (o *VariableDeleteOpts) retrieveEnvVars() ([]string, error) {
	o.manifestPath = o.ws.AppManifestFileName(o.appName)
	mft, err := o.readManifest()
	if err != nil {
		return nil, err
	}

	o.manifest = mft.(*manifest.LBFargateManifest)

	var keys []string
	var variables map[string]string

	if o.envName == "" {
		variables = o.manifest.Variables
	} else {
		variables = o.manifest.Environments[o.envName].Variables
	}

	for k := range variables {
		keys = append(keys, k)
	}
	return keys, nil
}

func (o *VariableDeleteOpts) askEnvVarName() error {
	if o.name != "" {
		return nil
	}

	envVarNames, err := o.retrieveEnvVars()
	if err != nil {
		return err
	}
	if len(envVarNames) == 0 {
		log.Infof("No environment variables found in app %s\n.", o.appName)
		return nil
	}
	name, err := o.prompt.SelectOne(
		fmt.Sprintf("Name:"),
		"The name of the variable.",
		envVarNames,
	)

	if err != nil {
		return fmt.Errorf("failed to get the name of the environment variable: %w", err)
	}

	o.name = name
	return nil
}

func (o *VariableDeleteOpts) retrieveProjects() ([]string, error) {
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

func (o *VariableDeleteOpts) workspaceAppNames() ([]string, error) {
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

// BuildVariableDeleteCmd removes an environment variable.
func BuildVariableDeleteCmd() *cobra.Command {
	opts := VariableDeleteOpts{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"remove"},
		Short:   "Deletes an environment variable.",
		Example: `
  /code $ dw_run.sh variable delete
`,
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
	cmd.Flags().StringVarP(&opts.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&opts.name, "name", "n", "", "Name of the environment variable.")
	cmd.Flags().StringP(projectFlag, projectFlagShort, "dw-run" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))

	return cmd
}
