// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	pipelineDeleteAppNamePrompt       = "Which application's pipeline would you like to delete?"
	pipelineDeleteAppNameHelpPrompt   = "An application is a collection of related services."
	pipelineDeleteConfirmPrompt       = "Are you sure you want to delete pipeline %s from application %s?"
	pipelineDeleteConfirmHelp         = "This will delete the deployment pipeline for the services in the workspace."
	pipelineSecretDeleteConfirmPrompt = "Are you sure you want to delete the source secret %s associated with pipeline %s?"
	pipelineDeleteSecretConfirmHelp   = "This will delete the token associated with the source of your pipeline."

	fmtPipelineDeletePrompt   = "Which deployed pipeline of application %s would you like to delete?"
	fmtDeletePipelineStart    = "Deleting pipeline %s from application %s."
	fmtDeletePipelineFailed   = "Failed to delete pipeline %s from application %s: %v.\n"
	fmtDeletePipelineComplete = "Deleted pipeline %s from application %s.\n"
)

var (
	errPipelineDeleteCancelled = errors.New("pipeline delete cancelled - no changes made")
)

type deletePipelineVars struct {
	appName            string
	name               string
	skipConfirmation   bool
	shouldDeleteSecret bool
}

type deletePipelineOpts struct {
	deletePipelineVars

	ghAccessTokenSecretName string

	// Interfaces to dependencies
	pipelineDeployer pipelineDeployer
	codepipeline     pipelineGetter
	prog             progress
	sel              codePipelineSelector
	prompt           prompter
	secretsmanager   secretsManager
	ws               wsPipelineGetter
	store            store
}

func newDeletePipelineOpts(vars deletePipelineVars) (*deletePipelineOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	defaultSess, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline delete")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	ssmStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	prompter := prompt.New()
	codepipeline := codepipeline.New(defaultSess)

	opts := &deletePipelineOpts{
		deletePipelineVars: vars,
		codepipeline:       codepipeline,
		prog:               termprogress.NewSpinner(log.DiagnosticWriter),
		prompt:             prompter,
		secretsmanager:     secretsmanager.New(defaultSess),
		pipelineDeployer:   cloudformation.New(defaultSess),
		ws:                 ws,
		store:              ssmStore,
		sel:                selector.NewAppPipelineSelect(prompter, ssmStore, codepipeline),
	}

	return opts, nil
}

// Validate returns an error if the flag values for optional fields are invalid.
func (o *deletePipelineOpts) Validate() error {
	return nil
}

// Ask prompts for and validates required fields.
func (o *deletePipelineOpts) Ask() error {
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return err
		}
	} else {
		if err := o.askAppName(); err != nil {
			return err
		}
	}
	if o.name != "" {
		if _, err := o.codepipeline.GetPipeline(o.name); err != nil {
			return err
		}
	} else {
		pipelineName, err := askDeployedPipelineName(o.sel, o.appName, fmt.Sprintf(fmtPipelineDeletePrompt, color.HighlightUserInput(o.appName)))
		if err != nil {
			return err
		}
		o.name = pipelineName
	}

	if o.skipConfirmation {
		return nil
	}
	deleteConfirmed, err := o.prompt.Confirm(
		fmt.Sprintf(pipelineDeleteConfirmPrompt, o.name, o.appName),
		pipelineDeleteConfirmHelp,
		prompt.WithConfirmFinalMessage())

	if err != nil {
		return fmt.Errorf("pipeline delete confirmation prompt: %w", err)
	}

	if !deleteConfirmed {
		return errPipelineDeleteCancelled
	}
	return nil
}

// Execute deletes the secret and pipeline stack.
func (o *deletePipelineOpts) Execute() error {
	if err := o.getSecret(); err != nil {
		return err
	}
	if err := o.deleteSecret(); err != nil {
		return err
	}

	if err := o.deleteStack(); err != nil {
		return err
	}

	return nil
}

func askDeployedPipelineName(sel codePipelineSelector, appName, msg string) (string, error) {
	pipeline, err := sel.DeployedPipeline(msg, "", map[string]string{
		deploy.AppTagKey: appName,
	})
	if err != nil {
		return "", fmt.Errorf("select deployed pipelines: %w", err)
	}
	return pipeline, nil
}

