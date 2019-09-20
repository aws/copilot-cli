// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

const (
	mockTemplate        = "mockTemplate"
	mockEnvironmentName = "mockEnvName"
	mockProjectName     = "mockProjectName"
)

type mockCloudFormation struct {
	cloudformationiface.CloudFormationAPI
	t                                *testing.T
	mockCreateStack                  func(t *testing.T, input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
	mockWaitUntilStackCreateComplete func(t *testing.T, input *cloudformation.DescribeStacksInput) error
}

func (cf mockCloudFormation) CreateStack(in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	return cf.mockCreateStack(cf.t, in)
}

func (cf mockCloudFormation) WaitUntilStackCreateComplete(in *cloudformation.DescribeStacksInput) error {
	return cf.mockWaitUntilStackCreateComplete(cf.t, in)
}

func TestDeployEnvironment(t *testing.T) {
	testCases := map[string]struct {
		cf   CloudFormation
		env  archer.Environment
		want error
	}{
		"template file does not exist": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
				},
				box: emptyBox(),
			},
			want: fmt.Errorf("file does not exist"),
		},
		"ErrCodeAlreadyExistsException in CreateStack call": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateStack: func(t *testing.T, input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
						return nil, awserr.New(cloudformation.ErrCodeAlreadyExistsException, "error", nil)
					},
				},
				box: boxWithTemplateFile(),
			},
			want: nil,
		},
		"unhandled error in CreatStack call": {
			env: archer.Environment{
				Project: mockProjectName,
				Name:    mockEnvironmentName,
			},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateStack: func(t *testing.T, input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
						return nil, fmt.Errorf("some AWS error")
					},
				},
				box: boxWithTemplateFile(),
			},
			want: errors.New("failed to deploy the environment " + mockEnvironmentName + " with CloudFormation due to: some AWS error"),
		},
		"happy path": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateStack: func(t *testing.T, input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
						require.Equal(t, mockProjectName+"-"+mockEnvironmentName, *input.StackName)
						require.Equal(t, mockTemplate, *input.TemplateBody)

						return &cloudformation.CreateStackOutput{}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
			env: archer.Environment{
				Project: mockProjectName,
				Name:    mockEnvironmentName,
			},
			want: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.cf.DeployEnvironment(tc.env, false)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			}
		})
	}
}

func TestWait(t *testing.T) {
	testCases := map[string]struct {
		cf    CloudFormation
		input archer.Environment
		want  error
	}{
		"error in WaitUntilStackCreateComplete call": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockWaitUntilStackCreateComplete: func(t *testing.T, input *cloudformation.DescribeStacksInput) error {
						return fmt.Errorf("some AWS error")
					},
				},
			},
			want: fmt.Errorf("some AWS error"),
		},
		"happy path": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockWaitUntilStackCreateComplete: func(t *testing.T, input *cloudformation.DescribeStacksInput) error {
						require.Equal(t, mockProjectName+"-"+mockEnvironmentName, *input.StackName)

						return nil
					},
				},
			},
			input: archer.Environment{
				Project: mockProjectName,
				Name:    mockEnvironmentName,
			},
			want: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.cf.Wait(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

func emptyBox() packd.Box {
	return packd.NewMemoryBox()
}

func boxWithTemplateFile() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString(environmentTemplate, mockTemplate)

	return box
}
