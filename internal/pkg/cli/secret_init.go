// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsssm "github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/ssm"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

const (
	fmtSecretParameterName           = "/copilot/%s/%s/secrets/%s"
	fmtSecretParameterNameMftExample = "/copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/%s"
)

const (
	secretInitAppPrompt     = "Which application do you want to add the secret to?"
	secretInitAppPromptHelp = "The secret can then be versioned by your existing environments inside the application."

	secretInitSecretNamePrompt     = "What would you like to name this secret?"
	secretInitSecretNamePromptHelp = "The name of the secret, such as 'db_password'."

	fmtSecretInitSecretValuePrompt     = "What is the value of secret %s in environment %s?"
	fmtSecretInitSecretValuePromptHelp = "If you do not wish to add the secret %s to environment %s, you can leave this blank by pressing 'Enter' without entering any value."
)

type secretInitVars struct {
	appName string

	name          string
	values        map[string]string
	inputFilePath string
	overwrite     bool
}

type secretInitOpts struct {
	secretInitVars
	shouldShowOverwriteHint bool
	secretValues            map[string]map[string]string

	store                   store
	fs                      afero.Fs
	prompter                prompter
	selector                appSelector
	ws                      wsEnvironmentsLister
	envCompatibilityChecker map[string]versionCompatibilityChecker
	secretPutters           map[string]secretPutter

	configureClientsForEnv func(envName string) error
	readFile               func() ([]byte, error)
}

