// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"fmt"

	"github.com/awslabs/goformation/v4"
	"github.com/awslabs/goformation/v4/cloudformation/iam"
	"github.com/awslabs/goformation/v4/cloudformation/secretsmanager"
	"github.com/awslabs/goformation/v4/intrinsics"
)

// Output represents an output from a CloudFormation template.
type Output struct {
	// Name is the Logical ID of the output.
	Name string
	// IsSecret is true if the output value refers to a SecretsManager ARN. Otherwise, false.
	IsSecret bool
	// IsManagedPolicy is true if the output value refers to an IAM ManagedPolicy ARN. Otherwise, false.
	IsManagedPolicy bool
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
			IsSecret:        isSecret(output, tpl.GetAllSecretsManagerSecretResources()),
			IsManagedPolicy: isManagedPolicy(output, tpl.GetAllIAMManagedPolicyResources()),
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
