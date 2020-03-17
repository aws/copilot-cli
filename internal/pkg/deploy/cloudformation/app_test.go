// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCloudFormation_DeployApp(t *testing.T) {
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) cfnClient
	}{
		"does not call update if the stack is new": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
				return m
			},
		},
		"calls update if the stack already exists": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{
					Name: "name",
				})
				m.EXPECT().UpdateAndWait(gomock.Any())
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				cfnClient: tc.createMock(ctrl),
			}

			// WHEN
			err := c.DeployApp("template", "webhook", "", "myrole", map[string]string{
				"project": "myproject",
			}, map[string]string{
				"port": "80",
			})

			// THEN
			require.NoError(t, err)
		})
	}
}

func TestCloudFormation_DeleteApp(t *testing.T) {
	testCases := map[string]struct {
		in         deploy.DeleteAppInput
		createMock func(ctrl *gomock.Controller) cfnClient
	}{
		"calls delete with the appropriate stack name": {
			in: deploy.DeleteAppInput{
				AppName:     "webhook",
				EnvName:     "test",
				ProjectName: "kudos",
			},
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().DeleteAndWait("kudos-test-webhook")
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				cfnClient: tc.createMock(ctrl),
			}

			// WHEN
			err := c.DeleteApp(tc.in)

			// THEN
			require.NoError(t, err)
		})
	}
}
