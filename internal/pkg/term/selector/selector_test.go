// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dustin/go-humanize"

	"github.com/aws/aws-sdk-go/aws"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector/mocks"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deploySelectMocks struct {
	deploySvc *mocks.MockdeployedWorkloadsRetriever
	configSvc *mocks.MockconfigLister
	prompt    *mocks.Mockprompter
}

func TestDeploySelect_Topics(t *testing.T) {
	const (
		testApp = "mockApp"
		testEnv = "mockEnv"
		prodEnv = "mockProdEnv"
	)
	mockTopic, _ := deploy.NewTopic("arn:aws:sns:us-west-2:123456789012:mockApp-mockEnv-mockWkld-orders", testApp, testEnv, "mockWkld")
	mockTopic2, _ := deploy.NewTopic("arn:aws:sns:us-west-2:123456789012:mockApp-mockEnv-mockWkld-events", testApp, testEnv, "mockWkld")
	testCases := map[string]struct {
		setupMocks func(mocks deploySelectMocks)

		wantErr    error
		wantTopics []deploy.Topic
	}{
		"return error if fail to retrieve topics from deploy": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.EXPECT().ListEnvironments(testApp).Return(
					[]*config.Environment{{Name: testEnv}}, nil,
				)
				m.deploySvc.
					EXPECT().
					ListSNSTopics(testApp, testEnv).
					Return(nil, errors.New("some error"))
			},
			wantErr: fmt.Errorf("list SNS topics: some error"),
		},
		"return error if fail to select topics": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.EXPECT().ListEnvironments(testApp).Return(
					[]*config.Environment{{Name: testEnv}}, nil,
				)
				m.deploySvc.
					EXPECT().
					ListSNSTopics(testApp, testEnv).
					Return([]deploy.Topic{*mockTopic}, nil)
				m.prompt.
					EXPECT().
					MultiSelect("Select a deployed topic", "Help text", []string{"orders (mockWkld)"}, nil, gomock.Any()).
					Return(nil, errors.New("some error"))
			},
			wantErr: fmt.Errorf("select SNS topics: some error"),
		},
		"success": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.EXPECT().ListEnvironments(testApp).Return(
					[]*config.Environment{{Name: testEnv}, {Name: prodEnv}}, nil,
				)
				m.deploySvc.
					EXPECT().
					ListSNSTopics(testApp, testEnv).
					Return([]deploy.Topic{*mockTopic, *mockTopic2}, nil)
				m.deploySvc.
					EXPECT().
					ListSNSTopics(testApp, prodEnv).
					Return([]deploy.Topic{*mockTopic}, nil)
				m.prompt.
					EXPECT().
					MultiSelect("Select a deployed topic", "Help text", []string{"orders (mockWkld)"}, nil, gomock.Any()).
					Return([]string{"orders (mockWkld)"}, nil)
			},
			wantTopics: []deploy.Topic{*mockTopic},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockdeploySvc := mocks.NewMockdeployedWorkloadsRetriever(ctrl)
			mockconfigSvc := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := deploySelectMocks{
				deploySvc: mockdeploySvc,
				configSvc: mockconfigSvc,
				prompt:    mockprompt,
			}

			tc.setupMocks(mocks)

			sel := DeploySelector{
				ConfigSelector: &ConfigSelector{
					AppEnvSelector: &AppEnvSelector{
						appEnvLister: mockconfigSvc,
						prompt:       mockprompt,
					},
					workloadLister: mockconfigSvc,
				},
				deployStoreSvc: mockdeploySvc,
			}
			topics, err := sel.Topics("Select a deployed topic", "Help text", testApp)
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.wantTopics, topics)
			}
		})
	}
}

func TestIntersect(t *testing.T) {
	mockTopic1Env1, _ := deploy.NewTopic("arn:aws:sns:us-west-2:123456789012:mockApp-mockEnv1-mockWkld-orders", "mockApp", "mockEnv1", "mockWkld")
	mockTopic1Env2, _ := deploy.NewTopic("arn:aws:sns:us-west-2:123456789012:mockApp-mockEnv2-mockWkld-orders", "mockApp", "mockEnv2", "mockWkld")
	mockTopic2Env1, _ := deploy.NewTopic("arn:aws:sns:us-west-2:123456789012:mockApp-mockEnv1-mockWkld2-events", "mockApp", "mockEnv1", "mockWkld2")
	mockTopic2Env2, _ := deploy.NewTopic("arn:aws:sns:us-west-2:123456789012:mockApp-mockEnv2-mockWkld2-events", "mockApp", "mockEnv2", "mockWkld2")
	testCases := map[string]struct {
		inArray []deploy.Topic
		inMap   map[string]deploy.Topic

		wantedMap map[string]deploy.Topic
	}{
		"with no common entries": {
			inArray: []deploy.Topic{
				*mockTopic1Env1,
			},
			inMap: map[string]deploy.Topic{
				mockTopic2Env2.String(): *mockTopic2Env2,
			},
			wantedMap: map[string]deploy.Topic{},
		},
		"with common entries": {
			inArray: []deploy.Topic{
				*mockTopic1Env1,
				*mockTopic2Env1,
			},
			inMap: map[string]deploy.Topic{
				mockTopic2Env2.String(): *mockTopic2Env2,
				mockTopic1Env2.String(): *mockTopic1Env2,
			},
			wantedMap: map[string]deploy.Topic{
				mockTopic2Env1.String(): *mockTopic2Env1,
				mockTopic1Env1.String(): *mockTopic1Env1,
			},
		},
		"with one common entry, extra entry in array": {
			inArray: []deploy.Topic{
				*mockTopic1Env1,
				*mockTopic2Env1,
			},
			inMap: map[string]deploy.Topic{
				mockTopic2Env2.String(): *mockTopic2Env2,
			},
			wantedMap: map[string]deploy.Topic{
				mockTopic2Env1.String(): *mockTopic2Env1,
			},
		},
		"with one common entry, extra entry in map": {
			inArray: []deploy.Topic{
				*mockTopic1Env1,
			},
			inMap: map[string]deploy.Topic{
				mockTopic2Env2.String(): *mockTopic2Env2,
				mockTopic1Env2.String(): *mockTopic1Env2,
			},
			wantedMap: map[string]deploy.Topic{
				mockTopic1Env1.String(): *mockTopic1Env1,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN

			// WHEN
			out := intersect(tc.inMap, tc.inArray)

			// THEN
			require.Equal(t, out, tc.wantedMap)
		})
	}
}

