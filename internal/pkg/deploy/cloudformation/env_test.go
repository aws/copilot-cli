// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCloudFormation_UpgradeEnvironment(t *testing.T) {
	testCases := map[string]struct {
		in           *deploy.CreateEnvironmentInput
		mockDeployer func(t *testing.T, ctrl *gomock.Controller) *CloudFormation

		wantedErr error
	}{
		"upgrades using previous values when stack is available": {
			in: &deploy.CreateEnvironmentInput{
				AppName: "phonetool",
				Name:    "test",
				Version: "v1.0.0",
			},
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
			in: &deploy.CreateEnvironmentInput{
				AppName: "phonetool",
				Name:    "test",
				Version: "v1.0.0",
			},
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{}, nil).Times(2)
				gomock.InOrder(
					m.EXPECT().UpdateAndWait(gomock.Any()).Return(&cloudformation.ErrStackUpdateInProgress{
						Name: "phonetool-test",
					}),
					m.EXPECT().UpdateAndWait(gomock.Any()).Return(nil),
				)
				m.EXPECT().WaitForUpdate("phonetool-test").Return(nil)

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
			in: &deploy.CreateEnvironmentInput{
				AppName: "phonetool",
				Name:    "test",
				Version: "v1.0.0",
			},
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
