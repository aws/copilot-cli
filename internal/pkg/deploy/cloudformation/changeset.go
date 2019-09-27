// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/google/uuid"
)

// changeSet represents a CloudFormation Change Set
// See https://aws.amazon.com/blogs/aws/new-change-sets-for-aws-cloudformation/
type changeSet struct {
	name    string
	stackID string
}

func (set *changeSet) String() string {
	return fmt.Sprintf("name=%s, stackID=%s", set.name, set.stackID)
}

// createChangeSetOpt is a functional option to add additional settings to a CreateChangeSetInput.
type createChangeSetOpt func(in *cloudformation.CreateChangeSetInput)

func createChangeSetInput(stackName, templateBody string, options ...createChangeSetOpt) (*cloudformation.CreateChangeSetInput, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate random id for changeSet: %w", err)
	}

	// The change set name must match the regex [a-zA-Z][-a-zA-Z0-9]*. The generated UUID can start with a number,
	// by prefixing the uuid with a word we guarantee that we start with a letter.
	name := fmt.Sprintf("%s-%s", "ecscli", id.String())
	in := &cloudformation.CreateChangeSetInput{
		Capabilities:  []*string{aws.String(cloudformation.CapabilityCapabilityIam)},
		ChangeSetName: aws.String(name),
		StackName:     aws.String(stackName),
		TemplateBody:  aws.String(templateBody),
	}
	for _, option := range options {
		option(in)
	}
	return in, nil
}

func withParameters(params []*cloudformation.Parameter) createChangeSetOpt {
	return func(in *cloudformation.CreateChangeSetInput) {
		in.Parameters = params
	}
}

func withCreateChangeSetType() createChangeSetOpt {
	return func(in *cloudformation.CreateChangeSetInput) {
		in.ChangeSetType = aws.String(cloudformation.ChangeSetTypeCreate)
	}
}
