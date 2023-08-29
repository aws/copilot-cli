// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
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
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{&mockJob}, nil)
				m.EXPECT().GetWorkload("app", "mailer").Return(&mockJob, nil)
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
		"workload init error": {
			inAppName:         "app",
			inName:            "fe",
			inEnvName:         "test",
			wantedErr:         "add workload to app: some error",
			inDeployEnv:       aws.Bool(false),
			inShouldInit:      aws.Bool(true),
			mockSel:           func(m *mocks.MockwsSelector) {},
			mockPrompt:        func(m *mocks.Mockprompter) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {},
			mockCmd:           func(m *mocks.Mockcmd) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				// After env init/deploy
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
				// After wkld init
				m.EXPECT().GetWorkload("app", "fe").Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
			},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {
				m.EXPECT().AddWorkloadToApp("app", "fe", "Load Balanced Web Service").Return(errors.New("some error"))
			},
		},
		"confirm workload init error": {
			inAppName:   "app",
			inName:      "fe",
			inEnvName:   "test",
			wantedErr:   "confirm initialize workload: some error",
			inDeployEnv: aws.Bool(false),
			mockSel:     func(m *mocks.MockwsSelector) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Found manifest for uninitialized Load Balanced Web Service \"fe\". Initialize it?", gomock.Any(), gomock.Any()).Return(false, errors.New("some error"))
			},
			mockActionCommand: func(m *mocks.MockactionCommand) {},
			mockCmd:           func(m *mocks.Mockcmd) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&mockEnv, nil)
				// After env init/deploy
				m.EXPECT().ListWorkloads("app").Return(nil, nil)
				// After wkld init
				m.EXPECT().GetWorkload("app", "fe").Times(0)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
				m.EXPECT().ReadWorkloadManifest("fe").Return(mockManifest, nil)
			},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {},
		},
		"both uninitialized and initialized environments and workloads": {
			inAppName: "app",
			wantedErr: "",
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Workload("Select a service or job in your workspace", "").Return("fe", nil)
				m.EXPECT().Environment("Select an environment to deploy to", "", "app", prompt.Option{Value: "prod", Hint: "uninitialized"}).Return("prod", nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Environment \"prod\" does not exist in app \"app\". Initialize it?", "").Return(true, nil)
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
				m.EXPECT().ListEnvironments("app").Return([]*config.Environment{&mockEnv}, nil)
				m.EXPECT().GetEnvironment("app", "prod").Return(nil, &config.ErrNoSuchEnvironment{})
				// After env init/deploy
				m.EXPECT().ListWorkloads("app").Return([]*config.Workload{{Name: "fe", Type: "Load Balanced Web Service"}}, nil)
				m.EXPECT().GetWorkload("app", "fe").Return(&config.Workload{Name: "fe", Type: "Load Balanced Web Service"}, nil)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test", "prod"}, nil)
			},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {

			},
		},
		"error listing ws envs": {
			inAppName:         "app",
			inName:            "fe",
			wantedErr:         "get initialized environments: some error",
			mockSel:           func(m *mocks.MockwsSelector) {},
			mockPrompt:        func(m *mocks.Mockprompter) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {},
			mockCmd:           func(m *mocks.Mockcmd) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("app").Return(nil, errors.New("some error"))
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{}, nil)
			},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {},
		},
		"error selecting environment": {
			inAppName: "app",
			inName:    "fe",
			wantedErr: "get environment name: some error",
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Environment(gomock.Any(), "", "app", prompt.Option{Value: "prod", Hint: "uninitialized"}).Return("", errors.New("some error"))
			},
			mockPrompt:        func(m *mocks.Mockprompter) {},
			mockActionCommand: func(m *mocks.MockactionCommand) {},
			mockCmd:           func(m *mocks.Mockcmd) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("app").Return([]*config.Environment{&mockEnv}, nil)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"prod"}, nil)
			},
			mockInit: func(m *mocks.MockwkldInitializerWithoutManifest) {},
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

