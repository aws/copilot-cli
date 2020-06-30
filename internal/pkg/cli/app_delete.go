// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	awssession "github.com/aws/aws-sdk-go/aws/session"

	"github.com/spf13/cobra"
)

const (
	fmtDeleteAppConfirmPrompt = "Are you sure you want to delete application %s?"
	deleteAppConfirmHelp      = "This will delete all resources in your application: including services, environments, and pipelines."

	deleteAppCleanResourcesStartMsg = "Cleaning up deployment resources."
	deleteAppCleanResourcesStopMsg  = "Cleaned up deployment resources."

	deleteAppResourcesStartMsg = "Deleting application resources."
	deleteAppResourcesStopMsg  = "Deleted application resources."

	deleteAppConfigStartMsg = "Deleting application configuration."
	deleteAppConfigStopMsg  = "Deleted application configuration."

	fmtDeleteAppWsStartMsg = "Deleting local %s file."
	fmtDeleteAppWsStopMsg  = "Deleted local %s file."
)

var (
	errOperationCancelled = errors.New("operation cancelled")
)

type deleteAppVars struct {
	skipConfirmation bool
	envProfiles      map[string]string
	*GlobalOpts
}

type deleteAppOpts struct {
	deleteAppVars
	spinner progress

	store                store
	ws                   wsFileDeleter
	sessProvider         sessionProvider
	cfn                  deployer
	s3                   func(session *awssession.Session) bucketEmptier
	executor             func(svcName string) (executor, error)
	askExecutor          func(envName, envProfile string) (askExecutor, error)
	deletePipelineRunner func() (deletePipelineRunner, error)
}

func newDeleteAppOpts(vars deleteAppVars) (*deleteAppOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}

	provider := session.NewProvider()
	defaultSession, err := provider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}

	return &deleteAppOpts{
		deleteAppVars: vars,
		spinner:       termprogress.NewSpinner(),
		store:         store,
		ws:            ws,
		sessProvider:  provider,
		cfn:           cloudformation.New(defaultSession),
		s3: func(session *awssession.Session) bucketEmptier {
			return s3.New(session)
		},
		executor: func(svcName string) (executor, error) {
			opts, err := newDeleteSvcOpts(deleteSvcVars{
				SkipConfirmation: true, // always skip sub-confirmations
				GlobalOpts:       NewGlobalOpts(),
				Name:             svcName,
			})
			if err != nil {
				return nil, err
			}
			return opts, nil
		},
		askExecutor: func(envName, envProfile string) (askExecutor, error) {
			opts, err := newDeleteEnvOpts(deleteEnvVars{
				SkipConfirmation: true,
				GlobalOpts:       NewGlobalOpts(),
				EnvName:          envName,
				EnvProfile:       envProfile,
			})
			if err != nil {
				return nil, err
			}
			return opts, nil
		},
		deletePipelineRunner: func() (deletePipelineRunner, error) {
			opts, err := newDeletePipelineOpts(deletePipelineVars{
				GlobalOpts:       NewGlobalOpts(),
				SkipConfirmation: true,
				DeleteSecret:     true,
			})
			if err != nil {
				return nil, err
			}
			return opts, nil
		},
	}, nil
}

// Validate returns an error if the user's input is invalid.
func (o *deleteAppOpts) Validate() error {
	if o.AppName() == "" {
		return errNoAppInWorkspace
	}
	return nil
}

// Ask prompts the user for any required flags that they didn't provide.
func (o *deleteAppOpts) Ask() error {
	if o.skipConfirmation {
		return nil
	}

	manualConfirm, err := o.prompt.Confirm(
		fmt.Sprintf(fmtDeleteAppConfirmPrompt, o.AppName()),
		deleteAppConfirmHelp,
		prompt.WithTrueDefault())
	if err != nil {
		return fmt.Errorf("confirm app deletion: %w", err)
	}
	if !manualConfirm {
		return errOperationCancelled
	}
	return nil
}

// Execute deletes the application.
// It removes all the services from each environment, the environments, the pipeline S3 buckets,
// the pipeline, the application, removes the variables from the config store, and deletes the local workspace.
func (o *deleteAppOpts) Execute() error {
	if err := o.deleteSvcs(); err != nil {
		return err
	}

	if err := o.deleteEnvs(); err != nil {
		return err
	}

	if err := o.emptyS3Bucket(); err != nil {
		return err
	}

	// deletePipeline must happen before deleteAppResources and deleteWs, since the pipeline delete command relies
	// on the application stackset as well as the workspace directory to still exist.
	if err := o.deletePipeline(); err != nil {
		if !errors.Is(err, workspace.ErrNoPipelineInWorkspace) {
			return err
		}
	}

	if err := o.deleteAppResources(); err != nil {
		return err
	}

	if err := o.deleteAppConfigs(); err != nil {
		return err
	}

	if err := o.deleteWs(); err != nil {
		return err
	}

	return nil
}

