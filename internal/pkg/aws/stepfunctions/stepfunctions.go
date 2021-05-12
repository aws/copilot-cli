// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package stepfunctions provides a client to make API requests to Amazon Step Functions.
package stepfunctions

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
)

type api interface {
	DescribeStateMachine(input *sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error)
}

// StepFunctions wraps an AWS StepFunctions client.
type StepFunctions struct {
	client api
}

// New returns StepFunctions configured against the input session.
func New(s *session.Session) *StepFunctions {
	return &StepFunctions{
		client: sfn.New(s),
	}
}

// StateMachineDefinition returns the JSON-based state machine definition.
func (s *StepFunctions) StateMachineDefinition(stateMachineARN string) (string, error) {
	out, err := s.client.DescribeStateMachine(&sfn.DescribeStateMachineInput{
		StateMachineArn: aws.String(stateMachineARN),
	})
	if err != nil {
		return "", fmt.Errorf("describe state machine: %w", err)
	}

	return aws.StringValue(out.Definition), nil
}
