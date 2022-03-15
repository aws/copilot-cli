// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package apprunner

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/apprunner/mocks"
	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type clientMocks struct {
	rgMock        *mocks.MockresourceGetter
	appRunnerMock *mocks.MockappRunnerClient
}

func TestClient_ForceUpdateService(t *testing.T) {
	mockError := errors.New("some error")
	const (
		mockApp         = "mockApp"
		mockSvc         = "mockSvc"
		mockEnv         = "mockEnv"
		mockSvcARN      = "mockSvcARN"
		mockOperationID = "mockOperationID"
	)
	getRgInput := map[string]string{
		deploy.AppTagKey:     mockApp,
		deploy.EnvTagKey:     mockEnv,
		deploy.ServiceTagKey: mockSvc,
	}
	tests := map[string]struct {
		mock func(m *clientMocks)

		wantErr error
	}{
		"fail get the app runner service": {
			mock: func(m *clientMocks) {
				m.rgMock.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("get App Runner service with tags (mockApp, mockEnv, mockSvc): some error"),
		},
		"no app runner service found": {
			mock: func(m *clientMocks) {
				m.rgMock.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
					Return([]*resourcegroups.Resource{}, nil)
			},
			wantErr: fmt.Errorf("no App Runner service found for mockSvc in environment mockEnv"),
		},
		"more than one app runner service found": {
			mock: func(m *clientMocks) {
				m.rgMock.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
					Return([]*resourcegroups.Resource{
						{}, {},
					}, nil)
			},
			wantErr: fmt.Errorf("more than one App Runner service with the name mockSvc found in environment mockEnv"),
		},
		"error if fail to start new deployment": {
			mock: func(m *clientMocks) {
				m.rgMock.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
					Return([]*resourcegroups.Resource{
						{
							ARN: mockSvcARN,
						},
					}, nil)
				m.appRunnerMock.EXPECT().StartDeployment(mockSvcARN).Return("", mockError)
			},
			wantErr: fmt.Errorf("some error"),
		},
		"error if fail to wait for deployment": {
			mock: func(m *clientMocks) {
				m.rgMock.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
					Return([]*resourcegroups.Resource{
						{
							ARN: mockSvcARN,
						},
					}, nil)
				m.appRunnerMock.EXPECT().StartDeployment(mockSvcARN).Return(mockOperationID, nil)
				m.appRunnerMock.EXPECT().WaitForOperation(mockOperationID, mockSvcARN).Return(mockError)
			},
			wantErr: fmt.Errorf("some error"),
		},
		"success": {
			mock: func(m *clientMocks) {
				m.rgMock.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
					Return([]*resourcegroups.Resource{
						{
							ARN: mockSvcARN,
						},
					}, nil)
				m.appRunnerMock.EXPECT().StartDeployment(mockSvcARN).Return(mockOperationID, nil)
				m.appRunnerMock.EXPECT().WaitForOperation(mockOperationID, mockSvcARN).Return(nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRg := mocks.NewMockresourceGetter(ctrl)
			mockAppRunner := mocks.NewMockappRunnerClient(ctrl)
			m := &clientMocks{
				rgMock:        mockRg,
				appRunnerMock: mockAppRunner,
			}
			tc.mock(m)

			c := Client{
				appRunnerClient: mockAppRunner,
				rgGetter:        mockRg,
			}

			gotErr := c.ForceUpdateService(mockApp, mockEnv, mockSvc)

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestClient_LastUpdatedAt(t *testing.T) {
	mockError := errors.New("some error")
	const (
		mockApp    = "mockApp"
		mockSvc    = "mockSvc"
		mockEnv    = "mockEnv"
		mockSvcARN = "mockSvcARN"
	)
	mockTime := time.Unix(1494505756, 0)
	getRgInput := map[string]string{
		deploy.AppTagKey:     mockApp,
		deploy.EnvTagKey:     mockEnv,
		deploy.ServiceTagKey: mockSvc,
	}
	tests := map[string]struct {
		mock func(m *clientMocks)

		wantErr error
		want    time.Time
	}{
		"error if fail to describe service": {
			mock: func(m *clientMocks) {
				m.rgMock.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
					Return([]*resourcegroups.Resource{
						{
							ARN: mockSvcARN,
						},
					}, nil)
				m.appRunnerMock.EXPECT().DescribeService(mockSvcARN).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("describe service: some error"),
		},
		"succeed": {
			mock: func(m *clientMocks) {
				m.rgMock.EXPECT().GetResourcesByTags(serviceResourceType, getRgInput).
					Return([]*resourcegroups.Resource{
						{
							ARN: mockSvcARN,
						},
					}, nil)
				m.appRunnerMock.EXPECT().DescribeService(mockSvcARN).Return(&apprunner.Service{
					DateUpdated: mockTime,
				}, nil)
			},
			want: mockTime,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRg := mocks.NewMockresourceGetter(ctrl)
			mockAppRunner := mocks.NewMockappRunnerClient(ctrl)
			m := &clientMocks{
				rgMock:        mockRg,
				appRunnerMock: mockAppRunner,
			}
			tc.mock(m)

			c := Client{
				appRunnerClient: mockAppRunner,
				rgGetter:        mockRg,
			}

			got, gotErr := c.LastUpdatedAt(mockApp, mockEnv, mockSvc)

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.want, got)
			}
		})
	}
}
