// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/spf13/cobra"
)

const (
	fmtDeleteAppConfirmPrompt = "Are you sure you want to delete application %s?"
	deleteAppConfirmHelp      = "This will delete all resources in your application: including services, environments, and pipelines."

	deleteAppCleanResourcesStartMsg = "Cleaning up deployment resources."
	deleteAppCleanResourcesStopMsg  = "Cleaned up deployment resources.\n"

	deleteAppResourcesStartMsg = "Deleting application resources."
	deleteAppResourcesStopMsg  = "Deleted application resources.\n"

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

	store                store
	ws                   wsFileDeleter
	sessProvider         sessionProvider
	prompt               prompter
	cfn                  func(session *session.Session) deployer
	s3                   func(session *session.Session) bucketEmptier
	ecr                  func(sess *session.Session) imageRemover
	svcDeleteExecutor    func(svcName string) (executor, error)
	jobDeleteExecutor    func(jobName string) (executor, error)
	envDeleteExecutor    func(envName string) (executeAsker, error)
	deletePipelineRunner func() (deletePipelineRunner, error)

	envs []*config.Environment // Used to minimize code duplication between task delete and env delete.
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

	provider := sessions.NewProvider()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}

	return &deleteAppOpts{
		deleteAppVars: vars,
		spinner:       termprogress.NewSpinner(),
		store:         store,
		ws:            ws,
		sessProvider:  provider,
		prompt:        prompt.New(),
		cfn: func(session *session.Session) deployer {
			return cloudformation.New(session)
		},
		s3: func(session *session.Session) bucketEmptier {
			return s3.New(session)
		},
		ecr: func(sess *session.Session) imageRemover {
			return ecr.New(sess)
		},
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
		deletePipelineRunner: func() (deletePipelineRunner, error) {
			opts, err := newDeletePipelineOpts(deletePipelineVars{
				appName:            vars.name,
				skipConfirmation:   true,
				shouldDeleteSecret: true,
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
	if o.name == "" {
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
		fmt.Sprintf(fmtDeleteAppConfirmPrompt, o.name),
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

	if err := o.deleteJobs(); err != nil {
		return err
	}

	if err := o.deleteTasks(); err != nil {
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

func (o *deleteAppOpts) deleteTasks() error {

	envs, err := o.store.ListEnvironments(o.name)
	if err != nil {
		return fmt.Errorf("list environments for application %s: %w", o.name, err)
	}
	o.envs = envs

	// Delete tasks from each environment (that is, delete the tasks that were created with each environment's manager role)
	var envTasks []deploy.TaskStackInfo
	for _, env := range envs {
		envSess, err := o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}
		envCF := o.cfn(envSess)
		envECR := o.ecr(envSess)
		envTasks, err = envCF.GetTaskStackInfo(o.name, env.Name)
		if err != nil {
			return fmt.Errorf("get tasks deployed in environment %s: %w", env.Name, err)
		}
		for _, t := range envTasks {
			o.spinner.Start(fmt.Sprintf("Deleting task %s from environment %s.", t.TaskName(), env.Name))
			if err := envECR.ClearRepository(t.ECRRepoName()); err != nil {
				o.spinner.Stop(log.Serrorf("Error emptying ECR repository for task %s\n", t.TaskName()))
				return fmt.Errorf("empty ECR repository for task %s: %w", t.TaskName(), err)
			}
			if err := envCF.DeleteTask(t); err != nil {
				o.spinner.Stop(log.Serrorf("Error deleting task %s from environment %s.\n", t.TaskName(), t.Env))
				return fmt.Errorf("delete task %s from env %s: %w", t.TaskName(), t.Env, err)
			}
			o.spinner.Stop(log.Ssuccessf("Deleted task %s from environment %s.\n", t.TaskName(), t.Env))
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

// Should be called after deleteTasks; that's where envs are populated
func (o *deleteAppOpts) deleteEnvs() error {
	for _, env := range o.envs {
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
	sess, err := o.sessProvider.Default()
	if err != nil {
		return err
	}
	cfn := o.cfn(sess)
	appResources, err := cfn.GetRegionalAppResources(app)
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

func (o *deleteAppOpts) deletePipeline() error {
	cmd, err := o.deletePipelineRunner()
	if err != nil {
		return err
	}
	return cmd.Run()
}

func (o *deleteAppOpts) deleteAppResources() error {
	sess, err := o.sessProvider.Default()
	if err != nil {
		return err
	}
	cfn := o.cfn(sess)
	o.spinner.Start(deleteAppResourcesStartMsg)
	if err := cfn.DeleteApp(o.name); err != nil {
		o.spinner.Stop(log.Serrorln("Error deleting application resources."))
		return fmt.Errorf("delete app resources: %w", err)
	}
	o.spinner.Stop(log.Ssuccess(deleteAppResourcesStopMsg))
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
	o.spinner.Start(fmt.Sprintf(fmtDeleteAppWsStartMsg, workspace.SummaryFileName))
	if err := o.ws.DeleteWorkspaceFile(); err != nil {
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

			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}

	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
