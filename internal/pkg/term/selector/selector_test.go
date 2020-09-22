// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/selector/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deploySelectMocks struct {
	deploySvc *mocks.MockDeployStoreClient
	configSvc *mocks.MockAppEnvLister
	prompt    *mocks.MockPrompter
}

func TestDeploySelect_Service(t *testing.T) {
	const testApp = "mockApp"
	testCases := map[string]struct {
		setupMocks func(mocks deploySelectMocks)
		svc        string
		env        string

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
			wantErr: fmt.Errorf("list deployed service for environment test: some error"),
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
					SelectOne("Select a deployed service", "Help text", []string{"mockSvc1 (test)", "mockSvc2 (test)"}).
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
					SelectOne("Select a deployed service", "Help text", []string{"mockSvc1 (test)", "mockSvc2 (test)"}).
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockdeploySvc := mocks.NewMockDeployStoreClient(ctrl)
			mockconfigSvc := mocks.NewMockAppEnvLister(ctrl)
			mockprompt := mocks.NewMockPrompter(ctrl)
			mocks := deploySelectMocks{
				deploySvc: mockdeploySvc,
				configSvc: mockconfigSvc,
				prompt:    mockprompt,
			}
			tc.setupMocks(mocks)

			sel := DeploySelect{
				Select: &Select{
					lister: mockconfigSvc,
					prompt: mockprompt,
				},
				deployStoreSvc: mockdeploySvc,
			}
			gotDeployed, err := sel.DeployedService("Select a deployed service", "Help text", testApp, WithEnv(tc.env), WithSvc(tc.svc))
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.wantSvc, gotDeployed.Svc)
				require.Equal(t, tc.wantEnv, gotDeployed.Env)
			}
		})
	}
}

type workspaceSelectMocks struct {
	workloadLister *mocks.MockWsWorkloadLister
	prompt         *mocks.MockPrompter
}

