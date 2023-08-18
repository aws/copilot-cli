// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
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
	mockManifest := workspace.WorkloadManifest(`
name: fe
type: Load Balanced Web Service`)
	testCases := map[string]struct {
		inAppName       string
		inName          string
		inShouldInit    *bool
		inShouldNotInit bool

		wantedErr         string
		mockSel           func(m *mocks.MockwsSelector)
		mockPrompt        func(m *mocks.Mockprompter)
		mockActionCommand func(m *mocks.MockactionCommand)
		mockStore         func(m *mocks.Mockstore)
		mockWs            func(m *mocks.MockwsWlDirReader)
		mockInit          func(m *mocks.MockwkldInitializerWithoutManifest)
	}{
		"prompts for workload selection": {
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
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit:   func(m *mocks.MockwkldInitializerWithoutManifest) {},
		},
		"prompts for initializing workload": {
			inAppName:    "app",
			inName:       "fe",
			inShouldInit: nil,
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp("app", "fe", manifestinfo.LoadBalancedWebServiceType).Return(nil)
			},
		},
		"initializes workload with flag specified": {
			inAppName:    "app",
			inName:       "fe",
			inShouldInit: aws.Bool(true),
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp("app", "fe", manifestinfo.LoadBalancedWebServiceType).Return(nil)
			},
		},
		"errors if noInit specified": {
			inAppName:       "app",
			inName:          "fe",
			inShouldInit:    aws.Bool(false),
			inShouldNotInit: true,
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Times(0)
				m.EXPECT().Validate().Times(0)
				m.EXPECT().Execute().Times(0)
				m.EXPECT().RecommendActions().Times(0)
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "workload fe is uninitialized but --no-init-wkld was specified",
		},
		"errors reading manifest": {
			inAppName:       "app",
			inName:          "fe",
			inShouldNotInit: true,
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(nil, errors.New("some error"))
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Times(0)
				m.EXPECT().Validate().Times(0)
				m.EXPECT().Execute().Times(0)
				m.EXPECT().RecommendActions().Times(0)
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "read manifest for workload fe: some error",
		},
		"error getting workload type": {
			inAppName:       "app",
			inName:          "fe",
			inShouldNotInit: false,
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(workspace.WorkloadManifest(`type: nothing here`), nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Times(0)
				m.EXPECT().Validate().Times(0)
				m.EXPECT().Execute().Times(0)
				m.EXPECT().RecommendActions().Times(0)
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "unrecognized workload type \"nothing here\" in manifest for workload fe",
		},
		"error listing workloads": {
			inAppName: "app",
			inName:    "fe",
			mockSel:   func(m *mocks.MockwsSelector) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListWorkloads("app").Return(nil, errors.New("some error"))
				m.EXPECT().GetWorkload("app", "fe").Times(0)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {},
			mockPrompt:        func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
			},
			wantedErr: "retrieve workloads: some error",
		},
		"initializes and deploys local manifest with prompts": {
			inAppName: "app",
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload(gomock.Any(), gomock.Any()).Return("fe", nil)
			},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp("app", "fe", manifestinfo.LoadBalancedWebServiceType).Return(nil)
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
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockJob}, nil)
				m.EXPECT().GetWorkload("app", "mailer").Return(&mockJob, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
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
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
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
			mockPrompt:        func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
			},
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
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)

			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
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
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)

			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
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
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)

			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
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
			mockWs := mocks.NewMockwsWlDirReader(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockInit := mocks.NewMockwkldInitializerWithoutManifest(ctrl)

			tc.mockStore(mockStore)
			tc.mockSel(mockSel)
			tc.mockActionCommand(mockCmd)
			tc.mockWs(mockWs)
			tc.mockPrompt(mockPrompt)
			tc.mockInit(mockInit)

			opts := &deployOpts{
				deployVars: deployVars{
					deployWkldVars: deployWkldVars{
						appName: tc.inAppName,
						name:    tc.inName,
						envName: "test",
					},
					yesInitWkld: tc.inShouldInit,
				},
				deployWkld: mockCmd,
				sel:        mockSel,
				prompt:     mockPrompt,
				store:      mockStore,
				ws:         mockWs,

				newWorkloadAdder: func() wkldInitializerWithoutManifest { return mockInit },

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
