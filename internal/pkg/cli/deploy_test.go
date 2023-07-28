// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeployOpts_Run(t *testing.T) {
	mockWl := config.Workload{
		App:  "app",
		Name: "fe",
		Type: "Load Balanced Web Service",
	}
	mockJob := config.Workload{
		App:  "app",
		Name: "mailer",
		Type: "Scheduled Job",
	}
	testCases := map[string]struct {
		inAppName string
		inName    string

		wantedErr string

		mockSel           func(m *mocks.MockwsSelector)
		mockActionCommand func(m *mocks.MockactionCommand)
		mockStore         func(m *mocks.Mockstore)
	}{
		"prompts for workload": {
			inAppName: "app",
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload("Select a service or job in your workspace", "").Return("fe", nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
		},
		"errors correctly if job returned": {
			inAppName: "app",
			wantedErr: "ask job deploy: some error",
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload("Select a service or job in your workspace", "").Return("mailer", nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Return(errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetWorkload("app", "mailer").Return(&mockJob, nil)
			},
		},
		"doesn't prompt if name is specified": {
			inAppName: "app",
			inName:    "fe",

			mockSel: func(m *mocks.MockwsSelector) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
		},
		"get name error": {
			inAppName: "app",
			wantedErr: "select service or job: some error",
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {},
			mockStore:         func(m *mocks.Mockstore) {},
		},
		"ask error": {
			inAppName: "app",
			inName:    "fe",
			wantedErr: "ask svc deploy: some error",

			mockSel: func(m *mocks.MockwsSelector) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Return(errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
		},
		"validate error": {
			inAppName: "app",
			inName:    "fe",
			wantedErr: "validate svc deploy: some error",

			mockSel: func(m *mocks.MockwsSelector) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate().Return(errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
		},
		"execute error": {
			inAppName: "app",
			inName:    "fe",
			wantedErr: "execute svc deploy: some error",

			mockSel: func(m *mocks.MockwsSelector) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute().Return(errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSel := mocks.NewMockwsSelector(ctrl)
			mockCmd := mocks.NewMockactionCommand(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockStore(mockStore)
			tc.mockSel(mockSel)
			tc.mockActionCommand(mockCmd)
			opts := &deployOpts{
				deployVars: deployVars{
					deployWkldVars: deployWkldVars{
						appName: tc.inAppName,
						name:    tc.inName,
						envName: "test",
					},
					yesInitWkld: false,
				},
				deployWkld: mockCmd,
				sel:        mockSel,
				store:      mockStore,

				setupDeployCmd: func(o *deployOpts, wlType string) {},
			}

			// WHEN
			err := opts.Run()

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			}
		})
	}
}
