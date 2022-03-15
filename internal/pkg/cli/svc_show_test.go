// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
)

type showSvcMocks struct {
	storeSvc  *mocks.Mockstore
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
	// NOTE: no optional flag needs to be validated for this command.
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
		"validate instead of prompting application name and svc name": {
			inputApp: "my-app",
			inputSvc: "my-svc",
			setupMocks: func(m showSvcMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{}, nil)
				m.storeSvc.EXPECT().GetService("my-app", "my-svc").Return(&config.Workload{}, nil)
			},
			wantedApp: "my-app",
			wantedSvc: "my-svc",
		},
		"prompt for app name": {
			inputSvc: "my-svc",
			setupMocks: func(m showSvcMocks) {
				m.sel.EXPECT().Application(gomock.Any(), gomock.Any(), gomock.Any()).Return("my-app", nil)
				m.storeSvc.EXPECT().GetApplication("my-app").Times(0)
				m.storeSvc.EXPECT().GetService("my-app", "my-svc").AnyTimes()
			},
			wantedApp: "my-app",
			wantedSvc: "my-svc",
		},
		"error when fail to select apps": {
			inputSvc: "my-svc",
			setupMocks: func(m showSvcMocks) {
				m.sel.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return("", errors.New("some error"))
			},
			wantedError: fmt.Errorf("select application name: some error"),
		},
		"prompt for service name": {
			inputApp: "my-app",
			setupMocks: func(m showSvcMocks) {
				m.sel.EXPECT().Service(fmt.Sprintf(svcShowSvcNamePrompt, "my-app"), svcShowSvcNameHelpPrompt, "my-app").Return("my-svc", nil)
				m.storeSvc.EXPECT().GetService("my-app", "my-svc").Times(0)
				m.storeSvc.EXPECT().GetApplication("my-app").AnyTimes()
				m.sel.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).AnyTimes()
			},
			wantedApp: "my-app",
			wantedSvc: "my-svc",
		},
		"error when fail to select services": {
			setupMocks: func(m showSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return("my-app", nil),
					m.sel.EXPECT().Service(fmt.Sprintf(svcShowSvcNamePrompt, "my-app"), svcShowSvcNameHelpPrompt, "my-app").Return("", errors.New("some error")),
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
			mockWorkspace := mocks.NewMockwsSvcReader(ctrl)
			mockSelector := mocks.NewMockconfigSelector(ctrl)

			mocks := showSvcMocks{
				storeSvc: mockStoreReader,
				ws:       mockWorkspace,
				sel:      mockSelector,
			}

			tc.setupMocks(mocks)

			showSvcs := &showSvcOpts{
				showSvcVars: showSvcVars{
					svcName: tc.inputSvc,
					appName: tc.inputApp,
				},
				store: mockStoreReader,
				sel:   mockSelector,
			}

			// WHEN
			err := showSvcs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, showSvcs.appName, "expected app name to match")
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
					appName:          appName,
				},
				describer:     mockSvcDescriber,
				initDescriber: func() error { return nil },
				w:             b,
			}

			// WHEN
			err := showSvcs.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}
