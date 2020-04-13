// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/s3"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	awssession "github.com/aws/aws-sdk-go/aws/session"

	"github.com/spf13/cobra"
)

const (
	defaultProfile = "default"

	fmtConfirmProjectDeletePrompt = `Are you sure you want to delete project %s?
	This will delete your project as well as any apps, environments, and pipelines.`
	confirmProjectDeleteHelp       = "Deleting a project will remove all associated resources. (apps, envs, pipelines, etc.)"
	cleanResourcesStartMsg         = "Cleaning up deployment resources."
	cleanResourcesStopMsg          = "Cleaned up deployment resources."
	deleteProjectResourcesStartMsg = "Deleting project resources."
	deleteProjectResourcesStopMsg  = "Deleted project resources."
	deleteProjectParamsStartMsg    = "Deleting project metadata."
	deleteProjectParamsStopMsg     = "Deleted project metadata."
	deleteLocalWsStartMsg          = "Deleting local workspace folder."
	deleteLocalWsStopMsg           = "Deleted local workspace folder."
)

var (
	errOperationCancelled = errors.New("operation cancelled")
)

type deleteProjVars struct {
	skipConfirmation bool
	envProfiles      map[string]string
	*GlobalOpts
}

type deleteProjOpts struct {
	deleteProjVars
	spinner progress

	store                        projectService
	ws                           workspaceDeleter
	sessProvider                 sessionProvider
	deployer                     deployer
	getBucketEmptier             func(session *awssession.Session) bucketEmptier
	executorProvider             func(appName string) (executor, error)
	askExecutorProvider          func(envName, envProfile string) (askExecutor, error)
	deletePipelineRunnerProvider func() (deletePipelineRunner, error)
}

func newDeleteProjOpts(vars deleteProjVars) (*deleteProjOpts, error) {
	store, err := store.New()
	if err != nil {
		return nil, err
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}

	provider := session.NewProvider()
	defaultSession, err := provider.Default()
	if err != nil {
		return nil, err
	}

	cf := cloudformation.New(defaultSession)

	return &deleteProjOpts{
		deleteProjVars: vars,
		spinner:        termprogress.NewSpinner(),
		store:          store,
		ws:             ws,
		sessProvider:   provider,
		deployer:       cf,
		getBucketEmptier: func(session *awssession.Session) bucketEmptier {
			return s3.New(session)
		},
		executorProvider: func(appName string) (executor, error) {
			vars := deleteAppVars{
				SkipConfirmation: true, // always skip sub-confirmations
				GlobalOpts:       NewGlobalOpts(),
				AppName:          appName,
			}

			deleteAppOpts, err := newDeleteAppOpts(vars)
			if err != nil {
				return nil, err
			}

			return deleteAppOpts, nil
		},
		askExecutorProvider: func(envName, envProfile string) (askExecutor, error) {
			vars := deleteEnvVars{
				SkipConfirmation: true,
				GlobalOpts:       NewGlobalOpts(),
				EnvName:          envName,
			}

			deleteEnvOpts, err := newDeleteEnvOpts(vars)
			deleteEnvOpts.EnvProfile = envProfile

			if err != nil {
				return nil, err
			}

			return deleteEnvOpts, nil
		},
		deletePipelineRunnerProvider: func() (deletePipelineRunner, error) {
			vars := deletePipelineVars{
				GlobalOpts:       NewGlobalOpts(),
				SkipConfirmation: true,
				DeleteSecret:     true,
			}

			deletePipelineOpts, err := newDeletePipelineOpts(vars)
			if err != nil {
				return nil, err
			}

			return deletePipelineOpts, nil
		},
	}, nil
}

func (o *deleteProjOpts) Validate() error {
	if o.ProjectName() == "" {
		return errNoProjectInWorkspace
	}

	return nil
}

func (o *deleteProjOpts) Ask() error {
	if o.skipConfirmation {
		return nil
	}

	manualConfirm, err := o.prompt.Confirm(
		fmt.Sprintf(fmtConfirmProjectDeletePrompt, o.ProjectName()),
		confirmProjectDeleteHelp,
		prompt.WithTrueDefault())

	if err != nil {
		return err
	}

	if !manualConfirm {
		return errOperationCancelled
	}

	return nil
}

