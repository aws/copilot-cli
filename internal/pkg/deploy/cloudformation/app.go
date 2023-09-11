// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/aws/copilot-cli/internal/pkg/version"

	"golang.org/x/sync/errgroup"

	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"

	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	sdkcloudformationiface "github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

type errNoRegionalResources struct {
	appName string
	region  string
}

func (e *errNoRegionalResources) Error() string {
	return fmt.Sprintf("no regional resources for application %s in region %s found", e.appName, e.region)
}

// DeployApp sets up everything required for our application-wide resources.
// These resources include things that are regional, rather than scoped to a particular
// environment, such as ECR Repos, CodePipeline KMS keys & S3 buckets.
// We deploy application resources through StackSets - that way we can have one
// template that we update and all regional stacks are updated.
func (cf CloudFormation) DeployApp(in *deploy.CreateAppInput) error {
	appConfig := stack.NewAppStackConfig(in)
	s, err := toStack(appConfig)
	if err != nil {
		return err
	}

	if err := cf.executeAndRenderChangeSet(cf.newCreateChangeSetInput(cf.console, s)); err != nil {
		var alreadyExists *cloudformation.ErrStackAlreadyExists
		if !errors.As(err, &alreadyExists) {
			return err
		}
	}

	blankAppTemplate, err := appConfig.ResourceTemplate(&stack.AppResourcesConfig{
		App: appConfig.Name,
	})
	if err != nil {
		return err
	}
	stackSetAdminRoleARN, err := appConfig.StackSetAdminRoleARN(cf.region)
	if err != nil {
		return fmt.Errorf("get stack set administrator role arn: %w", err)
	}
	return cf.appStackSet.Create(appConfig.StackSetName(), blankAppTemplate,
		stackset.WithDescription(appConfig.StackSetDescription()),
		stackset.WithExecutionRoleName(appConfig.StackSetExecutionRoleName()),
		stackset.WithAdministrationRoleARN(stackSetAdminRoleARN),
		stackset.WithTags(toMap(appConfig.Tags())))
}

// UpgradeApplication upgrades the application stack to the latest version.
func (cf CloudFormation) UpgradeApplication(in *deploy.CreateAppInput) error {
	appConfig := stack.NewAppStackConfig(in)
	appStack, err := cf.cfnClient.Describe(appConfig.StackName())
	if err != nil {
		return fmt.Errorf("get existing application infrastructure stack: %w", err)
	}
	in.DNSDelegationAccounts = stack.DNSDelegatedAccountsForStack(appStack.SDK())
	in.AdditionalTags = toMap(appStack.Tags)
	appConfig = stack.NewAppStackConfig(in)
	if err := cf.upgradeAppStack(appConfig); err != nil {
		var empty *cloudformation.ErrChangeSetEmpty
		if !errors.As(err, &empty) {
			return fmt.Errorf("upgrade stack %q: %w", appConfig.StackName(), err)
		}
	}
	return cf.upgradeAppStackSet(appConfig)
}

func (cf CloudFormation) upgradeAppStackSet(config *stack.AppStackConfig) error {
	for {
		ssName := config.StackSetName()
		if err := cf.appStackSet.WaitForStackSetLastOperationComplete(ssName); err != nil {
			return fmt.Errorf("wait for stack set %s last operation complete: %w", ssName, err)
		}
		previouslyDeployedConfig, err := cf.getLastDeployedAppConfig(config)
		if err != nil {
			return err
		}
		previouslyDeployedConfig.Version += 1
		err = cf.deployAppConfig(config, previouslyDeployedConfig, true /* updating template resources should update all instances*/)
		if err == nil {
			return nil
		}
		var stackSetOutOfDateErr *stackset.ErrStackSetOutOfDate
		if errors.As(err, &stackSetOutOfDateErr) {
			continue
		}
		return err
	}
}

func (cf CloudFormation) upgradeAppStack(conf *stack.AppStackConfig) error {
	s, err := toStack(conf)
	if err != nil {
		return err
	}
	in := &executeAndRenderChangeSetInput{
		stackName:        s.Name,
		stackDescription: fmt.Sprintf("Creating the infrastructure for the %s app.", s.Name),
	}
	in.createChangeSet = func() (changeSetID string, err error) {
		spinner := progress.NewSpinner(cf.console)
		label := fmt.Sprintf("Proposing infrastructure changes for %s.", s.Name)
		spinner.Start(label)
		defer stopSpinner(spinner, err, label)

		changeSetID, err = cf.cfnClient.Update(s)
		if err != nil {
			return "", err
		}
		return changeSetID, nil
	}

	return cf.executeAndRenderChangeSet(in)
}

