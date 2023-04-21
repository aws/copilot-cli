// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	fmtDeleteAppConfirmPrompt = "Are you sure you want to delete application %s?"
	deleteAppConfirmHelp      = "This will delete all resources in your application: including services, environments, and pipelines."

	deleteAppCleanResourcesStartMsg = "Cleaning up deployment resources."
	deleteAppCleanResourcesStopMsg  = "Cleaned up deployment resources.\n"

	deleteAppConfigStartMsg = "Deleting application configuration."
	deleteAppConfigStopMsg  = "Deleted application configuration.\n"

	fmtDeleteAppWsStartMsg = "Deleting local %s file."
	fmtDeleteAppWsStopMsg  = "Deleted local %s file.\n"
)

var (
	errOperationCancelled = errors.New("operation cancelled")
)

type deleteAppVars struct {
	name             string
	skipConfirmation bool
}

type deleteAppOpts struct {
	deleteAppVars
	spinner progress

	store                  store
	sessProvider           sessionProvider
	cfn                    deployer
	prompt                 prompter
	pipelineLister         deployedPipelineLister
	s3                     func(session *session.Session) bucketEmptier
	svcDeleteExecutor      func(svcName string) (executor, error)
	jobDeleteExecutor      func(jobName string) (executor, error)
	envDeleteExecutor      func(envName string) (executeAsker, error)
	taskDeleteExecutor     func(envName, taskName string) (executor, error)
	pipelineDeleteExecutor func(pipelineName string) (executor, error)
	ws                     func(fs afero.Fs) (wsFileDeleter, error)
}

func newDeleteAppOpts(vars deleteAppVars) (*deleteAppOpts, error) {
	provider := sessions.ImmutableProvider(sessions.UserAgentExtras("app delete"))
	defaultSession, err := provider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}

	return &deleteAppOpts{
		deleteAppVars: vars,
		spinner:       termprogress.NewSpinner(log.DiagnosticWriter),
		store:         config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region)),
		sessProvider:  provider,
		cfn:           cloudformation.New(defaultSession, cloudformation.WithProgressTracker(os.Stderr)),
		prompt:        prompt.New(),
		s3: func(session *session.Session) bucketEmptier {
			return s3.New(session)
		},
		pipelineLister: deploy.NewPipelineStore(rg.New(defaultSession)),
		svcDeleteExecutor: func(svcName string) (executor, error) {
			opts, err := newDeleteSvcOpts(deleteSvcVars{
				skipConfirmation: true, // always skip sub-confirmations
				name:             svcName,
				appName:          vars.name,
			})
			if err != nil {
				return nil, err
			}
			return opts, nil
		},
		jobDeleteExecutor: func(jobName string) (executor, error) {
			opts, err := newDeleteJobOpts(deleteJobVars{
				skipConfirmation: true,
				name:             jobName,
				appName:          vars.name,
			})
			if err != nil {
				return nil, err
			}
			return opts, nil
		},
		envDeleteExecutor: func(envName string) (executeAsker, error) {
			opts, err := newDeleteEnvOpts(deleteEnvVars{
				skipConfirmation: true,
				appName:          vars.name,
				name:             envName,
			})
			if err != nil {
				return nil, err
			}
			return opts, nil
		},
		taskDeleteExecutor: func(envName, taskName string) (executor, error) {
			opts, err := newDeleteTaskOpts(deleteTaskVars{
				app:              vars.name,
				env:              envName,
				name:             taskName,
				skipConfirmation: true,
			})
			if err != nil {
				return nil, err
			}
			return opts, nil
		},
		pipelineDeleteExecutor: func(pipelineName string) (executor, error) {
			opts, err := newDeletePipelineOpts(deletePipelineVars{
				appName:            vars.name,
				name:               pipelineName,
				skipConfirmation:   true,
				shouldDeleteSecret: true,
			})
			if err != nil {
				return nil, err
			}
			return opts, nil
		},
		ws: func(fs afero.Fs) (wsFileDeleter, error) {
			return workspace.Use(fs)
		},
	}, nil
}

// Validate is a no-op for this command.
func (o *deleteAppOpts) Validate() error {
	return nil
}

