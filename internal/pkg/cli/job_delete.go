// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/copilot-cli/internal/pkg/ecs"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/spf13/cobra"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
)

const (
	fmtJobDeleteConfirmPrompt        = "Are you sure you want to delete job %s from application %s?"
	fmtJobDeleteFromEnvConfirmPrompt = "Are you sure you want to delete job %s from environment %s?"
	jobDeleteAppNamePrompt           = "Which application's job would you like to delete?"
	jobDeleteJobNamePrompt           = "Which job would you like to delete?"
	jobDeleteConfirmHelp             = "This will remove the job from all environments and delete it from your app."
	fmtJobDeleteFromEnvConfirmHelp   = "This will remove the job from just the %s environment."
)

const (
	fmtJobTasksStopStart    = "Stopping running tasks of job %s from environment %s."
	fmtJobTasksStopFailed   = "Failed to stop running tasks of job %s from environment %s: %v.\n"
	fmtJobTasksStopComplete = "Stopped running tasks of job %s from environment %s.\n"
)

var (
	errJobDeleteCancelled = errors.New("job delete cancelled - no changes made")
)

type deleteJobVars struct {
	appName          string
	skipConfirmation bool
	name             string
	envName          string
}

type deleteJobOpts struct {
	deleteJobVars

	// Interfaces to dependencies.
	store           store
	prompt          prompter
	sel             configSelector
	sess            sessionProvider
	spinner         progress
	appCFN          jobRemoverFromApp
	newWlDeleter    func(sess *session.Session) wlDeleter
	newImageRemover func(sess *session.Session) imageRemover
	newTaskStopper  func(sess *session.Session) taskStopper
}

func newDeleteJobOpts(vars deleteJobVars) (*deleteJobOpts, error) {
	provider := sessions.ImmutableProvider(sessions.UserAgentExtras("job delete"))
	defaultSession, err := provider.Default()
	if err != nil {
		return nil, err
	}
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	return &deleteJobOpts{
		deleteJobVars: vars,

		store:   store,
		spinner: termprogress.NewSpinner(log.DiagnosticWriter),
		prompt:  prompt.New(),
		sel:     selector.NewConfigSelector(prompter, store),
		sess:    provider,
		appCFN:  cloudformation.New(defaultSession, cloudformation.WithProgressTracker(os.Stderr)),
		newWlDeleter: func(session *session.Session) wlDeleter {
			return cloudformation.New(session, cloudformation.WithProgressTracker(os.Stderr))
		},
		newImageRemover: func(session *session.Session) imageRemover {
			return ecr.New(session)
		},
		newTaskStopper: func(session *session.Session) taskStopper {
			return ecs.New(session)
		},
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deleteJobOpts) Validate() error {
	if o.name != "" {
		if _, err := o.store.GetJob(o.appName, o.name); err != nil {
			return err
		}
	}
	if o.envName != "" {
		return o.validateEnvName()
	}
	return nil
}

// Ask prompts the user for any required flags.
func (o *deleteJobOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askJobName(); err != nil {
		return err
	}

	if o.skipConfirmation {
		return nil
	}

	// When there's no env name passed in, we'll completely
	// remove the job from the application.
	deletePrompt := fmt.Sprintf(fmtJobDeleteConfirmPrompt, o.name, o.appName)
	deleteConfirmHelp := jobDeleteConfirmHelp
	if o.envName != "" {
		// When a customer provides a particular environment,
		// we'll just delete the job from that environment -
		// but keep it in the app.
		deletePrompt = fmt.Sprintf(fmtJobDeleteFromEnvConfirmPrompt, o.name, o.envName)
		deleteConfirmHelp = fmt.Sprintf(fmtJobDeleteFromEnvConfirmHelp, o.envName)
	}

	deleteConfirmed, err := o.prompt.Confirm(
		deletePrompt,
		deleteConfirmHelp,
		prompt.WithConfirmFinalMessage())

	if err != nil {
		return fmt.Errorf("job delete confirmation prompt: %w", err)
	}
	if !deleteConfirmed {
		return errJobDeleteCancelled
	}
	return nil
}

// Execute deletes the job's CloudFormation stack.
// If the job is being removed from the application, Execute will
// also delete the ECR repository and the SSM parameter.
func (o *deleteJobOpts) Execute() error {
	envs, err := o.appEnvironments()
	if err != nil {
		return err
	}

	if err := o.deleteJobs(envs); err != nil {
		return err
	}

	// Skip removing the job from the application if
	// we are only removing the stack from a particular environment.
	if !o.needsAppCleanup() {
		return nil
	}

	if err := o.emptyECRRepos(envs); err != nil {
		return err
	}
	if err := o.removeJobFromApp(); err != nil {
		return err
	}
	if err := o.deleteSSMParam(); err != nil {
		return err
	}

	log.Successf("Deleted job %s from application %s.\n", o.name, o.appName)

	return nil
}

func (o *deleteJobOpts) validateEnvName() error {
	if _, err := o.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (o *deleteJobOpts) targetEnv() (*config.Environment, error) {
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s from config store: %w", o.envName, err)
	}
	return env, nil
}

func (o *deleteJobOpts) askAppName() error {
	if o.appName != "" {
		return nil
	}

	name, err := o.sel.Application(jobDeleteAppNamePrompt, "")
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = name
	return nil
}

func (o *deleteJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Job(jobDeleteJobNamePrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select job: %w", err)
	}
	o.name = name
	return nil
}

func (o *deleteJobOpts) appEnvironments() ([]*config.Environment, error) {
	var envs []*config.Environment
	var err error
	if o.envName != "" {
		env, err := o.targetEnv()
		if err != nil {
			return nil, err
		}
		envs = append(envs, env)
	} else {
		envs, err = o.store.ListEnvironments(o.appName)
		if err != nil {
			return nil, fmt.Errorf("list environments: %w", err)
		}
	}
	return envs, nil
}

func (o *deleteJobOpts) deleteJobs(envs []*config.Environment) error {
	for _, env := range envs {
		sess, err := o.sess.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}
		// Delete job stack
		if err = o.deleteStack(sess, env); err != nil {
			return err
		}
		// Delete orphan tasks
		if err = o.deleteTasks(sess, env.Name); err != nil {
			return err
		}
	}
	return nil
}

