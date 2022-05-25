// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy ECS resources with AWS CloudFormation.
package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
)

// Environment stack's parameters that need to updated while moving the legacy template to a newer version.
const (
	includeLoadBalancerParamKey = "IncludePublicLoadBalancer"
)

// CreateAndRenderEnvironment creates the CloudFormation stack for an environment, and render the stack creation to out.
func (cf CloudFormation) CreateAndRenderEnvironment(out progress.FileWriter, env *deploy.CreateEnvironmentInput) error {
	cfnStack, err := cf.environmentStack(env)
	if err != nil {
		return err
	}
	in := newRenderEnvironmentInput(out, cfnStack)
	in.createChangeSet = func() (changeSetID string, err error) {
		spinner := progress.NewSpinner(out)
		label := fmt.Sprintf("Proposing infrastructure changes for the %s environment.", cfnStack.Name)
		spinner.Start(label)
		defer stopSpinner(spinner, err, label)
		changeSetID, err = cf.cfnClient.Create(cfnStack)
		if err != nil {
			return "", err
		}
		return changeSetID, nil
	}
	return cf.renderStackChanges(in)
}

// UpdateAndRenderEnvironment updates the CloudFormation stack for an environment, and render the stack creation to out.
func (cf CloudFormation) UpdateAndRenderEnvironment(out progress.FileWriter, env *deploy.CreateEnvironmentInput, opts ...cloudformation.StackOption) error {
	cfnStack, err := cf.environmentStack(env)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(cfnStack)
	}
	in := newRenderEnvironmentInput(out, cfnStack)
	in.createChangeSet = func() (changeSetID string, err error) {
		spinner := progress.NewSpinner(out)
		label := fmt.Sprintf("Proposing infrastructure changes for the %s environment.", cfnStack.Name)
		spinner.Start(label)
		defer stopSpinner(spinner, err, label)
		changeSetID, err = cf.cfnClient.Update(cfnStack)
		if err != nil {
			return "", err
		}
		return changeSetID, nil
	}
	return cf.renderStackChanges(in)
}

