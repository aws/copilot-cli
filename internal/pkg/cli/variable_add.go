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

// VariableAddOpts contains the fields to collect to create an environment variable.
type VariableAddOpts struct {
	appName string
	name    string
	value   string

	manifestPath string

	storeReader storeReader

	ws archer.Workspace

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *VariableAddOpts) Validate() error {
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
func (o *VariableAddOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	if err := o.askAppName(); err != nil {
		return err
	}

	if err := o.askEnvVarName(); err != nil {
		return err
	}
	return o.askEnvVarValue()
}

// Execute adds an environment variable.
func (o *VariableAddOpts) Execute() error {
	o.manifestPath = o.ws.AppManifestFileName(o.appName)
	mft, err := o.readManifest()
	if err != nil {
		return err
	}

	// add the env var to the manifest
	lbmft := mft.(*manifest.LBFargateManifest)
	if lbmft.Variables == nil {
		lbmft.Variables = make(map[string]string)
	}
	lbmft.Variables[o.name] = o.value

	if err = o.writeManifest(lbmft); err != nil {
		return err
	}

	log.Successf("Saved the environment variable %s to the manifest.\n",
		color.HighlightUserInput(o.name))
	return nil
}

func (o *VariableAddOpts) readManifest() (archer.Manifest, error) {
	raw, err := o.ws.ReadFile(o.manifestPath)
	if err != nil {
		return nil, err
	}
	return manifest.UnmarshalApp(raw)
}

func (o *VariableAddOpts) writeManifest(manifest *manifest.LBFargateManifest) error {
	manifestBytes, err := yaml.Marshal(manifest)
	_, err = o.ws.WriteFile(manifestBytes, o.manifestPath)
	return err
}

func (o *VariableAddOpts) askProject() error {
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

func (o *VariableAddOpts) askAppName() error {
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

func (o *VariableAddOpts) askEnvVarName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.prompt.Get(
		fmt.Sprintf("Name:"),
		fmt.Sprintf(`The name of your environment variable.`),
		validateEnvVarName)

	if err != nil {
		return fmt.Errorf("failed to get the variable: %w", err)
	}

	o.name = name
	return nil
}

func (o *VariableAddOpts) askEnvVarValue() error {
	if o.value != "" {
		return nil
	}

	value, err := o.prompt.Get(
		fmt.Sprintf("Value:"),
		fmt.Sprintf(`The value of the environment variable.`),
		noValidation)

	if err != nil {
		return fmt.Errorf("failed to get the value: %w", err)
	}

	o.value = value
	return nil
}

func (o *VariableAddOpts) retrieveProjects() ([]string, error) {
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

func (o *VariableAddOpts) workspaceAppNames() ([]string, error) {
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

// BuildVariableAddCmd adds an environment variable.
func BuildVariableAddCmd() *cobra.Command {
	opts := VariableAddOpts{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"create"},
		Short:   "Adds an environment variable.",
		Example: `
  /code $ dw_run.sh variable add
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
	cmd.Flags().StringVarP(&opts.name, "name", "n", "", "Name of the environment variable.")
	cmd.Flags().StringVarP(&opts.value, "value", "v", "", "Value of the environment variable.")
	cmd.Flags().StringP(projectFlag, projectFlagShort, "dw-run" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))

	return cmd
}