func (o *deleteJobOpts) deleteStack(sess *session.Session, env *config.Environment) error {
	cfClient := o.newWlDeleter(sess)
	if err := cfClient.DeleteWorkload(deploy.DeleteWorkloadInput{
		Name:             o.name,
		EnvName:          env.Name,
		AppName:          o.appName,
		ExecutionRoleARN: env.ExecutionRoleARN,
	}); err != nil {
		return fmt.Errorf("delete job stack: %w", err)
	}
	return nil
}

func (o *deleteJobOpts) deleteTasks(sess *session.Session, env string) error {
	o.spinner.Start(fmt.Sprintf(fmtJobTasksStopStart, o.name, env))
	if err := o.newTaskStopper(sess).StopWorkloadTasks(o.appName, env, o.name); err != nil {
		o.spinner.Stop(log.Serrorf(fmtJobTasksStopFailed, o.name, env, err))
		return fmt.Errorf("stop tasks for environment %s: %w", env, err)
	}
	o.spinner.Stop(log.Ssuccessf(fmtJobTasksStopComplete, o.name, env))
	return nil
}

func (o *deleteJobOpts) needsAppCleanup() bool {
	// Only remove a job from the app if
	// we're removing it from every environment.
	// If we're just removing the job from one
	// env, we keep the app configuration.
	return o.envName == ""
}

// This is to make mocking easier in unit tests
func (o *deleteJobOpts) emptyECRRepos(envs []*config.Environment) error {
	var uniqueRegions []string
	for _, env := range envs {
		if !slices.Contains(uniqueRegions, env.Region) {
			uniqueRegions = append(uniqueRegions, env.Region)
		}
	}

	repoName := clideploy.RepoName(o.appName, o.name)
	for _, region := range uniqueRegions {
		sess, err := o.sess.DefaultWithRegion(region)
		if err != nil {
			return err
		}
		client := o.newImageRemover(sess)
		if err := client.ClearRepository(repoName); err != nil {
			return err
		}
	}
	return nil
}

func (o *deleteJobOpts) removeJobFromApp() error {
	proj, err := o.store.GetApplication(o.appName)
	if err != nil {
		return err
	}

	if err := o.appCFN.RemoveJobFromApp(proj, o.name); err != nil {
		if !isStackSetNotExistsErr(err) {
			return err
		}
	}
	return nil
}

func (o *deleteJobOpts) deleteSSMParam() error {
	if err := o.store.DeleteJob(o.appName, o.name); err != nil {
		return fmt.Errorf("delete job %s in application %s from config store: %w", o.name, o.appName, err)
	}

	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *deleteJobOpts) RecommendActions() error {
	logRecommendedActions([]string{
		fmt.Sprintf("Run %s to update the corresponding pipeline if it exists.",
			color.HighlightCode("copilot pipeline deploy")),
	})
	return nil
}

// buildJobDeleteCmd builds the command to delete job(s).
func buildJobDeleteCmd() *cobra.Command {
	vars := deleteJobVars{}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes a job from an application.",
		Example: `
  Delete the "report-generator" job from the my-app application.
  /code $ copilot job delete --name report-generator --app my-app

  Delete the "report-generator" job from just the prod environment.
  /code $ copilot job delete --name report-generator --env prod

  Delete the "report-generator" job from the my-app application from outside of the workspace.
  /code $ copilot job delete --name report-generator --app my-app

  Delete the "report-generator" job without confirmation prompt.
  /code $ copilot job delete --name report-generator --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteJobOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}

	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
