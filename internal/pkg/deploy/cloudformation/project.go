// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
)

const (
	maxDeleteStackSetAttempts   = 10
	deleteStackSetSleepDuration = 30 * time.Second
)

// DeployProject sets up everything required for our project-wide resources.
// These resources include things that are regional, rather than scoped to a particular
// environment, such as ECR Repos, CodePipeline KMS keys & S3 buckets.
// We deploy project resources through StackSets - that way we can have one
// template that we update and all regional stacks are updated.
func (cf CloudFormation) DeployProject(in *deploy.CreateProjectInput) error {
	projectConfig := stack.NewProjectStackConfig(in, cf.box)

	// First deploy the project roles needed by StackSets. These roles
	// allow the stack set to set up our regional stacks.
	if err := cf.create(projectConfig); err == nil {
		_, err := cf.waitForStackCreation(projectConfig)
		if err != nil {
			return err
		}
	} else {
		// If the stack already exists - we can move on
		// to creating the StackSet.
		var alreadyExists *ErrStackAlreadyExists
		if !errors.As(err, &alreadyExists) {
			return err
		}
	}

	blankProjectTemplate, err := projectConfig.ResourceTemplate(&stack.ProjectResourcesConfig{
		Project: projectConfig.Project,
	})

	if err != nil {
		return err
	}

	_, err = cf.client.CreateStackSet(&cloudformation.CreateStackSetInput{
		Description:           aws.String(projectConfig.StackSetDescription()),
		StackSetName:          aws.String(projectConfig.StackSetName()),
		TemplateBody:          aws.String(blankProjectTemplate),
		ExecutionRoleName:     aws.String(projectConfig.StackSetExecutionRoleName()),
		AdministrationRoleARN: aws.String(projectConfig.StackSetAdminRoleARN()),
		Tags:                  projectConfig.Tags(),
	})

	if err != nil && !stackSetExists(err) {
		return err
	}

	return nil
}

// DelegateDNSPermissions grants the provided account ID the ability to write to this project's
// DNS HostedZone. This allows us to perform cross account DNS delegation.
func (cf CloudFormation) DelegateDNSPermissions(project *archer.Project, accountID string) error {
	deployProject := deploy.CreateProjectInput{
		Project:    project.Name,
		AccountID:  project.AccountID,
		DomainName: project.Domain,
	}

	projectConfig := stack.NewProjectStackConfig(&deployProject, cf.box)

	describeStack := cloudformation.DescribeStacksInput{
		StackName: aws.String(projectConfig.StackName()),
	}
	projectStack, err := cf.describeStack(&describeStack)

	if err != nil {
		return fmt.Errorf("getting existing project infrastructure stack: %w", err)
	}

	dnsDelegatedAccounts := stack.DNSDelegatedAccountsForStack(projectStack)
	deployProject.DNSDelegationAccounts = append(dnsDelegatedAccounts, accountID)
	updatedProjectConfig := stack.NewProjectStackConfig(&deployProject, cf.box)

	if err := cf.update(updatedProjectConfig); err != nil {
		return fmt.Errorf("updating project to allow DNS delegation: %w", err)
	}

	return cf.client.WaitUntilStackUpdateCompleteWithContext(context.Background(), &describeStack, cf.waiters...)
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
	projectConfig := stack.NewProjectStackConfig(&deploy.CreateProjectInput{
		Project:   project.Name,
		AccountID: project.AccountID}, cf.box)
	listStackInstancesInput := &cloudformation.ListStackInstancesInput{
		StackSetName:         aws.String(projectConfig.StackSetName()),
		StackInstanceAccount: aws.String(project.AccountID),
	}

	if region != nil {
		listStackInstancesInput.StackInstanceRegion = region
	}

	stackInstances, err := cf.client.ListStackInstances(listStackInstancesInput)

	if err != nil {
		return nil, fmt.Errorf("listing stack instances: %w", err)
	}

	regionalResources := []*archer.ProjectRegionalResources{}
	for _, stackInstance := range stackInstances.Summaries {
		// Since these stacks will likely be in another region, we can't use
		// the default cf client. Instead, we'll have to create a new client
		// configured with the stack's region.
		regionAwareCFClient := cf.regionalClientProvider.Client(*stackInstance.Region)
		cfStack, err := cf.describeStackWithClient(&cloudformation.DescribeStacksInput{
			StackName: stackInstance.StackId,
		}, regionAwareCFClient)

		if err != nil {
			return nil, fmt.Errorf("getting outputs for stack %s in region %s: %w", *stackInstance.StackId, *stackInstance.Region, err)
		}

		regionalResource, err := stack.ToProjectRegionalResources(cfStack)
		if err != nil {
			return nil, err
		}
		regionalResource.Region = *stackInstance.Region
		regionalResources = append(regionalResources, regionalResource)
	}

	return regionalResources, nil
}