// removeDNSDelegationAndCrossAccountAccess removes the provided account ID from the list of accounts that can write to the
// application's DNS HostedZone. It does this by creating the new list of DNS delegated accounts, updating the app
// infrastructure roles stack, then redeploying all the stackset instances with the new list of accounts.
// If the list of accounts already excludes the account to remove, we return early for idempotency.
func (cf CloudFormation) removeDNSDelegationAndCrossAccountAccess(appStack *stack.AppStackConfig, accountID string) error {
	// Get the most recently deployed list of delegated accounts.
	appStackDesc, err := cf.cfnClient.Describe(appStack.StackName())
	if err != nil {
		return fmt.Errorf("get existing application infrastructure stack: %w", err)
	}
	dnsDelegatedAccounts := cf.dnsDelegatedAccountsForStack(appStackDesc.SDK())

	// Remove the desired account from this list.
	var newAccountList []string
	for _, account := range dnsDelegatedAccounts {
		if account == accountID {
			continue
		}
		newAccountList = append(newAccountList, account)
	}
	// If the lists are equal length, the account has already been removed and we don't have to redeploy the stackset instances.
	if len(newAccountList) == len(dnsDelegatedAccounts) {
		return nil
	}
	// Create a new AppStackConfig using the new account list.
	newCfg := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:                  appStack.Name,
		AccountID:             appStack.AccountID,
		DNSDelegationAccounts: newAccountList,
		DomainName:            appStack.DomainName,
		DomainHostedZoneID:    appStack.DomainHostedZoneID,
		PermissionsBoundary:   appStack.PermissionsBoundary,
		AdditionalTags:        appStack.AdditionalTags,
		Version:               appStack.Version,
	})
	// Redeploy the infrastructure roles stack.
	s, err := toStack(newCfg)
	if err != nil {
		return err
	}
	// Update app stack.
	if err := cf.cfnClient.UpdateAndWait(s); err != nil {
		var errNoUpdates *cloudformation.ErrChangeSetEmpty
		if errors.As(err, &errNoUpdates) {
			return nil
		}
		return fmt.Errorf("update application to remove DNS delegation from account %s: %w", accountID, err)
	}

	// Update stackset instances to remove account.
	appResourcesConfig, err := cf.getLastDeployedAppConfig(appStack)
	if err != nil {
		return err
	}
	newDeploymentConfig := &stack.AppResourcesConfig{
		Version:   appResourcesConfig.Version + 1,
		Workloads: appResourcesConfig.Workloads,
		Accounts:  newAccountList,
		App:       appResourcesConfig.App,
	}
	if err := cf.deployAppConfig(newCfg, newDeploymentConfig, true); err != nil {
		return err
	}
	return nil
}

// DelegateDNSPermissions grants the provided account ID the ability to write to this application's
// DNS HostedZone. This allows us to perform cross account DNS delegation.
func (cf CloudFormation) DelegateDNSPermissions(app *config.Application, accountID string) error {
	deployApp := deploy.CreateAppInput{
		Name:               app.Name,
		AccountID:          app.AccountID,
		DomainName:         app.Domain,
		DomainHostedZoneID: app.DomainHostedZoneID,
		Version:            version.LatestTemplateVersion(),
	}

	appConfig := stack.NewAppStackConfig(&deployApp)
	appStack, err := cf.cfnClient.Describe(appConfig.StackName())
	if err != nil {
		return fmt.Errorf("getting existing application infrastructure stack: %w", err)
	}

	dnsDelegatedAccounts := stack.DNSDelegatedAccountsForStack(appStack.SDK())
	deployApp.DNSDelegationAccounts = append(dnsDelegatedAccounts, accountID)

	s, err := toStack(stack.NewAppStackConfig(&deployApp))
	if err != nil {
		return err
	}
	if err := cf.cfnClient.UpdateAndWait(s); err != nil {
		var errNoUpdates *cloudformation.ErrChangeSetEmpty
		if errors.As(err, &errNoUpdates) {
			return nil
		}
		return fmt.Errorf("updating application to allow DNS delegation: %w", err)
	}
	return nil
}

