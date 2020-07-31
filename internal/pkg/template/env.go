// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
)

// Paths of service cloudformation templates under templates/services/.
const (
	// EnvCFTemplatePath is the path where the cloudformation for the environment is written.
	EnvCFTemplatePath     = "environment/cf.yml"
	envNestCFTemplatePath = "environment/cf/%s.yml"
)

var (
	// Template names under "environment/cf/".
	nestEnvCFTemplateNames = []string{
		"cfn-execution-role",
		"custom-resources",
		"custom-resources-role",
		"environment-manager-role",
		"lambdas",
		"vpc-resources",
	}
)

// EnvOpts holds data that can be provided to enable features in an environment stack template.
type EnvOpts struct {
	DNSDelegationLambda       string
	ACMValidationLambda       string
	EnableLongARNFormatLambda string
	ImportVpc                 *ImportVpcOpts
	VpcConfig                 *AdjustVpcOpts
}

// ImportVpcOpts holds the fields to import VPC resources.
type ImportVpcOpts struct {
	ID               string // ID for the VPC.
	PublicSubnetIDs  []string
	PrivateSubnetIDs []string
}

// AdjustVpcOpts holds the fields to adjust default VPC resources.
type AdjustVpcOpts struct {
	CIDR               string // CIDR range for the VPC.
	PublicSubnetCIDRs  []string
	PrivateSubnetCIDRs []string
}

// ParseEnv parses an environment's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseEnv(data interface{}, options ...ParseOption) (*Content, error) {
	tpl, err := t.parse("base", EnvCFTemplatePath, options...)
	if err != nil {
		return nil, err
	}
	for _, templateName := range nestEnvCFTemplateNames {
		nestedTpl, err := t.parse(templateName, fmt.Sprintf(envNestCFTemplatePath, templateName), options...)
		if err != nil {
			return nil, err
		}
		_, err = tpl.AddParseTree(templateName, nestedTpl.Tree)
		if err != nil {
			return nil, fmt.Errorf("add parse tree of %s to base template: %w", templateName, err)
		}
	}
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("execute environment template with data %v: %w", data, err)
	}
	return &Content{buf}, nil
}