func (cf CloudFormation) environmentStack(env *deploy.CreateEnvironmentInput) (*cloudformation.Stack, error) {
	bucketARN, err := arn.Parse(env.ArtifactBucketARN)
	if err != nil {
		return nil, err
	}
	stackConfig := stack.NewEnvStackConfig(env)
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

func newRenderEnvironmentInput(out progress.FileWriter, cfnStack *cloudformation.Stack) *renderStackChangesInput {
	return &renderStackChangesInput{
		w:                out,
		stackName:        cfnStack.Name,
		stackDescription: fmt.Sprintf("Creating the infrastructure for the %s environment.", cfnStack.Name),
	}
}

// DeleteEnvironment deletes the CloudFormation stack of an environment.
func (cf CloudFormation) DeleteEnvironment(appName, envName, cfnExecRoleARN string) error {
	conf := stack.NewEnvStackConfig(&deploy.CreateEnvironmentInput{
		App: deploy.AppInformation{
			Name: appName,
		},
		Name: envName,
	})
	return cf.cfnClient.DeleteAndWaitWithRoleARN(conf.StackName(), cfnExecRoleARN)
}

// GetEnvironment returns the Environment metadata from the CloudFormation stack.
func (cf CloudFormation) GetEnvironment(appName, envName string) (*config.Environment, error) {
	conf := stack.NewEnvStackConfig(&deploy.CreateEnvironmentInput{
		App: deploy.AppInformation{
			Name: appName,
		},
		Name: envName,
	})
	descr, err := cf.cfnClient.Describe(conf.StackName())
	if err != nil {
		return nil, err
	}
	return conf.ToEnv(descr.SDK())
}

// EnvironmentTemplate returns the environment's stack's template.
func (cf CloudFormation) EnvironmentTemplate(appName, envName string) (string, error) {
	stackName := stack.NameForEnv(appName, envName)
	return cf.cfnClient.TemplateBody(stackName)
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

// UpgradeEnvironment updates an environment stack's template to a newer version.
func (cf CloudFormation) UpgradeEnvironment(in *deploy.CreateEnvironmentInput) error {
	return cf.upgradeEnvironment(in, func(param *awscfn.Parameter) *awscfn.Parameter {
		// Use existing parameter values.
		return &awscfn.Parameter{
			ParameterKey:     param.ParameterKey,
			UsePreviousValue: aws.Bool(true),
		}
	})
}

// UpgradeLegacyEnvironment updates a legacy environment stack to a newer version.
//
// UpgradeEnvironment and UpgradeLegacyEnvironment are separate methods because the legacy cloudformation stack has the
// "IncludePublicLoadBalancer" parameter which has been deprecated in favor of the "ALBWorkloads".
// UpgradeLegacyEnvironment does the necessary transformation to use the "ALBWorkloads" parameter instead.
func (cf CloudFormation) UpgradeLegacyEnvironment(in *deploy.CreateEnvironmentInput, lbWebServices ...string) error {
	return cf.upgradeEnvironment(in, func(param *awscfn.Parameter) *awscfn.Parameter {
		if aws.StringValue(param.ParameterKey) == includeLoadBalancerParamKey {
			// "IncludePublicLoadBalancer" has been deprecated in favor of "ALBWorkloads".
			// We need to populate this parameter so that the env ALB is not deleted.
			return &awscfn.Parameter{
				ParameterKey:   aws.String(stack.EnvParamALBWorkloadsKey),
				ParameterValue: aws.String(strings.Join(lbWebServices, ",")),
			}
		}
		return &awscfn.Parameter{
			ParameterKey:     param.ParameterKey,
			UsePreviousValue: aws.Bool(true),
		}
	})
}

func (cf CloudFormation) upgradeEnvironment(in *deploy.CreateEnvironmentInput, transformParam func(param *awscfn.Parameter) *awscfn.Parameter) error {
	bucketARN, err := arn.Parse(in.ArtifactBucketARN)
	if err != nil {
		return err
	}
	stackConfig := stack.NewEnvStackConfig(in)
	url, err := cf.uploadStackTemplateToS3(bucketARN.Resource, stackConfig)
	if err != nil {
		return err
	}
	s, err := toStackFromS3(stackConfig, url)
	if err != nil {
		return err
	}

	for {
		descr, err := cf.cfnClient.Describe(s.Name)
		if err != nil {
			return fmt.Errorf("describe stack %s: %w", s.Name, err)
		}

		if cloudformation.StackStatus(aws.StringValue(descr.StackStatus)).InProgress() {
			// There is already an update happening to the environment stack.
			// Best-effort try to wait for the existing update to be over before retrying.
			_ = cf.cfnClient.WaitForUpdate(context.Background(), s.Name)
			continue
		}

		// Remove params that only exist in old template; use previous values for params that
		// exist in both old and new template; use new values for params that only exist in new template.
		paramSet := make(map[string]*awscfn.Parameter)
		for _, param := range s.Parameters {
			paramSet[aws.StringValue(param.ParameterKey)] = param
		}
		for _, param := range descr.Parameters {
			param = transformParam(param)
			paramKey := aws.StringValue(param.ParameterKey)
			if _, ok := paramSet[paramKey]; !ok {
				continue
			}
			paramSet[paramKey] = param
		}
		var params []*awscfn.Parameter
		for _, param := range paramSet {
			params = append(params, param)
		}
		s.Parameters = params

		// Keep the tags of the stack.
		s.Tags = descr.Tags

		// Apply a service role if provided.
		if in.CFNServiceRoleARN != "" {
			s.RoleARN = aws.String(in.CFNServiceRoleARN)
		}

		// Attempt to update the stack template.
		err = cf.cfnClient.UpdateAndWait(s)
		if err == nil { // Success.
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
