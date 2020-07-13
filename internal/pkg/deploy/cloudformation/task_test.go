// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
    "errors"
    "testing"

    "github.com/aws/copilot-cli/internal/pkg/deploy"

    "github.com/stretchr/testify/require"

    "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"

    "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
    "github.com/golang/mock/gomock"
)

func TestCloudFormation_DeployTask(t *testing.T) {
    mockTask := &deploy.CreateTaskResourcesInput{
        Name: "my-task",
    }

    testCases := map[string]struct {
        mockCfnClient func(m *mocks.MockcfnClient)
        wantedError error
    }{
        "create a new stack": {
            mockCfnClient: func(m *mocks.MockcfnClient) {
                m.EXPECT().CreateAndWait(gomock.Any()).Return(nil)
                m.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
            },
        },
        "failed to create stack": {
            mockCfnClient: func(m *mocks.MockcfnClient) {
                m.EXPECT().CreateAndWait(gomock.Any()).Return(errors.New("error"))
                m.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
            },
            wantedError: errors.New("create stack: error"),
        },
        "update the stack": {
            mockCfnClient: func(m *mocks.MockcfnClient) {
                m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{
                    Name: "my-task",
                })
                m.EXPECT().UpdateAndWait(gomock.Any()).Times(1).Return(nil)
            },
        },
        "failed to update stack": {
            mockCfnClient: func(m *mocks.MockcfnClient) {
                m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{
                    Name: "my-task",
                })
                m.EXPECT().UpdateAndWait(gomock.Any()).Return(errors.New("error"))
            },
            wantedError: errors.New("update stack: error"),
        },
    }

    for name, tc := range testCases {
        t.Run(name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            mockCfnClient := mocks.NewMockcfnClient(ctrl)
            if tc.mockCfnClient != nil {
                tc.mockCfnClient(mockCfnClient)
            }

            cf := CloudFormation{
                cfnClient: mockCfnClient,
            }

            err := cf.DeployTask(mockTask)
            if tc.wantedError != nil {
                require.EqualError(t, tc.wantedError, err.Error())
            } else {
                require.NoError(t, err)
            }
        })
    }
}