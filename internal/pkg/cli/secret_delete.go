package cli

import (
	"fmt"
	"reflect"
	"strings"

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

// SecretDeleteOpts contains the fields to collect to delete a secret.
type SecretDeleteOpts struct {
	appName    string
	envName    string
	secretName string

	manifestPath string
	manifest     *manifest.LBFargateManifest

	secretManager archer.SecretsManager
	storeReader   storeReader

	ws archer.Workspace

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *SecretDeleteOpts) Validate() error {
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
func (o *SecretDeleteOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	if err := o.askAppName(); err != nil {
		return err
	}

	return o.askSecretName()
}

// Execute deletes the secret.
func (o *SecretDeleteOpts) Execute() error {
	name := strings.ToLower(o.secretName)
	name = strings.ReplaceAll(name, "_", "-")

	key := fmt.Sprintf("/ecs-cli-v2/%s/applications/%s/secrets/%s", o.GlobalOpts.ProjectName(),
		o.appName, name)
	if o.envName != "" {
		key = fmt.Sprintf("%s-%s", key, o.envName)
	}

	if err := o.secretManager.DeleteSecret(key); err != nil {
		return err
	}

	log.Successf("Deleted %s in %s under project %s.\n", color.HighlightUserInput(o.secretName),
		color.HighlightResource(o.appName), color.HighlightResource(o.GlobalOpts.ProjectName()))

	if o.envName == "" {
		delete(o.manifest.Secrets, o.secretName)
	} else {
		delete(o.manifest.Environments[o.envName].Secrets, o.secretName)
	}

	if err := o.writeManifest(o.manifest); err != nil {
		return err
	}

	log.Successf("Removed the secret %s from the manifest\n", o.secretName)
	return nil
}

func (o *SecretDeleteOpts) readManifest() (archer.Manifest, error) {
	raw, err := o.ws.ReadFile(o.manifestPath)
	if err != nil {
		return nil, err
	}
	return manifest.UnmarshalApp(raw)
}

func (o *SecretDeleteOpts) writeManifest(mft *manifest.LBFargateManifest) error {
	if len(o.manifest.Environments[o.envName].Secrets) == 0 {
		envConf := o.manifest.Environments[o.envName]
		envConf.Secrets = nil
		o.manifest.Environments[o.envName] = envConf
	}
	if reflect.DeepEqual(o.manifest.Environments[o.envName], manifest.LBFargateConfig{}) {
		delete(o.manifest.Environments, o.envName)
	}
	manifestBytes, err := yaml.Marshal(mft)
	_, err = o.ws.WriteFile(manifestBytes, o.manifestPath)
	return err
}

func (o *SecretDeleteOpts) askProject() error {
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

func (o *SecretDeleteOpts) askAppName() error {
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

func (o *SecretDeleteOpts) retrieveSecrets() ([]string, error) {
	o.manifestPath = o.ws.AppManifestFileName(o.appName)
	mft, err := o.readManifest()
	if err != nil {
		return nil, err
	}

	o.manifest = mft.(*manifest.LBFargateManifest)

	var keys []string
	var secrets map[string]string

	if o.envName == "" {
		secrets = o.manifest.Secrets
	} else {
		secrets = o.manifest.Environments[o.envName].Secrets
	}

	for k := range secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func (o *SecretDeleteOpts) askSecretName() error {
	if o.secretName != "" {
		return nil
	}

	secrets, err := o.retrieveSecrets()
	if err != nil {
		return err
	}
	if len(secrets) == 0 {
		log.Infof("No secrets found in app %s\n.", o.appName)
		return nil
	}
	name, err := o.prompt.SelectOne(
		fmt.Sprintf("Secret name:"),
		"The name of the secret.",
		secrets,
	)

	if err != nil {
		return fmt.Errorf("failed to get the name of the secret: %w", err)
	}

	o.secretName = name
	return nil
}

func (o *SecretDeleteOpts) retrieveProjects() ([]string, error) {
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

func (o *SecretDeleteOpts) workspaceAppNames() ([]string, error) {
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

// BuildSecretDeleteCmd removes a secret.
func BuildSecretDeleteCmd() *cobra.Command {
	opts := SecretDeleteOpts{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"remove"},
		Short:   "Delete a secret.",
		Example: `
  /code $ dw_run.sh secret delete -n secret-name
`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ssmStore, err := store.New()
			if err != nil {
				return fmt.Errorf("connect to environment datastore: %w", err)
			}
			ws, err := workspace.New()
			if err != nil {
				return fmt.Errorf("new workspace: %w", err)
			}
			opts.ws = ws
			opts.storeReader = ssmStore
			opts.secretManager = ssmStore

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
	cmd.Flags().StringVarP(&opts.secretName, "secret-name", "n", "", "Name of the secret.")
	cmd.Flags().StringP(projectFlag, projectFlagShort, "dw-run" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))

	return cmd
}
