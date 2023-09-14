// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	awscfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/iam"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	stackdescr "github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	envDeleteAppNameHelpPrompt = "An environment will be deleted in the selected application."
	envDeleteNamePrompt        = "Which environment would you like to delete?"
	fmtDeleteEnvPrompt         = "Are you sure you want to delete environment %q from application %q?"
)

const (
	fmtRetainEnvRolesStart    = "Retain IAM roles before deleting the %q environment"
	fmtRetainEnvRolesFailed   = "Failed to retain IAM roles for the %q environment\n"
	fmtRetainEnvRolesComplete = "Retained IAM roles for the %q environment\n"

	fmtDeleteEnvStart     = "Deleting IAM roles and deregistering environment %q from application %q."
	fmtDeleteEnvIAMFailed = "Failed to delete IAM roles of environment %q from application %q.\n"
	fmtDeleteEnvSSMFailed = "Failed to deregister environment %q from application %q.\n"
	fmtDeleteEnvComplete  = "Deleted environment %q from application %q.\n"
)

var (
	envDeleteAppNamePrompt = fmt.Sprintf("In which %s would you like to delete the environment?", color.Emphasize("application"))
)

var (
	errEnvDeleteCancelled = errors.New("env delete cancelled - no changes made")
)

type resourceGetter interface {
	GetResources(*resourcegroupstaggingapi.GetResourcesInput) (*resourcegroupstaggingapi.GetResourcesOutput, error)
}

type deleteEnvVars struct {
	appName          string
	name             string
	skipConfirmation bool
}

type deleteEnvOpts struct {
	deleteEnvVars

	// Interfaces for dependencies.
	store                  environmentStore
	rg                     resourceGetter
	deployer               environmentDeployer
	envDeleterFromApp      envDeleterFromApp
	iam                    roleDeleter
	s3                     bucketEmptier
	envStackDescriber      stackDescriber
	deployedPipelineLister deployedPipelineLister
	pipelineGetter         pipelineGetter
	prog                   progress
	prompt                 prompter
	sel                    configSelector

	// cached data to avoid fetching the same information multiple times.
	envConfig *config.Environment
	appConfig *config.Application

	// initRuntimeClients is overridden in tests.
	initRuntimeClients func(*deleteEnvOpts) error
}

func newDeleteEnvOpts(vars deleteEnvVars) (*deleteEnvOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("env delete"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}
	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))

	prompter := prompt.New()
	return &deleteEnvOpts{
		deleteEnvVars: vars,

		store:  store,
		prog:   termprogress.NewSpinner(log.DiagnosticWriter),
		sel:    selector.NewConfigSelector(prompter, store),
		prompt: prompter,

		initRuntimeClients: func(o *deleteEnvOpts) error {
			env, err := o.getEnvConfig()
			if err != nil {
				return err
			}
			sess, err := sessProvider.FromRole(env.ManagerRoleARN, env.Region)
			if err != nil {
				return fmt.Errorf("create session from environment manager role %s in region %s: %w", env.ManagerRoleARN, env.Region, err)
			}
			o.rg = resourcegroupstaggingapi.New(sess)
			o.iam = iam.New(sess)
			o.s3 = s3.New(sess)
			o.envStackDescriber = stackdescr.NewStackDescriber(stack.NameForEnv(o.appName, o.name), sess)
			o.deployer = cloudformation.New(sess, cloudformation.WithProgressTracker(os.Stderr))
			o.envDeleterFromApp = cloudformation.New(defaultSess, cloudformation.WithProgressTracker(os.Stderr))
			o.pipelineGetter = codepipeline.New(defaultSess)
			o.deployedPipelineLister = deploy.NewPipelineStore(rg.New(defaultSess))
			return nil
		},
	}, nil
}