func (o *deletePipelineOpts) askAppName() error {
	app, err := o.sel.Application(pipelineDeleteAppNamePrompt, pipelineDeleteAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *deletePipelineOpts) getSecret() error {
	// Look for default secret name for GHv1 access token based on default pipeline name.
	o.ghAccessTokenSecretName = o.pipelineSecretName()
	output, err := o.secretsmanager.DescribeSecret(o.ghAccessTokenSecretName)
	if err == nil {
		// Check for Copilot-assigned tag for added assurance. If tags not found, fall through.
		for _, tag := range output.Tags {
			if aws.StringValue(tag.Key) == "copilot-application" && aws.StringValue(tag.Value) == output.CreatedDate.UTC().Format(time.UnixDate) {
				log.Infof("Found secret '%s'.\n", o.ghAccessTokenSecretName)
				return nil
			}
		}
	}
	var notFoundErr *secretsmanager.ErrSecretNotFound
	if err != nil && !errors.As(err, &notFoundErr) {
		return fmt.Errorf("describe secret %s: %w", o.ghAccessTokenSecretName, err)
	}
	// To get here, either the secret was found but tags didn't match, or Secrets Manager returned a ResourceNotFoundException.
	log.Infof("Found no Copilot-generated Secrets Manager secrets to delete for pipeline '%s'.\n", o.name)
	o.ghAccessTokenSecretName = ""
	return nil
}

// With GHv1 sources, we stored access tokens in SecretsManager. Pipelines generated prior to Copilot v1.4 have secrets named 'github-token-[appName]-[repoName]'. Pipelines prior to 1.16(?) were given default names of `pipeline-[appName]-[repoName]`. Users may have changed pipeline names, so this is our best-guess approach to deleting legacy pipeline secrets.
func (o *deletePipelineOpts) pipelineSecretName() string {
	appAndRepo := strings.TrimPrefix(o.name, fmt.Sprintf("pipeline-"))
	return fmt.Sprintf("github-token-%s", appAndRepo)
}

func (o *deletePipelineOpts) deleteSecret() error {
	if o.ghAccessTokenSecretName == "" {
		return nil
	}
	// Only pipelines created with GitHubV1 have personal access tokens saved as secrets.
	if !o.shouldDeleteSecret {
		confirmDeletion, err := o.prompt.Confirm(
			fmt.Sprintf(pipelineSecretDeleteConfirmPrompt, o.ghAccessTokenSecretName, o.name),
			pipelineDeleteSecretConfirmHelp,
		)
		if err != nil {
			return fmt.Errorf("pipeline delete secret confirmation prompt: %w", err)
		}

		if !confirmDeletion {
			log.Infof("Skipping deletion of secret %s.\n", o.ghAccessTokenSecretName)
			return nil
		}
	}

	if err := o.secretsmanager.DeleteSecret(o.ghAccessTokenSecretName); err != nil {
		return err
	}

	log.Successf("Deleted secret %s.\n", o.ghAccessTokenSecretName)

	return nil
}

func (o *deletePipelineOpts) deleteStack() error {
	o.prog.Start(fmt.Sprintf(fmtDeletePipelineStart, o.name, o.appName))
	if err := o.pipelineDeployer.DeletePipeline(o.name); err != nil {
		o.prog.Stop(log.Serrorf(fmtDeletePipelineFailed, o.name, o.appName, err))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtDeletePipelineComplete, o.name, o.appName))
	return nil
}

// RecommendActions is a no-op for this command.
func (o *deletePipelineOpts) RecommendActions() error {
	return nil
}

// buildPipelineDeleteCmd build the command for deleting an existing pipeline.
func buildPipelineDeleteCmd() *cobra.Command {
	vars := deletePipelineVars{}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes the pipeline associated with your workspace.",
		Example: `
  Delete the pipeline associated with your workspace.
  /code $ copilot pipeline delete
`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeletePipelineOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", pipelineFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldDeleteSecret, deleteSecretFlag, false, deleteSecretFlagDescription)
	return cmd
}