// AddAppToProject attempts to add new App specific resources to the Project resource stack.
// Currently, this means that we'll set up an ECR repo with a policy for all envs to be able
// to pull from it.
func (cf CloudFormation) AddAppToProject(project *archer.Project, appName string) error {
	projectConfig := stack.NewProjectStackConfig(&deploy.CreateProjectInput{
		Project:   project.Name,
		AccountID: project.AccountID,
	}, cf.box)
	previouslyDeployedConfig, err := cf.getLastDeployedProjectConfig(projectConfig)
	if err != nil {
		return fmt.Errorf("adding %s app resources to project %s: %w", appName, project.Name, err)
	}

	// We'll generate a new list of Accounts to add to our project
	// infrastructure by appending the environment's account if it
	// doesn't already exist.
	var appList []string
	shouldAddNewApp := true
	for _, app := range previouslyDeployedConfig.Apps {
		appList = append(appList, app)
		if app == appName {
			shouldAddNewApp = false
		}
	}

	if !shouldAddNewApp {
		return nil
	}

	appList = append(appList, appName)

	newDeploymentConfig := stack.ProjectResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Apps:     appList,
		Accounts: previouslyDeployedConfig.Accounts,
		Project:  projectConfig.Project,
	}
	if err := cf.deployProjectConfig(projectConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("adding %s app resources to project: %w", appName, err)
	}

	return nil
}

// RemoveAppFromProject attempts to remove App specific resources (ECR repositories) from the Project resource stack.
func (cf CloudFormation) RemoveAppFromProject(project *archer.Project, appName string) error {
	projectConfig := stack.NewProjectStackConfig(&deploy.CreateProjectInput{
		Project:   project.Name,
		AccountID: project.AccountID,
	}, cf.box)
	previouslyDeployedConfig, err := cf.getLastDeployedProjectConfig(projectConfig)
	if err != nil {
		return fmt.Errorf("get previous project %s config: %w", project.Name, err)
	}

	// We'll generate a new list of Accounts to remove the account associated
	// with the input app to be removed.
	var appList []string
	shouldRemoveApp := false
	for _, app := range previouslyDeployedConfig.Apps {
		if app == appName {
			shouldRemoveApp = true
			continue
		}
		appList = append(appList, app)
	}

	if !shouldRemoveApp {
		return nil
	}

	newDeploymentConfig := stack.ProjectResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Apps:     appList,
		Accounts: previouslyDeployedConfig.Accounts,
		Project:  projectConfig.Project,
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
	projectConfig := stack.NewProjectStackConfig(&deploy.CreateProjectInput{
		Project:   project.Name,
		AccountID: project.AccountID,
	}, cf.box)
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

	newDeploymentConfig := stack.ProjectResourcesConfig{
		Version:  previouslyDeployedConfig.Version + 1,
		Apps:     previouslyDeployedConfig.Apps,
		Accounts: accountList,
		Project:  projectConfig.Project,
	}

	if err := cf.deployProjectConfig(projectConfig, &newDeploymentConfig); err != nil {
		return fmt.Errorf("adding %s environment resources to project: %w", env.Name, err)
	}

	if err := cf.addNewProjectStackInstances(projectConfig, env.Region); err != nil {
		return fmt.Errorf("adding new stack instance for environment %s: %w", env.Name, err)
	}

	return nil
}

var getRegionFromClient = func(client cloudformationiface.CloudFormationAPI) (string, error) {
	concrete, ok := client.(*cloudformation.CloudFormation)
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
	projectConfig := stack.NewProjectStackConfig(&deploy.CreateProjectInput{
		Project:   project.Name,
		AccountID: project.AccountID,
	}, cf.box)

	// conditionally create a new stack instance in the project region
	// if there's no existing stack instance.
	if err := cf.addNewProjectStackInstances(projectConfig, projectRegion); err != nil {
		return fmt.Errorf("failed to add stack instance for pipeline, project: %s, region: %s, error: %w",
			project.Name, projectRegion, err)
	}

	return nil
}

func (cf CloudFormation) deployProjectConfig(projectConfig *stack.ProjectStackConfig, resources *stack.ProjectResourcesConfig) error {
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
	input := cloudformation.UpdateStackSetInput{
		TemplateBody:          aws.String(newTemplateToDeploy),
		OperationId:           aws.String(fmt.Sprintf("%d", resources.Version)),
		StackSetName:          aws.String(projectConfig.StackSetName()),
		Description:           aws.String(projectConfig.StackSetDescription()),
		ExecutionRoleName:     aws.String(projectConfig.StackSetExecutionRoleName()),
		AdministrationRoleARN: aws.String(projectConfig.StackSetAdminRoleARN()),
		Tags:                  projectConfig.Tags(),
	}
	output, err := cf.client.UpdateStackSet(&input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case cloudformation.ErrCodeOperationIdAlreadyExistsException, cloudformation.ErrCodeOperationInProgressException, cloudformation.ErrCodeStaleRequestException:
				return &ErrStackSetOutOfDate{projectName: projectConfig.Project, parentErr: err}
			}
		}
		return fmt.Errorf("updating project resources: %w", err)
	}

	return cf.waitForStackSetOperation(projectConfig.StackSetName(), *output.OperationId)
}

