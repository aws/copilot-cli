// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSelector_SelectWorkspaceService(t *testing.T) {
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

	slct := selector{prompt: mockPrompt}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := slct.SelectWorkspaceService(&selectWorkspaceServiceRequest{
				Prompt: "Select a local service",
				Help:   "Help text",
				Lister: mockServiceLister,
			})
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSelector_Service(t *testing.T) {
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

	slct := selector{prompt: mockPrompt}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := slct.SelectService(&selectServiceRequest{
				Prompt: "Select a service",
				Help:   "Help text",
				Lister: mockServiceLister,
				App:    appName,
			})
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSelector_SelectEnvironment(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockEnvLister := mocks.NewMockenvironmentLister(ctrl)
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

	slct := selector{prompt: mockPrompt}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := slct.SelectEnvironment(&selectEnvRequest{
				Prompt: "Select an environment",
				Help:   "Help text",
				Lister: mockEnvLister,
				App:    appName,
			})
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSelector_SelectApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockAppLister := mocks.NewMockapplicationLister(ctrl)
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
			wantErr: fmt.Errorf("select app: error selecting"),
		},
	}

	slct := selector{prompt: mockPrompt}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got, err := slct.SelectApplication(&selectAppRequest{
				Prompt: "Select an app",
				Help:   "Help text",
				Lister: mockAppLister,
			})
			if tc.wantErr != nil {
				require.EqualError(t, tc.wantErr, err.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}