// GetAppResourcesByRegion fetches all the regional resources for a particular region.
func (cf CloudFormation) GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error) {
	resources, err := cf.getResourcesForStackInstances(app, &region)
	if err != nil {
		return nil, fmt.Errorf("describing application resources: %w", err)
	}
	if len(resources) == 0 {
		return nil, &errNoRegionalResources{app.Name, region}
	}

	return resources[0], nil
}

// GetRegionalAppResources fetches all the regional resources for a particular application.
func (cf CloudFormation) GetRegionalAppResources(app *config.Application) ([]*stack.AppRegionalResources, error) {
	resources, err := cf.getResourcesForStackInstances(app, nil)
	if err != nil {
		return nil, fmt.Errorf("describing application resources: %w", err)
	}
	return resources, nil
}

func (cf CloudFormation) getResourcesForStackInstances(app *config.Application, region *string) ([]*stack.AppRegionalResources, error) {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:      app.Name,
		AccountID: app.AccountID,
	})
	opts := []stackset.InstanceSummariesOption{
		stackset.FilterSummariesByAccountID(app.AccountID),
	}
	if region != nil {
		opts = append(opts, stackset.FilterSummariesByRegion(*region))
	}
	summaries, err := cf.appStackSet.InstanceSummaries(appConfig.StackSetName(), opts...)
	if err != nil {
		return nil, err
	}
	var regionalResources []*stack.AppRegionalResources
	for _, summary := range summaries {
		// Since these stacks will likely be in another region, we can't use
		// the default cf client. Instead, we'll have to create a new client
		// configured with the stack's region.
		regionalCFClient := cf.regionalClient(summary.Region)
		cfStack, err := regionalCFClient.Describe(summary.StackID)
		if err != nil {
			return nil, fmt.Errorf("getting outputs for stack %s in region %s: %w", summary.StackID, summary.Region, err)
		}
		regionalResource, err := stack.ToAppRegionalResources(cfStack.SDK())
		if err != nil {
			return nil, err
		}
		regionalResource.Region = summary.Region
		regionalResources = append(regionalResources, regionalResource)
	}

	return regionalResources, nil
}

// AddWorkloadToAppOpt allows passing optional parameters to AddServiceToApp.
type AddWorkloadToAppOpt func(*stack.AppResourcesWorkload)

// AddWorkloadToAppOptWithoutECR adds a workload to app without creating an ECR repo.
func AddWorkloadToAppOptWithoutECR(s *stack.AppResourcesWorkload) {
	s.WithECR = false
}

// AddServiceToApp attempts to add new service specific resources to the application resource stack.
// Currently, this means that we'll set up an ECR repo with a policy for all envs to be able
// to pull from it.
func (cf CloudFormation) AddServiceToApp(app *config.Application, svcName string, opts ...AddWorkloadToAppOpt) error {
	if err := cf.addWorkloadToApp(app, svcName, opts...); err != nil {
		return fmt.Errorf("adding service %s resources to application %s: %w", svcName, app.Name, err)
	}
	return nil
}

// AddJobToApp attempts to add new job-specific resources to the application resource stack.
// Currently, this means that we'll set up an ECR repo with a policy for all envs to be able
// to pull from it.
func (cf CloudFormation) AddJobToApp(app *config.Application, jobName string, opts ...AddWorkloadToAppOpt) error {
	if err := cf.addWorkloadToApp(app, jobName, opts...); err != nil {
		return fmt.Errorf("adding job %s resources to application %s: %w", jobName, app.Name, err)
	}
	return nil
}

