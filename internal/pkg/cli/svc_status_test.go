// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSvcStatus_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputApp         string
		inputSvc         string
		inputEnvironment string
		mockStoreReader  func(m *mocks.Mockstore)

		wantedError error
	}{
		"invalid app name": {
			inputApp: "my-app",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid service name": {
			inputApp: "my-app",
			inputSvc: "my-svc",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().GetService("my-app", "my-svc").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid environment name": {
			inputApp:         "my-app",
			inputEnvironment: "test",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"success": {
			inputApp:         "my-app",
			inputSvc:         "my-svc",
			inputEnvironment: "test",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name: "test",
				}, nil)
				m.EXPECT().GetService("my-app", "my-svc").Return(&config.Service{
					Name: "my-svc",
				}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			tc.mockStoreReader(mockStoreReader)

			svcStatus := &svcStatusOpts{
				svcStatusVars: svcStatusVars{
					svcName: tc.inputSvc,
					envName: tc.inputEnvironment,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				store: mockStoreReader,
			}

			// WHEN
			err := svcStatus.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestSvcStatus_Ask(t *testing.T) {
	mockServiceArn := ecs.ServiceArn("mockArn")
	mockError := errors.New("some error")
	mockStackNotFoundErr := fmt.Errorf("describe stack my-app-test-my-svc: %w",
		awserr.New("ValidationError", "Stack with id my-app-test-my-svc does not exist", nil))
	testCases := map[string]struct {
		inputApp             string
		inputSvc             string
		inputEnvironment     string
		mockSelector         func(m *mocks.MockconfigSelector)
		mockStoreReader      func(m *mocks.Mockstore)
		mockServiceDescriber func(m *mocks.MockserviceArnGetter)
		mockPrompt           func(m *mocks.Mockprompter)

		wantedError error
	}{
		"skip asking": {
			inputApp:         "mockApp",
			inputSvc:         "mockSvc",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *mocks.Mockstore) {},
			mockSelector:    func(m *mocks.MockconfigSelector) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
		},
		"errors if failed to select application": {
			mockStoreReader: func(m *mocks.Mockstore) {},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcStatusAppNamePrompt, svcStatusAppNameHelpPrompt).Return("", mockError)
			},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {},
			mockPrompt:           func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("select application: some error"),
		},
		"errors if failed to list service": {
			inputApp: "mockApp",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{}, mockError)
			},
			mockSelector:         func(m *mocks.MockconfigSelector) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {},
			mockPrompt:           func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("list services for application mockApp: some error"),
		},
		"errors if no available service found": {
			inputApp: "mockApp",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{}, nil)
			},
			mockSelector:         func(m *mocks.MockconfigSelector) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {},
			mockPrompt:           func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("no services found in application mockApp"),
		},
		"errors if failed to check if service deployed in env": {
			inputApp:         "mockApp",
			inputSvc:         "mockSvc",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *mocks.Mockstore) {},
			mockSelector:    func(m *mocks.MockconfigSelector) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(nil, mockError)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("check if service mockSvc is deployed in env mockEnv: some error"),
		},
		"errors if no deployed service found": {
			inputApp:         "mockApp",
			inputSvc:         "mockSvc",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *mocks.Mockstore) {},
			mockSelector:    func(m *mocks.MockconfigSelector) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(nil, mockStackNotFoundErr)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("no deployed services found in application mockApp"),
		},
		"errors if failed to select deployed service": {
			inputApp: "mockApp",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockEnv1",
					},
					{
						Name: "mockEnv2",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(svcLogNamePrompt, svcLogNameHelpPrompt,
					[]string{"mockSvc (mockEnv1)", "mockSvc (mockEnv2)"}).Return("", mockError)
			},

			wantedError: fmt.Errorf("select deployed services for application mockApp: some error"),
		},
		"success": {
			inputApp: "mockApp",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockEnv1",
					},
					{
						Name: "mockEnv2",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(svcLogNamePrompt, svcLogNameHelpPrompt,
					[]string{"mockSvc (mockEnv1)", "mockSvc (mockEnv2)"}).Return("mockSvc (mockEnv1)", nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockServiceDescriber := mocks.NewMockserviceArnGetter(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockSelector := mocks.NewMockconfigSelector(ctrl)
			tc.mockStoreReader(mockStoreReader)
			tc.mockServiceDescriber(mockServiceDescriber)
			tc.mockPrompt(mockPrompt)
			tc.mockSelector(mockSelector)

			svcStatus := &svcStatusOpts{
				svcStatusVars: svcStatusVars{
					svcName: tc.inputSvc,
					envName: tc.inputEnvironment,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
						prompt:  mockPrompt,
					},
				},
				svcDescriber:     mockServiceDescriber,
				sel:              mockSelector,
				initSvcDescriber: func(*svcStatusOpts, string, string) error { return nil },
				store:            mockStoreReader,
			}

			// WHEN
			err := svcStatus.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestSvcStatus_Execute(t *testing.T) {
	mockError := errors.New("some error")
	mockServiceStatus := &describe.ServiceStatusDesc{}
	testCases := map[string]struct {
		shouldOutputJSON    bool
		mockStatusDescriber func(m *mocks.MockstatusDescriber)
		wantedError         error
	}{
		"errors if failed to describe the status of the service": {
			mockStatusDescriber: func(m *mocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe status of service mockSvc: some error"),
		},
		"success with JSON output": {
			shouldOutputJSON: true,

			mockStatusDescriber: func(m *mocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(mockServiceStatus, nil)
			},
		},
		"success with HumanString": {
			mockStatusDescriber: func(m *mocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(mockServiceStatus, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStatusDescriber := mocks.NewMockstatusDescriber(ctrl)
			tc.mockStatusDescriber(mockStatusDescriber)

			svcStatus := &svcStatusOpts{
				svcStatusVars: svcStatusVars{
					svcName:          "mockSvc",
					envName:          "mockEnv",
					shouldOutputJSON: tc.shouldOutputJSON,
					GlobalOpts: &GlobalOpts{
						appName: "mockApp",
					},
				},
				statusDescriber:     mockStatusDescriber,
				initStatusDescriber: func(*svcStatusOpts) error { return nil },
				w:                   b,
			}

			// WHEN
			err := svcStatus.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.NotEmpty(t, b.String(), "expected output content to not be empty")
			}
		})
	}
}
