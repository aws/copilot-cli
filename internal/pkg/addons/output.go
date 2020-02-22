// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

// Output represents an output from a CloudFormation template.
type Output struct {
	// Name is the Logical ID of the output.
	Name string

	isSecret        bool
	isManagedPolicy bool
}

// IsSecret returns true if the output value refers to a SecretsManager ARN. Otherwise, returns false.
func (o Output) IsSecret() bool {
	return o.isSecret
}

// IsManagedPolicy returns true if the output value refers to an IAM ManagedPolicy ARN. Otherwise, returns false.
func (o Output) IsManagedPolicy() bool {
	return o.isManagedPolicy
}