func (cf CloudFormation) addWorkloadToApp(app *config.Application, wlName string, opts ...AddWorkloadToAppOpt) error {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:           app.Name,
		AccountID:      app.AccountID,
		AdditionalTags: app.Tags,
		Version:        version.LatestTemplateVersion(),
	})
	previouslyDeployedConfig, err := cf.getLastDeployedAppConfig(appConfig)
	if err != nil {
		return err
	}

	// We'll generate a new list of Accounts to add to our application
	// infrastructure by appending the environment's account if it
	// doesn't already exist.
	var wlList []stack.AppResourcesWorkload
	shouldAddNewWl := true
	for _, wl := range previouslyDeployedConfig.Workloads {
		wlList = append(wlList, wl)
		if wl.Name == wlName {
			shouldAddNewWl = false
		}
	}
	if !shouldAddNewWl {
		return nil
	}
	newAppResourcesService := &stack.AppResourcesWorkload{
		Name:    wlName,
		WithECR: true,
	}
	for _, opt := range opts {
		opt(newAppResourcesService)
	}
	wlList = append(wlList, *newAppResourcesService)

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:   previouslyDeployedConfig.Version + 1,
		Workloads: wlList,
		Accounts:  previouslyDeployedConfig.Accounts,
		App:       appConfig.Name,
	}
	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig, shouldAddNewWl); err != nil {
		return err
	}

	return nil
}

// RemoveServiceFromApp attempts to remove service-specific resources (ECR repositories) from the application resource stack.
func (cf CloudFormation) RemoveServiceFromApp(app *config.Application, svcName string) error {
	if err := cf.removeWorkloadFromApp(app, svcName); err != nil {
		return fmt.Errorf("removing %s service resources from application: %w", svcName, err)
	}
	return nil
}

// RemoveJobFromApp attempts to remove job-specific resources (ECR repositories) from the application resource stack.
func (cf CloudFormation) RemoveJobFromApp(app *config.Application, jobName string) error {
	if err := cf.removeWorkloadFromApp(app, jobName); err != nil {
		return fmt.Errorf("removing %s job resources from application: %w", jobName, err)
	}
	return nil
}

// RemoveEnvFromAppOpts contains the parameters to call RemoveEnvFromApp.
type RemoveEnvFromAppOpts struct {
	App          *config.Application
	EnvToDelete  *config.Environment
	Environments []*config.Environment
}

// RemoveEnvFromApp optionally redeploys the app stack to remove the account and, if necessary, empties
// ECR repos and a regional S3 bucket before deleting the stackset instance for that region. This method
// cannot check that deleting the stackset or removing the app won't break copilot. Be careful.
func (cf CloudFormation) RemoveEnvFromApp(opts *RemoveEnvFromAppOpts) error {
	var accountHasOtherEnvs, regionHasOtherEnvs bool
	for _, env := range opts.Environments {
		if env.Name != opts.EnvToDelete.Name {
			if env.AccountID == opts.EnvToDelete.AccountID {
				accountHasOtherEnvs = true
			}
			if env.Region == opts.EnvToDelete.Region {
				regionHasOtherEnvs = true
			}
		}
	}

	// This is a no-op if there are remaining environments in the region or account.
	if regionHasOtherEnvs && accountHasOtherEnvs {
		return nil
	}

	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:               opts.App.Name,
		AccountID:          opts.App.AccountID,
		DomainName:         opts.App.Domain,
		DomainHostedZoneID: opts.App.DomainHostedZoneID,
		Version:            opts.App.Version,
	})

	if !regionHasOtherEnvs {
		if err := cf.cleanUpRegionalResources(opts.App, opts.EnvToDelete.Region); err != nil {
			return err
		}
		if err := cf.deleteStackSetInstance(appConfig.StackSetName(), opts.EnvToDelete.AccountID, opts.EnvToDelete.Region); err != nil {
			return err
		}
	}

	if !accountHasOtherEnvs {
		return cf.removeDNSDelegationAndCrossAccountAccess(appConfig, opts.EnvToDelete.AccountID)
	}

	return nil
}

// cleanUpRegionalResources checks for existing regional resources and optionally empties ECR Repos and S3 buckets.
// If there are no regional resources in that region (i.e. a delete call has already been made) it returns nil.
func (cf CloudFormation) cleanUpRegionalResources(app *config.Application, region string) error {
	resources, err := cf.GetAppResourcesByRegion(app, region)
	if err != nil {
		// Return early for idempotency if resources not found.
		var errNotFound *errNoRegionalResources
		if errors.As(err, &errNotFound) {
			return nil
		}
		return err
	}
	s3 := cf.regionalS3Client(region)
	if err := s3.EmptyBucket(resources.S3Bucket); err != nil {
		return err
	}
	ecr := cf.regionalECRClient(region)
	for repo := range resources.RepositoryURLs {
		if err := ecr.ClearRepository(repo); err != nil {
			return err
		}
	}
	return nil
}