func Test_deployOpts_checkEnvExists(t *testing.T) {
	mockError := errors.New("some error")
	tests := map[string]struct {
		wantEnvExistsInApp, wantEnvExistsInWs bool

		mockStore func(m *mocks.Mockstore)
		mockWs    func(m *mocks.MockwsWlDirReader)

		wantErr string
	}{
		"error getting environment": {
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(nil, mockError)
			},
			mockWs:  func(m *mocks.MockwsWlDirReader) {},
			wantErr: "get environment from config store: some error",
		},
		"env exists in ws but not app": {
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return([]string{"test"}, nil)
			},
			wantEnvExistsInWs:  true,
			wantEnvExistsInApp: false,
		},
		"env exists in app but not ws": {
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&config.Environment{
					App:  "app",
					Name: "test",
				}, nil)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return(nil, nil)
			},
			wantEnvExistsInWs:  false,
			wantEnvExistsInApp: true,
		},
		"env does not exist anywhere": {
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(nil, &config.ErrNoSuchEnvironment{})
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return(nil, nil)
			},
			wantErr: "environment \"test\" does not exist in the workspace",
		},
		"error listing envs": {
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("app", "test").Return(&config.Environment{
					App:  "app",
					Name: "test",
				}, nil)
			},
			mockWs: func(m *mocks.MockwsWlDirReader) {
				m.EXPECT().ListEnvironments().Return(nil, mockError)
			},
			wantErr: "list environments in workspace: some error",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)
			mockWs := mocks.NewMockwsWlDirReader(ctrl)

			tc.mockWs(mockWs)
			tc.mockStore(mockStore)

			o := &deployOpts{
				deployVars: deployVars{
					deployWkldVars: deployWkldVars{
						envName: "test",
						appName: "app",
					},
				},
				store: mockStore,
				ws:    mockWs,
			}

			err := o.checkEnvExists()
			if err != nil {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.Equal(t, tc.wantEnvExistsInApp, o.envExistsInApp)
				require.Equal(t, tc.wantEnvExistsInWs, o.envExistsInWs)
			}
		})
	}
}

func Test_deployOpts_maybeInitEnv(t *testing.T) {
	mockError := errors.New("some error")
	tests := map[string]struct {
		envExistsInApp bool
		envExistsInWs  bool
		yesInitEnv     *bool
		deployEnv      *bool

		mockPrompt     func(m *mocks.Mockprompter)
		mockInitEnvCmd func(m *mocks.Mockcmd)

		wantDeployEnv *bool
		wantErr       string
	}{
		"env does not exist": {
			envExistsInWs:  false,
			envExistsInApp: false,

			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {},
			wantErr:        `environment "test" does not exist in the workspace`,
		},
		"env already exists": {
			envExistsInApp: true,

			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {},

			deployEnv:     nil,
			wantDeployEnv: nil,
		},
		"error prompt to confirm env init": {
			envExistsInWs:  true,
			envExistsInApp: false,
			yesInitEnv:     nil,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any()).Return(false, mockError)
			},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {},
			wantErr:        "confirm env init: some error",
		},
		"confirm env init returns false": {
			envExistsInWs:  true,
			envExistsInApp: false,
			yesInitEnv:     nil,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any()).Return(false, nil)
			},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {},
			wantErr:        "env test does not exist in app app",
		},
		"error validating initEnvCmd": {
			envExistsInApp: false,
			envExistsInWs:  true,
			yesInitEnv:     aws.Bool(true),

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {
				m.EXPECT().Validate().Return(mockError)
			},

			wantErr: "some error",
		},
		"error in initEnvCmd.Ask": {
			envExistsInApp: false,
			envExistsInWs:  true,
			yesInitEnv:     aws.Bool(true),

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {
				m.EXPECT().Validate().Return(nil)
				m.EXPECT().Ask().Return(mockError)
			},

			wantErr: "some error",
		},
		"error in initEnvCmd.Execute": {
			envExistsInApp: false,
			envExistsInWs:  true,
			yesInitEnv:     aws.Bool(true),

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {
				m.EXPECT().Validate().Return(nil)
				m.EXPECT().Ask().Return(nil)
				m.EXPECT().Execute().Return(mockError)
			},

			wantErr: "some error",
		},
		"error when env initialized but deploy is false": {
			envExistsInApp: false,
			envExistsInWs:  true,
			yesInitEnv:     aws.Bool(true),
			deployEnv:      aws.Bool(false),

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {
				m.EXPECT().Validate().Return(nil)
				m.EXPECT().Ask().Return(nil)
				m.EXPECT().Execute().Return(nil)
			},

			wantErr: "environment test was initialized but has not been deployed",
		},
		"error when environment was not initialized due to prompting or flags": {
			envExistsInApp: false,
			envExistsInWs:  true,
			yesInitEnv:     aws.Bool(false),
			deployEnv:      aws.Bool(false),

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {
				m.EXPECT().Validate().Return(nil).Times(0)
				m.EXPECT().Ask().Return(nil).Times(0)
				m.EXPECT().Execute().Return(nil).Times(0)
			},

			wantErr: "env test does not exist in app app",
		},
		"deployEnv set correctly after initializing app": {
			envExistsInApp: false,
			envExistsInWs:  true,
			yesInitEnv:     aws.Bool(true),

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockInitEnvCmd: func(m *mocks.Mockcmd) {
				m.EXPECT().Validate().Return(nil)
				m.EXPECT().Ask().Return(nil)
				m.EXPECT().Execute().Return(nil)
			},
			wantDeployEnv: aws.Bool(true),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockInitEnvCmd := mocks.NewMockcmd(ctrl)

			tc.mockPrompt(mockPrompt)
			tc.mockInitEnvCmd(mockInitEnvCmd)

			o := &deployOpts{
				deployVars: deployVars{
					deployWkldVars: deployWkldVars{
						envName: "test",
						appName: "app",
					},
					deployEnv:  tc.deployEnv,
					yesInitEnv: tc.yesInitEnv,
				},
				envExistsInApp: tc.envExistsInApp,
				envExistsInWs:  tc.envExistsInWs,
				prompt:         mockPrompt,
				newInitEnvCmd: func(o *deployOpts) (cmd, error) {
					return mockInitEnvCmd, nil
				},
			}

			err := o.maybeInitEnv()
			if err != nil {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.Equal(t, tc.wantDeployEnv, o.deployEnv)
				require.NoError(t, err)
			}
		})
	}
}

