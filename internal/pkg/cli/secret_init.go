// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/term/color"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
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

	resourceTags map[string]string
}

type secretInitOpts struct {
	secretInitVars

	store store
	fs    afero.Fs

	prompter prompter
	selector appSelector
}

func newSecretInitOpts(vars secretInitVars) (*secretInitOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	prompter := prompt.New()
	opts := secretInitOpts{
		secretInitVars: vars,
		store:          store,
		fs:             &afero.Afero{Fs: afero.NewOsFs()},

		prompter: prompter,
		selector: selector.NewSelect(prompter, store),
	}
	return &opts, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *secretInitOpts) Validate() error {
	if o.appName != "" {
		_, err := o.store.GetApplication(o.appName)
		if err != nil {
			return fmt.Errorf("get application %s: %w", o.appName, err)
		}
	}

	if o.name != "" {
		if err := validateSecretName(o.name); err != nil {
			return err
		}
	}

	if o.values != nil {
		for env := range o.values {
			if _, err := o.store.GetEnvironment(o.appName, env); err != nil {
				return fmt.Errorf("get environment %s in application %s: %w", env, o.appName, err)
			}
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
		log.Infof("You have specified %s flag. Please note that overwriting an existing secret may break your deployed service.\n", color.HighlightCode("--overwrite"))
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

// Execute creates the secrets.
func (o *secretInitOpts) Execute() error {
	return nil
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
		prompt.WithFinalMessage("secret name: "))
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
		log.Errorf("Secrets are environment-level resource. Please run %s before running %s.\n",
			color.HighlightCode("copilot env init"),
			color.HighlightCode("copilot secret init"))
		return fmt.Errorf("no environment is found in app %s", o.appName)
	}

	values := make(map[string]string)
	for _, env := range envs {
		value, err := o.prompter.GetSecret(
			fmt.Sprintf(fmtSecretInitSecretValuePrompt, color.HighlightUserInput(o.name), env.Name),
			fmt.Sprintf(fmtSecretInitSecretValuePromptHelp, color.HighlightUserInput(o.name), env.Name))
		if err != nil {
			return fmt.Errorf("get secret value for %s in environment %s: %w", color.HighlightUserInput(o.name), env.Name, err)
		}

		values[env.Name] = value
	}
	o.values = values
	return nil
}

// BuildSecretInitCmd build the command for creating a new secret or updating an existing one.
func BuildSecretInitCmd() *cobra.Command {
	vars := secretInitVars{}
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Create or update an SSM SecureString parameter.",
		Example: ``, // TODO
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
			if err := opts.Execute(); err != nil {
				return err
			}
			return nil
		}),
	}

	cmd.Flags().StringVar(&vars.appName, appFlag, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.name, nameFlag, "", secretNameFlagDescription)
	cmd.Flags().StringToStringVar(&vars.values, valuesFlag, nil, secretValuesFlagDescription)
	cmd.Flags().BoolVar(&vars.overwrite, overwriteFlag, false, secretOverwriteFlagDescription)
	cmd.Flags().StringVar(&vars.inputFilePath, inputFilePathFlag, "", secretInputFilePathFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	return cmd
}
