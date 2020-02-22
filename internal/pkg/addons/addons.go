// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addons contains the service to manage addons.
package addons

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/awslabs/goformation/v4"
	"github.com/awslabs/goformation/v4/cloudformation/iam"
	"github.com/awslabs/goformation/v4/cloudformation/secretsmanager"
	"github.com/awslabs/goformation/v4/intrinsics"
)

const (
	addonsTemplatePath = "addons/cf.yml"
)

type workspaceService interface {
	ReadAddonFiles(appName string) (*workspace.AddonFiles, error)
}

// Addons represents additional resources for an application.
type Addons struct {
	appName string

	parser template.Parser
	ws     workspaceService
}

// New creates an Addons object given an application name.
func New(appName string) (*Addons, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}
	return &Addons{
		appName: appName,
		parser:  template.New(),
		ws:      ws,
	}, nil
}

// Template merges the files under the "addons/" directory of an application
// into a single CloudFormation template and returns it.
func (a *Addons) Template() (string, error) {
	out, err := a.ws.ReadAddonFiles(a.appName)
	if err != nil {
		return "", err
	}
	content, err := a.parser.Parse(addonsTemplatePath, struct {
		AppName      string
		AddonContent *workspace.AddonFiles
	}{
		AppName:      a.appName,
		AddonContent: out,
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Outputs parses the Outputs section of a CloudFormation template to extract logical IDs and returns them.
func Outputs(template string) ([]Output, error) {
	// goformation needs to evaluate CFN intrinsic functions to render the template.
	// However, by default "Ref" evaluates to nil and results in the deletion of the field.
	//
	// Instead, we want to retain the input logical ID for "Ref" so that we can check if an output refers
	// to a particular CFN resource type.
	tpl, err := goformation.ParseYAMLWithOptions([]byte(template), &intrinsics.ProcessorOptions{
		IntrinsicHandlerOverrides: map[string]intrinsics.IntrinsicHandler{
			// Given an output with "Value: !Ref AdditionalResourcesPolicy",
			// this override evaluates to "Value: AdditionalResourcesPolicy".
			"Ref": func(_ string, input interface{}, _ interface{}) interface{} {
				if logicalID, ok := input.(string); ok {
					return logicalID
				}
				return nil
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("parse CloudFormation template %s: %w", template, err)
	}

	var outputs []Output
	for logicalID, output := range tpl.Outputs {
		outputs = append(outputs, Output{
			Name:            logicalID,
			isSecret:        isSecret(output, tpl.GetAllSecretsManagerSecretResources()),
			isManagedPolicy: isManagedPolicy(output, tpl.GetAllIAMManagedPolicyResources()),
		})
	}
	return outputs, nil
}

func isSecret(output interface{}, secrets map[string]*secretsmanager.Secret) bool {
	props, ok := output.(map[string]interface{})
	if !ok {
		return false
	}
	value, ok := props["Value"].(string)
	if !ok {
		return false
	}
	_, hasKey := secrets[value]
	return hasKey
}

func isManagedPolicy(output interface{}, policies map[string]*iam.ManagedPolicy) bool {
	props, ok := output.(map[string]interface{})
	if !ok {
		return false
	}
	value, ok := props["Value"].(string)
	if !ok {
		return false
	}
	_, hasKey := policies[value]
	return hasKey
}
