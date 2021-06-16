// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	sdkcloudformationiface "github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

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
	if err := cf.cfnClient.CreateAndWait(s); err != nil {
		// If the stack already exists - we can move on to creating the StackSet.
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
	stackSetAdminRoleARN, err := appConfig.StackSetAdminRoleARN()
	if err != nil {
		return fmt.Errorf("get stack set administrator role arn: %w", err)
	}
	return cf.appStackSet.Create(appConfig.StackSetName(), blankAppTemplate,
		stackset.WithDescription(appConfig.StackSetDescription()),
		stackset.WithExecutionRoleName(appConfig.StackSetExecutionRoleName()),
		stackset.WithAdministrationRoleARN(stackSetAdminRoleARN),
		stackset.WithTags(toMap(appConfig.Tags())))
}

func (cf CloudFormation) UpgradeApplication(in *deploy.CreateAppInput) error {
	appConfig := stack.NewAppStackConfig(in)
	s, err := toStack(appConfig)
	if err != nil {
		return err
	}
	if err := cf.upgradeAppStack(s); err != nil {
		return err
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
		err = cf.deployAppConfig(config, previouslyDeployedConfig)
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

func (cf CloudFormation) upgradeAppStack(s *cloudformation.Stack) error {
	for {
		// Upgrade app stack.
		descr, err := cf.cfnClient.Describe(s.Name)
		if err != nil {
			return fmt.Errorf("describe stack %s: %w", s.Name, err)
		}
		if cloudformation.StackStatus(aws.StringValue(descr.StackStatus)).InProgress() {
			// There is already an update happening to the app stack.
			// Best-effort try to wait for the existing update to be over before retrying.
			_ = cf.cfnClient.WaitForUpdate(context.Background(), s.Name)
			continue
		}
		// We only need the tags from the previously deployed stack.
		s.Tags = descr.Tags

		err = cf.cfnClient.UpdateAndWait(s)
		if err == nil {
			return nil
		}
		if retryable := isRetryableUpdateError(s.Name, err); retryable {
			continue
		}
		// The changes are already applied, nothing to do. Exit successfully.
		var emptyChangeSet *cloudformation.ErrChangeSetEmpty
		if errors.As(err, &emptyChangeSet) {
			return nil
		}
		return fmt.Errorf("update and wait for stack %s: %w", s.Name, err)
	}
}

// DelegateDNSPermissions grants the provided account ID the ability to write to this application's
// DNS HostedZone. This allows us to perform cross account DNS delegation.
func (cf CloudFormation) DelegateDNSPermissions(app *config.Application, accountID string) error {
	deployApp := deploy.CreateAppInput{
		Name:               app.Name,
		AccountID:          app.AccountID,
		DomainName:         app.Domain,
		DomainHostedZoneID: app.DomainHostedZoneID,
		Version:            deploy.LatestAppTemplateVersion,
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
		return nil, fmt.Errorf("no regional resources for application %s in region %s found", app.Name, region)
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

// AddServiceToApp attempts to add new service specific resources to the application resource stack.
// Currently, this means that we'll set up an ECR repo with a policy for all envs to be able
// to pull from it.
func (cf CloudFormation) AddServiceToApp(app *config.Application, svcName string) error {
	if err := cf.addWorkloadToApp(app, svcName); err != nil {
		return fmt.Errorf("adding service %s resources to application %s: %w", svcName, app.Name, err)
	}
	return nil
}

// AddJobToApp attempts to add new job-specific resources to the application resource stack.
// Currently, this means that we'll set up an ECR repo with a policy for all envs to be able
// to pull from it.
func (cf CloudFormation) AddJobToApp(app *config.Application, jobName string) error {
	if err := cf.addWorkloadToApp(app, jobName); err != nil {
		return fmt.Errorf("adding job %s resources to application %s: %w", jobName, app.Name, err)
	}
	return nil
}

func (cf CloudFormation) addWorkloadToApp(app *config.Application, wlName string) error {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:           app.Name,
		AccountID:      app.AccountID,
		AdditionalTags: app.Tags,
		Version:        deploy.LatestAppTemplateVersion,
	})
	previouslyDeployedConfig, err := cf.getLastDeployedAppConfig(appConfig)
	if err != nil {
		return err
	}

	// We'll generate a new list of Accounts to add to our application
	// infrastructure by appending the environment's account if it
	// doesn't already exist.
	var wlList []string
	shouldAddNewWl := true
	// For now, AppResourcesConfig.Services refers to workloads, including both services and jobs.
	for _, wl := range previouslyDeployedConfig.Services {
		wlList = append(wlList, wl)
		if wl == wlName {
			shouldAddNewWl = false
		}
	}
	if !shouldAddNewWl {
		return nil
	}

	wlList = append(wlList, wlName)

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Services: wlList,
		Accounts: previouslyDeployedConfig.Accounts,
		App:      appConfig.Name,
	}
	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig); err != nil {
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

func (cf CloudFormation) removeWorkloadFromApp(app *config.Application, wlName string) error {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:      app.Name,
		AccountID: app.AccountID,
		Version:   deploy.LatestAppTemplateVersion,
	})
	previouslyDeployedConfig, err := cf.getLastDeployedAppConfig(appConfig)
	if err != nil {
		return fmt.Errorf("get previous application %s config: %w", app.Name, err)
	}

	// We'll generate a new list of Accounts to remove the account associated
	// with the input workload to be removed.
	var wlList []string
	shouldRemoveWl := false
	// For now, AppResourcesConfig.Services refers to workloads, including both services and jobs.
	for _, wl := range previouslyDeployedConfig.Services {
		if wl == wlName {
			shouldRemoveWl = true
			continue
		}
		wlList = append(wlList, wl)
	}

	if !shouldRemoveWl {
		return nil
	}

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Services: wlList,
		Accounts: previouslyDeployedConfig.Accounts,
		App:      appConfig.Name,
	}
	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig); err != nil {
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
		Version:        deploy.LatestAppTemplateVersion,
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
		Version:  previouslyDeployedConfig.Version + 1,
		Services: previouslyDeployedConfig.Services,
		Accounts: accountList,
		App:      appConfig.Name,
	}

	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("adding %s environment resources to application: %w", opts.EnvName, err)
	}

	if err := cf.addNewAppStackInstances(appConfig, opts.EnvRegion); err != nil {
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
		Version:   deploy.LatestAppTemplateVersion,
	})

	// conditionally create a new stack instance in the application region
	// if there's no existing stack instance.
	if err := cf.addNewAppStackInstances(appConfig, appRegion); err != nil {
		return fmt.Errorf("failed to add stack instance for pipeline, application: %s, region: %s, error: %w",
			app.Name, appRegion, err)
	}

	return nil
}

