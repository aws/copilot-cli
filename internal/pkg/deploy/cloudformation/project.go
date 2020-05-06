// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	sdkcloudformationiface "github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

// DeployProject sets up everything required for our project-wide resources.
// These resources include things that are regional, rather than scoped to a particular
// environment, such as ECR Repos, CodePipeline KMS keys & S3 buckets.
// We deploy project resources through StackSets - that way we can have one
// template that we update and all regional stacks are updated.
func (cf CloudFormation) DeployProject(in *deploy.CreateAppInput) error {
	projectConfig := stack.NewAppStackConfig(in)
	s, err := toStack(projectConfig)
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

	blankProjectTemplate, err := projectConfig.ResourceTemplate(&stack.AppResourcesConfig{
		App: projectConfig.Name,
	})
	if err != nil {
		return err
	}

	return cf.projectStackSet.Create(projectConfig.StackSetName(), blankProjectTemplate,
		stackset.WithDescription(projectConfig.StackSetDescription()),
		stackset.WithExecutionRoleName(projectConfig.StackSetExecutionRoleName()),
		stackset.WithAdministrationRoleARN(projectConfig.StackSetAdminRoleARN()),
		stackset.WithTags(toMap(projectConfig.Tags())))
}

// DelegateDNSPermissions grants the provided account ID the ability to write to this project's
// DNS HostedZone. This allows us to perform cross account DNS delegation.
func (cf CloudFormation) DelegateDNSPermissions(project *archer.Project, accountID string) error {
	deployProject := deploy.CreateAppInput{
		Name:       project.Name,
		AccountID:  project.AccountID,
		DomainName: project.Domain,
	}

	projectConfig := stack.NewAppStackConfig(&deployProject)
	projectStack, err := cf.cfnClient.Describe(projectConfig.StackName())
	if err != nil {
		return fmt.Errorf("getting existing project infrastructure stack: %w", err)
	}

	dnsDelegatedAccounts := stack.DNSDelegatedAccountsForStack(projectStack.SDK())
	deployProject.DNSDelegationAccounts = append(dnsDelegatedAccounts, accountID)

	s, err := toStack(stack.NewAppStackConfig(&deployProject))
	if err != nil {
		return err
	}
	if err := cf.cfnClient.UpdateAndWait(s); err != nil {
		var errNoUpdates *cloudformation.ErrChangeSetEmpty
		if errors.As(err, &errNoUpdates) {
			return nil
		}
		return fmt.Errorf("updating project to allow DNS delegation: %w", err)
	}
	return nil
}

// GetProjectResourcesByRegion fetches all the regional resources for a particular region.
func (cf CloudFormation) GetProjectResourcesByRegion(project *archer.Project, region string) (*archer.ProjectRegionalResources, error) {
	resources, err := cf.getResourcesForStackInstances(project, &region)
	if err != nil {
		return nil, fmt.Errorf("describing project resources: %w", err)
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("no regional resources for project %s in region %s found", project.Name, region)
	}

	return resources[0], nil
}

// GetRegionalProjectResources fetches all the regional resources for a particular project.
func (cf CloudFormation) GetRegionalProjectResources(project *archer.Project) ([]*archer.ProjectRegionalResources, error) {
	resources, err := cf.getResourcesForStackInstances(project, nil)
	if err != nil {
		return nil, fmt.Errorf("describing project resources: %w", err)
	}
	return resources, nil
}

func (cf CloudFormation) getResourcesForStackInstances(project *archer.Project, region *string) ([]*archer.ProjectRegionalResources, error) {
	projectConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:      project.Name,
		AccountID: project.AccountID,
	})
	opts := []stackset.InstanceSummariesOption{
		stackset.FilterSummariesByAccountID(project.AccountID),
	}
	if region != nil {
		opts = append(opts, stackset.FilterSummariesByRegion(*region))
	}

	summaries, err := cf.projectStackSet.InstanceSummaries(projectConfig.StackSetName(), opts...)
	if err != nil {
		return nil, err
	}
	var regionalResources []*archer.ProjectRegionalResources
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

// AddAppToProject attempts to add new App specific resources to the Project resource stack.
// Currently, this means that we'll set up an ECR repo with a policy for all envs to be able
// to pull from it.
func (cf CloudFormation) AddAppToProject(project *archer.Project, appName string) error {
	projectConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:           project.Name,
		AccountID:      project.AccountID,
		AdditionalTags: project.Tags,
	})
	previouslyDeployedConfig, err := cf.getLastDeployedProjectConfig(projectConfig)
	if err != nil {
		return fmt.Errorf("adding %s app resources to project %s: %w", appName, project.Name, err)
	}

	// We'll generate a new list of Accounts to add to our project
	// infrastructure by appending the environment's account if it
	// doesn't already exist.
	var appList []string
	shouldAddNewApp := true
	for _, app := range previouslyDeployedConfig.Services {
		appList = append(appList, app)
		if app == appName {
			shouldAddNewApp = false
		}
	}

	if !shouldAddNewApp {
		return nil
	}

	appList = append(appList, appName)

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Services: appList,
		Accounts: previouslyDeployedConfig.Accounts,
		App:      projectConfig.Name,
	}
	if err := cf.deployProjectConfig(projectConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("adding %s app resources to project: %w", appName, err)
	}

	return nil
}