// Ask prompts the user for any required flags that they didn't provide.
func (o *deleteAppOpts) Ask() error {
	if o.skipConfirmation {
		return nil
	}

	manualConfirm, err := o.prompt.Confirm(
		fmt.Sprintf(fmtDeleteAppConfirmPrompt, o.name),
		deleteAppConfirmHelp,
		prompt.WithTrueDefault(),
		prompt.WithConfirmFinalMessage())
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

	if err := o.deleteJobs(); err != nil {
		return err
	}

	if err := o.deleteEnvs(); err != nil {
		return err
	}

	if err := o.emptyS3Bucket(); err != nil {
		return err
	}

	// deletePipelines must happen before deleteAppResources and deleteWs, since the pipeline delete command relies
	// on the application stackset as well as the workspace directory to still exist.
	if err := o.deletePipelines(); err != nil {
		return err
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
	svcs, err := o.store.ListServices(o.name)
	if err != nil {
		return fmt.Errorf("list services for application %s: %w", o.name, err)
	}

	for _, svc := range svcs {
		cmd, err := o.svcDeleteExecutor(svc.Name)
		if err != nil {
			return err
		}
		if err := cmd.Execute(); err != nil {
			return fmt.Errorf("execute svc delete: %w", err)
		}
	}
	return nil
}

func (o *deleteAppOpts) deleteJobs() error {
	jobs, err := o.store.ListJobs(o.name)
	if err != nil {
		return fmt.Errorf("list jobs for application %s: %w", o.name, err)
	}

	for _, job := range jobs {
		cmd, err := o.jobDeleteExecutor(job.Name)
		if err != nil {
			return err
		}
		if err := cmd.Execute(); err != nil {
			return fmt.Errorf("execute job delete: %w", err)
		}
	}
	return nil
}

func (o *deleteAppOpts) deleteEnvs() error {
	envs, err := o.store.ListEnvironments(o.name)
	if err != nil {
		return fmt.Errorf("list environments for application %s: %w", o.name, err)
	}

	for _, env := range envs {
		// Delete tasks from each environment.
		tasks, err := o.cfn.ListTaskStacks(o.name, env.Name)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			taskCmd, err := o.taskDeleteExecutor(env.Name, task.TaskName())
			if err != nil {
				return err
			}
			if err := taskCmd.Execute(); err != nil {
				return fmt.Errorf("execute task delete")
			}
		}

		cmd, err := o.envDeleteExecutor(env.Name)
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
	app, err := o.store.GetApplication(o.name)
	if err != nil {
		return fmt.Errorf("get application %s: %w", o.name, err)
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
			o.spinner.Stop(log.Serrorln("Error cleaning up deployment resources."))
			return fmt.Errorf("empty bucket %s: %w", resource.S3Bucket, err)
		}
	}
	o.spinner.Stop(log.Ssuccess(deleteAppCleanResourcesStopMsg))
	return nil
}

func (o *deleteAppOpts) deletePipelines() error {
	pipelines, err := o.pipelineLister.ListDeployedPipelines(o.name)
	if err != nil {
		return fmt.Errorf("list pipelines for application %s: %w", o.name, err)
	}

	for _, pipeline := range pipelines {
		cmd, err := o.pipelineDeleteExecutor(pipeline.Name)
		if err != nil {
			return err
		}
		if err := cmd.Execute(); err != nil {
			return fmt.Errorf("execute pipeline delete: %w", err)
		}
	}
	return nil
}

func (o *deleteAppOpts) deleteAppResources() error {
	if err := o.cfn.DeleteApp(o.name); err != nil {
		return fmt.Errorf("delete app resources: %w", err)
	}
	return nil
}

func (o *deleteAppOpts) deleteAppConfigs() error {
	o.spinner.Start(deleteAppConfigStartMsg)
	if err := o.store.DeleteApplication(o.name); err != nil {
		o.spinner.Stop(log.Serrorln("Error deleting application configuration."))
		return fmt.Errorf("delete application %s configuration: %w", o.name, err)
	}
	o.spinner.Stop(log.Ssuccess(deleteAppConfigStopMsg))
	return nil
}

func (o *deleteAppOpts) deleteWs() error {
	ws, err := o.ws(afero.NewOsFs())
	if err != nil {
		return err
	}
	o.spinner.Start(fmt.Sprintf(fmtDeleteAppWsStartMsg, workspace.SummaryFileName))
	if err := ws.DeleteWorkspaceFile(); err != nil {
		o.spinner.Stop(log.Serrorf("Error deleting %s file.\n", workspace.SummaryFileName))
		return fmt.Errorf("delete %s file: %w", workspace.SummaryFileName, err)
	}
	o.spinner.Stop(log.Ssuccessf(fmt.Sprintf(fmtDeleteAppWsStopMsg, workspace.SummaryFileName)))
	return nil
}

// buildAppDeleteCommand builds the `app delete` subcommand.
func buildAppDeleteCommand() *cobra.Command {
	vars := deleteAppVars{}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete all resources associated with the application.",
		Example: `
  Force delete the application with environments "test" and "prod".
  /code $ copilot app delete --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteAppOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}

	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