func newSecretInitOpts(vars secretInitVars) (*secretInitOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("secret init"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	fs := afero.NewOsFs()
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSession), awsssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	opts := secretInitOpts{
		secretInitVars: vars,
		store:          store,
		fs:             fs,
		ws:             ws,

		envCompatibilityChecker: make(map[string]versionCompatibilityChecker),
		secretPutters:           make(map[string]secretPutter),

		prompter: prompter,
		selector: selector.NewAppEnvSelector(prompter, store),
	}

	opts.configureClientsForEnv = func(envName string) error {
		checker, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
			App:         opts.appName,
			Env:         envName,
			ConfigStore: opts.store,
		})
		if err != nil {
			return fmt.Errorf("new environment compatibility checker: %v", err)
		}
		opts.envCompatibilityChecker[envName] = checker

		env, err := opts.targetEnv(envName)
		if err != nil {
			return err
		}
		sess, err := sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return fmt.Errorf("create session from environment manager role %s in region %s: %w", env.ManagerRoleARN, env.Region, err)
		}
		opts.secretPutters[envName] = ssm.New(sess)

		return nil
	}

	opts.readFile = func() ([]byte, error) {
		file, err := opts.fs.Open(opts.inputFilePath)
		if err != nil {
			return nil, fmt.Errorf("open input file %s: %w", opts.inputFilePath, err)
		}
		defer file.Close()
		f, err := afero.ReadFile(opts.fs, file.Name())
		if err != nil {
			return nil, fmt.Errorf("read input file %s: %w", opts.inputFilePath, err)
		}

		return f, nil
	}
	return &opts, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *secretInitOpts) Validate() error {
	if o.inputFilePath != "" && o.name != "" {
		return errors.New("cannot specify `--cli-input-yaml` with `--name`")
	}

	if o.inputFilePath != "" && o.values != nil {
		return errors.New("cannot specify `--cli-input-yaml` with `--values`")
	}

	if o.appName != "" {
		_, err := o.store.GetApplication(o.appName)
		if err != nil {
			return fmt.Errorf("get application %s: %w", o.appName, err)
		}
		if o.values != nil {
			for env := range o.values {
				if _, err := o.targetEnv(env); err != nil {
					return err
				}
			}
		}
	}

	if o.name != "" {
		if err := validateSecretName(o.name); err != nil {
			return err
		}
	}

	if o.inputFilePath != "" {
		if _, err := o.fs.Stat(o.inputFilePath); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required or important fields that are not provided.
func (o *secretInitOpts) Ask() error {
	if o.overwrite {
		log.Warningf("You have specified %s flag. Please note that overwriting an existing secret may break your deployed service.\n", color.HighlightCode(fmt.Sprintf("--%s", overwriteFlag)))
	}

	if o.inputFilePath != "" {
		return nil
	}

	if err := o.askForAppName(); err != nil {
		return err
	}
	if err := o.askForSecretName(); err != nil {
		return err
	}
	if err := o.askForSecretValues(); err != nil {
		return err
	}
	return nil
}

// Execute creates or updates the secrets.
func (o *secretInitOpts) Execute() error {
	if o.inputFilePath != "" {
		secrets, err := o.parseSecretsInputFile()
		if err != nil {
			return err
		}

		o.secretValues = secrets

		if err := o.configureClientsAndUpgradeForEnvironments(secrets); err != nil {
			return err
		}

		var errs []*errSecretFailedInSomeEnvironments
		for secretName, secretValues := range secrets {
			if err := o.putSecret(secretName, secretValues); err != nil {
				errs = append(errs, err.(*errSecretFailedInSomeEnvironments))
			}
			log.Infoln("")
		}

		if len(errs) != 0 {
			return &errBatchPutSecretsFailed{
				errors: errs,
			}
		}
		return nil
	}

	o.secretValues = map[string]map[string]string{
		o.name: o.values,
	}
	if err := o.configureClientsAndUpgradeForEnvironments(o.secretValues); err != nil {
		return err
	}
	return o.putSecret(o.name, o.values)
}

func (o *secretInitOpts) configureClientsAndUpgradeForEnvironments(secrets map[string]map[string]string) error {
	envNames := make(map[string]struct{})
	for _, values := range secrets {
		for envName := range values {
			envNames[envName] = struct{}{}
		}
	}

	for envName := range envNames {
		if err := o.configureClientsForEnv(envName); err != nil {
			return err
		}
		if err := validateMinEnvVersion(o.ws, o.envCompatibilityChecker[envName], o.appName, envName, template.SecretInitMinEnvVersion, "secret init"); err != nil {
			return err
		}
	}
	return nil
}

func (o *secretInitOpts) putSecret(secretName string, values map[string]string) error {
	envs := make([]string, 0)
	for env := range values {
		envs = append(envs, env)
	}

	if len(envs) == 0 {
		return nil
	}

	log.Infof("...Put secret %s to environment %s\n", color.HighlightUserInput(secretName), english.WordSeries(envs, "and"))

	errorsForEnvironments := make(map[string]error)
	for envName, value := range values {
		err := o.putSecretInEnv(secretName, envName, value)
		if err != nil {
			errorsForEnvironments[envName] = err
			continue
		}
	}

	for envName := range errorsForEnvironments {
		log.Errorf("Failed to put secret %s in environment %s. See error message below.\n", color.HighlightUserInput(secretName), color.HighlightUserInput(envName))
	}

	if len(errorsForEnvironments) != 0 {
		return &errSecretFailedInSomeEnvironments{
			secretName:            secretName,
			errorsForEnvironments: errorsForEnvironments,
		}
	}

	return nil
}

func (o *secretInitOpts) putSecretInEnv(secretName, envName, value string) error {
	name := fmt.Sprintf(fmtSecretParameterName, o.appName, envName, secretName)
	in := ssm.PutSecretInput{
		Name:      name,
		Value:     value,
		Overwrite: o.overwrite,
		Tags: map[string]string{
			deploy.AppTagKey: o.appName,
			deploy.EnvTagKey: envName,
		},
	}

	out, err := o.secretPutters[envName].PutSecret(in)
	if err != nil {
		var targetErr *ssm.ErrParameterAlreadyExists
		if errors.As(err, &targetErr) {
			o.shouldShowOverwriteHint = true
			log.Successf("Secret %s already exists in environment %s as %s. Did not overwrite. \n", color.HighlightUserInput(secretName), color.HighlightUserInput(envName), color.HighlightResource(name))
			return nil
		}
		return err
	}

	version := aws.Int64Value(out.Version)
	if version != 1 {
		log.Successln(fmt.Sprintf("Secret %s already exists in environment %s. Overwritten.", name, color.HighlightUserInput(envName)))
		return nil
	}

	log.Successln(fmt.Sprintf("Successfully put secret %s in environment %s as %s.", color.HighlightUserInput(secretName), color.HighlightUserInput(envName), color.HighlightResource(name)))
	return nil
}

func (o *secretInitOpts) parseSecretsInputFile() (map[string]map[string]string, error) {
	raw, err := o.readFile()
	if err != nil {
		return nil, err
	}

	type inputFile struct {
		Secrets map[string]map[string]string `yaml:",inline"`
	}
	var f inputFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("unmarshal input file: %w", err)
	}
	return f.Secrets, nil
}

func (o *secretInitOpts) askForAppName() error {
	if o.appName != "" {
		return nil
	}

	app, err := o.selector.Application(secretInitAppPrompt, secretInitAppPromptHelp)
	if err != nil {
		return fmt.Errorf("ask for an application to add the secret to: %w", err)
	}
	o.appName = app
	return nil
}

func (o *secretInitOpts) askForSecretName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.prompter.Get(secretInitSecretNamePrompt,
		secretInitSecretNamePromptHelp,
		validateSecretName,
		prompt.WithFinalMessage("Secret name: "))
	if err != nil {
		return fmt.Errorf("ask for the secret name: %w", err)
	}

	o.name = name
	return nil
}

