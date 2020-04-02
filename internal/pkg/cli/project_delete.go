// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/profile"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/s3"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	"github.com/spf13/cobra"
)

const (
	defaultProfile = "default"

	fmtConfirmProjectDeletePrompt = `Are you sure you want to delete project %s?
	This will delete your project as well as any apps, environments, and pipelines.`
	confirmProjectDeleteHelp = "Deleting a project will remove all associated resources. (apps, envs, pipelines, etc.)"
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
	store               projectService
	deployer            deployer
	projResourcesGetter projectResourcesGetter
	s3Client            bucketEmptier
	initBucketEmptySvc  func(*deleteProjOpts, string) error
	ws                  workspaceDeleter
	spinner             progress
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

	s, err := session.NewProvider().Default()
	if err != nil {
		return nil, err
	}
	cf := cloudformation.New(s)

	return &deleteProjOpts{
		deleteProjVars:      vars,
		store:               store,
		ws:                  ws,
		deployer:            cf,
		projResourcesGetter: cf,
		initBucketEmptySvc: func(o *deleteProjOpts, region string) error {
			sess, err := session.NewProvider().DefaultWithRegion(region)
			if err != nil {
				return err
			}
			o.s3Client = s3.New(sess)
			return nil
		},
		spinner: termprogress.NewSpinner(),
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
		return err
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
		ado, err := newDeleteAppOpts(deleteAppVars{
			GlobalOpts: NewGlobalOpts(),
		})
		if err != nil {
			return err
		}
		ado.AppName = a.Name
		ado.SkipConfirmation = true // always skip sub-confirmations

		if err := ado.Execute(); err != nil {
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

	// TODO: move this dependency configuration into newDeleteEnvOpts() function.
	cfg, err := profile.NewConfig()
	if err != nil {
		return err
	}

	for _, e := range envs {
		vars := deleteEnvVars{
			GlobalOpts:       NewGlobalOpts(),
			EnvName:          e.Name,
			SkipConfirmation: true,
		}

		deo, err := newDeleteEnvOpts(vars)
		if err != nil {
			return err
		}
		deo.profileConfig = cfg
		deo.storeClient = o.store
		// Check to see if a profile was passed in for this environment
		// for deletion - otherwise we won't set it, which triggers
		// env delete's ask.
		if envProfile, ok := o.envProfiles[e.Name]; ok {
			deo.EnvProfile = envProfile
		}

		if err := deo.Ask(); err != nil {
			return err
		}

		if err := deo.Execute(); err != nil {
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
	projResources, err := o.projResourcesGetter.GetRegionalProjectResources(proj)
	if err != nil {
		return fmt.Errorf("get regional resources for %s: %w", proj.Name, err)
	}
	o.spinner.Start("Cleaning up deployment resources.")
	for _, projResource := range projResources {
		o.initBucketEmptySvc(o, projResource.Region)
		if err := o.s3Client.EmptyBucket(projResource.S3Bucket); err != nil {
			o.spinner.Stop(log.Serror("Error cleaning up deployment resources."))
			return fmt.Errorf("empty bucket %s: %w", projResource.S3Bucket, err)
		}
	}
	o.spinner.Stop(log.Ssuccess("Cleaned up deployment resources."))
	return nil
}

func (o *deleteProjOpts) deleteProjectResources() error {
	o.spinner.Start("Deleting project resources.")
	if err := o.deployer.DeleteProject(o.ProjectName()); err != nil {
		o.spinner.Stop(log.Serror("Error deleting project resources."))
		return fmt.Errorf("delete project resources: %w", err)
	}
	o.spinner.Stop(log.Ssuccess("Deleted project resources."))

	return nil
}

func (o *deleteProjOpts) deleteProjectParams() error {
	o.spinner.Start("Deleting project metadata.")
	if err := o.store.DeleteProject(o.ProjectName()); err != nil {
		o.spinner.Stop(log.Serror("Error deleting project metadata."))

		return err
	}
	o.spinner.Stop(log.Ssuccess("Deleted project metadata."))

	return nil
}

func (o *deleteProjOpts) deleteLocalWorkspace() error {
	o.spinner.Start("Deleting local workspace folder.")
	if err := o.ws.DeleteAll(); err != nil {
		o.spinner.Stop(log.Serror("Error deleting local workspace folder."))

		return fmt.Errorf("delete workspace: %w", err)
	}
	o.spinner.Stop(log.Ssuccess("Deleted local workspace folder."))

	return nil
}

func (o *deleteProjOpts) deleteProjectPipeline() error {
	vars := deletePipelineVars{
		GlobalOpts:       NewGlobalOpts(),
		SkipConfirmation: true,
		DeleteSecret:     true,
	}

	deletePipelineOpts, err := newDeletePipelineOpts(vars)
	if err != nil {
		return err
	}

	return deletePipelineOpts.Run()
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
