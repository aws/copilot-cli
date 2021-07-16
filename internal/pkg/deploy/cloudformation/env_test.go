// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var mockCreateEnvInput = deploy.CreateEnvironmentInput{
	App: deploy.AppInformation{
		Name: "phonetool",
	},
	Name:    "test",
	Version: "v1.0.0",
	CustomResourcesURLs: map[string]string{
		template.DNSCertValidatorFileName: "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey1",
		template.DNSDelegationFileName:    "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey2",
		template.EnableLongARNsFileName:   "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey3",
		template.CustomDomainFileName:     "https://mockbucket.s3-us-west-2.amazonaws.com/mockkey4",
	},
}

func TestCloudFormation_UpgradeEnvironment(t *testing.T) {
	testCases := map[string]struct {
		in           *deploy.CreateEnvironmentInput
		mockDeployer func(t *testing.T, ctrl *gomock.Controller) *CloudFormation

		wantedErr error
	}{
		"upgrades using previous values when stack is available": {
			in: &mockCreateEnvInput,
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("phonetool-test").Return(&cloudformation.StackDescription{
					Parameters: []*awscfn.Parameter{
						{
							ParameterKey:   aws.String("ALBWorkloads"),
							ParameterValue: aws.String("frontend,admin"),
						},
					},
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(nil).Do(func(s *cloudformation.Stack) {
					require.ElementsMatch(t, s.Parameters, []*awscfn.Parameter{
						{
							ParameterKey:     aws.String("ALBWorkloads"),
							UsePreviousValue: aws.Bool(true),
						},
					})
				})

				return &CloudFormation{
					cfnClient: m,
				}
			},
		},
		"waits until stack is available for update": {
			in: &mockCreateEnvInput,
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				m := mocks.NewMockcfnClient(ctrl)

				gomock.InOrder(
					m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{}, nil).AnyTimes(),
					m.EXPECT().UpdateAndWait(gomock.Any()).Return(&cloudformation.ErrStackUpdateInProgress{
						Name: "phonetool-test",
					}).AnyTimes(),
					m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{
						StackStatus: aws.String("UPDATE_IN_PROGRESS"),
					}, nil).AnyTimes(),
					m.EXPECT().WaitForUpdate(gomock.Any(), "phonetool-test").Return(nil).AnyTimes(),
					m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{}, nil).AnyTimes(),
					m.EXPECT().UpdateAndWait(gomock.Any()).Return(nil),
				)
				return &CloudFormation{
					cfnClient: m,
				}
			},
		},
		"should exit successfully if there are no updates needed": {
			in: &mockCreateEnvInput,
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(fmt.Errorf("update and wait: %w", &cloudformation.ErrChangeSetEmpty{}))
				return &CloudFormation{
					cfnClient: m,
				}
			},
		},
		"should retry if the changeset request becomes obsolete": {
			in: &mockCreateEnvInput,
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{}, nil).Times(2)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(fmt.Errorf("update and wait: %w", &cloudformation.ErrChangeSetNotExecutable{}))
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(nil)
				return &CloudFormation{
					cfnClient: m,
				}
			},
		},
		"wrap error on unexpected update err": {
			in: &mockCreateEnvInput,
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(errors.New("some error"))

				return &CloudFormation{
					cfnClient: m,
				}
			},

			wantedErr: errors.New("update and wait for stack phonetool-test: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := tc.mockDeployer(t, ctrl)

			// WHEN
			err := cf.UpgradeEnvironment(tc.in)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCloudFormation_UpgradeLegacyEnvironment(t *testing.T) {
	testCases := map[string]struct {
		in            *deploy.CreateEnvironmentInput
		lbWebServices []string
		mockDeployer  func(t *testing.T, ctrl *gomock.Controller) *CloudFormation

		wantedErr error
	}{
		"replaces IncludePublicLoadBalancer param with ALBWorkloads and preserves existing params": {
			in:            &mockCreateEnvInput,
			lbWebServices: []string{"frontend", "admin"},
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("phonetool-test").Return(&cloudformation.StackDescription{
					Parameters: []*awscfn.Parameter{
						{
							ParameterKey:   aws.String("IncludePublicLoadBalancer"),
							ParameterValue: aws.String("true"),
						},
						{
							ParameterKey:   aws.String("EnvironmentName"),
							ParameterValue: aws.String("test"),
						},
					},
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(nil).Do(func(s *cloudformation.Stack) {
					require.ElementsMatch(t, s.Parameters, []*awscfn.Parameter{
						{
							ParameterKey:   aws.String("ALBWorkloads"),
							ParameterValue: aws.String("frontend,admin"),
						},
						{
							ParameterKey:     aws.String("EnvironmentName"),
							UsePreviousValue: aws.Bool(true),
						},
					})
				})

				return &CloudFormation{
					cfnClient: m,
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := tc.mockDeployer(t, ctrl)

			// WHEN
			err := cf.UpgradeLegacyEnvironment(tc.in, tc.lbWebServices...)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

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
