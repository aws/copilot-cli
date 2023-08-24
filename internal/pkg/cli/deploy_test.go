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
	mockEnv := config.Environment{
		App:  "app",
		Name: "test",
	}
	mockManifest := workspace.WorkloadManifest(`
name: fe
type: Load Balanced Web Service`)
	testCases := map[string]struct {
		inAppName    string
		inName       string
		inEnvName    string
		inShouldInit *bool
		inDeployEnv  *bool
		inInitEnv    *bool

		wantedErr         string
		mockSel           func(m *mocks.MockwsSelector)
		mockPrompt        func(m *mocks.Mockprompter)
		mockActionCommand func(m *mocks.MockactionCommand)
		mockCmd           func(m *mocks.Mockcmd)
		mockStore         func(m *mocks.Mockstore)
		mockWs            func(m *mocks.MockwsWlDirReader)
		mockInit          func(m *mocks.MockwkldInitializerWithoutManifest)
	}{
		"prompts for initialization and deployment of environment when workload initialized": {
			inAppName: "app",
			inName:    "fe",
			inEnvName: "test",
			wantedErr: "",
			mockSel:   func(m *mocks.MockwsSelector) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Environment \"test\" does not exist in app \"app\". Initialize it?", "").Return(true, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				// Deploy svc
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockCmd: func(m *mocks.Mockcmd) {
				// Init env
				m.EXPECT().Validate()
				m.EXPECT().Ask()
				m.EXPECT().Execute()
				// Deploy env
				m.EXPECT().Validate()
				m.EXPECT().Ask()
				m.EXPECT().Execute()
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(nil, &config.ErrNoSuchEnvironment{})
				// After env init/deploy
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{{Name: "fe", Type: "Load Balanced Web Service"}}, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&config.Workload{Name: "fe", Type: "Load Balanced Web Service"}, nil)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
			},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {

			},
		},
		"prompts for workload selection": {
			inAppName:   "app",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload("Select a service or job in your workspace", "").Return("fe", nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockCmd: func(m *mocks.Mockcmd) {

			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit:   func(m *mocks.MockwkldInitializerWithoutManifest) {},
		},
		"prompts for initializing workload": {
			inAppName:    "app",
			inName:       "fe",
			inEnvName:    "test",
			inInitEnv:    aws.Bool(false),
			inDeployEnv:  aws.Bool(false),
			inShouldInit: nil,
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockCmd: func(m *mocks.Mockcmd) {

			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp("app", "fe", manifestinfo.LoadBalancedWebServiceType).Return(nil)
			},
		},
		"initializes workload with flag specified": {
			inAppName:    "app",
			inName:       "fe",
			inEnvName:    "test",
			inInitEnv:    aws.Bool(false),
			inDeployEnv:  aws.Bool(false),
			inShouldInit: aws.Bool(true),
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockCmd: func(m *mocks.Mockcmd) {

			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp("app", "fe", manifestinfo.LoadBalancedWebServiceType).Return(nil)
			},
		},
		"errors if noInit specified": {
			inAppName:    "app",
			inName:       "fe",
			inEnvName:    "test",
			inInitEnv:    aws.Bool(false),
			inDeployEnv:  aws.Bool(false),
			inShouldInit: aws.Bool(false),
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Times(0)
				m.EXPECT().Validate().Times(0)
				m.EXPECT().Execute().Times(0)
				m.EXPECT().RecommendActions().Times(0)
			},
			mockCmd: func(m *mocks.Mockcmd) {

			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "workload fe is uninitialized but --init-wkld=false was specified",
		},
		"errors reading manifest": {
			inAppName:   "app",
			inName:      "fe",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Return(nil, errors.New("some error"))
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Times(0)
				m.EXPECT().Validate().Times(0)
				m.EXPECT().Execute().Times(0)
				m.EXPECT().RecommendActions().Times(0)
			},
			mockCmd: func(m *mocks.Mockcmd) {},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "read manifest for workload fe: some error",
		},
		"error getting workload type": {
			inAppName:   "app",
			inName:      "fe",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Return(workspace.WorkloadManifest(`type: nothing here`), nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Times(0)
				m.EXPECT().Validate().Times(0)
				m.EXPECT().Execute().Times(0)
				m.EXPECT().RecommendActions().Times(0)
			},
			mockCmd: func(m *mocks.Mockcmd) {

			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "unrecognized workload type \"nothing here\" in manifest for workload fe",
		},
		"error listing workloads": {
			inAppName:   "app",
			inName:      "fe",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			mockSel:     func(m *mocks.MockwsSelector) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return(nil, errors.New("some error"))
				m.EXPECT().GetWorkload("app", "fe").Times(0)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {},
			mockCmd: func(m *mocks.Mockcmd) {

			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
			},
			wantedErr: "retrieve workloads: some error",
		},
		"initializes and deploys local manifest with prompts": {
			inAppName:   "app",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockCmd: func(m *mocks.Mockcmd) {

			},
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload(gomock.Any(), gomock.Any()).Return("fe", nil)
			},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp("app", "fe", manifestinfo.LoadBalancedWebServiceType).Return(nil)
			},
		},
		"errors correctly if job returned": {
			inAppName:   "app",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			wantedErr:   "ask job deploy: some error",
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload("Select a service or job in your workspace", "").Return("mailer", nil)
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Return(errors.New("some error"))
			},
			mockCmd: func(m *mocks.Mockcmd) {

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
			inAppName:   "app",
			inName:      "fe",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			mockSel:     func(m *mocks.MockwsSelector) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute()
				m.EXPECT().RecommendActions()
			},
			mockCmd: func(m *mocks.Mockcmd) {

			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
			},
		},
		"get name error": {
			inAppName:   "app",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			wantedErr:   "select service or job: some error",
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {},
			mockCmd:           func(m *mocks.Mockcmd) {},
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
			inAppName:   "app",
			inName:      "fe",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			wantedErr:   "ask svc deploy: some error",

			mockSel: func(m *mocks.MockwsSelector) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask().Return(errors.New("some error"))
			},
			mockCmd: func(m *mocks.Mockcmd) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)

			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
			},
		},
		"validate error": {
			inAppName:   "app",
			inName:      "fe",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			wantedErr:   "validate svc deploy: some error",

			mockSel: func(m *mocks.MockwsSelector) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate().Return(errors.New("some error"))
			},
			mockCmd: func(m *mocks.Mockcmd) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&mockWl, nil)
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockWl}, nil)

			},
			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Times(0)
			},
		},
		"execute error": {
			inAppName:   "app",
			inName:      "fe",
			inEnvName:   "test",
			inInitEnv:   aws.Bool(false),
			inDeployEnv: aws.Bool(false),
			wantedErr:   "execute svc deploy: some error",

			mockSel: func(m *mocks.MockwsSelector) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {
				m.EXPECT().Ask()
				m.EXPECT().Validate()
				m.EXPECT().Execute().Return(errors.New("some error"))
			},
			mockCmd: func(m *mocks.Mockcmd) {},
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
			mockNoActionCmd := mocks.NewMockcmd(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockWs := mocks.NewMockwsWlDirReader(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockInit := mocks.NewMockwkldInitializerWithoutManifest(ctrl)

			tc.mockStore(mockStore)
			tc.mockSel(mockSel)
			tc.mockActionCommand(mockCmd)
			tc.mockCmd(mockNoActionCmd)
			tc.mockWs(mockWs)
			tc.mockPrompt(mockPrompt)
			tc.mockInit(mockInit)

			opts := &deployOpts{
				deployVars: deployVars{
					deployWkldVars: deployWkldVars{
						appName: tc.inAppName,
						name:    tc.inName,
						envName: tc.inEnvName,
					},
					yesInitWkld: tc.inShouldInit,
					deployEnv:   tc.inDeployEnv,
					yesInitEnv:  tc.inInitEnv,
				},
				deployWkld:      mockCmd,
				newInitEnvCmd:   func(o *deployOpts) (cmd, error) { return mockNoActionCmd, nil },
				newDeployEnvCmd: func(o *deployOpts) (cmd, error) { return mockNoActionCmd, nil },
				sel:             mockSel,
				prompt:          mockPrompt,
				store:           mockStore,
				ws:              mockWs,

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
