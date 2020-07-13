// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"

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

	return cf.appStackSet.Create(appConfig.StackSetName(), blankAppTemplate,
		stackset.WithDescription(appConfig.StackSetDescription()),
		stackset.WithExecutionRoleName(appConfig.StackSetExecutionRoleName()),
		stackset.WithAdministrationRoleARN(appConfig.StackSetAdminRoleARN()),
		stackset.WithTags(toMap(appConfig.Tags())))
}

// DelegateDNSPermissions grants the provided account ID the ability to write to this application's
// DNS HostedZone. This allows us to perform cross account DNS delegation.
func (cf CloudFormation) DelegateDNSPermissions(app *config.Application, accountID string) error {
	deployApp := deploy.CreateAppInput{
		Name:       app.Name,
		AccountID:  app.AccountID,
		DomainName: app.Domain,
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
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:           app.Name,
		AccountID:      app.AccountID,
		AdditionalTags: app.Tags,
	})
	previouslyDeployedConfig, err := cf.getLastDeployedAppConfig(appConfig)
	if err != nil {
		return fmt.Errorf("adding %s service resources to application %s: %w", svcName, app.Name, err)
	}

	// We'll generate a new list of Accounts to add to our application
	// infrastructure by appending the environment's account if it
	// doesn't already exist.
	var svcList []string
	shouldAddNewSvc := true
	for _, svc := range previouslyDeployedConfig.Services {
		svcList = append(svcList, svc)
		if svc == svcName {
			shouldAddNewSvc = false
		}
	}

	if !shouldAddNewSvc {
		return nil
	}

	svcList = append(svcList, svcName)

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Services: svcList,
		Accounts: previouslyDeployedConfig.Accounts,
		App:      appConfig.Name,
	}
	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("adding %s service resources to application: %w", svcName, err)
	}

	return nil
}

// RemoveServiceFromApp attempts to remove service specific resources (ECR repositories) from the application resource stack.
func (cf CloudFormation) RemoveServiceFromApp(app *config.Application, svcName string) error {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:      app.Name,
		AccountID: app.AccountID,
	})
	previouslyDeployedConfig, err := cf.getLastDeployedAppConfig(appConfig)
	if err != nil {
		return fmt.Errorf("get previous application %s config: %w", app.Name, err)
	}

	// We'll generate a new list of Accounts to remove the account associated
	// with the input service to be removed.
	var svcList []string
	shouldRemoveSvc := false
	for _, svc := range previouslyDeployedConfig.Services {
		if svc == svcName {
			shouldRemoveSvc = true
			continue
		}
		svcList = append(svcList, svc)
	}

	if !shouldRemoveSvc {
		return nil
	}

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Services: svcList,
		Accounts: previouslyDeployedConfig.Accounts,
		App:      appConfig.Name,
	}
	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("removing %s service resources from application: %w", svcName, err)
	}

	return nil
}

// AddEnvToApp takes a new environment and updates the application configuration
// with new Account IDs in resource policies (KMS Keys and ECR Repos) - and
// sets up a new stack instance if the environment is in a new region.
func (cf CloudFormation) AddEnvToApp(app *config.Application, env *config.Environment) error {
	appConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:           app.Name,
		AccountID:      app.AccountID,
		AdditionalTags: app.Tags,
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
		if accountID == env.AccountID {
			shouldAddNewAccountID = false
		}
	}

	if shouldAddNewAccountID {
		accountList = append(accountList, env.AccountID)
	}

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Services: previouslyDeployedConfig.Services,
		Accounts: accountList,
		App:      appConfig.Name,
	}

	if err := cf.deployAppConfig(appConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("adding %s environment resources to application: %w", env.Name, err)
	}

	if err := cf.addNewAppStackInstances(appConfig, env.Region); err != nil {
		return fmt.Errorf("adding new stack instance for environment %s: %w", env.Name, err)
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
	return cf.appStackSet.UpdateAndWait(appConfig.StackSetName(), newTemplateToDeploy,
		stackset.WithOperationID(fmt.Sprintf("%d", resources.Version)),
		stackset.WithDescription(appConfig.StackSetDescription()),
		stackset.WithExecutionRoleName(appConfig.StackSetExecutionRoleName()),
		stackset.WithAdministrationRoleARN(appConfig.StackSetAdminRoleARN()),
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
	// Check the existing deploy stack template. From that template, we'll parse out the list of apps and accounts that
	// are deployed in the stack.
	descr, err := cf.appStackSet.Describe(appConfig.StackSetName())
	if err != nil {
		return nil, err
	}
	previouslyDeployedConfig, err := stack.AppConfigFrom(&descr.Template)
	if err != nil {
		return nil, fmt.Errorf("parse previous deployed stackset %w", err)
	}
	return previouslyDeployedConfig, nil
}

// DeleteApp deletes all application specific StackSet and Stack resources.
func (cf CloudFormation) DeleteApp(appName string) error {
	if err := cf.appStackSet.Delete(fmt.Sprintf("%s-infrastructure", appName)); err != nil {
		return err
	}
	return cf.cfnClient.DeleteAndWait(fmt.Sprintf("%s-infrastructure-roles", appName))
}