func (o *deleteProjOpts) Execute() error {
	if err := o.deleteApps(); err != nil {
		return err
	}

	if err := o.deleteEnvs(); err != nil {
		return err
	}

	if err := o.emptyS3Bucket(); err != nil {
		return err
	}

	// deleteProjectPipeline must happen before deleteProjectResources and
	// deleteLocalWorkspace, since the pipeline delete command relies on the
	// project stackset as well as the workspace directory to still exist.
	if err := o.deleteProjectPipeline(); err != nil {
		if !errors.Is(err, workspace.ErrNoPipelineInWorkspace) {
			return err
		}
	}

	if err := o.deleteProjectResources(); err != nil {
		return err
	}

	if err := o.deleteProjectParams(); err != nil {
		return err
	}

	if err := o.deleteLocalWorkspace(); err != nil {
		return err
	}

	return nil
}

func (o *deleteProjOpts) deleteApps() error {
	apps, err := o.store.ListApplications(o.ProjectName())
	if err != nil {
		return err
	}

	for _, a := range apps {
		deleter, err := o.executorProvider(a.Name)
		if err != nil {
			return err
		}
		if err := deleter.Execute(); err != nil {
			return err
		}
	}

	return nil
}

func (o *deleteProjOpts) deleteEnvs() error {
	envs, err := o.store.ListEnvironments(o.ProjectName())
	if err != nil {
		return err
	}

	for _, e := range envs {
		// Check to see if a profile was passed in for this environment
		// for deletion - otherwise it will be passed as an empty
		// string, which triggers env delete's ask.
		envProfile, _ := o.envProfiles[e.Name]

		deleter, err := o.askExecutorProvider(e.Name, envProfile)
		if err != nil {
			return err
		}
		if err := deleter.Ask(); err != nil {
			return err
		}

		if err := deleter.Execute(); err != nil {
			return err
		}
	}

	return nil
}

func (o *deleteProjOpts) emptyS3Bucket() error {
	proj, err := o.store.GetProject(o.ProjectName())
	if err != nil {
		return fmt.Errorf("get project %s: %w", o.ProjectName(), err)
	}
	projResources, err := o.deployer.GetRegionalProjectResources(proj)
	if err != nil {
		return fmt.Errorf("get regional resources for %s: %w", proj.Name, err)
	}
	o.spinner.Start(cleanResourcesStartMsg)
	for _, projResource := range projResources {
		sess, err := o.sessProvider.DefaultWithRegion(projResource.Region)
		if err != nil {
			return err
		}

		s3Client := o.getBucketEmptier(sess)

		if err := s3Client.EmptyBucket(projResource.S3Bucket); err != nil {
			o.spinner.Stop(log.Serror("Error cleaning up deployment resources."))
			return fmt.Errorf("empty bucket %s: %w", projResource.S3Bucket, err)
		}
	}
	o.spinner.Stop(log.Ssuccess(cleanResourcesStopMsg))
	return nil
}

func (o *deleteProjOpts) deleteProjectPipeline() error {
	deleter, err := o.deletePipelineRunnerProvider()
	if err != nil {
		return err
	}

	return deleter.Run()
}

func (o *deleteProjOpts) deleteProjectResources() error {
	o.spinner.Start(deleteProjectResourcesStartMsg)
	if err := o.deployer.DeleteProject(o.ProjectName()); err != nil {
		o.spinner.Stop(log.Serror("Error deleting project resources."))
		return fmt.Errorf("delete project resources: %w", err)
	}
	o.spinner.Stop(log.Ssuccess(deleteProjectResourcesStopMsg))

	return nil
}

func (o *deleteProjOpts) deleteProjectParams() error {
	o.spinner.Start(deleteProjectParamsStartMsg)
	if err := o.store.DeleteProject(o.ProjectName()); err != nil {
		o.spinner.Stop(log.Serror("Error deleting project metadata."))

		return err
	}
	o.spinner.Stop(log.Ssuccess(deleteProjectParamsStopMsg))

	return nil
}

func (o *deleteProjOpts) deleteLocalWorkspace() error {
	o.spinner.Start(deleteLocalWsStartMsg)
	if err := o.ws.DeleteAll(); err != nil {
		o.spinner.Stop(log.Serror("Error deleting local workspace folder."))

		return fmt.Errorf("delete workspace: %w", err)
	}
	o.spinner.Stop(log.Ssuccess(deleteLocalWsStopMsg))

	return nil
}

// BuildProjectDeleteCommand builds the `project delete` subcommand.
func BuildProjectDeleteCommand() *cobra.Command {
	vars := deleteProjVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete all resources associated with the local project.",
		Example: `
  /code $ ecs-preview project delete --yes --env-profiles test=default,prod=prod-profile`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteProjOpts(vars)
			if err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			if err := opts.Ask(); err != nil {
				return err
			}

			return opts.Execute()
		}),
	}

	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	cmd.Flags().StringToStringVar(&vars.envProfiles, envProfilesFlag, nil, envProfilesFlagDescription)

	return cmd
}