func TestDeploySelect_Service(t *testing.T) {
	const testApp = "mockApp"
	testCases := map[string]struct {
		setupMocks func(mocks deploySelectMocks)
		svc        string
		env        string
		opts       []GetDeployedWorkloadOpts

		wantErr error
		wantEnv string
		wantSvc string
	}{
		"return error if fail to retrieve environment": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return(nil, errors.New("some error"))

			},
			wantErr: fmt.Errorf("list environments: list environments: some error"),
		},
		"return error if fail to list deployed services": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test").
					Return(nil, errors.New("some error"))
			},
			wantErr: fmt.Errorf("list deployed services for environment test: some error"),
		},
		"return error if no deployed services found": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test").
					Return([]string{}, nil)
			},
			wantErr: fmt.Errorf("no deployed services found in application %s", testApp),
		},
		"return error if fail to select": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test").
					Return([]string{"mockSvc1", "mockSvc2"}, nil)

				m.prompt.
					EXPECT().
					SelectOne("Select a deployed service", "Help text", []string{"mockSvc1 (test)", "mockSvc2 (test)"}, gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantErr: fmt.Errorf("select deployed services for application %s: some error", testApp),
		},
		"success": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test").
					Return([]string{"mockSvc1", "mockSvc2"}, nil)

				m.prompt.
					EXPECT().
					SelectOne("Select a deployed service", "Help text", []string{"mockSvc1 (test)", "mockSvc2 (test)"}, gomock.Any()).
					Return("mockSvc1 (test)", nil)
			},
			wantEnv: "test",
			wantSvc: "mockSvc1",
		},
		"skip with only one deployed service": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test").
					Return([]string{"mockSvc"}, nil)
			},
			wantEnv: "test",
			wantSvc: "mockSvc",
		},
		"return error if fail to check if service passed in by flag is deployed or not": {
			env: "test",
			svc: "mockSvc",
			setupMocks: func(m deploySelectMocks) {
				m.deploySvc.
					EXPECT().
					IsServiceDeployed(testApp, "test", "mockSvc").
					Return(false, errors.New("some error"))
			},
			wantErr: fmt.Errorf("check if service mockSvc is deployed in environment test: some error"),
		},
		"success with flags": {
			env: "test",
			svc: "mockSvc",
			setupMocks: func(m deploySelectMocks) {
				m.deploySvc.
					EXPECT().
					IsServiceDeployed(testApp, "test", "mockSvc").
					Return(true, nil)
			},
			wantEnv: "test",
			wantSvc: "mockSvc",
		},
		"filter deployed services": {
			opts: []GetDeployedWorkloadOpts{
				WithWkldFilter(func(svc *DeployedWorkload) (bool, error) {
					return svc.Env == "test1", nil
				}),
				WithServiceTypesFilter([]string{manifest.BackendServiceType}),
			},
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListWorkloads(testApp).
					Return([]*config.Workload{
						{
							App:  testApp,
							Name: "mockSvc1",
							Type: manifest.BackendServiceType,
						},
						{
							App:  testApp,
							Name: "mockSvc2",
							Type: manifest.BackendServiceType,
						},
						{
							App:  testApp,
							Name: "mockSvc3",
							Type: manifest.LoadBalancedWebServiceType,
						},
						{
							App:  testApp,
							Name: "mockJob1",
							Type: manifest.ScheduledJobType,
						},
					}, nil)

				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{Name: "test1"},
						{Name: "test2"},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test1").
					Return([]string{"mockSvc1", "mockSvc2", "mockSvc3"}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test2").
					Return([]string{"mockSvc1", "mockSvc2", "mockSvc3"}, nil)

				m.prompt.
					EXPECT().
					SelectOne("Select a deployed service", "Help text", []string{"mockSvc1 (test1)", "mockSvc2 (test1)"}, gomock.Any()).
					Return("mockSvc1 (test1)", nil)
			},
			wantEnv: "test1",
			wantSvc: "mockSvc1",
		},
		"filter returns error": {
			opts: []GetDeployedWorkloadOpts{
				WithWkldFilter(func(svc *DeployedWorkload) (bool, error) {
					return svc.Env == "test1", fmt.Errorf("filter error")
				}),
			},
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListWorkloads(testApp).
					Return([]*config.Workload{
						{
							App:  testApp,
							Name: "mockSvc1",
							Type: manifest.BackendServiceType,
						},
						{
							App:  testApp,
							Name: "mockSvc2",
							Type: manifest.BackendServiceType,
						},
						{
							App:  testApp,
							Name: "mockSvc3",
							Type: manifest.LoadBalancedWebServiceType,
						},
					}, nil)

				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{Name: "test1"},
						{Name: "test2"},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test1").
					Return([]string{"mockSvc1", "mockSvc2", "mockSvc3"}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedServices(testApp, "test2").
					Return([]string{"mockSvc1", "mockSvc2", "mockSvc3"}, nil)
			},
			wantErr: fmt.Errorf("filter error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockdeploySvc := mocks.NewMockdeployedWorkloadsRetriever(ctrl)
			mockconfigSvc := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := deploySelectMocks{
				deploySvc: mockdeploySvc,
				configSvc: mockconfigSvc,
				prompt:    mockprompt,
			}
			tc.setupMocks(mocks)

			sel := DeploySelector{
				ConfigSelector: &ConfigSelector{
					AppEnvSelector: &AppEnvSelector{
						appEnvLister: mockconfigSvc,
						prompt:       mockprompt,
					},
					workloadLister: mockconfigSvc,
				},
				deployStoreSvc: mockdeploySvc,
			}
			opts := append([]GetDeployedWorkloadOpts{WithEnv(tc.env), WithName(tc.svc)}, tc.opts...)

			gotDeployed, err := sel.DeployedService("Select a deployed service", "Help text", testApp, opts...)
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.wantSvc, gotDeployed.Name)
				require.Equal(t, tc.wantEnv, gotDeployed.Env)
			}
		})
	}
}

func TestDeploySelect_Job(t *testing.T) {
	const testApp = "mockApp"
	testCases := map[string]struct {
		setupMocks func(mocks deploySelectMocks)
		job        string
		env        string
		opts       []GetDeployedWorkloadOpts

		wantErr error
		wantEnv string
		wantJob string
	}{
		"return error if fail to retrieve environment": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return(nil, errors.New("some error"))

			},
			wantErr: fmt.Errorf("list environments: list environments: some error"),
		},
		"return error if fail to list deployed job": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test").
					Return(nil, errors.New("some error"))
			},
			wantErr: fmt.Errorf("list deployed jobs for environment test: some error"),
		},
		"return error if no deployed jobs found": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test").
					Return([]string{}, nil)
			},
			wantErr: fmt.Errorf("no deployed jobs found in application %s", testApp),
		},
		"return error if fail to select": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test").
					Return([]string{"mockJob1", "mockJob2"}, nil)

				m.prompt.
					EXPECT().
					SelectOne("Select a deployed job", "Help text", []string{"mockJob1 (test)", "mockJob2 (test)"}, gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantErr: fmt.Errorf("select deployed jobs for application %s: some error", testApp),
		},
		"success": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test").
					Return([]string{"mockJob1", "mockJob2"}, nil)

				m.prompt.
					EXPECT().
					SelectOne("Select a deployed job", "Help text", []string{"mockJob1 (test)", "mockJob2 (test)"}, gomock.Any()).
					Return("mockJob1 (test)", nil)
			},
			wantEnv: "test",
			wantJob: "mockJob1",
		},
		"skip with only one deployed job": {
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{
							Name: "test",
						},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test").
					Return([]string{"mockJob"}, nil)
			},
			wantEnv: "test",
			wantJob: "mockJob",
		},
		"return error if fail to check if job passed in by flag is deployed or not": {
			env: "test",
			job: "mockJob",
			setupMocks: func(m deploySelectMocks) {
				m.deploySvc.
					EXPECT().
					IsJobDeployed(testApp, "test", "mockJob").
					Return(false, errors.New("some error"))
			},
			wantErr: fmt.Errorf("check if job mockJob is deployed in environment test: some error"),
		},
		"success with flags": {
			env: "test",
			job: "mockJob",
			setupMocks: func(m deploySelectMocks) {
				m.deploySvc.
					EXPECT().
					IsJobDeployed(testApp, "test", "mockJob").
					Return(true, nil)
			},
			wantEnv: "test",
			wantJob: "mockJob",
		},
		"filter deployed jobs": {
			opts: []GetDeployedWorkloadOpts{
				WithWkldFilter(func(job *DeployedWorkload) (bool, error) {
					return job.Env == "test2", nil
				}),
				WithServiceTypesFilter([]string{manifest.ScheduledJobType}),
			},
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListWorkloads(testApp).
					Return([]*config.Workload{
						{
							App:  testApp,
							Name: "mockSvc1",
							Type: manifest.BackendServiceType,
						},
						{
							App:  testApp,
							Name: "mockSvc2",
							Type: manifest.BackendServiceType,
						},
						{
							App:  testApp,
							Name: "mockJob1",
							Type: manifest.ScheduledJobType,
						},
						{
							App:  testApp,
							Name: "mockJob2",
							Type: manifest.ScheduledJobType,
						},
					}, nil)

				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{Name: "test1"},
						{Name: "test2"},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test1").
					Return([]string{"mockJob1"}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test2").
					Return([]string{"mockJob1", "mockJob2"}, nil)

				m.prompt.
					EXPECT().
					SelectOne("Select a deployed job", "Help text", []string{"mockJob1 (test2)", "mockJob2 (test2)"}, gomock.Any()).
					Return("mockJob1 (test2)", nil)
			},
			wantEnv: "test2",
			wantJob: "mockJob1",
		},
		"filter returns error": {
			opts: []GetDeployedWorkloadOpts{
				WithWkldFilter(func(job *DeployedWorkload) (bool, error) {
					return job.Env == "test1", fmt.Errorf("filter error")
				}),
			},
			setupMocks: func(m deploySelectMocks) {
				m.configSvc.
					EXPECT().
					ListWorkloads(testApp).
					Return([]*config.Workload{
						{
							App:  testApp,
							Name: "mockJob1",
							Type: manifest.ScheduledJobType,
						},
						{
							App:  testApp,
							Name: "mockJob2",
							Type: manifest.ScheduledJobType,
						},
						{
							App:  testApp,
							Name: "mockSvc3",
							Type: manifest.LoadBalancedWebServiceType,
						},
					}, nil)

				m.configSvc.
					EXPECT().
					ListEnvironments(testApp).
					Return([]*config.Environment{
						{Name: "test1"},
						{Name: "test2"},
					}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test1").
					Return([]string{"mockJob1", "mockJob2"}, nil)

				m.deploySvc.
					EXPECT().
					ListDeployedJobs(testApp, "test2").
					Return([]string{"mockJob1", "mockJob2"}, nil)
			},
			wantErr: fmt.Errorf("filter error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockdeploySvc := mocks.NewMockdeployedWorkloadsRetriever(ctrl)
			mockconfigSvc := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := deploySelectMocks{
				deploySvc: mockdeploySvc,
				configSvc: mockconfigSvc,
				prompt:    mockprompt,
			}
			tc.setupMocks(mocks)

			sel := DeploySelector{
				ConfigSelector: &ConfigSelector{
					AppEnvSelector: &AppEnvSelector{
						appEnvLister: mockconfigSvc,
						prompt:       mockprompt,
					},
					workloadLister: mockconfigSvc,
				},
				deployStoreSvc: mockdeploySvc,
			}
			opts := append([]GetDeployedWorkloadOpts{WithEnv(tc.env), WithName(tc.job)}, tc.opts...)

			gotDeployed, err := sel.DeployedJob("Select a deployed job", "Help text", testApp, opts...)
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantJob, gotDeployed.Name)
				require.Equal(t, tc.wantEnv, gotDeployed.Env)
			}
		})
	}
}

