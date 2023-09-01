// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy ECS resources with AWS CloudFormation.
package cloudformation

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
)

// CreateAndRenderEnvironment creates the CloudFormation stack for an environment, and render the stack creation to out.
func (cf CloudFormation) CreateAndRenderEnvironment(conf StackConfiguration, bucketARN string) error {
	cfnStack, err := cf.toUploadedStack(bucketARN, conf)
	if err != nil {
		return err
	}
	in := newRenderEnvironmentInput(cfnStack)
	in.createChangeSet = func() (changeSetID string, err error) {
		spinner := progress.NewSpinner(cf.console)
		label := fmt.Sprintf("Proposing infrastructure changes for the %s environment.", cfnStack.Name)
		spinner.Start(label)
		defer stopSpinner(spinner, err, label)
		changeSetID, err = cf.cfnClient.Create(cfnStack)
		if err != nil {
			return "", err
		}
		return changeSetID, nil
	}
	return cf.executeAndRenderChangeSet(in)
}

// UpdateAndRenderEnvironment updates the CloudFormation stack for an environment, and render the stack creation to out.
func (cf CloudFormation) UpdateAndRenderEnvironment(conf StackConfiguration, bucketARN string, detach bool, opts ...cloudformation.StackOption) error {
	cfnStack, err := cf.toUploadedStack(bucketARN, conf)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(cfnStack)
	}
	in := newRenderEnvironmentInput(cfnStack)
	in.createChangeSet = func() (changeSetID string, err error) {
		spinner := progress.NewSpinner(cf.console)
		label := fmt.Sprintf("Proposing infrastructure changes for the %s environment.", cfnStack.Name)
		spinner.Start(label)
		defer stopSpinner(spinner, err, label)
		changeSetID, err = cf.cfnClient.Update(cfnStack)
		if err != nil {
			return "", err
		}
		return changeSetID, nil
	}
	in.enableInterrupt = true
	in.detach = detach
	return cf.executeAndRenderChangeSet(in)
}

func newRenderEnvironmentInput(cfnStack *cloudformation.Stack) *executeAndRenderChangeSetInput {
	return &executeAndRenderChangeSetInput{
		stackName:        cfnStack.Name,
		stackDescription: fmt.Sprintf("Creating the infrastructure for the %s environment.", cfnStack.Name),
	}
}

// DeleteEnvironment deletes the CloudFormation stack of an environment.
func (cf CloudFormation) DeleteEnvironment(appName, envName, cfnExecRoleARN string) error {
	stackName := stack.NameForEnv(appName, envName)
	description := fmt.Sprintf("Delete environment stack %s", stackName)
	return cf.deleteAndRenderStack(deleteAndRenderInput{
		stackName:   stackName,
		description: description,
		deleteFn: func() error {
			return cf.cfnClient.DeleteAndWaitWithRoleARN(stackName, cfnExecRoleARN)
		},
	})
}

// GetEnvironment returns the Environment metadata from the CloudFormation stack.
func (cf CloudFormation) GetEnvironment(appName, envName string) (*config.Environment, error) {
	conf := stack.NewBootstrapEnvStackConfig(&stack.EnvConfig{
		App: deploy.AppInformation{
			Name: appName,
		},
		Name: envName,
	})
	descr, err := cf.cfnClient.Describe(conf.StackName())
	if err != nil {
		return nil, err
	}
	return conf.ToEnvMetadata(descr.SDK())
}

// ForceUpdateOutputID returns the environment stack's last force update ID.
func (cf CloudFormation) ForceUpdateOutputID(app, env string) (string, error) {
	stackDescr, err := cf.cachedStack(stack.NameForEnv(app, env))
	if err != nil {
		return "", err
	}
	for _, output := range stackDescr.Outputs {
		if aws.StringValue(output.OutputKey) == template.LastForceDeployIDOutputName {
			return aws.StringValue(output.OutputValue), nil
		}
	}
	return "", nil
}

// DeployedEnvironmentParameters returns the environment stack's parameters.
func (cf CloudFormation) DeployedEnvironmentParameters(appName, envName string) ([]*awscfn.Parameter, error) {
	isInitial, err := cf.isInitialDeployment(appName, envName)
	if err != nil {
		return nil, err
	}
	if isInitial {
		return nil, nil
	}
	out, err := cf.cachedStack(stack.NameForEnv(appName, envName))
	if err != nil {
		return nil, err
	}
	return out.Parameters, nil
}

// UpdateEnvironmentTemplate updates the cloudformation stack's template body while maintaining the parameters and tags.
func (cf CloudFormation) UpdateEnvironmentTemplate(appName, envName, templateBody, cfnExecRoleARN string) error {
	stackName := stack.NameForEnv(appName, envName)
	descr, err := cf.cfnClient.Describe(stackName)
	if err != nil {
		return fmt.Errorf("describe stack %s: %w", stackName, err)
	}
	s := cloudformation.NewStack(stackName, templateBody)
	s.Parameters = descr.Parameters
	s.Tags = descr.Tags
	s.RoleARN = aws.String(cfnExecRoleARN)
	return cf.cfnClient.UpdateAndWait(s)
}

func (cf CloudFormation) toUploadedStack(artifactBucketARN string, stackConfig StackConfiguration) (*cloudformation.Stack, error) {
	bucketARN, err := arn.Parse(artifactBucketARN)
	if err != nil {
		return nil, err
	}
	url, err := cf.uploadStackTemplateToS3(bucketARN.Resource, stackConfig)
	if err != nil {
		return nil, err
	}
	cfnStack, err := toStackFromS3(stackConfig, url)
	if err != nil {
		return nil, err
	}
	return cfnStack, nil
}

func (cf CloudFormation) waitAndDescribeStack(stackName string) (*cloudformation.StackDescription, error) {
	var (
		stackDescription *cloudformation.StackDescription
		err              error
	)
	for {
		stackDescription, err = cf.cfnClient.Describe(stackName)
		if err != nil {
			return nil, fmt.Errorf("describe stack %s: %w", stackName, err)
		}

		if cloudformation.StackStatus(aws.StringValue(stackDescription.StackStatus)).InProgress() {
			// There is already an update happening to the environment stack.
			// Best-effort try to wait for the existing update to be over before retrying.
			_ = cf.cfnClient.WaitForUpdate(context.Background(), stackName)
			continue
		}
		break
	}
	return stackDescription, err
}

func (cf CloudFormation) cachedStack(stackName string) (*cloudformation.StackDescription, error) {
	if cf.cachedDeployedStack != nil {
		return cf.cachedDeployedStack, nil
	}
	stackDescr, err := cf.waitAndDescribeStack(stackName)
	if err != nil {
		return nil, err
	}
	cf.cachedDeployedStack = stackDescr
	return cf.cachedDeployedStack, nil
}

// isInitialDeployment returns whether this is the first deployment of the environment stack.
func (cf CloudFormation) isInitialDeployment(appName, envName string) (bool, error) {
	raw, err := cf.cfnClient.Metadata(cloudformation.MetadataWithStackName(stack.NameForEnv(appName, envName)))
	if err != nil {
		return false, fmt.Errorf("get metadata of stack %q: %w", stack.NameForEnv(appName, envName), err)
	}
	metadata := struct {
		Version string `yaml:"Version"`
	}{}
	if err := yaml.Unmarshal([]byte(raw), &metadata); err != nil {
		return false, fmt.Errorf("unmarshal Metadata property to read Version: %w", err)
	}
	return metadata.Version == version.EnvTemplateBootstrap, nil
}
