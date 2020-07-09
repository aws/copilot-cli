// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showSvcMocks struct {
	storeSvc  *mocks.Mockstore
	prompt    *mocks.Mockprompter
	describer *mocks.Mockdescriber
	ws        *mocks.MockwsSvcReader
	sel       *mocks.MockconfigSelector
}

type mockDescribeData struct {
	data string
	err  error
}

func (m *mockDescribeData) HumanString() string {
	return m.data
}

func (m *mockDescribeData) JSONString() (string, error) {
	return m.data, m.err
}

func TestSvcShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputApp   string
		inputSvc   string
		setupMocks func(mocks showSvcMocks)

		wantedError error
	}{
		"valid app name and service name": {
			inputApp: "my-app",
			inputSvc: "my-svc",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
						Name: "my-app",
					}, nil),
					m.storeSvc.EXPECT().GetService("my-app", "my-svc").Return(&config.Service{
						Name: "my-svc",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"fail to get app": {
			inputApp: "my-app",
			inputSvc: "my-svc",

			setupMocks: func(m showSvcMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"fail to get service": {
			inputApp: "my-app",
			inputSvc: "my-svc",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
						Name: "my-app",
					}, nil),
					m.storeSvc.EXPECT().GetService("my-app", "my-svc").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)

			mocks := showSvcMocks{
				storeSvc: mockStoreReader,
			}

			tc.setupMocks(mocks)

			showSvcs := &showSvcOpts{
				showSvcVars: showSvcVars{
					svcName: tc.inputSvc,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				store: mockStoreReader,
			}

			// WHEN
			err := showSvcs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestSvcShow_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputApp string
		inputSvc string

		setupMocks func(mocks showSvcMocks)

		wantedApp   string
		wantedSvc   string
		wantedError error
	}{
		"with all flags": {
			inputApp:   "my-app",
			inputSvc:   "my-svc",
			setupMocks: func(mocks showSvcMocks) {},

			wantedApp:   "my-app",
			wantedSvc:   "my-svc",
			wantedError: nil,
		},
		"retrieve all service names if fail to retrieve service name from local": {
			inputApp: "",
			inputSvc: "",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcShowAppNamePrompt, svcShowAppNameHelpPrompt).Return("my-app", nil),

					m.ws.EXPECT().ServiceNames().Return(nil, errors.New("some error")),
					m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Service{
						{Name: "my-svc"},
						{Name: "archer-svc"},
					}, nil),
					m.prompt.EXPECT().SelectOne(fmt.Sprintf(svcShowSvcNamePrompt, "my-app"), svcShowSvcNameHelpPrompt, []string{"my-svc", "archer-svc"}).Return("my-svc", nil).Times(1),
				)
			},

			wantedApp:   "my-app",
			wantedSvc:   "my-svc",
			wantedError: nil,
		},
		"retrieve all service names if no service found in local dir": {
			inputApp: "",
			inputSvc: "",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcShowAppNamePrompt, svcShowAppNameHelpPrompt).Return("my-app", nil),

					m.ws.EXPECT().ServiceNames().Return([]string{}, nil),
					m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Service{
						{Name: "my-svc"},
						{Name: "archer-svc"},
					}, nil),

					m.prompt.EXPECT().SelectOne(fmt.Sprintf(svcShowSvcNamePrompt, "my-app"), svcShowSvcNameHelpPrompt, []string{"my-svc", "archer-svc"}).Return("my-svc", nil).Times(1),
				)
			},

			wantedApp:   "my-app",
			wantedSvc:   "my-svc",
			wantedError: nil,
		},
		"retrieve local service names": {
			inputApp: "",
			inputSvc: "",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcShowAppNamePrompt, svcShowAppNameHelpPrompt).Return("my-app", nil),

					m.ws.EXPECT().ServiceNames().Return([]string{"my-svc", "archer-svc"}, nil),
					m.prompt.EXPECT().SelectOne(fmt.Sprintf(svcShowSvcNamePrompt, "my-app"), svcShowSvcNameHelpPrompt, []string{"my-svc", "archer-svc"}).Return("my-svc", nil).Times(1),
				)
			},

			wantedApp:   "my-app",
			wantedSvc:   "my-svc",
			wantedError: nil,
		},
		"skip selecting if only one service found": {
			inputApp: "my-app",
			inputSvc: "",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ServiceNames().Return(nil, errors.New("some error")),
					m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Service{
						{
							Name: "my-svc",
						},
					}, nil),
				)
			},

			wantedApp:   "my-app",
			wantedSvc:   "my-svc",
			wantedError: nil,
		},
		"returns error when fail to select apps": {
			inputApp: "",
			inputSvc: "",

			setupMocks: func(m showSvcMocks) {
				m.sel.EXPECT().Application(svcShowAppNamePrompt, svcShowAppNameHelpPrompt).Return("", errors.New("some error"))
			},

			wantedError: fmt.Errorf("select application name: some error"),
		},
		"returns error when fail to list services": {
			inputApp: "",
			inputSvc: "",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcShowAppNamePrompt, svcShowAppNameHelpPrompt).Return("my-app", nil),

					m.ws.EXPECT().ServiceNames().Return(nil, errors.New("some error")),
					m.storeSvc.EXPECT().ListServices("my-app").Return(nil, fmt.Errorf("some error")),
				)
			},

			wantedError: fmt.Errorf("list services for application my-app: some error"),
		},
		"returns error when fail to select services": {
			inputApp: "",
			inputSvc: "",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcShowAppNamePrompt, svcShowAppNameHelpPrompt).Return("my-app", nil),

					m.ws.EXPECT().ServiceNames().Return(nil, errors.New("some error")),
					m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Service{
						{Name: "my-svc"},
						{Name: "archer-svc"},
					}, nil),

					m.prompt.EXPECT().SelectOne(fmt.Sprintf(svcShowSvcNamePrompt, "my-app"), svcShowSvcNameHelpPrompt, []string{"my-svc", "archer-svc"}).Return("", fmt.Errorf("some error")).Times(1),
				)
			},

			wantedError: fmt.Errorf("select service for application my-app: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)
			mockWorkspace := mocks.NewMockwsSvcReader(ctrl)
			mockSelector := mocks.NewMockconfigSelector(ctrl)

			mocks := showSvcMocks{
				storeSvc: mockStoreReader,
				prompt:   mockPrompter,
				ws:       mockWorkspace,
				sel:      mockSelector,
			}

			tc.setupMocks(mocks)

			showSvcs := &showSvcOpts{
				showSvcVars: showSvcVars{
					svcName: tc.inputSvc,
					GlobalOpts: &GlobalOpts{
						prompt:  mockPrompter,
						appName: tc.inputApp,
					},
				},
				store: mockStoreReader,
				ws:    mockWorkspace,
				sel:   mockSelector,
			}

			// WHEN
			err := showSvcs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedApp, showSvcs.AppName(), "expected app name to match")
				require.Equal(t, tc.wantedSvc, showSvcs.svcName, "expected service name to match")
			}
		})
	}
}