type workspaceSelectMocks struct {
	ws           *mocks.MockworkspaceRetriever
	prompt       *mocks.Mockprompter
	configLister *mocks.MockconfigLister
}

func TestWorkspaceSelect_Service(t *testing.T) {
	testCases := map[string]struct {
		setupMocks func(mocks workspaceSelectMocks)
		wantErr    error
		want       string
	}{
		"with no workspace services and no store services": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListServices().Return(
					[]string{}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					[]*config.Workload{}, nil).Times(1)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			wantErr: fmt.Errorf("no services found"),
		},
		"with one workspace service but no store services": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListServices().Return(
					[]string{
						"service1",
					}, nil).
					Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					[]*config.Workload{}, nil).Times(1)
			},
			wantErr: fmt.Errorf("no services found"),
		},
		"with one store service but no workspace services": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListServices().Return(
					[]string{}, nil).
					Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "service1",
							Type: "load balanced web service",
						},
					}, nil).Times(1)
			},
			wantErr: fmt.Errorf("no services found"),
		},
		"with only one service in both workspace and store (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListServices().Return(
					[]string{
						"service1",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "service1",
							Type: "load balanced web service",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			want: "service1",
		},
		"with multiple workspace services but only one store service (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListServices().Return(
					[]string{
						"service1",
						"service2",
						"service3",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "service1",
							Type: "load balanced web service",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			want: "service1",
		},
		"with multiple store services but only one workspace service (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListServices().Return(
					[]string{
						"service3",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "service1",
							Type: "load balanced web service",
						},
						{
							App:  "app-name",
							Name: "service2",
							Type: "load balanced web service",
						},
						{
							App:  "app-name",
							Name: "service3",
							Type: "load balanced web service",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			want: "service3",
		},
		"with multiple workspace services and multiple store services, of which multiple overlap": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.
					EXPECT().ListServices().Return(
					[]string{
						"service1",
						"service2",
						"service3",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "service2",
							Type: "load balanced web service",
						},
						{
							App:  "app-name",
							Name: "service3",
							Type: "load balanced web service",
						},
						{
							App:  "app-name",
							Name: "service4",
							Type: "load balanced web service",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select a service"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"service2", "service3"}),
						gomock.Any()).
					Return("service2", nil).Times(1)
			},
			want: "service2",
		},
		"with error retrieving services from workspace": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.
					EXPECT().ListServices().Return(
					[]string{""}, errors.New("some error"))
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
			},
			wantErr: errors.New("retrieve services from workspace: some error"),
		},
		"with error retrieving services from store": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListServices().Return(
					[]string{
						"service1",
						"service2",
					}, nil).
					Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					nil, errors.New("some error"))
			},
			wantErr: errors.New("retrieve services from store: some error"),
		},
		"with error selecting services": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.
					EXPECT().ListServices().Return(
					[]string{
						"service1",
						"service2",
					}, nil).
					Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListServices("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "service1",
							Type: "load balanced web service",
						},
						{
							App:  "app-name",
							Name: "service2",
							Type: "load balanced web service",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"service1", "service2"}), gomock.Any()).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select service: error selecting"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockwsRetriever := mocks.NewMockworkspaceRetriever(ctrl)
			MockconfigLister := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := workspaceSelectMocks{
				ws:           mockwsRetriever,
				configLister: MockconfigLister,
				prompt:       mockprompt,
			}
			tc.setupMocks(mocks)

			sel := LocalWorkloadSelector{
				ConfigSelector: &ConfigSelector{
					AppEnvSelector: &AppEnvSelector{
						prompt:       mockprompt,
						appEnvLister: MockconfigLister,
					},
					workloadLister: MockconfigLister,
				},
				ws: mockwsRetriever,
			}
			got, err := sel.Service("Select a service", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestWorkspaceSelect_Job(t *testing.T) {
	testCases := map[string]struct {
		setupMocks func(mocks workspaceSelectMocks)
		wantErr    error
		want       string
	}{
		"with no workspace jobs and no store jobs": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.
					EXPECT().ListJobs().Return(
					[]string{}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListJobs("app-name").Return(
					[]*config.Workload{}, nil).Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			wantErr: fmt.Errorf("no jobs found"),
		},
		"with one workspace job but no store jobs": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.
					EXPECT().ListJobs().Return(
					[]string{
						"job1",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListJobs("app-name").Return(
					[]*config.Workload{}, nil).Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			wantErr: fmt.Errorf("no jobs found"),
		},
		"with one store job but no workspace jobs": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.
					EXPECT().ListJobs().Return(
					[]string{}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListJobs("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "job1",
							Type: "Scheduled Job",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			wantErr: fmt.Errorf("no jobs found"),
		},
		"with only one in both workspace and store (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {

				m.ws.
					EXPECT().ListJobs().Return(
					[]string{
						"resizer",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListJobs("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "resizer",
							Type: "Scheduled Job",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			want: "resizer",
		},
		"with multiple workspace jobs but only one store job (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListJobs().Return(
					[]string{
						"job1",
						"job2",
						"job3",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListJobs("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "job2",
							Type: "Scheduled Job",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			want: "job2",
		},
		"with multiple store jobs but only one workspace job (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListJobs().Return(
					[]string{
						"job3",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListJobs("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "job1",
							Type: "Scheduled Job",
						},
						{
							App:  "app-name",
							Name: "job2",
							Type: "Scheduled Job",
						},
						{
							App:  "app-name",
							Name: "job3",
							Type: "Scheduled Job",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			want: "job3",
		},
		"with multiple workspace jobs and multiple store jobs, of which multiple overlap": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListJobs().Return(
					[]string{
						"job1",
						"job2",
						"job3",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.
					EXPECT().
					ListJobs("app-name").
					Return(
						[]*config.Workload{
							{
								App:  "app-name",
								Name: "job2",
								Type: "Scheduled Job",
							},
							{
								App:  "app-name",
								Name: "job3",
								Type: "Scheduled Job",
							},
							{
								App:  "app-name",
								Name: "job4",
								Type: "Scheduled Job",
							},
						}, nil).Times(1)
				m.prompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select a job"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"job2", "job3"}),
						gomock.Any()).
					Return("job2", nil).
					Times(1)
			},
			want: "job2",
		},
		"with error retrieving jobs from workspace": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.
					EXPECT().ListJobs().Return(
					[]string{""}, errors.New("some error"))
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
			},
			wantErr: errors.New("retrieve jobs from workspace: some error"),
		},
		"with error retrieving jobs from store": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListJobs().Return(
					[]string{
						"service1",
						"service2",
					}, nil).
					Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListJobs("app-name").Return(
					nil, errors.New("some error"))
			},
			wantErr: errors.New("retrieve jobs from store: some error"),
		},
		"with error selecting jobs": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().ListJobs().Return(
					[]string{
						"resizer1",
						"resizer2",
					}, nil).Times(1)
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "app-name",
					}, nil)
				m.configLister.EXPECT().ListJobs("app-name").Return(
					[]*config.Workload{
						{
							App:  "app-name",
							Name: "resizer1",
							Type: "Scheduled Job",
						},
						{
							App:  "app-name",
							Name: "resizer2",
							Type: "Scheduled Job",
						},
					}, nil).Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"resizer1", "resizer2"}), gomock.Any()).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select job: error selecting"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockwsRetriever := mocks.NewMockworkspaceRetriever(ctrl)
			MockconfigLister := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := workspaceSelectMocks{
				ws:           mockwsRetriever,
				configLister: MockconfigLister,
				prompt:       mockprompt,
			}
			tc.setupMocks(mocks)

			sel := LocalWorkloadSelector{
				ConfigSelector: &ConfigSelector{
					AppEnvSelector: &AppEnvSelector{
						prompt:       mockprompt,
						appEnvLister: MockconfigLister,
					},
					workloadLister: MockconfigLister,
				},
				ws: mockwsRetriever,
			}
			got, err := sel.Job("Select a job", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestWorkspaceSelect_EnvironmentsInWorkspace(t *testing.T) {
	testCases := map[string]struct {
		setupMocks func(mocks workspaceSelectMocks)
		wantErr    error
		want       string
	}{
		"fail to retrieve workspace app name": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("read workspace summary: some error"),
		},
		"fail to list environments in workspace": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("retrieve environments from workspace: some error"),
		},
		"fail to list environments in store": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv1", "mockEnv2"}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("retrieve environments from store: some error"),
		},
		"fail to select an environment": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv1", "mockEnv2"}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						App:  "mockApp",
						Name: "mockEnv1",
					},
					{
						App:  "mockApp",
						Name: "mockEnv2",
					},
				}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"mockEnv1", "mockEnv2"}), gomock.Any()).
					Return("", errors.New("some error")).
					Times(1)
			},
			wantErr: fmt.Errorf("select environment: some error"),
		},
		"with no workspace environments and no store environments": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: fmt.Errorf("no environments found"),
		},
		"with one workspace environment but no store environment": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv"}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: fmt.Errorf("no environments found"),
		},
		"with one store environment but no workspace environments": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						App:  "mockApp",
						Name: "mockEnv",
					},
				}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: fmt.Errorf("no environments found"),
		},
		"with only one in both workspace and store (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv"}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						App:  "mockApp",
						Name: "mockEnv",
					},
				}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			want: "mockEnv",
		},
		"with multiple workspace environments but only one store environment (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv1", "mockEnv2"}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						App:  "mockApp",
						Name: "mockEnv1",
					},
				}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			want: "mockEnv1",
		},
		"with multiple store environments but only one workspace environment (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv1"}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						App:  "mockApp",
						Name: "mockEnv1",
					},
					{
						App:  "mockApp",
						Name: "mockEnv2",
					},
				}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			want: "mockEnv1",
		},
		"with multiple workspace environments and multiple store environments, of which multiple overlap": {
			setupMocks: func(m workspaceSelectMocks) {
				m.ws.EXPECT().Summary().Return(
					&workspace.Summary{
						Application: "mockApp",
					}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv1", "mockEnv2", "mockEnv3"}, nil).Times(1)
				m.configLister.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						App:  "mockApp",
						Name: "mockEnv1",
					},
					{
						App:  "mockApp",
						Name: "mockEnv2",
					},
					{
						App:  "mockApp",
						Name: "mockEnv4",
					},
				}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"mockEnv1", "mockEnv2"}), gomock.Any()).
					Return("mockEnv1", nil).
					Times(1)
			},
			want: "mockEnv1",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := workspaceSelectMocks{
				ws:           mocks.NewMockworkspaceRetriever(ctrl),
				configLister: mocks.NewMockconfigLister(ctrl),
				prompt:       mocks.NewMockprompter(ctrl),
			}
			tc.setupMocks(m)

			sel := LocalEnvironmentSelector{
				AppEnvSelector: &AppEnvSelector{
					prompt:       m.prompt,
					appEnvLister: m.configLister,
				},
				ws: m.ws,
			}
			got, err := sel.LocalEnvironment("Select an environment", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

type configSelectMocks struct {
	workloadLister *mocks.MockconfigLister
	prompt         *mocks.Mockprompter
}

func TestConfigSelect_Service(t *testing.T) {
	appName := "myapp"
	testCases := map[string]struct {
		setupMocks func(m configSelectMocks)
		wantErr    error
		want       string
	}{
		"with no services": {
			setupMocks: func(m configSelectMocks) {
				m.workloadLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Workload{}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("no services found in app myapp"),
		},
		"with only one service (skips prompting)": {
			setupMocks: func(m configSelectMocks) {
				m.workloadLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Workload{
						{
							App:  appName,
							Name: "service1",
							Type: "load balanced web service",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "service1",
		},
		"with multiple services": {
			setupMocks: func(m configSelectMocks) {
				m.workloadLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Workload{
						{
							App:  appName,
							Name: "service1",
							Type: "load balanced web service",
						},
						{
							App:  appName,
							Name: "service2",
							Type: "backend service",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select a service"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"service1", "service2"}),
						gomock.Any()).
					Return("service2", nil).
					Times(1)
			},
			want: "service2",
		},
		"with error selecting services": {
			setupMocks: func(m configSelectMocks) {
				m.workloadLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Workload{
						{
							App:  appName,
							Name: "service1",
							Type: "load balanced web service",
						},
						{
							App:  appName,
							Name: "service2",
							Type: "backend service",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"service1", "service2"}), gomock.Any()).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select service: error selecting"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockconfigLister := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := configSelectMocks{
				workloadLister: mockconfigLister,
				prompt:         mockprompt,
			}
			tc.setupMocks(mocks)

			sel := ConfigSelector{
				AppEnvSelector: &AppEnvSelector{
					prompt: mockprompt,
				},
				workloadLister: mockconfigLister,
			}

			got, err := sel.Service("Select a service", "Help text", appName)
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestConfigSelect_Job(t *testing.T) {
	appName := "myapp"
	testCases := map[string]struct {
		setupMocks func(m configSelectMocks)
		wantErr    error
		want       string
	}{
		"with no jobs": {
			setupMocks: func(m configSelectMocks) {
				m.workloadLister.
					EXPECT().
					ListJobs(gomock.Eq(appName)).
					Return([]*config.Workload{}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("no jobs found in app myapp"),
		},
		"with only one job (skips prompting)": {
			setupMocks: func(m configSelectMocks) {
				m.workloadLister.
					EXPECT().
					ListJobs(gomock.Eq(appName)).
					Return([]*config.Workload{
						{
							App:  appName,
							Name: "job1",
							Type: "load balanced web service",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "job1",
		},
		"with multiple jobs": {
			setupMocks: func(m configSelectMocks) {
				m.workloadLister.
					EXPECT().
					ListJobs(gomock.Eq(appName)).
					Return([]*config.Workload{
						{
							App:  appName,
							Name: "job1",
							Type: "load balanced web service",
						},
						{
							App:  appName,
							Name: "job2",
							Type: "backend service",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select a job"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"job1", "job2"}),
						gomock.Any()).
					Return("job2", nil).
					Times(1)
			},
			want: "job2",
		},
		"with error selecting jobs": {
			setupMocks: func(m configSelectMocks) {
				m.workloadLister.
					EXPECT().
					ListJobs(gomock.Eq(appName)).
					Return([]*config.Workload{
						{
							App:  appName,
							Name: "job1",
							Type: "load balanced web service",
						},
						{
							App:  appName,
							Name: "job2",
							Type: "backend service",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"job1", "job2"}), gomock.Any()).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select job: error selecting"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockconfigLister := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := configSelectMocks{
				workloadLister: mockconfigLister,
				prompt:         mockprompt,
			}
			tc.setupMocks(mocks)

			sel := ConfigSelector{
				AppEnvSelector: &AppEnvSelector{
					prompt: mockprompt,
				},
				workloadLister: mockconfigLister,
			}

			got, err := sel.Job("Select a job", "Help text", appName)
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

type environmentMocks struct {
	envLister *mocks.MockconfigLister
	prompt    *mocks.Mockprompter
}

func TestSelect_Environment(t *testing.T) {
	appName := "myapp"
	additionalOpt1, additionalOpt2 := "opt1", "opt2"

	testCases := map[string]struct {
		inAdditionalOpts []string

		setupMocks func(m environmentMocks)
		wantErr    error
		want       string
	}{
		"with no environments": {
			setupMocks: func(m environmentMocks) {
				m.envLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("no environments found in app myapp"),
		},
		"with only one environment (skips prompting)": {
			setupMocks: func(m environmentMocks) {
				m.envLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{
						{
							App:  appName,
							Name: "env1",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "env1",
		},
		"with multiple environments": {
			setupMocks: func(m environmentMocks) {
				m.envLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{
						{
							App:  appName,
							Name: "env1",
						},
						{
							App:  appName,
							Name: "env2",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select an environment"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"env1", "env2"}),
						gomock.Any()).
					Return("env2", nil).
					Times(1)
			},
			want: "env2",
		},
		"with error selecting environments": {
			setupMocks: func(m environmentMocks) {
				m.envLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{
						{
							App:  appName,
							Name: "env1",
						},
						{
							App:  appName,
							Name: "env2",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"env1", "env2"}), gomock.Any()).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select environment: error selecting"),
		},
		"no environment but with one additional option": {
			inAdditionalOpts: []string{additionalOpt1},
			setupMocks: func(m environmentMocks) {
				m.envLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},

			want: additionalOpt1,
		},
		"no environment but with multiple additional options": {
			inAdditionalOpts: []string{additionalOpt1, additionalOpt2},
			setupMocks: func(m environmentMocks) {
				m.envLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), []string{additionalOpt1, additionalOpt2}, gomock.Any()).
					Times(1).
					Return(additionalOpt2, nil)
			},

			want: additionalOpt2,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockenvLister := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := environmentMocks{
				envLister: mockenvLister,
				prompt:    mockprompt,
			}
			tc.setupMocks(mocks)

			sel := AppEnvSelector{
				prompt:       mockprompt,
				appEnvLister: mockenvLister,
			}

			got, err := sel.Environment("Select an environment", "Help text", appName, tc.inAdditionalOpts...)
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSelect_Environments(t *testing.T) {
	appName := "myapp"
	hardcodedOpt := "[No additional environments]"

	testCases := map[string]struct {
		setupMocks func(m environmentMocks)
		wantErr    error
		want       []string
	}{
		"with no environments": {
			setupMocks: func(m environmentMocks) {
				gomock.InOrder(
					m.envLister.
						EXPECT().
						ListEnvironments(gomock.Eq(appName)).
						Return([]*config.Environment{}, nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Times(0),
				)
			},
			wantErr: fmt.Errorf("no environments found in app myapp"),
		},
		"with one environment": {
			setupMocks: func(m environmentMocks) {
				gomock.InOrder(
					m.envLister.
						EXPECT().
						ListEnvironments(gomock.Eq(appName)).
						Return([]*config.Environment{
							{
								App:  appName,
								Name: "env1",
							},
						}, nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"env1", hardcodedOpt}), gomock.Any()).
						Return("env1", nil).
						Times(1),
				)
			},
			want: []string{"env1"},
		},
		"with multiple environments (selection list reduces with each iteration, returns envs in order selected)": {
			setupMocks: func(m environmentMocks) {
				gomock.InOrder(
					m.envLister.
						EXPECT().
						ListEnvironments(gomock.Eq(appName)).
						Return([]*config.Environment{
							{
								App:  appName,
								Name: "env1",
							},
							{
								App:  appName,
								Name: "env2",
							},
							{
								App:  appName,
								Name: "env3",
							},
						}, nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(
							gomock.Eq("Select an environment"),
							gomock.Eq("Help text"),
							gomock.Eq([]string{"env1", "env2", "env3", hardcodedOpt}),
							gomock.Any()).
						Return("env2", nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(
							gomock.Eq("Select an environment"),
							gomock.Eq("Help text"),
							gomock.Eq([]string{"env1", "env3", hardcodedOpt}),
							gomock.Any()).
						Return("env1", nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(
							gomock.Eq("Select an environment"),
							gomock.Eq("Help text"),
							gomock.Eq([]string{"env3", hardcodedOpt}),
							gomock.Any()).
						Return("env3", nil).
						Times(1),
				)
			},
			want: []string{"env2", "env1", "env3"},
		},
		"stops prompting when user selects '[No additional environments]'; quit opt not in env list": {
			setupMocks: func(m environmentMocks) {
				gomock.InOrder(
					m.envLister.
						EXPECT().
						ListEnvironments(gomock.Eq(appName)).
						Return([]*config.Environment{
							{
								App:  appName,
								Name: "env1",
							},
							{
								App:  appName,
								Name: "env2",
							},
							{
								App:  appName,
								Name: "env3",
							},
						}, nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(
							gomock.Eq("Select an environment"),
							gomock.Eq("Help text"),
							gomock.Eq([]string{"env1", "env2", "env3", hardcodedOpt}),
							gomock.Any()).
						Return("env2", nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(
							gomock.Eq("Select an environment"),
							gomock.Eq("Help text"),
							gomock.Eq([]string{"env1", "env3", hardcodedOpt}),
							gomock.Any()).
						Return("env1", nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(
							gomock.Eq("Select an environment"),
							gomock.Eq("Help text"),
							gomock.Eq([]string{"env3", hardcodedOpt}),
							gomock.Any()).
						Return(hardcodedOpt, nil).
						Times(1),
				)
			},
			want: []string{"env2", "env1"},
		},
		"with error selecting environments": {
			setupMocks: func(m environmentMocks) {
				gomock.InOrder(
					m.envLister.
						EXPECT().
						ListEnvironments(gomock.Eq(appName)).
						Return([]*config.Environment{
							{
								App:  appName,
								Name: "env1",
							},
							{
								App:  appName,
								Name: "env2",
							},
						}, nil).
						Times(1),
					m.prompt.
						EXPECT().
						SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"env1", "env2", hardcodedOpt}), gomock.Any()).
						Return("", fmt.Errorf("error selecting")).
						Times(1),
				)
			},
			wantErr: fmt.Errorf("select environments: error selecting"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockenvLister := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := environmentMocks{
				envLister: mockenvLister,
				prompt:    mockprompt,
			}
			tc.setupMocks(mocks)

			sel := AppEnvSelector{
				prompt:       mockprompt,
				appEnvLister: mockenvLister,
			}

			got, err := sel.Environments("Select an environment", "Help text", appName, func(order int) prompt.PromptConfig {
				return prompt.WithFinalMessage(fmt.Sprintf("%s stage:", humanize.Ordinal(order)))
			})
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

type applicationMocks struct {
	appLister *mocks.MockconfigLister
	prompt    *mocks.Mockprompter
}

func TestSelect_Application(t *testing.T) {
	testCases := map[string]struct {
		setupMocks func(m applicationMocks)
		wantErr    error
		want       string
	}{
		"with no apps": {
			setupMocks: func(m applicationMocks) {
				m.appLister.
					EXPECT().
					ListApplications().
					Return([]*config.Application{}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("no apps found"),
		},
		"with only one app (skips prompting)": {
			setupMocks: func(m applicationMocks) {
				m.appLister.
					EXPECT().
					ListApplications().
					Return([]*config.Application{
						{
							Name: "app1",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "app1",
		},
		"with multiple apps": {
			setupMocks: func(m applicationMocks) {
				m.appLister.
					EXPECT().
					ListApplications().
					Return([]*config.Application{
						{
							Name: "app1",
						},
						{
							Name: "app2",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select an app"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"app1", "app2"}),
						gomock.Any()).
					Return("app2", nil).
					Times(1)
			},
			want: "app2",
		},
		"with error selecting apps": {
			setupMocks: func(m applicationMocks) {
				m.appLister.
					EXPECT().
					ListApplications().
					Return([]*config.Application{
						{
							Name: "app1",
						},
						{
							Name: "app2",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"app1", "app2"}), gomock.Any()).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select application: error selecting"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockappLister := mocks.NewMockconfigLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := applicationMocks{
				appLister: mockappLister,
				prompt:    mockprompt,
			}
			tc.setupMocks(mocks)

			sel := AppEnvSelector{
				prompt:       mockprompt,
				appEnvLister: mockappLister,
			}

			got, err := sel.Application("Select an app", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestOther_Dockerfile(t *testing.T) {
	dockerfiles := []string{
		"./Dockerfile",
		"backend/Dockerfile",
		"frontend/Dockerfile",
	}
	dockerfileOptions := []string{
		"./Dockerfile",
		"backend/Dockerfile",
		"frontend/Dockerfile",
		"Enter custom path for your Dockerfile",
		"Use an existing image instead",
	}
	testCases := map[string]struct {
		mockWs     func(retriever *mocks.MockworkspaceRetriever)
		mockPrompt func(*mocks.Mockprompter)

		wantedErr        error
		wantedDockerfile string
	}{
		"choose an existing Dockerfile": {
			mockWs: func(m *mocks.MockworkspaceRetriever) {
				m.EXPECT().ListDockerfiles().Return(dockerfiles, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(
					gomock.Any(), gomock.Any(),
					gomock.Eq(dockerfileOptions),
					gomock.Any(),
				).Return("frontend/Dockerfile", nil)
			},
			wantedErr:        nil,
			wantedDockerfile: "frontend/Dockerfile",
		},
		"prompts user for custom path": {
			mockWs: func(m *mocks.MockworkspaceRetriever) {
				m.EXPECT().ListDockerfiles().Return([]string{}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(
					gomock.Any(), gomock.Any(),
					gomock.Eq([]string{
						"Enter custom path for your Dockerfile",
						"Use an existing image instead",
					}),
					gomock.Any(),
				).Return("Enter custom path for your Dockerfile", nil)
				m.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("crazy/path/Dockerfile", nil)
			},
			wantedErr:        nil,
			wantedDockerfile: "crazy/path/Dockerfile",
		},
		"returns an error if fail to list Dockerfile": {
			mockWs: func(m *mocks.MockworkspaceRetriever) {
				m.EXPECT().ListDockerfiles().Return(nil, errors.New("some error"))
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			wantedErr:  fmt.Errorf("list Dockerfiles: some error"),
		},
		"returns an error if fail to select Dockerfile": {
			mockWs: func(m *mocks.MockworkspaceRetriever) {
				m.EXPECT().ListDockerfiles().Return(dockerfiles, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("select Dockerfile: some error"),
		},
		"returns an error if fail to get custom Dockerfile path": {
			mockWs: func(m *mocks.MockworkspaceRetriever) {
				m.EXPECT().ListDockerfiles().Return(dockerfiles, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(
					gomock.Any(), gomock.Any(),
					gomock.Eq(dockerfileOptions),
					gomock.Any(),
				).Return("Enter custom path for your Dockerfile", nil)
				m.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get custom Dockerfile path: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			p := mocks.NewMockprompter(ctrl)
			cfg := mocks.NewMockworkspaceRetriever(ctrl)
			tc.mockPrompt(p)
			tc.mockWs(cfg)

			sel := WorkspaceSelector{
				prompt: p,
				ws:     cfg,
			}

			mockPromptText := "prompt"
			mockHelpText := "help"

			// WHEN
			dockerfile, err := sel.Dockerfile(
				mockPromptText,
				mockPromptText,
				mockHelpText,
				mockHelpText,
				func(v interface{}) error { return nil },
			)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedDockerfile, dockerfile)
			}
		})
	}
}

func TestWorkspaceSelect_Schedule(t *testing.T) {
	scheduleTypePrompt := "HAY WHAT SCHEDULE"
	scheduleTypeHelp := "NO"

	testCases := map[string]struct {
		mockWs         func(retriever *mocks.MockworkspaceRetriever)
		mockPrompt     func(*mocks.Mockprompter)
		wantedSchedule string
		wantedErr      error
	}{
		"error asking schedule type": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get schedule type: some error"),
		},
		"ask for rate": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(rate, nil),
					m.EXPECT().Get(ratePrompt, rateHelp, gomock.Any(), gomock.Any()).Return("1h30m", nil),
				)
			},
			wantedSchedule: "@every 1h30m",
		},
		"error getting rate": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(rate, nil),
					m.EXPECT().Get(ratePrompt, rateHelp, gomock.Any(), gomock.Any()).Return("", fmt.Errorf("some error")),
				)
			},
			wantedErr: errors.New("get schedule rate: some error"),
		},
		"ask for cron": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Daily", nil),
				)
			},
			wantedSchedule: "@daily",
		},
		"error getting cron": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get preset schedule: some error"),
		},
		"ask for custom schedule": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("0 * * * *", nil),
					m.EXPECT().Confirm(humanReadableCronConfirmPrompt, humanReadableCronConfirmHelp).Return(true, nil),
				)
			},
			wantedSchedule: "0 * * * *",
		},
		"error getting custom schedule": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get custom schedule: some error"),
		},
		"error confirming custom schedule": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("0 * * * *", nil),
					m.EXPECT().Confirm(humanReadableCronConfirmPrompt, humanReadableCronConfirmHelp).Return(false, errors.New("some error")),
				)
			},
			wantedErr: errors.New("confirm cron schedule: some error"),
		},
		"custom schedule using valid definition string results in no confirm": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("@hourly", nil),
				)
			},
			wantedSchedule: "@hourly",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			p := mocks.NewMockprompter(ctrl)
			cfg := mocks.NewMockworkspaceRetriever(ctrl)
			tc.mockPrompt(p)
			sel := WorkspaceSelector{
				prompt: p,
				ws:     cfg,
			}

			var mockValidator prompt.ValidatorFunc = func(interface{}) error { return nil }

			// WHEN
			schedule, err := sel.Schedule(scheduleTypePrompt, scheduleTypeHelp, mockValidator, mockValidator)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedSchedule, schedule)
			}
		})
	}
}

type wsPipelineSelectMocks struct {
	prompt *mocks.Mockprompter
	ws     *mocks.MockwsPipelinesLister
}

func TestWorkspaceSelect_WsPipeline(t *testing.T) {
	mockPipelineManifests := []workspace.PipelineManifest{
		{
			Name: "betaManifest",
			Path: "/copilot/pipelines/beta/manifest.yml",
		},
		{
			Name: "legacyInCopiDir",
			Path: "/copilot/pipeline.yml",
		},
		{
			Name: "prodManifest",
			Path: "/copilot/pipelines/prod/manifest.yml",
		},
	}
	singlePipelineManifest := &workspace.PipelineManifest{
		Name: "betaManifest",
		Path: "/copilot/pipelines/beta/manifest.yml",
	}
	testCases := map[string]struct {
		setupMocks     func(mocks wsPipelineSelectMocks)
		wantedErr      error
		wantedPipeline *workspace.PipelineManifest
	}{
		"with no workspace pipelines": {
			setupMocks: func(m wsPipelineSelectMocks) {
				m.ws.EXPECT().ListPipelines().Return(nil, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			wantedErr: fmt.Errorf("no pipelines found"),
		},
		"don't prompt to select if only one workspace pipeline": {
			setupMocks: func(m wsPipelineSelectMocks) {
				m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{
					{
						Name: "betaManifest",
						Path: "/copilot/pipelines/beta/manifest.yml",
					}}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			wantedPipeline: &workspace.PipelineManifest{
				Name: "betaManifest",
				Path: "/copilot/pipelines/beta/manifest.yml",
			},
		},
		"with multiple workspace pipelines": {
			setupMocks: func(m wsPipelineSelectMocks) {
				m.ws.EXPECT().ListPipelines().Return(mockPipelineManifests, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("betaManifest", nil)
			},
			wantedPipeline: singlePipelineManifest,
		},
		"with error selecting": {
			setupMocks: func(m wsPipelineSelectMocks) {
				m.ws.EXPECT().ListPipelines().Return(mockPipelineManifests, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New("select pipeline: some error"),
		},
		"wrap error from ListPipelines": {
			setupMocks: func(m wsPipelineSelectMocks) {
				m.ws.EXPECT().ListPipelines().Return(nil, errors.New("some error"))

			},
			wantedErr: errors.New("list pipelines: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockwsPipelinesLister := mocks.NewMockwsPipelinesLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := wsPipelineSelectMocks{
				prompt: mockprompt,
				ws:     mockwsPipelinesLister,
			}
			tc.setupMocks(mocks)

			sel := WsPipelineSelector{
				prompt: mockprompt,
				ws:     mockwsPipelinesLister,
			}
			got, err := sel.WsPipeline("Select a pipeline", "Help text")
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedPipeline, got)
			}
		})
	}
}

type codePipelineSelectMocks struct {
	prompt *mocks.Mockprompter
	cp     *mocks.MockcodePipelineLister
}

func TestCodePipelineSelect_DeployedPipeline(t *testing.T) {
	const (
		mockAppName                    = "coolapp"
		mockPipelineResourceName       = "pipeline-coolapp-my-pipeline-repo-ABCDERANDOMRANDOM"
		mockPipelineName               = "my-pipeline-repo"
		mockLegacyPipelineName         = "bad-goose"
		mockLegacyPipelineResourceName = mockLegacyPipelineName // legacy pipeline's resource name is the same as the pipeline name
	)
	mockPipeline := deploy.Pipeline{
		AppName:      mockAppName,
		ResourceName: mockPipelineResourceName,
		Name:         mockPipelineName,
		IsLegacy:     false,
	}
	mockLegacyPipeline := deploy.Pipeline{
		AppName:      mockAppName,
		ResourceName: mockLegacyPipelineResourceName,
		Name:         mockLegacyPipelineName,
		IsLegacy:     true,
	}
	testCases := map[string]struct {
		setupMocks     func(mocks codePipelineSelectMocks)
		wantedErr      error
		wantedPipeline deploy.Pipeline
	}{
		"with no workspace pipelines": {
			setupMocks: func(m codePipelineSelectMocks) {
				m.cp.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			wantedErr: fmt.Errorf("no deployed pipelines found"),
		},
		"don't prompt to select if only one workspace pipeline": {
			setupMocks: func(m codePipelineSelectMocks) {
				m.cp.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			wantedPipeline: mockPipeline,
		},
		"with multiple workspace pipelines": {
			setupMocks: func(m codePipelineSelectMocks) {
				m.cp.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline, mockLegacyPipeline}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("bad-goose", nil)
			},
			wantedPipeline: mockLegacyPipeline,
		},
		"with error selecting": {
			setupMocks: func(m codePipelineSelectMocks) {
				m.cp.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline, mockLegacyPipeline}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New("select pipeline: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCodePipelinesLister := mocks.NewMockcodePipelineLister(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mocks := codePipelineSelectMocks{
				prompt: mockPrompt,
				cp:     mockCodePipelinesLister,
			}
			tc.setupMocks(mocks)

			sel := CodePipelineSelector{
				prompt:         mockPrompt,
				pipelineLister: mockCodePipelinesLister,
			}
			got, err := sel.DeployedPipeline("Select a pipeline", "Help text", mockAppName)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedPipeline, got)
			}
		})
	}
}

func TestSelect_CFTask(t *testing.T) {
	taskPrompt := "TASK PLX"
	taskHelp := "NO"
	testTasks := []string{"abc", "db-migrate"}
	testDefaultTask := "db-migrate"
	testCases := map[string]struct {
		inDefaultCluster string
		inOpts           []GetDeployedTaskOpts

		mockStore  func(*mocks.MockconfigLister)
		mockPrompt func(*mocks.Mockprompter)
		mockCF     func(*mocks.MocktaskStackDescriber)

		wantedErr  error
		wantedTask string
	}{
		"choose an existing task": {
			inOpts: []GetDeployedTaskOpts{
				TaskWithAppEnv("phonetool", "prod-iad"),
			},
			mockStore: func(m *mocks.MockconfigLister) {},
			mockCF: func(m *mocks.MocktaskStackDescriber) {
				m.EXPECT().ListTaskStacks("phonetool", "prod-iad").Return([]deploy.TaskStackInfo{
					{
						StackName: "copilot-abc",
						App:       "phonetool",
						Env:       "prod-iad",
					},
					{
						StackName: "copilot-db-migrate",
						App:       "phonetool",
						Env:       "prod-iad",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(
					gomock.Any(), gomock.Any(),
					[]string{
						"abc",
						"db-migrate",
					},
					gomock.Any(),
				).Return("abc", nil)
			},
			wantedErr:  nil,
			wantedTask: testTasks[0],
		},
		"error when retrieving stacks": {
			inOpts: []GetDeployedTaskOpts{
				TaskWithAppEnv("phonetool", "prod-iad"),
			},
			mockStore: func(m *mocks.MockconfigLister) {},
			mockCF: func(m *mocks.MocktaskStackDescriber) {
				m.EXPECT().ListTaskStacks("phonetool", "prod-iad").Return(nil, errors.New("some error"))
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			wantedErr:  errors.New("get tasks in environment prod-iad: some error"),
		},
		"with default cluster task": {
			inOpts: []GetDeployedTaskOpts{
				TaskWithDefaultCluster(),
			},
			mockStore: func(m *mocks.MockconfigLister) {},
			mockCF: func(m *mocks.MocktaskStackDescriber) {
				m.EXPECT().ListDefaultTaskStacks().Return([]deploy.TaskStackInfo{
					{
						StackName: "task-oneoff",
					},
					{
						StackName: "copilot-db-migrate",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(
					gomock.Any(), gomock.Any(),
					[]string{
						"oneoff",
						"db-migrate",
					},
					gomock.Any(),
				).Return("db-migrate", nil)
			},
			wantedErr:  nil,
			wantedTask: testDefaultTask,
		},
		"with error getting default cluster tasks": {
			inOpts: []GetDeployedTaskOpts{
				TaskWithDefaultCluster(),
			},
			mockStore: func(m *mocks.MockconfigLister) {},
			mockCF: func(m *mocks.MocktaskStackDescriber) {
				m.EXPECT().ListDefaultTaskStacks().Return(nil, errors.New("some error"))
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			wantedErr:  errors.New("get tasks in default cluster: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			p := mocks.NewMockprompter(ctrl)
			s := mocks.NewMockconfigLister(ctrl)
			cf := mocks.NewMocktaskStackDescriber(ctrl)
			tc.mockPrompt(p)
			tc.mockCF(cf)
			tc.mockStore(s)

			sel := CFTaskSelector{
				AppEnvSelector: &AppEnvSelector{
					prompt:       p,
					appEnvLister: s,
				},
				cfStore: cf,
			}

			// WHEN
			choice, err := sel.Task(taskPrompt, taskHelp, tc.inOpts...)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedTask, choice)
			}
		})
	}
}

type taskSelectMocks struct {
	taskLister *mocks.MocktaskLister
	prompt     *mocks.Mockprompter
}

func TestTaskSelect_Task(t *testing.T) {
	const (
		mockApp        = "mockApp"
		mockEnv        = "mockEnv"
		mockPromptText = "Select a running task"
		mockHelpText   = "Help text"
	)
	mockTask1 := &awsecs.Task{
		TaskArn:           aws.String("arn:aws:ecs:us-west-2:123456789:task/4082490ee6c245e09d2145010aa1ba8d"),
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-west-2:123456789:task-definition/sample-fargate:2"),
	}
	mockTask2 := &awsecs.Task{
		TaskArn:           aws.String("arn:aws:ecs:us-west-2:123456789:task/0aa1ba8d4082490ee6c245e09d214501"),
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-west-2:123456789:task-definition/sample-fargate:3"),
	}
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks taskSelectMocks)
		app        string
		env        string
		useDefault bool

		wantErr  error
		wantTask *awsecs.Task
	}{
		"return error if fail to list active cluster tasks": {
			useDefault: true,
			setupMocks: func(m taskSelectMocks) {
				m.taskLister.EXPECT().ListActiveDefaultClusterTasks(ecs.ListTasksFilter{CopilotOnly: true}).Return(nil, mockErr)
			},
			wantErr: fmt.Errorf("list active tasks for default cluster: some error"),
		},
		"return error if fail to list active app env tasks": {
			app: mockApp,
			env: mockEnv,
			setupMocks: func(m taskSelectMocks) {
				m.taskLister.EXPECT().ListActiveAppEnvTasks(ecs.ListActiveAppEnvTasksOpts{
					App: mockApp,
					Env: mockEnv,
					ListTasksFilter: ecs.ListTasksFilter{
						CopilotOnly: true,
					},
				}).Return(nil, mockErr)
			},
			wantErr: fmt.Errorf("list active tasks in environment mockEnv: some error"),
		},
		"return error if no running tasks found": {
			app: mockApp,
			env: mockEnv,
			setupMocks: func(m taskSelectMocks) {
				m.taskLister.EXPECT().ListActiveAppEnvTasks(ecs.ListActiveAppEnvTasksOpts{
					App: mockApp,
					Env: mockEnv,
					ListTasksFilter: ecs.ListTasksFilter{
						CopilotOnly: true,
					},
				}).Return([]*awsecs.Task{}, nil)
			},
			wantErr: fmt.Errorf("no running tasks found"),
		},
		"return error if fail to select a task": {
			app: mockApp,
			env: mockEnv,
			setupMocks: func(m taskSelectMocks) {
				m.taskLister.EXPECT().ListActiveAppEnvTasks(ecs.ListActiveAppEnvTasksOpts{
					App: mockApp,
					Env: mockEnv,
					ListTasksFilter: ecs.ListTasksFilter{
						CopilotOnly: true,
					},
				}).Return([]*awsecs.Task{mockTask1, mockTask2}, nil)
				m.prompt.EXPECT().SelectOne(mockPromptText, mockHelpText, []string{
					"4082490e (sample-fargate:2)",
					"0aa1ba8d (sample-fargate:3)",
				}, gomock.Any()).Return("", mockErr)
			},
			wantErr: fmt.Errorf("select running task: some error"),
		},
		"success with one running task": {
			app: mockApp,
			env: mockEnv,
			setupMocks: func(m taskSelectMocks) {
				m.taskLister.EXPECT().ListActiveAppEnvTasks(ecs.ListActiveAppEnvTasksOpts{
					App: mockApp,
					Env: mockEnv,
					ListTasksFilter: ecs.ListTasksFilter{
						CopilotOnly: true,
					},
				}).Return([]*awsecs.Task{mockTask1}, nil)
			},
			wantTask: mockTask1,
		},
		"success": {
			app: mockApp,
			env: mockEnv,
			setupMocks: func(m taskSelectMocks) {
				m.taskLister.EXPECT().ListActiveAppEnvTasks(ecs.ListActiveAppEnvTasksOpts{
					App: mockApp,
					Env: mockEnv,
					ListTasksFilter: ecs.ListTasksFilter{
						CopilotOnly: true,
					},
				}).Return([]*awsecs.Task{mockTask1, mockTask2}, nil)
				m.prompt.EXPECT().SelectOne(mockPromptText, mockHelpText, []string{
					"4082490e (sample-fargate:2)",
					"0aa1ba8d (sample-fargate:3)",
				}, gomock.Any()).Return("0aa1ba8d (sample-fargate:3)", nil)
			},
			wantTask: mockTask2,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocktaskLister := mocks.NewMocktaskLister(ctrl)
			mockprompt := mocks.NewMockprompter(ctrl)
			mocks := taskSelectMocks{
				taskLister: mocktaskLister,
				prompt:     mockprompt,
			}
			tc.setupMocks(mocks)

			sel := TaskSelector{
				lister: mocktaskLister,
				prompt: mockprompt,
			}
			var gotTask *awsecs.Task
			var err error
			if tc.useDefault {
				gotTask, err = sel.RunningTask(mockPromptText, mockHelpText,
					WithAppEnv(tc.app, tc.env), WithDefault())
			} else {
				gotTask, err = sel.RunningTask(mockPromptText, mockHelpText,
					WithAppEnv(tc.app, tc.env))
			}
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, tc.wantErr)
				require.Equal(t, tc.wantTask, gotTask)
			}
		})
	}
}