func (cf CloudFormation) removeWorkloadFromApp(app *config.Application, wlName string) error {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:           app.Name,
		AccountID:      app.AccountID,
		AdditionalTags: app.Tags,
		Version:        app.Version,
	})
	previouslyDeployedConfig, err := cf.getLastDeployedAppConfig(appConfig)
	if err != nil {
		return fmt.Errorf("get previous application %s config: %w", app.Name, err)
	}

	// We'll generate a new list of Accounts to remove the account associated
	// with the input workload to be removed.
	var wlList []stack.AppResourcesWorkload
	shouldRemoveWl := false
	for _, wl := range previouslyDeployedConfig.Workloads {
		if wl.Name == wlName {
			shouldRemoveWl = true
			continue
		}
		wlList = append(wlList, wl)
	}

	if !shouldRemoveWl {
		return nil
	}

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:   previouslyDeployedConfig.Version + 1,
		Workloads: wlList,
		Accounts:  previouslyDeployedConfig.Accounts,
		App:       appConfig.Name,
	}
	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig, shouldRemoveWl); err != nil {
		return err
	}

	return nil
}

// AddEnvToAppOpts contains the parameters to call AddEnvToApp.
type AddEnvToAppOpts struct {
	App          *config.Application
	EnvName      string
	EnvAccountID string
	EnvRegion    string
}

// AddEnvToApp takes a new environment and updates the application configuration
// with new Account IDs in resource policies (KMS Keys and ECR Repos) - and
// sets up a new stack instance if the environment is in a new region.
func (cf CloudFormation) AddEnvToApp(opts *AddEnvToAppOpts) error {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:           opts.App.Name,
		AccountID:      opts.App.AccountID,
		AdditionalTags: opts.App.Tags,
		Version:        version.LatestTemplateVersion(),
	})
	previouslyDeployedConfig, err := cf.getLastDeployedAppConfig(appConfig)
	if err != nil {
		return fmt.Errorf("getting previous deployed stackset %w", err)
	}

	// We'll generate a new list of Accounts to add to our application
	// infrastructure by appending the environment's account if it
	// doesn't already exist.
	var accountList []string
	shouldAddNewAccountID := true
	for _, accountID := range previouslyDeployedConfig.Accounts {
		accountList = append(accountList, accountID)
		if accountID == opts.EnvAccountID {
			shouldAddNewAccountID = false
		}
	}

	if shouldAddNewAccountID {
		accountList = append(accountList, opts.EnvAccountID)
	}

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:   previouslyDeployedConfig.Version + 1,
		Workloads: previouslyDeployedConfig.Workloads,
		Accounts:  accountList,
		App:       appConfig.Name,
	}

	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig, shouldAddNewAccountID); err != nil {
		return fmt.Errorf("adding %s environment resources to application: %w", opts.EnvName, err)
	}

	if err := cf.addNewAppStackInstances(appConfig, previouslyDeployedConfig, opts.EnvRegion); err != nil {
		return fmt.Errorf("adding new stack instance for environment %s: %w", opts.EnvName, err)
	}

	return nil
}

var getRegionFromClient = func(client sdkcloudformationiface.CloudFormationAPI) (string, error) {
	concrete, ok := client.(*sdkcloudformation.CloudFormation)
	if !ok {
		return "", errors.New("failed to retrieve the region")
	}
	return *concrete.Client.Config.Region, nil
}

// AddPipelineResourcesToApp conditionally adds resources needed to support
// a pipeline in the application region (i.e. the same region that hosts our SSM store).
// This is necessary because the application region might not contain any environment.
func (cf CloudFormation) AddPipelineResourcesToApp(
	app *config.Application, appRegion string) error {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:      app.Name,
		AccountID: app.AccountID,
		Version:   version.LatestTemplateVersion(),
	})

	resourcesConfig, err := cf.getLastDeployedAppConfig(appConfig)
	if err != nil {
		return err
	}

	// conditionally create a new stack instance in the application region
	// if there's no existing stack instance.
	if err := cf.addNewAppStackInstances(appConfig, resourcesConfig, appRegion); err != nil {
		return fmt.Errorf("failed to add stack instance for pipeline, application: %s, region: %s, error: %w",
			app.Name, appRegion, err)
	}

	return nil
}

