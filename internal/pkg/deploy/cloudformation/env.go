// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy ECS resources with AWS CloudFormation.
package cloudformation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

// Environment stack's parameters that need to updated while moving the legacy template to a newer version.
const (
	includeLoadBalancerParamKey = "IncludePublicLoadBalancer"
	albWorkloadsParamKey        = "ALBWorkloads"
)

// DeployEnvironment creates the CloudFormation stack for an environment by creating and executing a change set.
//
// If the deployment succeeds, returns nil.
// If the stack already exists, returns a ErrStackAlreadyExists.
// If the change set to create the stack cannot be executed, returns a ErrNotExecutableChangeSet.
// Otherwise, returns a wrapped error.
func (cf CloudFormation) DeployEnvironment(env *deploy.CreateEnvironmentInput) error {
	s, err := toStack(stack.NewEnvStackConfig(env))
	if err != nil {
		return err
	}
	return cf.cfnClient.Create(s)
}

// StreamEnvironmentCreation streams resource update events while a deployment is taking place.
// Once the CloudFormation stack operation halts, the update channel is closed and a
// CreateEnvironmentResponse is sent to the second channel.
// Deprecated: Use stream.Stream with a stream.StackStreamer instead.
func (cf CloudFormation) StreamEnvironmentCreation(env *deploy.CreateEnvironmentInput) (<-chan []deploy.ResourceEvent, <-chan deploy.CreateEnvironmentResponse) {
	done := make(chan struct{})
	events := make(chan []deploy.ResourceEvent)
	resp := make(chan deploy.CreateEnvironmentResponse, 1)

	stack := stack.NewEnvStackConfig(env)
	go cf.streamResourceEvents(done, events, stack.StackName())
	go cf.streamEnvironmentResponse(done, resp, stack)
	return events, resp
}

// DeleteEnvironment deletes the CloudFormation stack of an environment.
func (cf CloudFormation) DeleteEnvironment(appName, envName, cfnExecRoleARN string) error {
	conf := stack.NewEnvStackConfig(&deploy.CreateEnvironmentInput{
		AppName: appName,
		Name:    envName,
	})
	return cf.cfnClient.DeleteAndWaitWithRoleARN(conf.StackName(), cfnExecRoleARN)
}

// streamEnvironmentResponse sends a CreateEnvironmentResponse to the response channel once the stack creation halts.
// The done channel is closed once this method exits to notify other streams that they should stop working.
func (cf CloudFormation) streamEnvironmentResponse(done chan struct{}, resp chan deploy.CreateEnvironmentResponse, stack *stack.EnvStackConfig) {
	defer close(done)
	if err := cf.cfnClient.WaitForCreate(stack.StackName()); err != nil {
		resp <- deploy.CreateEnvironmentResponse{Err: err}
		return
	}
	descr, err := cf.cfnClient.Describe(stack.StackName())
	if err != nil {
		resp <- deploy.CreateEnvironmentResponse{Err: err}
		return
	}
	env, err := stack.ToEnv(descr.SDK())
	resp <- deploy.CreateEnvironmentResponse{
		Env: env,
		Err: err,
	}
}

// GetEnvironment returns the Environment metadata from the CloudFormation stack.
func (cf CloudFormation) GetEnvironment(appName, envName string) (*config.Environment, error) {
	conf := stack.NewEnvStackConfig(&deploy.CreateEnvironmentInput{
		AppName: appName,
		Name:    envName,
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
				ParameterKey:   aws.String(albWorkloadsParamKey),
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
	s, err := toStack(stack.NewEnvStackConfig(in))
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
			_ = cf.cfnClient.WaitForUpdate(s.Name)
			continue
		}

		// Keep the parameters and tags of the stack.
		var params []*awscfn.Parameter
		for _, param := range descr.Parameters {
			params = append(params, transformParam(param))
		}
		s.Parameters = params
		s.Tags = descr.Tags

		// Apply a service role if provided.
		if in.CFNServiceRoleARN != "" {
			s.RoleARN = aws.String(in.CFNServiceRoleARN)
		}

		// Attempt to update the stack template.
		err = cf.cfnClient.UpdateAndWait(s)
		if err == nil { // Success.
			break
		}

		var emptyChangeSet *cloudformation.ErrChangeSetEmpty
		var alreadyInProgErr *cloudformation.ErrStackUpdateInProgress
		var obsoleteChangeSetErr *cloudformation.ErrChangeSetNotExecutable
		switch updateErr := err; {
		case errors.As(updateErr, &emptyChangeSet):
			// The changes are already applied, nothing to do. Exit successfully.
			return nil
		case errors.As(updateErr, &alreadyInProgErr):
			// There is another update going on in the environment, retry the upgrade.
			continue
		case errors.As(updateErr, &obsoleteChangeSetErr):
			// If there are two "upgradeEnvironments" calls happening in parallel, it's possible that
			// both invocations created a changeset to upgrade the environment stack.
			// CloudFormation will ensure that one of them goes through, while the other returns
			// an ErrChangeSetNotExecutable error.
			//
			// In that scenario, we should loop again, wait until the stack is updated,
			// and exit due to changeset is empty.
			continue
		default:
			return fmt.Errorf("update and wait for stack %s: %w", s.Name, err)
		}
	}
	return nil
}