// addNewStackInstances takes an environment and determines if we need to create a new
// stack instance. We only spin up a new stack instance if the env is in a new region.
func (cf CloudFormation) addNewProjectStackInstances(projectConfig *stack.ProjectStackConfig, region string) error {
	stackInstances, err := cf.client.ListStackInstances(&cloudformation.ListStackInstancesInput{
		StackSetName: aws.String(projectConfig.StackSetName()),
	})

	if err != nil {
		return fmt.Errorf("fetching existing project stack instances: %w", err)
	}

	// We only want to deploy a new StackInstance if we're
	// adding an environment in a new region.
	shouldDeployNewStackInstance := true
	for _, stackInstance := range stackInstances.Summaries {
		if *stackInstance.Region == region {
			shouldDeployNewStackInstance = false
		}
	}

	if !shouldDeployNewStackInstance {
		return nil
	}

	// Set up a new Stack Instance for the new region. The Stack Instance will inherit
	// the latest StackSet template.
	createStacksOutput, err := cf.client.CreateStackInstances(&cloudformation.CreateStackInstancesInput{
		Accounts:     []*string{aws.String(projectConfig.AccountID)},
		Regions:      []*string{aws.String(region)},
		StackSetName: aws.String(projectConfig.StackSetName()),
	})

	if err != nil {
		return fmt.Errorf("creating new project stack instances: %w", err)
	}

	return cf.waitForStackSetOperation(projectConfig.StackSetName(), *createStacksOutput.OperationId)
}

func (cf CloudFormation) getLastDeployedProjectConfig(projectConfig *stack.ProjectStackConfig) (*stack.ProjectResourcesConfig, error) {
	// Check the existing deploy stack template. From that template, we'll parse out the list of apps and accounts that
	// are deployed in the stack.
	describeOutput, err := cf.client.DescribeStackSet(&cloudformation.DescribeStackSetInput{
		StackSetName: aws.String(projectConfig.StackSetName()),
	})
	if err != nil {
		return nil, fmt.Errorf("describe stack set: %w", err)
	}
	previouslyDeployedConfig, err := stack.ProjectConfigFrom(describeOutput.StackSet.TemplateBody)
	if err != nil {
		return nil, fmt.Errorf("parse previous deployed stackset %w", err)
	}
	return previouslyDeployedConfig, nil
}

func (cf CloudFormation) waitForStackSetOperation(stackSetName, operationID string) error {
	for {
		response, err := cf.client.DescribeStackSetOperation(&cloudformation.DescribeStackSetOperationInput{
			OperationId:  aws.String(operationID),
			StackSetName: aws.String(stackSetName),
		})

		if err != nil {
			return fmt.Errorf("fetching stack set operation status: %w", err)
		}

		if *response.StackSetOperation.Status == "STOPPED" {
			return fmt.Errorf("project operation %s in stack set %s was manually stopped", operationID, stackSetName)
		}

		if *response.StackSetOperation.Status == "FAILED" {
			return fmt.Errorf("project operation %s in stack set %s failed", operationID, stackSetName)
		}

		if *response.StackSetOperation.Status == "SUCCEEDED" {
			return nil
		}

		time.Sleep(3 * time.Second)
	}
}

// DeleteProject deletes all project specific StackSet and Stack resources.
func (cf CloudFormation) DeleteProject(name string, accounts, regions []string) error {
	stackSetName := fmt.Sprintf("%s-infrastructure", name)

	if _, err := cf.client.DeleteStackInstances(&cloudformation.DeleteStackInstancesInput{
		Accounts:     aws.StringSlice(accounts),
		RetainStacks: aws.Bool(false),
		Regions:      aws.StringSlice(regions),
		StackSetName: aws.String(stackSetName),
	}); err != nil {
		return fmt.Errorf("DeleteStackInstances for stackset %s, accounts %s, and regions %s: %w",
			stackSetName, accounts, regions, err)
	}

	// The looping here is because there's no Wait convenience function the SDK provides
	// for waiting on StackSet Instances to delete. The DeleteStackSet call will return the
	// ErrCodeOperationInProgressException while instances are still actively being deleted.
	maxAttempts := maxDeleteStackSetAttempts
	for maxAttempts > 0 {
		_, err := cf.client.DeleteStackSet(&cloudformation.DeleteStackSetInput{
			StackSetName: aws.String(stackSetName),
		})

		if err != nil {
			awserr, ok := err.(awserr.Error)
			if !ok {
				return err
			}

			if awserr.Code() == cloudformation.ErrCodeOperationInProgressException {
				maxAttempts--
				time.Sleep(deleteStackSetSleepDuration)
				continue
			}
		}

		break
	}

	if _, err := cf.client.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(fmt.Sprintf("%s-infrastructure-roles", name)),
	}); err != nil {
		return err
	}

	return nil
}