func TestWorkspaceSelect_Service(t *testing.T) {
	testCases := map[string]struct {
		setupMocks func(mocks workspaceSelectMocks)
		wantErr    error
		want       string
	}{
		"with no workspace services": {
			setupMocks: func(m workspaceSelectMocks) {

				m.workloadLister.
					EXPECT().
					ServiceNames().
					Return([]string{}, nil).
					Times(1)

				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("list services: no services found in workspace"),
		},
		"with only one workspace service (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {

				m.workloadLister.
					EXPECT().
					ServiceNames().
					Return([]string{
						"service1",
					}, nil).
					Times(1)

				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "service1",
		},
		"with multiple workspace services": {
			setupMocks: func(m workspaceSelectMocks) {

				m.workloadLister.
					EXPECT().
					ServiceNames().
					Return([]string{
						"service1",
						"service2",
					}, nil).
					Times(1)

				m.prompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select a local service"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"service1", "service2"}),
						gomock.Any()).
					Return("service2", nil).
					Times(1)
			},
			want: "service2",
		},
		"with error selecting services": {
			setupMocks: func(m workspaceSelectMocks) {

				m.workloadLister.
					EXPECT().
					ServiceNames().
					Return([]string{
						"service1",
						"service2",
					}, nil).
					Times(1)

				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"service1", "service2"}), gomock.Any()).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select local service: error selecting"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockwsWorkloadLister := mocks.NewMockWsWorkloadLister(ctrl)
			mockprompt := mocks.NewMockPrompter(ctrl)
			mocks := workspaceSelectMocks{
				workloadLister: mockwsWorkloadLister,
				prompt:         mockprompt,
			}
			tc.setupMocks(mocks)

			sel := WorkspaceSelect{
				Select: &Select{
					prompt: mockprompt,
				},
				wlLister: mockwsWorkloadLister,
			}
			got, err := sel.Service("Select a local service", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
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
		"with no workspace jobs": {
			setupMocks: func(m workspaceSelectMocks) {

				m.workloadLister.
					EXPECT().
					JobNames().
					Return([]string{}, nil).
					Times(1)

				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("list jobs: no jobs found in workspace"),
		},
		"with only one workspace job (skips prompting)": {
			setupMocks: func(m workspaceSelectMocks) {

				m.workloadLister.
					EXPECT().
					JobNames().
					Return([]string{
						"resizer",
					}, nil).
					Times(1)

				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "resizer",
		},
		"with multiple workspace services": {
			setupMocks: func(m workspaceSelectMocks) {

				m.workloadLister.
					EXPECT().
					JobNames().
					Return([]string{
						"resizer1",
						"resizer2",
					}, nil).
					Times(1)

				m.prompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select a local job"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"resizer1", "resizer2"}),
						gomock.Any()).
					Return("resizer2", nil).
					Times(1)
			},
			want: "resizer2",
		},
		"with error selecting services": {
			setupMocks: func(m workspaceSelectMocks) {

				m.workloadLister.
					EXPECT().
					JobNames().
					Return([]string{
						"resizer1",
						"resizer2",
					}, nil).
					Times(1)

				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"resizer1", "resizer2"}), gomock.Any()).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select local job: error selecting"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockwsWorkloadLister := mocks.NewMockWsWorkloadLister(ctrl)
			mockprompt := mocks.NewMockPrompter(ctrl)
			mocks := workspaceSelectMocks{
				workloadLister: mockwsWorkloadLister,
				prompt:         mockprompt,
			}
			tc.setupMocks(mocks)

			sel := WorkspaceSelect{
				Select: &Select{
					prompt: mockprompt,
				},
				wlLister: mockwsWorkloadLister,
			}
			got, err := sel.Job("Select a local job", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

type configSelectMocks struct {
	serviceLister *mocks.MockConfigLister
	prompt        *mocks.MockPrompter
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
				m.serviceLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Service{}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("no services found in app myapp"),
		},
		"with only one service (skips prompting)": {
			setupMocks: func(m configSelectMocks) {
				m.serviceLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Service{
						{
							App:  appName,
							Name: "service1",
							Type: "load balanced web service",
						},
					}, nil).
					Times(1)
				m.prompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "service1",
		},
		"with multiple services": {
			setupMocks: func(m configSelectMocks) {
				m.serviceLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Service{
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
						gomock.Eq([]string{"service1", "service2"})).
					Return("service2", nil).
					Times(1)
			},
			want: "service2",
		},
		"with error selecting services": {
			setupMocks: func(m configSelectMocks) {
				m.serviceLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Service{
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
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"service1", "service2"})).
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

			mockconfigLister := mocks.NewMockConfigLister(ctrl)
			mockprompt := mocks.NewMockPrompter(ctrl)
			mocks := configSelectMocks{
				serviceLister: mockconfigLister,
				prompt:        mockprompt,
			}
			tc.setupMocks(mocks)

			sel := ConfigSelect{
				Select: &Select{
					prompt: mockprompt,
				},
				svcLister: mockconfigLister,
			}

			got, err := sel.Service("Select a service", "Help text", appName)
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

type environmentMocks struct {
	envLister *mocks.MockAppEnvLister
	prompt    *mocks.MockPrompter
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
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
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
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
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
						gomock.Eq([]string{"env1", "env2"})).
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
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"env1", "env2"})).
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
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
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
					SelectOne(gomock.Any(), gomock.Any(), []string{additionalOpt1, additionalOpt2}).
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

			mockenvLister := mocks.NewMockAppEnvLister(ctrl)
			mockprompt := mocks.NewMockPrompter(ctrl)
			mocks := environmentMocks{
				envLister: mockenvLister,
				prompt:    mockprompt,
			}
			tc.setupMocks(mocks)

			sel := Select{
				prompt: mockprompt,
				lister: mockenvLister,
			}

			got, err := sel.Environment("Select an environment", "Help text", appName, tc.inAdditionalOpts...)
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

type applicationMocks struct {
	appLister *mocks.MockAppEnvLister
	prompt    *mocks.MockPrompter
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
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
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
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
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
						gomock.Eq([]string{"app1", "app2"})).
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
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"app1", "app2"})).
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

			mockappLister := mocks.NewMockAppEnvLister(ctrl)
			mockprompt := mocks.NewMockPrompter(ctrl)
			mocks := applicationMocks{
				appLister: mockappLister,
				prompt:    mockprompt,
			}
			tc.setupMocks(mocks)

			sel := Select{
				prompt: mockprompt,
				lister: mockappLister,
			}

			got, err := sel.Application("Select an app", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}
