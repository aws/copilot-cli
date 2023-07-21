// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const workloadAskPrompt = "Which workload would you like to run locally?"

type ecsLocalClient interface {
	TaskDefinition(app, env, svc string) (*awsecs.TaskDefinition, error)
	DecryptedSSMSecrets(secrets []*awsecs.ContainerSecret, secretGetter *session.Session) ([]ecs.EnvVar, error)
}

type localRunVars struct {
	wkldName string
	wkldType string
	appName  string
	envName  string
}

type localRunOpts struct {
	localRunVars

	sel            deploySelector
	ecsLocalClient ecsLocalClient
	secretsManager secretsManager
	sess           *session.Session
	store          store
}

func newLocalRunOpts(vars localRunVars) (*localRunOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("local run"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	deployStore, err := deploy.NewStore(sessProvider, store)
	if err != nil {
		return nil, err
	}
	opts := &localRunOpts{
		localRunVars:   vars,
		sel:            selector.NewDeploySelect(prompt.New(), store, deployStore),
		store:          store,
		secretsManager: secretsmanager.New(defaultSess),
		ecsLocalClient: ecs.New(defaultSess),
		sess:           defaultSess,
	}
	return opts, nil
}

// Validate returns an error for any invalid optional flags.
func (o *localRunOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	// Ensure that the application name provided exists in the workspace
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return fmt.Errorf("get application %s: %w", o.appName, err)
	}
	return nil
}

// Ask prompts the user for any unprovided required fields and validates them.
func (o *localRunOpts) Ask() error {
	return o.validateAndAskWkldEnvName()
}

func (o *localRunOpts) validateAndAskWkldEnvName() error {
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
			return err
		}
	}
	if o.wkldName != "" {
		if _, err := o.store.GetWorkload(o.appName, o.wkldName); err != nil {
			return err
		}
	}

	deployedWorkload, err := o.sel.DeployedWorkload(workloadAskPrompt, "", o.appName, selector.WithEnv(o.envName), selector.WithName(o.wkldName))
	if err != nil {
		return fmt.Errorf("select a deployed workload from application %s: %w", o.appName, err)
	}
	o.wkldName = deployedWorkload.Name
	o.envName = deployedWorkload.Env
	o.wkldType = deployedWorkload.Type
	return nil
}

// Execute builds and runs the workload images locally.
func (o *localRunOpts) Execute() error {
	taskDef, err := o.ecsLocalClient.TaskDefinition(o.appName, o.envName, o.wkldName)
	if err != nil {
		return fmt.Errorf("get task definition: %w", err)
	}

	secrets := taskDef.Secrets()
	_, err = o.ecsLocalClient.DecryptedSSMSecrets(secrets, o.sess)
	if err != nil {
		return err
	}

	_, err = o.decryptedSecretManagerSecrets(secrets, o.sess)
	if err != nil {
		return err
	}

	return nil
}

func (o *localRunOpts) decryptedSecretManagerSecrets(secrets []*awsecs.ContainerSecret, awsSession *session.Session) ([]ecs.EnvVar, error) {
	var secretManagerSecrets []ecs.EnvVar
	for _, secret := range secrets {
		if strings.HasPrefix(secret.ValueFrom, "arn:aws:secretsmanager:") {
			secretValue, err := o.secretsManager.GetSecretValue(secret.ValueFrom)
			if err != nil {
				return nil, err
			}
			secretManagerSecrets = append(secretManagerSecrets, ecs.EnvVar{
				Name:  secret.Name,
				Value: secretValue,
			})
		}
	}
	return secretManagerSecrets, nil
}

// BuildLocalRunCmd builds the command for running a workload locally
func BuildLocalRunCmd() *cobra.Command {
	vars := localRunVars{}
	cmd := &cobra.Command{
		Use:    "local run",
		Short:  "Run the workload locally",
		Long:   "Run the workload locally",
		Hidden: true,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newLocalRunOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.wkldName, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	return cmd
}