// Validate returns an error if the individual user inputs are invalid.
func (o *deleteEnvOpts) Validate() error {
	if o.name != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *deleteEnvOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	if o.skipConfirmation {
		return nil
	}
	deleteConfirmed, err := o.prompt.Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, o.name, o.appName), "", prompt.WithConfirmFinalMessage())
	if err != nil {
		return fmt.Errorf("confirm to delete environment %s: %w", o.name, err)
	}
	if !deleteConfirmed {
		return errEnvDeleteCancelled
	}
	return nil
}

// Execute deletes the environment from the application by:
// 1. Emptying environment managed S3 buckets.
// 2. Deleting the cloudformation stack.
// 3. Deleting the EnvManagerRole and CFNExecutionRole.
// 4. Deleting the parameter from the SSM store.
// The environment is removed from the store only if other delete operations succeed.
// Execute assumes that Validate is invoked first.
func (o *deleteEnvOpts) Execute() error {
	if err := o.initRuntimeClients(o); err != nil {
		return err
	}
	if err := o.validateNoRunningServices(); err != nil {
		return err
	}

	if err := o.validateNoDependencyPipelines(); err != nil {
		return err
	}

	o.prog.Start(fmt.Sprintf(fmtRetainEnvRolesStart, o.name))
	if err := o.ensureRolesAreRetained(); err != nil {
		o.prog.Stop(log.Serrorf(fmtRetainEnvRolesFailed, o.name))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtRetainEnvRolesComplete, o.name))

	// EmptyBuckets checks for env managed s3 buckets and makes a best-effort attempt to delete them.
	if err := o.emptyBuckets(); err != nil {
		// Handle empty bucket error and recommend action, don't exit program. Otherwise swallow error and move on.
		var emptyBucketErr *errBucketEmptyingFailed
		if errors.As(err, &emptyBucketErr) {
			log.Errorln(emptyBucketErr.Error())
			log.Warningln(emptyBucketErr.RecommendActions())
		}
	}

	// DeleteStack streams the deletion events; we don't need a spinner over top of it.
	if err := o.deleteStack(); err != nil {
		return err
	}

	// Un-delegate DNS and optionally delete stackset instance.
	o.prog.Start("Cleaning up app-level resources and permissions\n")
	if err := o.cleanUpAppResources(); err != nil {
		o.prog.Stop(log.Serrorf("Failed to remove environment resources from app %q\n", o.appName))
		return err
	}
	o.prog.Stop(log.Ssuccessf("Cleaned up app-level resources for the %q environment\n", o.name))

	o.prog.Start(fmt.Sprintf(fmtDeleteEnvStart, o.name, o.appName))
	if err := o.tryDeleteRoles(); err != nil {
		o.prog.Stop(log.Serrorf(fmtDeleteEnvIAMFailed, o.name, o.appName))
		return err
	}
	// Only remove from SSM if the stack and roles were deleted. Otherwise, the command will error when re-run.
	if err := o.deleteFromStore(); err != nil {
		o.prog.Stop(log.Serrorf(fmtDeleteEnvSSMFailed, o.name, o.appName))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtDeleteEnvComplete, o.name, o.appName))
	return nil
}

// RecommendActions is a no-op for this command.
func (o *deleteEnvOpts) RecommendActions() error {
	return nil
}

func (o *deleteEnvOpts) validateEnvName() error {
	if _, err := o.getEnvConfig(); err != nil {
		return err
	}
	return nil
}