func (cf CloudFormation) deployAppConfig(appConfig *stack.AppStackConfig, resources *stack.AppResourcesConfig, hasInstanceUpdates bool) error {
	newTemplateToDeploy, err := appConfig.ResourceTemplate(resources)
	if err != nil {
		return err
	}
	// Every time we deploy the StackSet, we include a version field in the stack metadata.
	// When we go to update the StackSet, we include that version + 1 as the "Operation ID".
	// This ensures that we don't overwrite any changes that may have been applied between
	// us reading the stack and actually updating it.
	// As an example:
	//  * We read the stack with Version 1
	//  * Someone else reads the stack with Version 1
	//  * We update the StackSet with Version 2, the update completes.
	//  * Someone else tries to update the StackSet with their stale version 2.
	//  * "2" has already been used as an operation ID, and the stale write fails.
	stackSetAdminRoleARN, err := appConfig.StackSetAdminRoleARN(cf.region)
	if err != nil {
		return fmt.Errorf("get stack set administrator role arn: %w", err)
	}

	renderInput := renderStackSetInput{
		name:               appConfig.StackSetName(),
		template:           newTemplateToDeploy,
		hasInstanceUpdates: hasInstanceUpdates,
		createOpFn: func() (string, error) {
			return cf.appStackSet.Update(appConfig.StackSetName(), newTemplateToDeploy,
				stackset.WithOperationID(fmt.Sprintf("%d", resources.Version)),
				stackset.WithDescription(appConfig.StackSetDescription()),
				stackset.WithExecutionRoleName(appConfig.StackSetExecutionRoleName()),
				stackset.WithAdministrationRoleARN(stackSetAdminRoleARN),
				stackset.WithTags(toMap(appConfig.Tags())))
		},
		now: time.Now,
	}
	return cf.renderStackSet(renderInput)
}

// addNewAppStackInstances takes an environment and determines if we need to create a new
// stack instance. We only spin up a new stack instance if the env is in a new region.
func (cf CloudFormation) addNewAppStackInstances(appConfig *stack.AppStackConfig, resourcesConfig *stack.AppResourcesConfig, region string) error {
	summaries, err := cf.appStackSet.InstanceSummaries(appConfig.StackSetName())
	if err != nil {
		return err
	}

	// We only want to deploy a new StackInstance if we're
	// adding an environment in a new region.
	shouldDeployNewStackInstance := true
	for _, summary := range summaries {
		if summary.Region == region {
			shouldDeployNewStackInstance = false
		}
	}

	if !shouldDeployNewStackInstance {
		return nil
	}

	template, err := appConfig.ResourceTemplate(resourcesConfig)
	if err != nil {
		return err
	}

	// Set up a new Stack Instance for the new region. The Stack Instance will inherit the latest StackSet template.
	renderInput := renderStackSetInput{
		name:               appConfig.StackSetName(),
		template:           template,
		hasInstanceUpdates: shouldDeployNewStackInstance,
		createOpFn: func() (string, error) {
			return cf.appStackSet.CreateInstances(appConfig.StackSetName(), []string{appConfig.AccountID}, []string{region})
		},
		now: time.Now,
	}
	return cf.renderStackSet(renderInput)
}

func (cf CloudFormation) getLastDeployedAppConfig(appConfig *stack.AppStackConfig) (*stack.AppResourcesConfig, error) {
	// Check the existing deploy stack template. From that template, we'll parse out the list of services and accounts that
	// are deployed in the stack.
	descr, err := cf.appStackSet.Describe(appConfig.StackSetName())
	if err != nil {
		return nil, err
	}
	previouslyDeployedConfig, err := stack.AppConfigFrom(&descr.Template)
	if err != nil {
		return nil, fmt.Errorf("parse previous deployed stackset %w", err)
	}
	previouslyDeployedConfig.App = appConfig.Name
	return previouslyDeployedConfig, nil
}

