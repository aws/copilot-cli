// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/aws-sdk-go/aws/arn"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/resourcegroups"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
)

const (
	cloudformationResourceType = "AWS::CloudFormation::Stack"
)

type resourceGroupsClient interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]string, error)
}

// EnvDescription contains the information about an environment.
type EnvDescription struct {
	Environment  *archer.Environment   `json:"environment"`
	Applications []*archer.Application `json:"applications"`
	Tags         map[string]string     `json:"tags,omitempty"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	env  *archer.Environment
	proj *archer.Project
	apps []*archer.Application

	store    storeSvc
	rgClient resourceGroupsClient
}

// NewEnvDescriber instantiates an environment describer.
func NewEnvDescriber(projectName string, envName string) (*EnvDescriber, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	env, err := svc.GetEnvironment(projectName, envName)
	if err != nil {
		return nil, err
	}
	proj, err := svc.GetProject(projectName)
	apps, err := svc.ListApplications(projectName)
	if err != nil {
		return nil, err
	}
	sess, err := session.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("assuming role for environment %s: %w", env.ManagerRoleARN, err)
	}
	return &EnvDescriber{
		env:      env,
		store:    svc,
		proj:     proj,
		apps:     apps,
		rgClient: resourcegroups.New(sess),
	}, nil
}

// Describe returns info about a project's environment.
func (e *EnvDescriber) Describe() (*EnvDescription, error) {
	appsForEnv, err := e.filterAppsForEnv()
	if err != nil {
		return nil, err
	}

	return &EnvDescription{
		Environment:  e.env,
		Applications: appsForEnv,
		Tags:         e.proj.Tags,
	}, nil
}

func (e *EnvDescriber) filterAppsForEnv() ([]*archer.Application, error) {
	var appObjects []*archer.Application

	tags := map[string]string{
		stack.EnvTagKey: e.env.Name,
	}
	arns, err := e.rgClient.GetResourcesByTags(cloudformationResourceType, tags)
	if err != nil {
		return nil, err
	}

	stacksOfEnvironment := make(map[string]bool)
	for _, arn := range arns {
		stack, err := e.getStackName(arn)
		if err != nil {
			return nil, err
		}
		stacksOfEnvironment[stack] = true
	}

	for _, app := range e.apps {
		stackName := stack.NameForApp(e.proj.Name, e.env.Name, app.Name)
		if stacksOfEnvironment[stackName] {
			appObjects = append(appObjects, app)
		}
	}
	return appObjects, nil
}

func (e *EnvDescriber) getStackName(resourceArn string) (string, error) {
	parsedArn, err := arn.Parse(resourceArn)
	if err != nil {
		return "", fmt.Errorf("parse ARN %s: %w", resourceArn, err)
	}
	stack := strings.Split(parsedArn.Resource, "/")
	if len(stack) < 2 {
		return "", fmt.Errorf("cannot parse ARN resource %s", parsedArn.Resource)
	}
	return stack[1], nil
}

// JSONString returns the stringified EnvDescription struct with json format.
func (e *EnvDescription) JSONString() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
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
	fmt.Fprintf(writer, color.Bold.Sprint("\nApplications\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", "Type")
	for _, app := range e.Applications {
		fmt.Fprintf(writer, "  %s\t%s\n", app.Name, app.Type)
	}
	writer.Flush()
	if len(e.Tags) != 0 {
		fmt.Fprintf(writer, color.Bold.Sprint("\nTags\n\n"))
		writer.Flush()
		fmt.Fprintf(writer, "  %s\t%s\n", "Key", "Value")
		// sort Tags in alpha order by keys
		keys := make([]string, 0, len(e.Tags))
		for k := range e.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(writer, "  %s\t%s\n", key, e.Tags[key])
		}
	}
	writer.Flush()
	return b.String()
}
