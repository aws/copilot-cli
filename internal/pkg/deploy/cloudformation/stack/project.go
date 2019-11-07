// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"gopkg.in/yaml.v3"

	"github.com/gobuffalo/packd"
)

// DeployedProjectMetadata wraps the Metadata field of a deployed
// project StackSet.
type DeployedProjectMetadata struct {
	Metadata ProjectResourcesConfig `yaml:"Metadata"`
}

// ProjectResourcesConfig is a configuration for a deployed Project
// StackSet.
type ProjectResourcesConfig struct {
	Accounts []string `yaml:"Accounts,flow"`
	Apps     []string `yaml:"Apps,flow"`
	Project  string   `yaml:"Project"`
	Version  int      `yaml:"Version"`
}

// ProjectStackConfig is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type ProjectStackConfig struct {
	*deploy.CreateProjectInput
	box packd.Box
}

const (
	projectTemplatePath           = "project/project.yml"
	projectResourcesTemplatePath  = "project/cf.yml"
	projectAdminRoleParamName     = "AdminRoleName"
	projectExecutionRoleParamName = "ExecutionRoleName"
)

// ProjectConfigFrom takes a template file and extracts the metadata block,
// and parses it into a projectStackConfig
func ProjectConfigFrom(template *string) (*ProjectResourcesConfig, error) {
	resourceConfig := DeployedProjectMetadata{}
	err := yaml.Unmarshal([]byte(*template), &resourceConfig)
	return &resourceConfig.Metadata, err
}

// NewProjectStackConfig sets up a struct which can provide values to CloudFormation for
// spinning up an environment.
func NewProjectStackConfig(in *deploy.CreateProjectInput, box packd.Box) *ProjectStackConfig {
	return &ProjectStackConfig{
		CreateProjectInput: in,
		box:                box,
	}
}

// Template returns the environment CloudFormation template.
func (c *ProjectStackConfig) Template() (string, error) {
	template, err := c.box.FindString(projectTemplatePath)
	if err != nil {
		return "", &ErrTemplateNotFound{templateLocation: projectTemplatePath, parentErr: err}
	}
	return template, nil
}

// ResourceTemplate generates a StackSet template with all the Project-wide resources (ECR Repos, KMS keys, S3 buckets)
func (c *ProjectStackConfig) ResourceTemplate(config *ProjectResourcesConfig) (string, error) {
	stackSetTemplate, err := c.box.FindString(projectResourcesTemplatePath)
	if err != nil {
		return "", &ErrTemplateNotFound{templateLocation: projectResourcesTemplatePath, parentErr: err}
	}

	template, err := template.New("resourcetemplate").
		Funcs(template.FuncMap{
			"logicalIDSafe": func(logicalID string) string {
				return strings.ReplaceAll(logicalID, "-", "DASH")
			},
		}).
		Parse(stackSetTemplate)
	if err != nil {
		return "", err
	}
	// Sort the account IDs and Apps so that the template we generate is deterministic
	sort.Strings(config.Accounts)
	sort.Strings(config.Apps)

	var buf bytes.Buffer
	if err := template.Execute(&buf, config); err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}

// Parameters returns the parameters to be passed into a environment CloudFormation template.
func (c *ProjectStackConfig) Parameters() []*cloudformation.Parameter {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(projectAdminRoleParamName),
			ParameterValue: aws.String(c.stackSetAdminRoleName()),
		},
		{
			ParameterKey:   aws.String(projectExecutionRoleParamName),
			ParameterValue: aws.String(c.StackSetExecutionRoleName()),
		},
	}
}

// Tags returns the tags that should be applied to the project CloudFormation stack.
func (c *ProjectStackConfig) Tags() []*cloudformation.Tag {
	return []*cloudformation.Tag{
		{
			Key:   aws.String(projectTagKey),
			Value: aws.String(c.Project),
		},
	}
}

// StackName returns the name of the CloudFormation stack (based on the project name).
func (c *ProjectStackConfig) StackName() string {
	return fmt.Sprintf("%s-infrastructure-roles", c.Project)
}

// StackSetName returns the name of the CloudFormation StackSet (based on the project name).
func (c *ProjectStackConfig) StackSetName() string {
	return fmt.Sprintf("%s-infrastructure", c.Project)
}

// StackSetDescription returns the description of the StackSet for project resources.
func (c *ProjectStackConfig) StackSetDescription() string {
	return "ECS CLI Project Resources (ECR repos, KMS keys, S3 buckets)"
}

func (c *ProjectStackConfig) stackSetAdminRoleName() string {
	return fmt.Sprintf("%s-adminrole", c.Project)
}

// StackSetAdminRoleARN returns the role ARN of the role used to administer the Project
// StackSet.
func (c *ProjectStackConfig) StackSetAdminRoleARN() string {
	//TODO find a partition-neutral way to construct this ARN
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", c.AccountID, c.stackSetAdminRoleName())
}

// StackSetExecutionRoleName returns the role name of the role used to actually create
// Project resources.
func (c *ProjectStackConfig) StackSetExecutionRoleName() string {
	return fmt.Sprintf("%s-executionrole", c.Project)
}