// DeleteApp deletes all application specific StackSet and Stack resources.
func (cf CloudFormation) DeleteApp(appName string) error {
	spinner := progress.NewSpinner(cf.console)
	spinner.Start(fmt.Sprintf("Delete regional resources for application %q", appName))

	stackSetName := fmt.Sprintf("%s-infrastructure", appName)
	if err := cf.deleteStackSetInstances(stackSetName); err != nil {
		spinner.Stop(log.Serrorf("Error deleting regional resources for application %q\n", appName))
		return err
	}
	if err := cf.appStackSet.Delete(stackSetName); err != nil {
		spinner.Stop(log.Serrorf("Error deleting regional resources for application %q\n", appName))
		return err
	}
	spinner.Stop(log.Ssuccessf("Deleted regional resources for application %q\n", appName))
	stackName := fmt.Sprintf("%s-infrastructure-roles", appName)
	description := fmt.Sprintf("Delete application roles stack %s", stackName)
	return cf.deleteAndRenderStack(deleteAndRenderInput{
		stackName:   stackName,
		description: description,
		deleteFn: func() error {
			return cf.cfnClient.DeleteAndWait(stackName)
		},
	})
}

func (cf CloudFormation) deleteStackSetInstances(name string) error {
	opID, err := cf.appStackSet.DeleteAllInstances(name)
	if err != nil {
		if IsEmptyErr(err) {
			return nil
		}
		return err
	}
	return cf.appStackSet.WaitForOperation(name, opID)
}

func (cf CloudFormation) deleteStackSetInstance(name, account, region string) error {
	opId, err := cf.appStackSet.DeleteInstance(name, account, region)
	if err != nil {
		if IsEmptyErr(err) {
			return nil
		}
		return err
	}
	return cf.appStackSet.WaitForOperation(name, opId)
}

type renderStackSetInput struct {
	name               string                 // Name of the stack set.
	template           string                 // Template body for stack set instances.
	hasInstanceUpdates bool                   // True when the stack set update will force instances to also be updated.
	createOpFn         func() (string, error) // Function to create a stack set operation.
	now                func() time.Time
}

func (cf CloudFormation) renderStackSetImpl(in renderStackSetInput) error {
	titles, err := cloudformation.ParseTemplateDescriptions(in.template)
	if err != nil {
		return fmt.Errorf("parse resource descriptions from stack set template: %w", err)
	}

	// Start the operation.
	timestamp := in.now()
	opID, err := in.createOpFn()
	if err != nil {
		return err
	}

	// Collect streamers.
	setStreamer := stream.NewStackSetStreamer(cf.appStackSet, in.name, opID, timestamp)
	var stackStreamers []*stream.StackStreamer
	if in.hasInstanceUpdates {
		stackStreamers, err = setStreamer.InstanceStreamers(func(region string) stream.StackEventsDescriber {
			return cf.regionalClient(region)
		})
		if err != nil {
			return fmt.Errorf("retrieve stack instance streamers: %w", err)
		}
	}
	streamers := []stream.Streamer{setStreamer}
	for _, streamer := range stackStreamers {
		streamers = append(streamers, streamer)
	}

	// Collect renderers
	renderers := stackSetRenderers(setStreamer, stackStreamers, titles)

	// Render.
	waitCtx, cancelWait := context.WithTimeout(context.Background(), waitForStackTimeout)
	defer cancelWait()
	g, ctx := errgroup.WithContext(waitCtx)

	for _, streamer := range streamers {
		streamer := streamer // Create a new instance of streamer for the goroutine.
		g.Go(func() error {
			return stream.Stream(ctx, streamer)
		})
	}
	g.Go(func() error {
		_, err := progress.Render(ctx, progress.NewTabbedFileWriter(cf.console), progress.MultiRenderer(renderers...))
		return err
	})
	if err := g.Wait(); err != nil {
		return fmt.Errorf("render progress of stack set %q: %w", in.name, err)
	}
	return nil
}

func stackSetRenderers(setStreamer *stream.StackSetStreamer, stackStreamers []*stream.StackStreamer, resourceTitles map[string]string) []progress.DynamicRenderer {
	noStyle := progress.RenderOptions{}
	renderers := []progress.DynamicRenderer{
		progress.ListeningStackSetRenderer(setStreamer, fmt.Sprintf("Update regional resources with stack set %q", setStreamer.Name()), noStyle),
	}
	for _, streamer := range stackStreamers {
		title := fmt.Sprintf("Update stack set instance %q", streamer.Name())
		if region, ok := streamer.Region(); ok {
			title = fmt.Sprintf("Update resources in region %q", region)
		}
		r := progress.ListeningStackRenderer(streamer, streamer.Name(), title, resourceTitles, progress.NestedRenderOptions(noStyle))
		renderers = append(renderers, r)
	}
	return renderers
}