func Test_deployOpts_maybeDeployEnv(t *testing.T) {
	mockError := errors.New("some error")
	tests := map[string]struct {
		envExistsInWs bool
		deployEnv     *bool

		mockPrompt       func(m *mocks.Mockprompter)
		mockDeployEnvCmd func(m *mocks.Mockcmd)

		wantErr string
	}{
		"env does not exist in ws": {
			envExistsInWs:    false,
			mockPrompt:       func(m *mocks.Mockprompter) {},
			mockDeployEnvCmd: func(m *mocks.Mockcmd) {},
		},
		"env exists; deploy false when prompted": {
			envExistsInWs: true,
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			mockDeployEnvCmd: func(m *mocks.Mockcmd) {},
		},
		"env exists; deploy true when prompted": {
			envExistsInWs: true,
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
			},
			mockDeployEnvCmd: func(m *mocks.Mockcmd) {
				m.EXPECT().Validate().Return(nil)
				m.EXPECT().Ask().Return(nil)
				m.EXPECT().Execute().Return(nil)
			},
		},
		"error selecting whether to deploy env": {
			envExistsInWs: true,
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, mockError)
			},
			mockDeployEnvCmd: func(m *mocks.Mockcmd) {},
			wantErr:          "confirm env deployment: some error",
		},
		"env exists; deploy flag set": {
			envExistsInWs: true,
			deployEnv:     aws.Bool(true),
			mockPrompt:    func(m *mocks.Mockprompter) {},
			mockDeployEnvCmd: func(m *mocks.Mockcmd) {
				m.EXPECT().Validate().Return(nil)
				m.EXPECT().Ask().Return(nil)
				m.EXPECT().Execute().Return(nil)
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockDeployEnvCmd := mocks.NewMockcmd(ctrl)

			tc.mockPrompt(mockPrompt)
			tc.mockDeployEnvCmd(mockDeployEnvCmd)

			o := &deployOpts{
				deployVars: deployVars{
					deployWkldVars: deployWkldVars{
						envName: "test",
						appName: "app",
					},
					deployEnv: tc.deployEnv,
				},
				envExistsInWs: tc.envExistsInWs,
				prompt:        mockPrompt,
				newDeployEnvCmd: func(o *deployOpts) (cmd, error) {
					return mockDeployEnvCmd, nil
				},
			}

			err := o.maybeDeployEnv()
			if err != nil {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
