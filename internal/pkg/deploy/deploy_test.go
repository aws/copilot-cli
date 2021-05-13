// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"testing"

	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type storeMock struct {
	rgGetter    *mocks.MockresourceGetter
	configStore *mocks.MockConfigStoreClient
}

func TestStore_ListDeployedServices(t *testing.T) {
	testCases := map[string]struct {
		inputApp   string
		inputEnv   string
		setupMocks func(mocks storeMock)

		wantedError error
		wantedSvcs  []string
	}{
		"return error if fail to get resources by tag": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get resources by Copilot tags: some error"),
		},
		"return error if fail to get service name": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN", Tags: map[string]string{}}}, nil),
				)
			},

			wantedError: fmt.Errorf("service resource with ARN mockARN is not tagged with %s", ServiceTagKey),
		},
		"return error if fail to get config service": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN", Tags: map[string]string{ServiceTagKey: "mockSvc"}}}, nil),
					m.configStore.EXPECT().GetService("mockApp", "mockSvc").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get service mockSvc: some error"),
		},
		"success": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN1", Tags: map[string]string{ServiceTagKey: "mockSvc1"}},
						{ARN: "mockARN2", Tags: map[string]string{ServiceTagKey: "mockSvc2"}}}, nil),
					m.configStore.EXPECT().GetService("mockApp", "mockSvc1").Return(&config.Workload{
						App:  "mockApp",
						Name: "mockSvc1",
					}, nil),
					m.configStore.EXPECT().GetService("mockApp", "mockSvc2").Return(&config.Workload{
						App:  "mockApp",
						Name: "mockSvc2",
					}, nil),
				)
			},

			wantedSvcs: []string{"mockSvc1", "mockSvc2"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStore := mocks.NewMockConfigStoreClient(ctrl)
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (resourceGetter, error) { return mockRgGetter, nil },
			}

			// WHEN
			svcs, err := store.ListDeployedServices(tc.inputApp, tc.inputEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.ElementsMatch(t, svcs, tc.wantedSvcs)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStore_ListDeployedJobs(t *testing.T) {
	testCases := map[string]struct {
		inputApp   string
		inputEnv   string
		setupMocks func(mocks storeMock)

		wantedError error
		wantedSvcs  []string
	}{
		"success": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(stateMachineResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN1", Tags: map[string]string{ServiceTagKey: "mockJob1"}},
						{ARN: "mockARN2", Tags: map[string]string{ServiceTagKey: "mockJob2"}}}, nil),
					m.configStore.EXPECT().GetJob("mockApp", "mockJob1").Return(&config.Workload{
						App:  "mockApp",
						Name: "mockJob1",
					}, nil),
					m.configStore.EXPECT().GetJob("mockApp", "mockJob2").Return(&config.Workload{
						App:  "mockApp",
						Name: "mockJob2",
					}, nil),
				)
			},

			wantedSvcs: []string{"mockJob1", "mockJob2"},
		},
		"return error if fail to get job name": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(stateMachineResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN", Tags: map[string]string{}}}, nil),
				)
			},

			wantedError: fmt.Errorf("job resource with ARN mockARN is not tagged with %s", ServiceTagKey),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStore := mocks.NewMockConfigStoreClient(ctrl)
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (resourceGetter, error) { return mockRgGetter, nil },
			}
			// WHEN
			jobs, err := store.ListDeployedJobs(tc.inputApp, tc.inputEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.ElementsMatch(t, jobs, tc.wantedSvcs)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStore_ListEnvironmentsDeployedTo(t *testing.T) {
	testCases := map[string]struct {
		inputApp   string
		inputSvc   string
		setupMocks func(mocks storeMock)

		wantedError error
		wantedEnvs  []string
	}{
		"return error if fail to list all config environments": {
			inputApp: "mockApp",
			inputSvc: "mockSvc",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.configStore.EXPECT().ListEnvironments("mockApp").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("list environment for app mockApp: some error"),
		},
		"return error if fail to get resources by tags": {
			inputApp: "mockApp",
			inputSvc: "mockSvc",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.configStore.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
						{
							App:  "mockApp",
							Name: "mockEnv",
						},
					}, nil),
					m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
						AppTagKey:     "mockApp",
						EnvTagKey:     "mockEnv",
						ServiceTagKey: "mockSvc",
					}).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get resources by Copilot tags: some error"),
		},
		"success": {
			inputApp: "mockApp",
			inputSvc: "mockSvc",

			setupMocks: func(m storeMock) {
				m.configStore.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						App:  "mockApp",
						Name: "mockEnv1",
					},
					{
						App:  "mockApp",
						Name: "mockEnv2",
					},
				}, nil)
				m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
					AppTagKey:     "mockApp",
					EnvTagKey:     "mockEnv1",
					ServiceTagKey: "mockSvc",
				}).Return([]*rg.Resource{{ARN: "mockSvcARN"}}, nil)
				m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
					AppTagKey:     "mockApp",
					EnvTagKey:     "mockEnv2",
					ServiceTagKey: "mockSvc",
				}).Return([]*rg.Resource{}, nil)
			},

			wantedEnvs: []string{"mockEnv1"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStore := mocks.NewMockConfigStoreClient(ctrl)
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:         mockConfigStore,
				newRgClientFromRole: func(string, string) (resourceGetter, error) { return mockRgGetter, nil },
			}

			// WHEN
			envs, err := store.ListEnvironmentsDeployedTo(tc.inputApp, tc.inputSvc)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.ElementsMatch(t, envs, tc.wantedEnvs)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStore_IsServiceDeployed(t *testing.T) {
	testCases := map[string]struct {
		inputApp   string
		inputEnv   string
		inputSvc   string
		setupMocks func(mocks storeMock)

		wantedError    error
		wantedDeployed bool
	}{
		"return error if fail to get resources by tags": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",
			inputSvc: "mockSvc",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
						AppTagKey:     "mockApp",
						EnvTagKey:     "mockEnv",
						ServiceTagKey: "mockSvc",
					}).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get resources by Copilot tags: some error"),
		},
		"success with false": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",
			inputSvc: "mockSvc",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
						AppTagKey:     "mockApp",
						EnvTagKey:     "mockEnv",
						ServiceTagKey: "mockSvc",
					}).Return([]*rg.Resource{}, nil),
				)
			},

			wantedDeployed: false,
		},
		"success with true": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",
			inputSvc: "mockSvc",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(ecsServiceResourceType, map[string]string{
						AppTagKey:     "mockApp",
						EnvTagKey:     "mockEnv",
						ServiceTagKey: "mockSvc",
					}).Return([]*rg.Resource{{ARN: "mockSvcARN"}}, nil),
				)
			},

			wantedDeployed: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStore := mocks.NewMockConfigStoreClient(ctrl)
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (resourceGetter, error) { return mockRgGetter, nil },
			}

			// WHEN
			deployed, err := store.IsServiceDeployed(tc.inputApp, tc.inputEnv, tc.inputSvc)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.Equal(t, deployed, tc.wantedDeployed)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_IsJobDeployed(t *testing.T) {
	testCases := map[string]struct {
		inputApp   string
		inputEnv   string
		inputJob   string
		setupMocks func(mocks storeMock)

		wantedError    error
		wantedDeployed bool
	}{
		"return error if fail to get resources by tags": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",
			inputJob: "mockJob",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(stateMachineResourceType, map[string]string{
						AppTagKey:     "mockApp",
						EnvTagKey:     "mockEnv",
						ServiceTagKey: "mockJob",
					}).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get resources by Copilot tags: some error"),
		},
		"success with false": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",
			inputJob: "mockJob",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(stateMachineResourceType, map[string]string{
						AppTagKey:     "mockApp",
						EnvTagKey:     "mockEnv",
						ServiceTagKey: "mockJob",
					}).Return([]*rg.Resource{}, nil),
				)
			},
			wantedDeployed: false,
		},
		"success with true": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",
			inputJob: "mockJob",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(stateMachineResourceType, map[string]string{
						AppTagKey:     "mockApp",
						EnvTagKey:     "mockEnv",
						ServiceTagKey: "mockJob",
					}).Return([]*rg.Resource{{ARN: "mockJobARN"}}, nil),
				)
			},

			wantedDeployed: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStore := mocks.NewMockConfigStoreClient(ctrl)
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (resourceGetter, error) { return mockRgGetter, nil },
			}

			// WHEN
			deployed, err := store.IsJobDeployed(tc.inputApp, tc.inputEnv, tc.inputJob)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.Equal(t, deployed, tc.wantedDeployed)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
