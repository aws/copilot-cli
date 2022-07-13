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
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
)

// CreateAndRenderEnvironment creates the CloudFormation stack for an environment, and render the stack creation to out.
func (cf CloudFormation) CreateAndRenderEnvironment(out progress.FileWriter, env *deploy.CreateEnvironmentInput) error {
	cfnStack, err := cf.toUploadedStack(env.ArtifactBucketARN, stack.NewBootstrapEnvStackConfig(env))
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
	v, err := cf.forceUpdateOutputID(env.App.Name, env.Name)
	if err != nil {
		return err
	}
	cfnStack, err := cf.toUploadedStack(env.ArtifactBucketARN, stack.NewEnvConfigFromExistingStack(env, v))
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(cfnStack)
	}
	descr, err := cf.waitAndDescribeStack(cfnStack.Name)
	if err != nil {
		return err
	}
	params, err := cf.transformParameters(cfnStack.Parameters, descr.Parameters, transformEnvControllerParameters)
	if err != nil {
		return err
	}
	cfnStack.Parameters = params

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
	conf := stack.NewBootstrapEnvStackConfig(&deploy.CreateEnvironmentInput{
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

func (cf CloudFormation) forceUpdateOutputID(app, env string) (string, error) {
	stackDescr, err := cf.waitAndDescribeStack(stack.NameForEnv(app, env))
	if err != nil {
		return "", err
	}
	for _, output := range stackDescr.Outputs {
		if aws.StringValue(output.OutputKey) == template.DeploymentControllerOutputName {
			return aws.StringValue(output.OutputValue), nil
		}
	}
	return "", nil
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
	return cf.upgradeEnvironment(in, func(new awscfn.Parameter, old *awscfn.Parameter) *awscfn.Parameter {
		// If a parameter exists in both the new template and the old template, use its previous value.
		// Otherwise, keep the parameter untouched.
		if old == nil { // The ParamKey doesn't exist in the old stack, use the new value.
			return &new
		}
		// Use existing parameter values.
		return &awscfn.Parameter{
			ParameterKey:     new.ParameterKey,
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
	return cf.upgradeEnvironment(in, func(new awscfn.Parameter, old *awscfn.Parameter) *awscfn.Parameter {
		// If a parameter exists in both the new template and the old template, use its previous value.
		// Otherwise, if the parameter is `EnvParamALBWorkloadsKey` (currently "ALBWorkloads"), assign it a parameter value.
		// Otherwise, keep the parameter untouched.
		if aws.StringValue(new.ParameterKey) == stack.EnvParamALBWorkloadsKey {
			return &awscfn.Parameter{
				ParameterKey:   aws.String(stack.EnvParamALBWorkloadsKey),
				ParameterValue: aws.String(strings.Join(lbWebServices, ",")),
			}
		}
		if old == nil {
			return &new // The ParamKey doesn't exist in the old stack, use the new value.
		}
		return &awscfn.Parameter{
			ParameterKey:     new.ParameterKey,
			UsePreviousValue: aws.Bool(true),
		}
	})
}

func (cf CloudFormation) upgradeEnvironment(in *deploy.CreateEnvironmentInput, transformParam func(new awscfn.Parameter, old *awscfn.Parameter) *awscfn.Parameter) error {
	s, err := cf.toUploadedStack(in.ArtifactBucketARN, stack.NewEnvStackConfig(in))
	if err != nil {
		return err
	}

	for {
		var (
			descr *cloudformation.StackDescription
			err   error
		)
		descr, err = cf.waitAndDescribeStack(s.Name)
		if err != nil {
			return err
		}

		params, err := cf.transformParameters(s.Parameters, descr.Parameters, transformParam)
		if err != nil {
			return err
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
	if cf.cachedDeployedStack != nil {
		return cf.cachedDeployedStack, nil
	}
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
	cf.cachedDeployedStack = stackDescription
	return stackDescription, err
}

// transformParameters removes or transforms each of the current parameters and does not add any new parameters.
// This means that parameters that exist only in the old template are left out.
// The parameter`transform` is a function that transform a parameter, given its value in the new template and the old template.
// If `old` is `nil`, the parameter does not exist in the old template.
// `transform` should return `nil` if caller intends to delete the parameter.
func (cf CloudFormation) transformParameters(
	currParams []*awscfn.Parameter,
	oldParams []*awscfn.Parameter,
	transform func(new awscfn.Parameter, old *awscfn.Parameter) *awscfn.Parameter) ([]*awscfn.Parameter, error) {

	// Make a map out of `currParams` and out of `oldParams`.
	curr := make(map[string]awscfn.Parameter)
	for _, p := range currParams {
		curr[aws.StringValue(p.ParameterKey)] = *p
	}
	old := make(map[string]*awscfn.Parameter)
	for _, p := range oldParams {
		old[aws.StringValue(p.ParameterKey)] = p
	}

	// Remove or transform each of the current parameters.
	var params []*awscfn.Parameter
	for k, p := range curr {
		if transformed := transform(p, old[k]); transformed != nil {
			params = append(params, transformed)
		}
	}
	return params, nil
}

// transformEnvControllerParameters transforms a parameter such that it uses its previous value if:
// 1. The parameter exists in the old template.
// 2. The parameter is env-controller managed.
// Otherwise, it returns the parameter untouched.
func transformEnvControllerParameters(new awscfn.Parameter, old *awscfn.Parameter) *awscfn.Parameter {
	if old == nil { // The ParamKey doesn't exist in the old stack, use the new value.
		return &new
	}

	var (
		isEnvControllerManaged = make(map[string]struct{})
		exists                 = struct{}{}
	)
	for _, f := range template.AvailableEnvFeatures() {
		isEnvControllerManaged[f] = exists
	}
	if _, ok := isEnvControllerManaged[aws.StringValue(new.ParameterKey)]; !ok {
		return &new
	}
	return &awscfn.Parameter{
		ParameterKey:     new.ParameterKey,
		UsePreviousValue: aws.Bool(true),
	}
}
