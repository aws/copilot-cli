// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/gobuffalo/packd"
)

const (
	lbFargateAppTemplatePath = "lb-fargate-service/cf.yml"
)

type lbFargateTemplateParams struct {
	*deploy.CreateLBFargateAppInput

	// Additional fields needed to render the CloudFormation stack.
	Priority int

	// Field types to override.
	Image struct {
		URL  string
		Port int
	}
}

// LBFargateStackConfig represents the configuration needed to create a CloudFormation stack from a
// load balanced Fargate application.
type LBFargateStackConfig struct {
	*deploy.CreateLBFargateAppInput

	box packd.Box
}

// NewLBFargateStack creates a new LBFargateStackConfig from a load-balanced AWS Fargate application.
func NewLBFargateStack(in *deploy.CreateLBFargateAppInput) *LBFargateStackConfig {
	return &LBFargateStackConfig{
		CreateLBFargateAppInput: in,
		box:                     templates.Box(),
	}
}

// StackName returns the name of the stack.
func (c *LBFargateStackConfig) StackName() string {
	const maxLen = 128
	n := fmt.Sprintf("%s-%s-%s-app", c.Env.Project, c.Env.Name, c.App.Name)

	if len(n) > maxLen {
		return n[len(n)-maxLen:]
	}
	return n
}

// Template returns the CloudFormation template for the application parametrized for the environment.
func (c *LBFargateStackConfig) Template() (string, error) {
	content, err := c.box.FindString(lbFargateAppTemplatePath)
	if err != nil {
		return "", &ErrTemplateNotFound{templateLocation: lbFargateAppTemplatePath, parentErr: err}
	}
	tpl, err := template.New("template").Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse CloudFormation template for %s: %w", c.App.Type, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, c.toTemplateParams()); err != nil {
		return "", fmt.Errorf("execute CloudFormation template for %s: %w", c.App.Type, err)
	}
	return buf.String(), nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (c *LBFargateStackConfig) Parameters() []*cloudformation.Parameter {
	return nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (c *LBFargateStackConfig) SerializedParameters() (string, error) {
	return "", nil
}

// Tags returns the list of tags to apply to the CloudFormation stack.
func (c *LBFargateStackConfig) Tags() []*cloudformation.Tag {
	return nil
}

func (c *LBFargateStackConfig) toTemplateParams() *lbFargateTemplateParams {
	imgLoc := fmt.Sprintf("%s/%s/%s:%s", c.Env.Project, c.Env.Name, c.App.Name, c.ImageTag)
	url := fmt.Sprintf(ecrURLFormatString, c.Env.AccountID, c.Env.Region, imgLoc)
	return &lbFargateTemplateParams{
		CreateLBFargateAppInput: c.CreateLBFargateAppInput,
		Priority:                1, // TODO assign a unique path priority given a path.
		Image: struct {
			URL  string
			Port int
		}{
			URL:  url,
			Port: c.App.Image.Port,
		},
	}
}
