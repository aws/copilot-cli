// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"

	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
)

const (
	cloudformationResourceType = "AWS::CloudFormation::Stack"
)

type resourceGroupsClient interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]string, error)
}

// EnvDescription contains the information about an environment.
type EnvDescription struct {
	Environment *config.Environment `json:"environment"`
	Services    []*config.Service   `json:"services"`
	Tags        map[string]string   `json:"tags,omitempty"`
	Resources   []*CfnResource      `json:"resources,omitempty"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	app             *config.Application
	env             *config.Environment
	svcs            []*config.Service
	enableResources bool

	store          storeSvc
	rgClient       resourceGroupsClient
	stackDescriber stackAndResourcesDescriber
}

// NewEnvDescriber instantiates an environment describer.
func NewEnvDescriber(appName, envName string) (*EnvDescriber, error) {
	store, err := config.NewStore()

	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	env, err := store.GetEnvironment(appName, envName)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}
	app, err := store.GetApplication(appName)
	if err != nil {
		return nil, fmt.Errorf("get application %s: %w", appName, err)
	}
	svcs, err := store.ListServices(appName)
	if err != nil {
		return nil, err
	}
	sess, err := session.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("assuming role for environment %s: %w", env.ManagerRoleARN, err)
	}
	d := newStackDescriber(sess)
	return &EnvDescriber{
		app:             app,
		env:             env,
		svcs:            svcs,
		enableResources: false,

		store:          store,
		rgClient:       resourcegroups.New(sess),
		stackDescriber: stackAndResourcesDescriber(d),
	}, nil
}

// NewEnvDescriberWithResources instantiates an environment describer with stack resources.
func NewEnvDescriberWithResources(appName, envName string) (*EnvDescriber, error) {
	d, err := NewEnvDescriber(appName, envName)
	if err != nil {
		return nil, err
	}
	d.enableResources = true
	return d, nil
}

// Describe returns info about an application's environment.
func (e *EnvDescriber) Describe() (*EnvDescription, error) {
	svcs, err := e.filterSvcsForEnv()
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)
	envStack, err := e.stackDescriber.Stack(stack.NameForEnv(e.app.Name, e.env.Name))
	if err != nil {
		return nil, err
	}
	for _, tag := range envStack.Tags {
		tags[*tag.Key] = *tag.Value
	}

	var stackResources []*CfnResource
	if e.enableResources {
		stackResources, err = e.envOutputs()
		if err != nil {
			return nil, err
		}
	}

	return &EnvDescription{
		Environment: e.env,
		Services:    svcs,
		Tags:        tags,
		Resources:   stackResources,
	}, nil
}

func (e *EnvDescriber) filterSvcsForEnv() ([]*config.Service, error) {
	tags := map[string]string{
		stack.EnvTagKey: e.env.Name,
	}
	arns, err := e.rgClient.GetResourcesByTags(cloudformationResourceType, tags)
	if err != nil {
		return nil, fmt.Errorf("get %s resources for env %s: %w", cloudformationResourceType, e.env.Name, err)
	}

	stacksOfEnvironment := make(map[string]bool)
	for _, arn := range arns {
		stack, err := e.getStackName(arn)
		if err != nil {
			return nil, err
		}
		stacksOfEnvironment[stack] = true
	}
	var svcs []*config.Service
	for _, svc := range e.svcs {
		stackName := stack.NameForService(e.app.Name, e.env.Name, svc.Name)
		if stacksOfEnvironment[stackName] {
			svcs = append(svcs, svc)
		}
	}
	return svcs, nil
}

func (e *EnvDescriber) getStackName(resourceArn string) (string, error) {
	parsedArn, err := arn.Parse(resourceArn)
	if err != nil {
		return "", fmt.Errorf("parse ARN %s: %w", resourceArn, err)
	}
	stack := strings.Split(parsedArn.Resource, "/")
	if len(stack) < 2 {
		return "", fmt.Errorf("invalid ARN resource format %s. Ex: arn:partition:service:region:account-id:resource-type/resource-id", parsedArn.Resource)
	}
	return stack[1], nil
}

func (e *EnvDescriber) envOutputs() ([]*CfnResource, error) {
	envStack, err := e.stackDescriber.StackResources(stack.NameForEnv(e.app.Name, e.env.Name))
	if err != nil {
		return nil, fmt.Errorf("retrieve environment resources: %w", err)
	}
	outputs := flattenResources(envStack)
	return outputs, nil
}

// JSONString returns the stringified EnvDescription struct with json format.
func (e *EnvDescription) JSONString() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("marshal environment description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified EnvDescription struct with human readable format.
func (e *EnvDescription) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", e.Environment.Name)
	fmt.Fprintf(writer, "  %s\t%t\n", "Production", e.Environment.Prod)
	fmt.Fprintf(writer, "  %s\t%s\n", "Region", e.Environment.Region)
	fmt.Fprintf(writer, "  %s\t%s\n", "Account ID", e.Environment.AccountID)
	fmt.Fprintf(writer, color.Bold.Sprint("\nServices\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", "Type")
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "----", "----")
	writer.Flush()
	for _, svc := range e.Services {
		fmt.Fprintf(writer, "  %s\t%s\n", svc.Name, svc.Type)
	}
	writer.Flush()
	if len(e.Tags) != 0 {
		fmt.Fprintf(writer, color.Bold.Sprint("\nTags\n\n"))
		writer.Flush()
		fmt.Fprintf(writer, "  %s\t%s\n", "Key", "Value")
		fmt.Fprintf(writer, "  %s\t%s\n", "---", "-----")
		// sort Tags in alpha order by keys
		keys := make([]string, 0, len(e.Tags))
		for k := range e.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(writer, "  %s\t%s\n", key, e.Tags[key])
			writer.Flush()
		}
	}
	writer.Flush()
	if len(e.Resources) != 0 {
		fmt.Fprintf(writer, color.Bold.Sprint("\nResources\n\n"))
		writer.Flush()
		for _, resource := range e.Resources {
			fmt.Fprintf(writer, "  %s\t%s\n", resource.Type, resource.PhysicalID)
		}
	}
	writer.Flush()
	return b.String()
}
