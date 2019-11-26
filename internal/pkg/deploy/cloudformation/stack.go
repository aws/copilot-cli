// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// DeleteStack calls the CloudFormation DeleteStack API with the input stack name.
func (cf CloudFormation) DeleteStack(stackName string) error {
	if _, err := cf.client.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	}); err != nil {
		return fmt.Errorf("delete stack %s: %w", stackName, err)
	}

	return nil
}

// WaitForStackDelete calls the CloudFormation WaitUntilStackDeleteComplete API with the input stack name.
func (cf CloudFormation) WaitForStackDelete(stackName string) error {
	if err := cf.client.WaitUntilStackDeleteComplete(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}); err != nil {
		return fmt.Errorf("wait until stack delete complete %s: %w", stackName, err)
	}

	return nil
}

// DeleteStackAndWait orchestrates a call to DeleteStack followed by a call to WaitForStackDelete.
func (cf CloudFormation) DeleteStackAndWait(stackName string) error {
	if err := cf.DeleteStack(stackName); err != nil {
		return err
	}

	return cf.WaitForStackDelete(stackName)
}
