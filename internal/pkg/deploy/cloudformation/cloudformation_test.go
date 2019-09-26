// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

const (
	mockTemplate        = "mockTemplate"
	mockEnvironmentName = "mockEnvName"
	mockProjectName     = "mockProjectName"
	mockChangeSetID     = "mockChangeSetID"
	mockStackID         = "mockStackID"
)

type mockCloudFormation struct {
	cloudformationiface.CloudFormationAPI

	t                                    *testing.T
	mockCreateChangeSet                  func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error)
	mockWaitUntilChangeSetCreateComplete func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error
	mockExecuteChangeSet                 func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error)
	mockDescribeChangeSet                func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error)
	mockWaitUntilStackCreateComplete     func(t *testing.T, in *cloudformation.DescribeStacksInput) error
}

func (cf mockCloudFormation) CreateChangeSet(in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
	return cf.mockCreateChangeSet(cf.t, in)
}

func (cf mockCloudFormation) WaitUntilChangeSetCreateComplete(in *cloudformation.DescribeChangeSetInput) error {
	return cf.mockWaitUntilChangeSetCreateComplete(cf.t, in)
}

func (cf mockCloudFormation) ExecuteChangeSet(in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error) {
	return cf.mockExecuteChangeSet(cf.t, in)
}

func (cf mockCloudFormation) DescribeChangeSet(in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
	return cf.mockDescribeChangeSet(cf.t, in)
}

func (cf mockCloudFormation) WaitUntilStackCreateComplete(in *cloudformation.DescribeStacksInput) error {
	return cf.mockWaitUntilStackCreateComplete(cf.t, in)
}

func TestDeployEnvironment(t *testing.T) {
	testCases := map[string]struct {
		cf   CloudFormation
		env  *archer.Environment
		want error
	}{
		"should return error given file not found": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
				},
				box: emptyBox(),
			},
			want: errors.New(fmt.Sprintf("failed to find template %s for the environment: %s", environmentTemplate, "file does not exist")),
		},
		"should wrap error returned from CreateChangeSet call": {
			env: &archer.Environment{
				Project: mockProjectName,
				Name:    mockEnvironmentName,
			},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return nil, fmt.Errorf("some AWS error")
					},
				},
				box: boxWithTemplateFile(),
			},
			want: errors.New(fmt.Sprintf("failed to create changeSet for stack %s: %s", mockProjectName+"-"+mockEnvironmentName, "some AWS error")),
		},
		"should wrap error returned from WaitUntilChangeSetCreateComplete call": {
			env: &archer.Environment{
				Project: mockProjectName,
				Name:    mockEnvironmentName,
			},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateComplete: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return errors.New("some AWS error")
					},
				},
				box: boxWithTemplateFile(),
			},
			want: errors.New(fmt.Sprintf("failed to wait for changeSet creation %s: %s", fmt.Sprintf("name=%s, stackID=%s", mockChangeSetID, mockStackID), "some AWS error")),
		},
		"should wrap error returned from ExecuteChangeSet call": {
			env: &archer.Environment{
				Project: mockProjectName,
				Name:    mockEnvironmentName,
			},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateComplete: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return nil
					},
					mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (output *cloudformation.ExecuteChangeSetOutput, e error) {
						return nil, errors.New("some AWS error")
					},
				},
				box: boxWithTemplateFile(),
			},
			want: errors.New(fmt.Sprintf("failed to execute changeSet %s: %s", fmt.Sprintf("name=%s, stackID=%s", mockChangeSetID, mockStackID), "some AWS error")),
		},
		"should deploy an environment": {
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						require.Equal(t, mockProjectName+"-"+mockEnvironmentName, *in.StackName)
						require.True(t, isValidChangeSetName(*in.ChangeSetName))
						require.Equal(t, mockTemplate, *in.TemplateBody)

						return &cloudformation.CreateChangeSetOutput{
							Id:      aws.String(mockChangeSetID),
							StackId: aws.String(mockStackID),
						}, nil
					},
					mockWaitUntilChangeSetCreateComplete: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						require.Equal(t, mockStackID, *in.StackName)
						require.Equal(t, mockChangeSetID, *in.ChangeSetName)
						return nil
					},
					mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (output *cloudformation.ExecuteChangeSetOutput, e error) {
						require.Equal(t, mockStackID, *in.StackName)
						require.Equal(t, mockChangeSetID, *in.ChangeSetName)
						return nil, nil
					},
				},
				box: boxWithTemplateFile(),
			},
			env: &archer.Environment{
				Project:            mockProjectName,
				Name:               mockEnvironmentName,
				PublicLoadBalancer: false,
			},
			want: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.cf.DeployEnvironment(tc.env)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestWaitForEnvironmentCreation(t *testing.T) {
	testCases := map[string]struct {
		cf    CloudFormation
		input *archer.Environment
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
			input: &archer.Environment{
				Project: mockProjectName,
				Name:    mockEnvironmentName,
			},
			want: fmt.Errorf("failed to create stack %s: %s", mockProjectName+"-"+mockEnvironmentName, "some AWS error"),
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
			input: &archer.Environment{
				Project: mockProjectName,
				Name:    mockEnvironmentName,
			},
			want: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.cf.WaitForEnvironmentCreation(tc.input)

			if tc.want != nil {
				require.EqualError(t, tc.want, got.Error())
			} else {
				require.NoError(t, got)
			}
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

// A change set name can contain only alphanumeric, case sensitive characters
// and hyphens. It must start with an alphabetic character and cannot exceed
// 128 characters.
func isValidChangeSetName(name string) bool {
	if len(name) > 128 {
		return false
	}
	matchesPattern := regexp.MustCompile(`[a-zA-Z][-a-zA-Z0-9]*`).MatchString
	if !matchesPattern(name) {
		return false
	}
	return true
}
