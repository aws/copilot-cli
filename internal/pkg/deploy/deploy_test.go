// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type storeMock struct {
	rgGetter    *mocks.MockResourceGetter
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
					m.configStore.EXPECT().ListWorkloads("mockApp").Return([]*config.Workload{}, nil),
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get resources by Copilot tags: some error"),
		},
		"return nil if no services are found": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.configStore.EXPECT().ListWorkloads("mockApp").Return([]*config.Workload{}, nil),
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN", Tags: map[string]string{}}}, nil),
				)
			},

			wantedSvcs: nil,
		},
		"return error if fail to get config service": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.configStore.EXPECT().ListWorkloads("mockApp").Return(nil, errors.New("some error")),
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Times(0),
				)
			},

			wantedError: fmt.Errorf("list all workloads in application mockApp: some error"),
		},
		"success": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.configStore.EXPECT().ListWorkloads("mockApp").Return([]*config.Workload{
						{
							App:  "mockApp",
							Name: "mockSvc1",
							Type: manifestinfo.LoadBalancedWebServiceType,
						},
						{
							App:  "mockApp",
							Name: "mockSvc2",
							Type: manifestinfo.RequestDrivenWebServiceType,
						},
						{
							App:  "mockApp",
							Name: "mockJob",
							Type: manifestinfo.ScheduledJobType,
						},
					}, nil),
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN1", Tags: map[string]string{ServiceTagKey: "mockSvc1"}},
						{ARN: "mockARN2", Tags: map[string]string{ServiceTagKey: "mockSvc2"}}}, nil),
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
			mockRgGetter := mocks.NewMockResourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (ResourceGetter, error) { return mockRgGetter, nil },
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
					m.configStore.EXPECT().ListWorkloads("mockApp").Return([]*config.Workload{
						{
							App:  "mockApp",
							Name: "mockSvc1",
							Type: manifestinfo.LoadBalancedWebServiceType,
						},
						{
							App:  "mockApp",
							Name: "mockSvc2",
							Type: manifestinfo.RequestDrivenWebServiceType,
						},
						{
							App:  "mockApp",
							Name: "mockJob1",
							Type: manifestinfo.ScheduledJobType,
						},
						{
							App:  "mockApp",
							Name: "mockJob2",
							Type: manifestinfo.ScheduledJobType,
						},
					}, nil),
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN1", Tags: map[string]string{ServiceTagKey: "mockJob1"}},
						{ARN: "mockARN2", Tags: map[string]string{ServiceTagKey: "mockJob2"}}}, nil),
				)
			},

			wantedSvcs: []string{"mockJob1", "mockJob2"},
		},
		"return nil if no jobs are found": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.configStore.EXPECT().ListWorkloads("mockApp").Return([]*config.Workload{}, nil),
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN", Tags: map[string]string{}}}, nil),
				)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStore := mocks.NewMockConfigStoreClient(ctrl)
			mockRgGetter := mocks.NewMockResourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (ResourceGetter, error) { return mockRgGetter, nil },
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
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
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
				m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
					AppTagKey:     "mockApp",
					EnvTagKey:     "mockEnv1",
					ServiceTagKey: "mockSvc",
				}).Return([]*rg.Resource{{ARN: "mockSvcARN"}}, nil)
				m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
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
			mockRgGetter := mocks.NewMockResourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:         mockConfigStore,
				newRgClientFromRole: func(string, string) (ResourceGetter, error) { return mockRgGetter, nil },
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
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
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
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
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
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
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
			mockRgGetter := mocks.NewMockResourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (ResourceGetter, error) { return mockRgGetter, nil },
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
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
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
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
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
					m.rgGetter.EXPECT().GetResourcesByTags(stackResourceType, map[string]string{
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
			mockRgGetter := mocks.NewMockResourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (ResourceGetter, error) { return mockRgGetter, nil },
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

func TestStore_ListSNSTopics(t *testing.T) {
	testCases := map[string]struct {
		inputApp   string
		inputEnv   string
		setupMocks func(mocks storeMock)

		wantedError  error
		wantedTopics []Topic
	}{
		"return error if fail to get resources by tag": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(snsResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"return nil if no topics are found": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(snsResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{{ARN: "mockARN", Tags: map[string]string{}}}, nil),
				)
			},

			wantedTopics: nil,
		},
		"return nil if the only topic doesn't have workload tag": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				gomock.InOrder(
					m.rgGetter.EXPECT().GetResourcesByTags(snsResourceType, map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					}).Return([]*rg.Resource{
						{
							ARN:  "arn:aws:sns:us-west-2:012345678912:mockApp-mockEnv-mockSvc1-topic",
							Tags: map[string]string{},
						},
					}, nil),
				)
			},
			wantedTopics: []Topic{},
		},
		"success": {
			inputApp: "mockApp",
			inputEnv: "mockEnv",

			setupMocks: func(m storeMock) {
				m.rgGetter.EXPECT().GetResourcesByTags(
					snsResourceType,
					map[string]string{
						AppTagKey: "mockApp",
						EnvTagKey: "mockEnv",
					},
				).Return(
					[]*rg.Resource{
						{
							ARN:  "arn:aws:sns:us-west-2:012345678912:mockApp-mockEnv-mockSvc1-topic",
							Tags: map[string]string{ServiceTagKey: "mockSvc1"},
						}, {
							ARN:  "arn:aws:sns:us-west-2:012345678912:mockApp-mockEnv-mockSvc2-events",
							Tags: map[string]string{ServiceTagKey: "mockSvc2"},
						},
					},
					nil,
				)
			},
			wantedTopics: []Topic{
				{
					awsARN: arn.ARN{
						Partition: "aws",
						Region:    "us-west-2",
						Service:   "sns",
						AccountID: "012345678912",
						Resource:  "mockApp-mockEnv-mockSvc1-topic",
					},
					prefix: "mockApp-mockEnv-mockSvc1-",
					wkld:   "mockSvc1",
					name:   "topic",
				},
				{
					awsARN: arn.ARN{
						Partition: "aws",
						Region:    "us-west-2",
						Service:   "sns",
						AccountID: "012345678912",
						Resource:  "mockApp-mockEnv-mockSvc2-events",
					},
					prefix: "mockApp-mockEnv-mockSvc2-",
					wkld:   "mockSvc2",
					name:   "events",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStore := mocks.NewMockConfigStoreClient(ctrl)
			mockRgGetter := mocks.NewMockResourceGetter(ctrl)

			mocks := storeMock{
				rgGetter:    mockRgGetter,
				configStore: mockConfigStore,
			}

			tc.setupMocks(mocks)

			store := &Store{
				configStore:        mockConfigStore,
				newRgClientFromIDs: func(string, string) (ResourceGetter, error) { return mockRgGetter, nil },
			}

			// WHEN
			topics, err := store.ListSNSTopics(tc.inputApp, tc.inputEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.ElementsMatch(t, topics, tc.wantedTopics)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPipelineStore_ListDeployedPipelines(t *testing.T) {
	const (
		mockAppName           = "mockApp"
		mockLegacyPipelineARN = "arn:aws:codepipeline:us-west-2:1234567890:pipeline-dinder-badgoose-repo"
		mockPipelineARN       = "arn:aws:codepipeline:us-west-2:1234567890:pipeline-my-pipeline-repo"
	)
	testCases := map[string]struct {
		setupMocks func(mocks storeMock)

		wantedError     error
		wantedPipelines []Pipeline
	}{
		"return error if fail to get resources by tag": {
			setupMocks: func(m storeMock) {
				m.rgGetter.EXPECT().GetResourcesByTags(pipelineResourceType, map[string]string{
					AppTagKey: "mockApp",
				}).Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("get pipeline resources by tags for app mockApp: some error"),
		},
		"return error if fail to parse pipeline ARN": {
			setupMocks: func(m storeMock) {
				m.rgGetter.EXPECT().GetResourcesByTags(pipelineResourceType, map[string]string{
					AppTagKey: "mockApp",
				}).Return([]*rg.Resource{
					{
						ARN: "badARN",
					},
				}, nil)
			},

			wantedError: fmt.Errorf("parse pipeline ARN: badARN"),
		},
		"success": {
			setupMocks: func(m storeMock) {
				m.rgGetter.EXPECT().GetResourcesByTags(pipelineResourceType, map[string]string{
					AppTagKey: "mockApp",
				}).Return([]*rg.Resource{
					{
						ARN: mockLegacyPipelineARN,
						Tags: map[string]string{
							AppTagKey: mockAppName,
						},
					},
					{
						ARN: mockPipelineARN,
						Tags: map[string]string{
							AppTagKey:      mockAppName,
							PipelineTagKey: "my-pipeline-repo",
						},
					},
				}, nil)
			},

			wantedPipelines: []Pipeline{
				{
					ResourceName: "pipeline-dinder-badgoose-repo",
					IsLegacy:     true,
					AppName:      mockAppName,
				},
				{
					ResourceName: "my-pipeline-repo",
					IsLegacy:     false,
					AppName:      mockAppName,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRgGetter := mocks.NewMockResourceGetter(ctrl)

			mocks := storeMock{
				rgGetter: mockRgGetter,
			}

			tc.setupMocks(mocks)

			store := &PipelineStore{
				getter: mocks.rgGetter,
			}

			// WHEN
			pipelines, err := store.ListDeployedPipelines(mockAppName)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.ElementsMatch(t, pipelines, tc.wantedPipelines)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