func (o *deleteAppOpts) deleteSvcs() error {
	svcs, err := o.store.ListServices(o.AppName())
	if err != nil {
		return fmt.Errorf("list services for application %s: %w", o.AppName(), err)
	}

	for _, svc := range svcs {
		cmd, err := o.executor(svc.Name)
		if err != nil {
			return err
		}
		if err := cmd.Execute(); err != nil {
			return fmt.Errorf("execute svc delete: %w", err)
		}
	}
	return nil
}

func (o *deleteAppOpts) deleteEnvs() error {
	envs, err := o.store.ListEnvironments(o.AppName())
	if err != nil {
		return fmt.Errorf("list environments for application %s: %w", o.AppName(), err)
	}

	for _, env := range envs {
		// Check to see if a profile was passed in for this environment
		// for deletion - otherwise it will be passed as an empty
		// string, which triggers env delete's ask.
		profile := o.envProfiles[env.Name]

		cmd, err := o.askExecutor(env.Name, profile)
		if err != nil {
			return err
		}
		if err := cmd.Ask(); err != nil {
			return fmt.Errorf("ask env delete: %w", err)
		}
		if err := cmd.Execute(); err != nil {
			return fmt.Errorf("execute env delete: %w", err)
		}
	}
	return nil
}

func (o *deleteAppOpts) emptyS3Bucket() error {
	app, err := o.store.GetApplication(o.AppName())
	if err != nil {
		return fmt.Errorf("get application %s: %w", o.AppName(), err)
	}
	appResources, err := o.cfn.GetRegionalAppResources(app)
	if err != nil {
		return fmt.Errorf("get regional application resources for %s: %w", app.Name, err)
	}
	o.spinner.Start(deleteAppCleanResourcesStartMsg)
	for _, resource := range appResources {
		sess, err := o.sessProvider.DefaultWithRegion(resource.Region)
		if err != nil {
			return fmt.Errorf("default session with region %s: %w", resource.Region, err)
		}

		// Empty pipeline buckets.
		s3Client := o.s3(sess)
		if err := s3Client.EmptyBucket(resource.S3Bucket); err != nil {
			o.spinner.Stop(log.Serror("Error cleaning up deployment resources."))
			return fmt.Errorf("empty bucket %s: %w", resource.S3Bucket, err)
		}
	}
	o.spinner.Stop(log.Ssuccess(deleteAppCleanResourcesStopMsg))
	return nil
}

func (o *deleteAppOpts) deletePipeline() error {
	cmd, err := o.deletePipelineRunner()
	if err != nil {
		return err
	}
	return cmd.Run()
}

func (o *deleteAppOpts) deleteAppResources() error {
	o.spinner.Start(deleteAppResourcesStartMsg)
	if err := o.cfn.DeleteApp(o.AppName()); err != nil {
		o.spinner.Stop(log.Serror("Error deleting application resources."))
		return fmt.Errorf("delete app resources: %w", err)
	}
	o.spinner.Stop(log.Ssuccess(deleteAppResourcesStopMsg))
	return nil
}

func (o *deleteAppOpts) deleteAppConfigs() error {
	o.spinner.Start(deleteAppConfigStartMsg)
	if err := o.store.DeleteApplication(o.AppName()); err != nil {
		o.spinner.Stop(log.Serror("Error deleting application configuration."))
		return fmt.Errorf("delete application %s configuration: %w", o.AppName(), err)
	}
	o.spinner.Stop(log.Ssuccess(deleteAppConfigStopMsg))
	return nil
}

func (o *deleteAppOpts) deleteWs() error {
	o.spinner.Start(fmt.Sprintf(fmtDeleteAppWsStartMsg, workspace.SummaryFileName))
	if err := o.ws.DeleteWorkspaceFile(); err != nil {
		o.spinner.Stop(log.Serrorf("Error deleting %s file.", workspace.SummaryFileName))
		return fmt.Errorf("delete %s file: %w", workspace.SummaryFileName, err)
	}
	o.spinner.Stop(log.Ssuccess(fmt.Sprintf(fmtDeleteAppWsStopMsg, workspace.SummaryFileName)))
	return nil
}

// BuildAppDeleteCommand builds the `app delete` subcommand.
func BuildAppDeleteCommand() *cobra.Command {
	vars := deleteAppVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete all resources associated with the application.",
		Example: `
  Force delete the application with environments "test" and "prod".
  /code $ copilot app delete --yes --env-profiles test=default,prod=prod-profile`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteAppOpts(vars)
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
