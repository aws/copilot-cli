// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceSelect_Service(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockServiceLister := mocks.NewMockwsServiceLister(ctrl)
	mockPrompt := mocks.NewMockprompter(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		mocking func()
		wantErr error
		want    string
	}{
		"with no workspace services": {
			mocking: func() {

				mockServiceLister.
					EXPECT().
					ServiceNames().
					Return([]string{}, nil).
					Times(1)

				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("list services: no services found in workspace"),
		},
		"with only one workspace service (skips prompting)": {
			mocking: func() {

				mockServiceLister.
					EXPECT().
					ServiceNames().
					Return([]string{
						"service1",
					}, nil).
					Times(1)

				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "service1",
		},
		"with multiple workspace services": {
			mocking: func() {

				mockServiceLister.
					EXPECT().
					ServiceNames().
					Return([]string{
						"service1",
						"service2",
					}, nil).
					Times(1)

				mockPrompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select a local service"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"service1", "service2"})).
					Return("service2", nil).
					Times(1)
			},
			want: "service2",
		},
		"with error selecting services": {
			mocking: func() {

				mockServiceLister.
					EXPECT().
					ServiceNames().
					Return([]string{
						"service1",
						"service2",
					}, nil).
					Times(1)

				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"service1", "service2"})).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select local service: error selecting"),
		},
	}

	sel := WorkspaceSelect{
		Select: &Select{
			prompt: mockPrompt,
		},
		svcLister: mockServiceLister,
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := sel.Service("Select a local service", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestConfigSelect_Service(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockServiceLister := mocks.NewMockserviceLister(ctrl)
	mockPrompt := mocks.NewMockprompter(ctrl)
	defer ctrl.Finish()

	appName := "myapp"

	testCases := map[string]struct {
		mocking func()
		wantErr error
		want    string
	}{
		"with no services": {
			mocking: func() {
				mockServiceLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Service{}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("no services found in app myapp"),
		},
		"with only one service (skips prompting)": {
			mocking: func() {
				mockServiceLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Service{
						&config.Service{
							App:  appName,
							Name: "service1",
							Type: "load balanced web service",
						},
					}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "service1",
		},
		"with multiple services": {
			mocking: func() {
				mockServiceLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Service{
						&config.Service{
							App:  appName,
							Name: "service1",
							Type: "load balanced web service",
						},
						&config.Service{
							App:  appName,
							Name: "service2",
							Type: "backend service",
						},
					}, nil).
					Times(1)
				mockPrompt.
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
			mocking: func() {
				mockServiceLister.
					EXPECT().
					ListServices(gomock.Eq(appName)).
					Return([]*config.Service{
						&config.Service{
							App:  appName,
							Name: "service1",
							Type: "load balanced web service",
						},
						&config.Service{
							App:  appName,
							Name: "service2",
							Type: "backend service",
						},
					}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"service1", "service2"})).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select service: error selecting"),
		},
	}

	sel := ConfigSelect{
		Select: &Select{
			prompt: mockPrompt,
		},
		svcLister: mockServiceLister,
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := sel.Service("Select a service", "Help text", appName)
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSelect_Environment(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockEnvLister := mocks.NewMockstore(ctrl)
	mockPrompt := mocks.NewMockprompter(ctrl)
	defer ctrl.Finish()

	appName := "myapp"

	testCases := map[string]struct {
		mocking func()
		wantErr error
		want    string
	}{
		"with no environments": {
			mocking: func() {
				mockEnvLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("no environments found in app myapp"),
		},
		"with only one environment (skips prompting)": {
			mocking: func() {
				mockEnvLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{
						&config.Environment{
							App:  appName,
							Name: "env1",
						},
					}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "env1",
		},
		"with multiple environments": {
			mocking: func() {
				mockEnvLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{
						&config.Environment{
							App:  appName,
							Name: "env1",
						},
						&config.Environment{
							App:  appName,
							Name: "env2",
						},
					}, nil).
					Times(1)
				mockPrompt.
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
			mocking: func() {
				mockEnvLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{
						&config.Environment{
							App:  appName,
							Name: "env1",
						},
						&config.Environment{
							App:  appName,
							Name: "env2",
						},
					}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"env1", "env2"})).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select environment: error selecting"),
		},
	}

	sel := Select{
		prompt: mockPrompt,
		lister: mockEnvLister,
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := sel.Environment("Select an environment", "Help text", appName)
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSelect_Application(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockAppLister := mocks.NewMockstore(ctrl)
	mockPrompt := mocks.NewMockprompter(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		mocking func()
		wantErr error
		want    string
	}{
		"with no apps": {
			mocking: func() {
				mockAppLister.
					EXPECT().
					ListApplications().
					Return([]*config.Application{}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			wantErr: fmt.Errorf("no apps found"),
		},
		"with only one app (skips prompting)": {
			mocking: func() {
				mockAppLister.
					EXPECT().
					ListApplications().
					Return([]*config.Application{
						&config.Application{
							Name: "app1",
						},
					}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: "app1",
		},
		"with multiple apps": {
			mocking: func() {
				mockAppLister.
					EXPECT().
					ListApplications().
					Return([]*config.Application{
						&config.Application{
							Name: "app1",
						},
						&config.Application{
							Name: "app2",
						},
					}, nil).
					Times(1)
				mockPrompt.
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
			mocking: func() {
				mockAppLister.
					EXPECT().
					ListApplications().
					Return([]*config.Application{
						&config.Application{
							Name: "app1",
						},
						&config.Application{
							Name: "app2",
						},
					}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"app1", "app2"})).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select application: error selecting"),
		},
	}

	sel := Select{
		prompt: mockPrompt,
		lister: mockAppLister,
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := sel.Application("Select an app", "Help text")
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSelect_EnvironmentWithNone(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockEnvLister := mocks.NewMockstore(ctrl)
	mockPrompt := mocks.NewMockprompter(ctrl)
	defer ctrl.Finish()

	appName := "myapp"

	testCases := map[string]struct {
		mocking func()
		wantErr error
		want    string
	}{
		"with no environments": {
			mocking: func() {
				mockEnvLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)

			},
			want: config.EnvNameNone,
		},
		"with multiple environments": {
			mocking: func() {
				mockEnvLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{
						&config.Environment{
							App:  appName,
							Name: "env1",
						},
						&config.Environment{
							App:  appName,
							Name: "env2",
						},
					}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(
						gomock.Eq("Select an environment"),
						gomock.Eq("Help text"),
						gomock.Eq([]string{"env1", "env2", config.EnvNameNone})).
					Return("env1", nil).
					Times(1)
			},
			want: "env1",
		},
		"with error selecting environments": {
			mocking: func() {
				mockEnvLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return([]*config.Environment{
						&config.Environment{
							App:  appName,
							Name: "env1",
						},
						&config.Environment{
							App:  appName,
							Name: "env2",
						},
					}, nil).
					Times(1)
				mockPrompt.
					EXPECT().
					SelectOne(gomock.Any(), gomock.Any(), gomock.Eq([]string{"env1", "env2", config.EnvNameNone})).
					Return("", fmt.Errorf("error selecting")).
					Times(1)
			},
			wantErr: fmt.Errorf("select environment: error selecting"),
		},
		"with error listing environments": {
			mocking: func() {
				mockEnvLister.
					EXPECT().
					ListEnvironments(gomock.Eq(appName)).
					Return(nil, fmt.Errorf("error listing environments")).
					Times(1)
			},
			wantErr: fmt.Errorf("list environments: error listing environments"),
		},
	}

	sel := Select{
		prompt: mockPrompt,
		lister: mockEnvLister,
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := sel.EnvironmentWithNone("Select an environment", "Help text", appName)
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}
