// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCloudFormation_EnvironmentTemplate(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inClient  func(ctrl *gomock.Controller) *mocks.MockcfnClient
	}{
		"calls TemplateBody": {
			inAppName: "phonetool",
			inEnvName: "test",
			inClient: func(ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody("phonetool-test").Return("", nil)
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := &CloudFormation{
				cfnClient: tc.inClient(ctrl),
			}

			// WHEN
			cf.EnvironmentTemplate(tc.inAppName, tc.inEnvName)
		})
	}
}

func TestCloudFormation_EnvironmentParameters(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inClient  func(ctrl *gomock.Controller) *mocks.MockcfnClient

		wantedParams []*awscfn.Parameter
		wantedErr    error
	}{
		"should return stack parameters from a stack description": {
			inAppName: "phonetool",
			inEnvName: "test",
			inClient: func(ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("phonetool-test").Return(&cloudformation.StackDescription{
					Parameters: []*awscfn.Parameter{
						{
							ParameterKey:   aws.String("name"),
							ParameterValue: aws.String("test"),
						},
					},
				}, nil)
				return m
			},

			wantedParams: []*awscfn.Parameter{
				{
					ParameterKey:   aws.String("name"),
					ParameterValue: aws.String("test"),
				},
			},
		},
		"should return the error as is from a failed stack description": {
			inAppName: "phonetool",
			inEnvName: "test",
			inClient: func(ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: errors.New("describe stack phonetool-test: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := &CloudFormation{
				cfnClient: tc.inClient(ctrl),
			}

			// WHEN
			actual, err := cf.EnvironmentParameters(tc.inAppName, tc.inEnvName)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedParams, actual)
			}
		})
	}
}

func TestCloudFormation_ForceUpdateID(t *testing.T) {
	testCases := map[string]struct {
		inClient func(ctrl *gomock.Controller) *mocks.MockcfnClient

		wanted    string
		wantedErr error
	}{
		"should return stack parameters from a stack description": {
			inClient: func(ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("phonetool-test").Return(&cloudformation.StackDescription{
					Outputs: []*awscfn.Output{
						{
							OutputKey:   aws.String(template.LastForceDeployIDOutputName),
							OutputValue: aws.String("mockForceUpdateID"),
						},
					},
				}, nil)
				return m
			},
			wanted: "mockForceUpdateID",
		},
		"error describing the stack": {
			inClient: func(ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: errors.New("describe stack phonetool-test: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := &CloudFormation{
				cfnClient: tc.inClient(ctrl),
			}

			// WHEN
			actual, err := cf.ForceUpdateOutputID("phonetool", "test")
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, actual)
			}
		})
	}
}

func TestCloudFormation_UpdateEnvironmentTemplate(t *testing.T) {
	testCases := map[string]struct {
		inAppName      string
		inEnvName      string
		inTemplateBody string
		inExecRoleARN  string
		inClient       func(t *testing.T, ctrl *gomock.Controller) *mocks.MockcfnClient

		wantedError error
	}{
		"wraps error if describe fails": {
			inAppName: "phonetool",
			inEnvName: "test",
			inClient: func(t *testing.T, ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},

			wantedError: errors.New("describe stack phonetool-test: some error"),
		},
		"uses existing parameters, tags, and passed in new template and role arn on success": {
			inAppName:      "phonetool",
			inEnvName:      "test",
			inTemplateBody: "hello",
			inExecRoleARN:  "arn",
			inClient: func(t *testing.T, ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				params := []*awscfn.Parameter{
					{
						ParameterKey:   aws.String("ALBWorkloads"),
						ParameterValue: aws.String("frontend"),
					},
				}
				tags := []*awscfn.Tag{
					{
						Key:   aws.String("copilot-application"),
						Value: aws.String("phonetool"),
					},
				}
				m.EXPECT().Describe("phonetool-test").Return(&cloudformation.StackDescription{
					Parameters: params,
					Tags:       tags,
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(nil).
					Do(func(s *cloudformation.Stack) {
						require.Equal(t, "phonetool-test", s.Name)
						require.Equal(t, params, s.Parameters)
						require.Equal(t, tags, s.Tags)
						require.Equal(t, "hello", s.TemplateBody)
						require.Equal(t, aws.String("arn"), s.RoleARN)
					})
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := &CloudFormation{
				cfnClient: tc.inClient(t, ctrl),
			}

			// WHEN
			err := cf.UpdateEnvironmentTemplate(tc.inAppName, tc.inEnvName, tc.inTemplateBody, tc.inExecRoleARN)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