func (o *secretInitOpts) askForSecretValues() error {
	if o.values != nil {
		return nil
	}

	envs, err := o.store.ListEnvironments(o.appName)
	if err != nil {
		return fmt.Errorf("list environments in app %s: %w", o.appName, err)
	}

	if len(envs) == 0 {
		log.Errorf("Secrets are environment-level resources. Please run %s before running %s.\n",
			color.HighlightCode("copilot env init"),
			color.HighlightCode("copilot secret init"))
		return fmt.Errorf("no environment is found in app %s", o.appName)
	}

	values := make(map[string]string)
	for _, env := range envs {
		value, err := o.prompter.GetSecret(
			fmt.Sprintf(fmtSecretInitSecretValuePrompt, color.HighlightUserInput(o.name), env.Name),
			fmt.Sprintf(fmtSecretInitSecretValuePromptHelp, color.HighlightUserInput(o.name), env.Name),
			prompt.WithFinalMessage(fmt.Sprintf("%s secret value:", cases.Title(language.English).String(env.Name))),
		)
		if err != nil {
			return fmt.Errorf("get secret value for %s in environment %s: %w", color.HighlightUserInput(o.name), env.Name, err)
		}

		if value != "" {
			values[env.Name] = value
		}
	}
	o.values = values
	return nil
}

// RecommendActions shows recommended actions to do after running `secret init`.
func (o *secretInitOpts) RecommendActions() error {
	secretsManifestExample := "secrets:"
	for secretName := range o.secretValues {
		currSecret := fmt.Sprintf("%s: %s", secretName, fmt.Sprintf(fmtSecretParameterNameMftExample, secretName))
		secretsManifestExample = fmt.Sprintf("%s\n%s", secretsManifestExample, fmt.Sprintf("    %s", currSecret))
	}

	log.Infoln("You can refer to these secrets from your manifest file by editing the `secrets` section.")
	log.Infoln(color.HighlightCodeBlock(secretsManifestExample))
	return nil
}

type errSecretFailedInSomeEnvironments struct {
	secretName            string
	errorsForEnvironments map[string]error
}

type errBatchPutSecretsFailed struct {
	errors []*errSecretFailedInSomeEnvironments
}

func (e *errSecretFailedInSomeEnvironments) Error() string {
	// Sort failure messages by environment names.
	var envs []string
	for env := range e.errorsForEnvironments {
		envs = append(envs, env)
	}
	sort.Strings(envs)

	out := make([]string, 0)
	for _, env := range envs {
		out = append(out, fmt.Sprintf("put secret %s in environment %s: %s", e.secretName, env, e.errorsForEnvironments[env].Error()))
	}
	return strings.Join(out, "\n")
}

func (e *errSecretFailedInSomeEnvironments) name() string {
	return e.secretName
}

func (e *errBatchPutSecretsFailed) Error() string {
	out := []string{
		"batch put secrets:",
	}
	sort.SliceStable(e.errors, func(i, j int) bool {
		return e.errors[i].name() < e.errors[j].name()
	})
	for _, err := range e.errors {
		out = append(out, err.Error())
	}
	return strings.Join(out, "\n")
}

func (o *secretInitOpts) targetEnv(envName string) (*config.Environment, error) {
	env, err := o.store.GetEnvironment(o.appName, envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s in application %s: %w", envName, o.appName, err)
	}
	return env, nil
}

// buildSecretInitCmd build the command for creating a new secret or updating an existing one.
func buildSecretInitCmd() *cobra.Command {
	vars := secretInitVars{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create or update secrets in SSM Parameter Store.",
		Example: `
Create a secret with prompts. 
/code $ copilot secret init
Create a secret named db-password in multiple environments.
/code $ copilot secret init --name db-password
Create secrets from input.yml. For the format of the YAML file, please see https://aws.github.io/copilot-cli/docs/commands/secret-init/.
/code $ copilot secret init --cli-input-yaml input.yml`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSecretInitOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}

			err = opts.Execute()
			if opts.shouldShowOverwriteHint {
				log.Warningf("If you want to overwrite an existing secret, use the %s flag.\n", color.HighlightCode(fmt.Sprintf("--%s", overwriteFlag)))
			}
			if err != nil {
				return err
			}

			if err := opts.RecommendActions(); err != nil {
				return err
			}
			return nil
		}),
	}

	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", secretNameFlagDescription)
	cmd.Flags().StringToStringVar(&vars.values, valuesFlag, nil, secretValuesFlagDescription)
	cmd.Flags().BoolVar(&vars.overwrite, overwriteFlag, false, secretOverwriteFlagDescription)
	cmd.Flags().StringVar(&vars.inputFilePath, inputFilePathFlag, "", secretInputFilePathFlagDescription)
	return cmd
}