func (cf CloudFormation) deployAppConfig(appConfig *stack.AppStackConfig, resources *stack.AppResourcesConfig) error {
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
	stackSetAdminRoleARN, err := appConfig.StackSetAdminRoleARN()
	if err != nil {
		return fmt.Errorf("get stack set administrator role arn: %w", err)
	}
	return cf.appStackSet.UpdateAndWait(appConfig.StackSetName(), newTemplateToDeploy,
		stackset.WithOperationID(fmt.Sprintf("%d", resources.Version)),
		stackset.WithDescription(appConfig.StackSetDescription()),
		stackset.WithExecutionRoleName(appConfig.StackSetExecutionRoleName()),
		stackset.WithAdministrationRoleARN(stackSetAdminRoleARN),
		stackset.WithTags(toMap(appConfig.Tags())))
}

// addNewAppStackInstances takes an environment and determines if we need to create a new
// stack instance. We only spin up a new stack instance if the env is in a new region.
func (cf CloudFormation) addNewAppStackInstances(appConfig *stack.AppStackConfig, region string) error {
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

	// Set up a new Stack Instance for the new region. The Stack Instance will inherit the latest StackSet template.
	return cf.appStackSet.CreateInstancesAndWait(appConfig.StackSetName(), []string{appConfig.AccountID}, []string{region})
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
	if err := cf.appStackSet.Delete(fmt.Sprintf("%s-infrastructure", appName)); err != nil {
		return err
	}
	return cf.cfnClient.DeleteAndWait(fmt.Sprintf("%s-infrastructure-roles", appName))
}