func (o *deleteEnvOpts) askAppName() error {
	if o.appName != "" {
		return nil
	}

	app, err := o.sel.Application(envDeleteAppNamePrompt, envDeleteAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("ask for application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *deleteEnvOpts) askEnvName() error {
	if o.name != "" {
		return nil
	}
	env, err := o.sel.Environment(envDeleteNamePrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment to delete: %w", err)
	}
	o.name = env
	return nil
}

func (o *deleteEnvOpts) validateNoRunningServices() error {
	stacks, err := o.rg.GetResources(&resourcegroupstaggingapi.GetResourcesInput{
		ResourceTypeFilters: []*string{aws.String("cloudformation")},
		TagFilters: []*resourcegroupstaggingapi.TagFilter{
			{
				Key:    aws.String(deploy.ServiceTagKey),
				Values: []*string{}, // Matches any service stack.
			},
			{
				Key:    aws.String(deploy.EnvTagKey),
				Values: []*string{aws.String(o.name)},
			},
			{
				Key:    aws.String(deploy.AppTagKey),
				Values: []*string{aws.String(o.appName)},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("find service cloudformation stacks: %w", err)
	}
	if len(stacks.ResourceTagMappingList) > 0 {
		var svcNames []string
		for _, cfnStack := range stacks.ResourceTagMappingList {
			for _, t := range cfnStack.Tags {
				if *t.Key != deploy.ServiceTagKey {
					continue
				}
				svcNames = append(svcNames, *t.Value)
			}
		}
		return fmt.Errorf("service %q still exist within the environment %s", strings.Join(svcNames, ", "), o.name)
	}
	return nil
}

func (o *deleteEnvOpts) validateNoDependencyPipelines() error {
	pipelines, err := o.deployedPipelineLister.ListDeployedPipelines(o.appName)
	if err != nil {
		return fmt.Errorf("list deployed pipelines: %w", err)
	}
	for _, pipeline := range pipelines {
		info, err := o.pipelineGetter.GetPipeline(pipeline.ResourceName)
		if err != nil {
			return fmt.Errorf("get pipeline %s: %w", pipeline.ResourceName, err)
		}
		for _, stage := range info.Stages {
			if strings.TrimPrefix(stage.Name, deploy.StageFullNamePrefix) == o.name {
				return &errPipelineDependsOnEnv{
					pipeline: pipeline.Name,
					env:      o.name,
				}
			}
		}
	}
	return nil
}

// ensureRolesAreRetained guarantees that the CloudformationExecutionRole and the EnvironmentManagerRole
// are retained when the environment cloudformation stack is deleted.
//
// This method is needed because the environment stack is deleted using the CloudformationExecutionRole which means
// that the role has to be retained in order for us to delete the stack. Similarly, only the EnvironmentManagerRole
// has permissions to delete the CloudformationExecutionRole so it must be retained.
// In earlier versions of the CLI, pre-commit 7e5428a, environment stacks were created without these roles retained.
// In case we encounter a legacy stack, we need to first update the stack to make sure these roles are retained and then
// proceed with the regular flow.
func (o *deleteEnvOpts) ensureRolesAreRetained() error {
	body, err := o.deployer.Template(stack.NameForEnv(o.appName, o.name))
	if err != nil {
		var stackDoesNotExist *awscfn.ErrStackNotFound
		if errors.As(err, &stackDoesNotExist) {
			return nil
		}
		return fmt.Errorf("get template body for environment %s in application %s: %v", o.name, o.appName, err)
	}

	// Check if the execution role and the manager role are retained by the stack.
	tpl := struct {
		Resources yaml.Node `yaml:"Resources"`
	}{}
	if err := yaml.Unmarshal([]byte(body), &tpl); err != nil {
		return fmt.Errorf("unmarshal environment template body %s-%s to retrieve Resources: %w", o.appName, o.name, err)
	}
	roles := struct {
		ExecRole    yaml.Node `yaml:"CloudformationExecutionRole"`
		ManagerRole yaml.Node `yaml:"EnvironmentManagerRole"`
	}{}
	if err := tpl.Resources.Decode(&roles); err != nil {
		return fmt.Errorf("decode EnvironmentManagerRole and CloudformationExecutionRole from Resources: %w", err)
	}
	type roleProperties struct {
		DeletionPolicy string `yaml:"DeletionPolicy"`
	}
	var execRoleProps roleProperties
	if err := roles.ExecRole.Decode(&execRoleProps); err != nil {
		return fmt.Errorf("decode CloudformationExecutionRole's deletion policy: %w", err)
	}
	var managerRoleProps roleProperties
	if err := roles.ManagerRole.Decode(&managerRoleProps); err != nil {
		return fmt.Errorf("decode EnvironmentManagerRole's deletion policy: %w", err)
	}
	const retainPolicy = "Retain"
	retainsExecRole := execRoleProps.DeletionPolicy == retainPolicy
	retainsManagerRole := managerRoleProps.DeletionPolicy == retainPolicy
	if retainsExecRole && retainsManagerRole {
		// Nothing to do, this is **not** a legacy environment stack. Exit successfully.
		return nil
	}

	// Otherwise, update the body with the new deletion policies.
	newBody := body
	if !retainsExecRole {
		parts := strings.Split(newBody, "  CloudformationExecutionRole:\n")
		newBody = parts[0] + "  CloudformationExecutionRole:\n    DeletionPolicy: Retain\n" + parts[1]
	}
	if !retainsManagerRole {
		parts := strings.Split(newBody, "  EnvironmentManagerRole:\n")
		newBody = parts[0] + "  EnvironmentManagerRole:\n    DeletionPolicy: Retain\n" + parts[1]
	}

	env, err := o.getEnvConfig()
	if err != nil {
		return err
	}
	if err := o.deployer.UpdateEnvironmentTemplate(o.appName, o.name, newBody, env.ExecutionRoleARN); err != nil {
		return fmt.Errorf("update environment stack to retain environment roles: %w", err)
	}
	return nil
}

// emptyBuckets returns nil if buckets were deleted successfully. Otherwise, returns the error.
func (o *deleteEnvOpts) emptyBuckets() error {
	s3buckets, err := o.rg.GetResources(&resourcegroupstaggingapi.GetResourcesInput{
		ResourceTypeFilters: aws.StringSlice([]string{"s3:bucket"}),
		TagFilters: []*resourcegroupstaggingapi.TagFilter{
			{
				Key:    aws.String(stack.StackNameTagKey),
				Values: []*string{aws.String(stack.NameForEnv(o.appName, o.name))},
			},
			{
				Key:    aws.String(stack.LogicalIDTagKey),
				Values: aws.StringSlice(stack.EnvManagedS3BucketLogicalIds),
			},
			{
				Key:    aws.String(deploy.EnvTagKey),
				Values: []*string{aws.String(o.name)},
			},
			{
				Key:    aws.String(deploy.AppTagKey),
				Values: []*string{aws.String(o.appName)},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("find s3 bucket resources: %w", err)
	}

	envResources, err := o.envStackDescriber.Resources()
	if err != nil {
		return fmt.Errorf("find stack resources: %w", err)
	}

	var stackBucketARNs []string
	for _, resource := range envResources {
		if slices.Contains(stack.EnvManagedS3BucketLogicalIds, resource.LogicalID) {
			stackBucketARNs = append(stackBucketARNs, resource.PhysicalID)
		}
	}

	var failedBuckets []string
	var bucketErrors []error
	for _, resourceTagMapping := range s3buckets.ResourceTagMappingList {
		bucketARN, err := arn.Parse(aws.StringValue(resourceTagMapping.ResourceARN))
		if err != nil {
			return fmt.Errorf("parse the arn %s: %w", aws.StringValue(resourceTagMapping.ResourceARN), err)
		}

		// Attempt to empty all buckets found via GetResources API call
		if err = o.s3.EmptyBucket(bucketARN.Resource); err != nil {
			failedBuckets = append(failedBuckets, bucketARN.Resource)
			bucketErrors = append(bucketErrors, err)
			continue
		}

		// Warn about hanging bucket when bucket is found via API call but is not in the env CFN stack
		// Those found via API call but not CFN stack cannot be deleted by Copilot
		if !slices.Contains(stackBucketARNs, bucketARN.String()) {
			log.Warningf(`Bucket %q was emptied, but was not found in the Cloudformation stack. This resource is now dangling, and can be deleted from the S3 console.\n`, bucketARN)
		}
	}

	if len(failedBuckets) > 0 {
		return &errBucketEmptyingFailed{
			failedBuckets: failedBuckets,
			bucketErrors:  bucketErrors,
		}
	}

	return nil
}

// deleteStack returns nil if the stack was deleted successfully. Otherwise, returns the error.
func (o *deleteEnvOpts) deleteStack() error {
	env, err := o.getEnvConfig()
	if err != nil {
		return err
	}
	if err := o.deployer.DeleteEnvironment(o.appName, o.name, env.ExecutionRoleARN); err != nil {
		return fmt.Errorf("delete environment %s stack: %w", o.name, err)
	}
	return nil
}

func (o *deleteEnvOpts) cleanUpAppResources() error {
	// Get list of environments and check if there are any other environments in this account OR region.
	envs, err := o.store.ListEnvironments(o.appName)
	if err != nil {
		return err
	}
	currentEnv, err := o.getEnvConfig()
	if err != nil {
		return err
	}
	app, err := o.getAppConfig()
	if err != nil {
		return err
	}

	if err := o.envDeleterFromApp.RemoveEnvFromApp(&cloudformation.RemoveEnvFromAppOpts{
		App:          app,
		EnvToDelete:  currentEnv,
		Environments: envs,
	}); err != nil {
		return fmt.Errorf("remove environment %s from application %s: %w", currentEnv.Name, app.Name, err)
	}
	return nil
}

// tryDeleteRoles attempts to delete the retained IAM roles part of an environment stack.
// The operation is best-effort because of the ManagerRole. Since the iam client is created with a
// session that assumes the ManagerRole, attempting to delete the same role can result in the following error:
// "AccessDenied: User: arn:aws:sts::1111:assumed-role/app-env-EnvManagerRole is not authorized to perform:
// iam:DeleteRole on resource: role app-env-EnvManagerRole"
// This error occurs because to delete a role you have to first remove all of its policies, so the role loses
// permission to delete itself and then attempts to delete itself. We think that due to eventual consistency this
// operation succeeds most of the time but on occasions we have observed it to fail.
func (o *deleteEnvOpts) tryDeleteRoles() error {
	env, err := o.getEnvConfig()
	if err != nil {
		return err
	}
	_ = o.iam.DeleteRole(env.ExecutionRoleARN)
	_ = o.iam.DeleteRole(env.ManagerRoleARN)
	return nil
}

func (o *deleteEnvOpts) deleteFromStore() error {
	if err := o.store.DeleteEnvironment(o.appName, o.name); err != nil {
		return fmt.Errorf("delete environment %s configuration from application %s", o.name, o.appName)
	}
	return nil
}

func (o *deleteEnvOpts) getEnvConfig() (*config.Environment, error) {
	if o.envConfig != nil {
		// Already fetched once, return.
		return o.envConfig, nil
	}
	env, err := o.store.GetEnvironment(o.appName, o.name)
	if err != nil {
		return nil, fmt.Errorf("get environment %s configuration from app %s: %v", o.name, o.appName, err)
	}
	o.envConfig = env
	return env, nil
}

func (o *deleteEnvOpts) getAppConfig() (*config.Application, error) {
	if o.appConfig != nil {
		// Already fetched; return.
		return o.appConfig, nil
	}
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return nil, fmt.Errorf("get application %q configuration: %w", o.appName, err)
	}
	o.appConfig = app
	return app, nil
}

// buildEnvDeleteCmd builds the command to delete environment(s).
func buildEnvDeleteCmd() *cobra.Command {
	vars := deleteEnvVars{}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes an environment from your application.",
		Example: `
  Delete the "test" environment.
  /code $ copilot env delete --name test

  Delete the "test" environment without prompting.
  /code $ copilot env delete --name test --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteEnvOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