// RemoveAppFromProject attempts to remove App specific resources (ECR repositories) from the Project resource stack.
func (cf CloudFormation) RemoveAppFromProject(project *archer.Project, appName string) error {
	projectConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:      project.Name,
		AccountID: project.AccountID,
	})
	previouslyDeployedConfig, err := cf.getLastDeployedProjectConfig(projectConfig)
	if err != nil {
		return fmt.Errorf("get previous project %s config: %w", project.Name, err)
	}

	// We'll generate a new list of Accounts to remove the account associated
	// with the input app to be removed.
	var appList []string
	shouldRemoveApp := false
	for _, app := range previouslyDeployedConfig.Services {
		if app == appName {
			shouldRemoveApp = true
			continue
		}
		appList = append(appList, app)
	}

	if !shouldRemoveApp {
		return nil
	}

	newDeploymentConfig := stack.AppResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Services: appList,
		Accounts: previouslyDeployedConfig.Accounts,
		App:      projectConfig.Name,
	}
	if err := cf.deployProjectConfig(projectConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("removing %s app resources from project: %w", appName, err)
	}

	return nil
}

// AddEnvToProject takes a new environment and updates the Project configuration
// with new Account IDs in resource policies (KMS Keys and ECR Repos) - and
// sets up a new stack instance if the environment is in a new region.
func (cf CloudFormation) AddEnvToProject(project *archer.Project, env *archer.Environment) error {
	projectConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:           project.Name,
		AccountID:      project.AccountID,
		AdditionalTags: project.Tags,
	})
	previouslyDeployedConfig, err := cf.getLastDeployedProjectConfig(projectConfig)
	if err != nil {
		return fmt.Errorf("getting previous deployed stackset %w", err)
	}

	// We'll generate a new list of Accounts to add to our project
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
		App:      projectConfig.Name,
	}

	if err := cf.deployProjectConfig(projectConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("adding %s environment resources to project: %w", env.Name, err)
	}

	if err := cf.addNewProjectStackInstances(projectConfig, env.Region); err != nil {
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

// AddPipelineResourcesToProject conditionally adds resources needed to support
// a pipeline in the project region (i.e. the same region that hosts our SSM store).
// This is necessary because the project region might not contain any environment.
func (cf CloudFormation) AddPipelineResourcesToProject(
	project *archer.Project, projectRegion string) error {
	projectConfig := stack.NewAppStackConfig(&deploy.CreateAppInput{
		Name:      project.Name,
		AccountID: project.AccountID,
	})

	// conditionally create a new stack instance in the project region
	// if there's no existing stack instance.
	if err := cf.addNewProjectStackInstances(projectConfig, projectRegion); err != nil {
		return fmt.Errorf("failed to add stack instance for pipeline, project: %s, region: %s, error: %w",
			project.Name, projectRegion, err)
	}

	return nil
}

func (cf CloudFormation) deployProjectConfig(projectConfig *stack.AppStackConfig, resources *stack.AppResourcesConfig) error {
	newTemplateToDeploy, err := projectConfig.ResourceTemplate(resources)
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
	return cf.projectStackSet.UpdateAndWait(projectConfig.StackSetName(), newTemplateToDeploy,
		stackset.WithOperationID(fmt.Sprintf("%d", resources.Version)),
		stackset.WithDescription(projectConfig.StackSetDescription()),
		stackset.WithExecutionRoleName(projectConfig.StackSetExecutionRoleName()),
		stackset.WithAdministrationRoleARN(projectConfig.StackSetAdminRoleARN()),
		stackset.WithTags(toMap(projectConfig.Tags())))
}

// addNewStackInstances takes an environment and determines if we need to create a new
// stack instance. We only spin up a new stack instance if the env is in a new region.
func (cf CloudFormation) addNewProjectStackInstances(projectConfig *stack.AppStackConfig, region string) error {
	summaries, err := cf.projectStackSet.InstanceSummaries(projectConfig.StackSetName())
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
	return cf.projectStackSet.CreateInstancesAndWait(projectConfig.StackSetName(), []string{projectConfig.AccountID}, []string{region})
}

func (cf CloudFormation) getLastDeployedProjectConfig(projectConfig *stack.AppStackConfig) (*stack.AppResourcesConfig, error) {
	// Check the existing deploy stack template. From that template, we'll parse out the list of apps and accounts that
	// are deployed in the stack.
	descr, err := cf.projectStackSet.Describe(projectConfig.StackSetName())
	if err != nil {
		return nil, err
	}
	previouslyDeployedConfig, err := stack.AppConfigFrom(&descr.Template)
	if err != nil {
		return nil, fmt.Errorf("parse previous deployed stackset %w", err)
	}
	return previouslyDeployedConfig, nil
}

// DeleteProject deletes all project specific StackSet and Stack resources.
func (cf CloudFormation) DeleteProject(projectName string) error {
	if err := cf.projectStackSet.Delete(fmt.Sprintf("%s-infrastructure", projectName)); err != nil {
		return err
	}
	return cf.cfnClient.DeleteAndWait(fmt.Sprintf("%s-infrastructure-roles", projectName))
}