func TestSvcShow_Execute(t *testing.T) {
	appName := "my-app"
	webSvc := mockDescribeData{
		data: "mockData",
		err:  errors.New("some error"),
	}
	testCases := map[string]struct {
		inputSvc         string
		shouldOutputJSON bool

		setupMocks func(mocks showSvcMocks)

		wantedContent string
		wantedError   error
	}{
		"noop if service name is empty": {
			setupMocks: func(m showSvcMocks) {
				m.describer.EXPECT().Describe().Times(0)
			},
		},
		"success": {
			inputSvc: "my-svc",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(&webSvc, nil),
				)
			},

			wantedContent: "mockData",
		},
		"return error if fail to generate JSON output": {
			inputSvc:         "my-svc",
			shouldOutputJSON: true,

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(&webSvc, nil),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"return error if fail to describe service": {
			inputSvc: "my-svc",

			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("describe service my-svc: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockSvcDescriber := mocks.NewMockdescriber(ctrl)

			mocks := showSvcMocks{
				describer: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			showSvcs := &showSvcOpts{
				showSvcVars: showSvcVars{
					svcName:          tc.inputSvc,
					shouldOutputJSON: tc.shouldOutputJSON,
					GlobalOpts: &GlobalOpts{
						appName: appName,
					},
				},
				describer:     mockSvcDescriber,
				initDescriber: func(bool) error { return nil },
				w:             b,
			}

			// WHEN
			err := showSvcs.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}
