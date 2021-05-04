// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/config"
)

type initSecretVars struct {
	appName string

	name          string
	values        map[string]string
	inputFilePath string
	overwrite     bool

	resourceTags map[string]string
}

type secretInitOpts struct {
	initSecretVars

	store store
	fs    afero.Fs
}

func newSecretInitOpts(vars initSecretVars) (*secretInitOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	opts := secretInitOpts{
		initSecretVars: vars,
		store:          store,
		fs:             &afero.Afero{Fs: afero.NewOsFs()},
	}
	return &opts, nil
}

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
		for env, _ := range o.values {
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

func (o *secretInitOpts) Ask() error {
	return nil
}

func (o *secretInitOpts) Execute() error {
	return nil
}

// BuildSecretInitCmd build the command for creating or updating a new secret.
func BuildSecretInitCmd() *cobra.Command {
	vars := initSecretVars{}
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Create or update an SSM SecureString parameter.",
		Example: `secret init`,
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
