// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deployPipelineMocks struct {
	store                  *mocks.Mockstore
	prompt                 *mocks.Mockprompter
	prog                   *mocks.Mockprogress
	deployer               *mocks.MockpipelineDeployer
	pipelineStackConfig    *mocks.MockstackConfiguration
	mockDiffWriter         *strings.Builder
	ws                     *mocks.MockwsPipelineReader
	actionCmd              *mocks.MockactionCommand
	deployedPipelineLister *mocks.MockdeployedPipelineLister
	versionGetter          *mocks.MockversionGetter
}

func TestDeployPipelineOpts_Ask(t *testing.T) {
	const (
		testAppName        = "badgoose"
		testPipelineName   = "pipeline-badgoose-honkpipes"
		testPipelineSecret = "github-token-badgoose-honkpipes"
	)
	pipeline := workspace.PipelineManifest{
		Name: testPipelineName,
		Path: "copilot/pipeline.yml",
	}
	testCases := map[string]struct {
		inAppName      string
		inWsAppName    string
		inPipelineName string
		mockWs         func(m *mocks.MockwsPipelineReader)
		mockSel        func(m *mocks.MockwsPipelineSelector)
		mockStore      func(m *mocks.Mockstore)

		wantedApp       string
		wantedAppConfig *config.Application
		wantedPipeline  *workspace.PipelineManifest
		wantedError     error
	}{
		"return error if can't read app name from workspace file": {
			inWsAppName: "",
			mockStore:   func(m *mocks.Mockstore) {},
			mockWs:      func(m *mocks.MockwsPipelineReader) {},
			mockSel:     func(m *mocks.MockwsPipelineSelector) {},

			wantedError: errNoAppInWorkspace,
		},
		"return error if passed-in app name doesn't match workspace app": {
			inAppName:   "badAppName",
			inWsAppName: testAppName,
			mockStore:   func(m *mocks.Mockstore) {},
			mockWs:      func(m *mocks.MockwsPipelineReader) {},
			mockSel:     func(m *mocks.MockwsPipelineSelector) {},

			wantedError: errors.New("cannot specify app badAppName because the workspace is already registered with app badgoose"),
		},
		"return error if passed-in app name can't be validated": {
			inWsAppName: testAppName,
			inAppName:   testAppName,
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(testAppName).Return(nil, errors.New("some error"))
			},
			mockWs:  func(m *mocks.MockwsPipelineReader) {},
			mockSel: func(m *mocks.MockwsPipelineSelector) {},

			wantedError: errors.New("get application badgoose configuration: some error"),
		},
		"return error if passed-in pipeline name not found": {
			inAppName:      testAppName,
			inWsAppName:    testAppName,
			inPipelineName: "someOtherPipelineName",

			mockSel: func(m *mocks.MockwsPipelineSelector) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(testAppName).Return(nil, nil)
			},
			mockWs: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil)
			},
			wantedError: errors.New("pipeline someOtherPipelineName not found in the workspace"),
		},
		"return error if fail to select pipeline": {
			inAppName:   testAppName,
			inWsAppName: testAppName,
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(testAppName).Return(nil, nil)
			},
			mockSel: func(m *mocks.MockwsPipelineSelector) {
				m.EXPECT().WsPipeline(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			mockWs: func(m *mocks.MockwsPipelineReader) {},

			wantedError: fmt.Errorf("select pipeline: some error"),
		},
		"success with app flag and pipeline flag": {
			inWsAppName:    testAppName,
			inAppName:      testAppName,
			inPipelineName: testPipelineName,
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(testAppName).Return(&config.Application{
					Name: testAppName,
				}, nil)
			},
			mockSel: func(m *mocks.MockwsPipelineSelector) {},
			mockWs: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil)
			},

			wantedApp:       testAppName,
			wantedAppConfig: &config.Application{Name: testAppName},
			wantedPipeline:  &pipeline,
			wantedError:     nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSel := mocks.NewMockwsPipelineSelector(ctrl)
			mockWs := mocks.NewMockwsPipelineReader(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockSel(mockSel)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			opts := deployPipelineOpts{
				deployPipelineVars: deployPipelineVars{
					appName: tc.inAppName,
					name:    tc.inPipelineName,
				},
				wsAppName: tc.inWsAppName,
				sel:       mockSel,
				ws:        mockWs,
				store:     mockStore,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedPipeline, opts.pipeline)
				require.Equal(t, tc.wantedApp, opts.appName)
				require.Equal(t, tc.wantedAppConfig, opts.app)
			}
		})
	}
}

func TestDeployPipelineOpts_Execute(t *testing.T) {
	const (
		appName              = "badgoose"
		region               = "us-west-2"
		accountID            = "123456789012"
		pipelineName         = "pipepiper"
		badPipelineName      = "pipeline-badgoose-honkpipes"
		pipelineManifestPath = "someStuff/someMoreStuff/aws-copilot-sample-service/copilot/pipelines/pipepiper/manifest.yml"
		relativePath         = "/copilot/pipelines/pipepiper/manifest.yml"
		mockTemplateVersion  = "v1.28.0"
		mockFutureVersion    = "v1.30.0"
	)
	mockPipelineManifest := &manifest.Pipeline{
		Name:    "pipepiper",
		Version: 1,
		Source: &manifest.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"repository": "aws/somethingCool",
				"branch":     "main",
			},
		},
		Stages: []manifest.PipelineStage{
			{
				Name:         "chicken",
				TestCommands: []string{"make test", "echo 'made test'"},
			},
			{
				Name:         "wings",
				TestCommands: []string{"echo 'bok bok bok'"},
			},
		},
	}
	app := config.Application{
		AccountID: accountID,
		Name:      appName,
		Domain:    "amazon.com",
	}
	mockResources := []*stack.AppRegionalResources{
		{
			S3Bucket:  "someBucket",
			KMSKeyARN: "someKey",
		},
	}
	mockResource := &stack.AppRegionalResources{
		S3Bucket: "someOtherBucket",
	}
	mockEnv := &config.Environment{
		Name:      "test",
		App:       appName,
		Region:    region,
		AccountID: accountID,
	}
	testCases := map[string]struct {
		inApp            *config.Application
		inAppName        string
		inPipelineName   string
		inRegion         string
		inPipelineFile   string
		inAllowDowngrade bool
		callMocks        func(m deployPipelineMocks)
		expectedError    error
		inShowDiff       bool
	}{
		"create and deploy pipeline": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(false, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployStart, pipelineName)).Times(1),
					m.deployer.EXPECT().CreatePipeline(gomock.Any(), gomock.Any()).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployComplete, pipelineName)).Times(1),
				)
			},
			expectedError: nil,
		},
		"update and deploy pipeline with new naming": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(true, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(true, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployProposalStart, pipelineName)).Times(1),
					m.deployer.EXPECT().UpdatePipeline(gomock.Any(), gomock.Any()).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployProposalComplete, pipelineName)).Times(1),
				)
			},
			expectedError: nil,
		},
		"update and deploy pipeline with legacy naming": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{
						{
							ResourceName: pipelineName,
							IsLegacy:     true,
						},
					}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(true, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(true, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployProposalStart, pipelineName)).Times(1),
					m.deployer.EXPECT().UpdatePipeline(gomock.Any(), gomock.Any()).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployProposalComplete, pipelineName)).Times(1),
				)
			},
			expectedError: nil,
		},
		"do not deploy pipeline if decline to redeploy an existing pipeline": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(true, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(false, nil),
				)
			},
			expectedError: nil,
		},
		"returns an error if fails to prompt for pipeline deploy": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(true, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(false, errors.New("some error")),
				)
			},
			expectedError: fmt.Errorf("prompt for pipeline deploy: some error"),
		},
		"returns an error if try to downgrade pipeline template": {
			inApp:          &app,
			inRegion:       region,
			inAppName:      appName,
			inPipelineName: pipelineName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockFutureVersion, nil),
				)
			},
			expectedError: fmt.Errorf(`cannot downgrade pipeline "pipepiper" (currently in version v1.30.0) to version v1.28.0`),
		},
		"returns an error if fail to add pipeline resources to app": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),

					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(errors.New("some error")),
					m.prog.EXPECT().Stop(log.Serrorf(fmtPipelineDeployResourcesFailed, appName)).Times(1),
				)
			},
			expectedError: fmt.Errorf("add pipeline resources to application %s in %s: some error", appName, region),
		},
		"returns an error if fail to read pipeline file": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, errors.New("some error")),
				)
			},
			expectedError: fmt.Errorf("read pipeline manifest: some error"),
		},
		"returns an error if unable to unmarshal pipeline file": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(nil, errors.New("some error")),
				)
			},
			expectedError: fmt.Errorf("read pipeline manifest: some error"),
		},
		"returns an error if pipeline name fails validation": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				mockBadPipelineManifest := &manifest.Pipeline{
					Name:    "12345678101234567820123456783012345678401234567850123456786012345678701234567880123456789012345671001",
					Version: 1,
					Source: &manifest.Source{
						ProviderName: "GitHub",
						Properties: map[string]interface{}{
							"repository": "aws/somethingCool",
							"branch":     "main",
						},
					},
				}
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockBadPipelineManifest, nil),
				)
			},
			expectedError: fmt.Errorf("validate pipeline manifest: pipeline name '12345678101234567820123456783012345678401234567850123456786012345678701234567880123456789012345671001' must be shorter than 100 characters"),
		},
		"returns an error if provider is not a supported type": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				mockBadPipelineManifest := &manifest.Pipeline{
					Name:    badPipelineName,
					Version: 1,
					Source: &manifest.Source{
						ProviderName: "NotGitHub",
						Properties: map[string]interface{}{
							"access_token_secret": "github-token-badgoose-backend",
							"repository":          "aws/somethingCool",
							"branch":              "main",
						},
					},
				}
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockBadPipelineManifest, nil),
				)
			},
			expectedError: fmt.Errorf("read source from manifest: invalid repo source provider: NotGitHub"),
		},
		"returns an error if unable to convert environments to deployment stage": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Return(errors.New("some error")),
				)
			},
			expectedError: fmt.Errorf("convert environments to deployment stage: get local services: some error"),
		},
		"returns an error if fails to get cross-regional resources": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, errors.New("some error")),
				)
			},
			expectedError: fmt.Errorf("get cross-regional resources: some error"),
		},
		"returns an error if fails to check if pipeline exists": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(false, errors.New("some error")),
				)
			},
			expectedError: fmt.Errorf("check if pipeline exists: some error"),
		},
		"returns an error if fails to create pipeline": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(false, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployStart, pipelineName)).Times(1),
					m.deployer.EXPECT().CreatePipeline(gomock.Any(), gomock.Any()).Return(errors.New("some error")),
					m.prog.EXPECT().Stop(log.Serrorf(fmtPipelineDeployFailed, pipelineName)).Times(1),
				)
			},
			expectedError: fmt.Errorf("create pipeline: some error"),
		},
		"returns an error if fails to update pipeline": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					// bootstrap pipeline resources
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(true, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(true, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployProposalStart, pipelineName)).Times(1),
					m.deployer.EXPECT().UpdatePipeline(gomock.Any(), gomock.Any()).Return(errors.New("some error")),
					m.prog.EXPECT().Stop(log.Serrorf(fmtPipelineDeployProposalFailed, pipelineName)).Times(1),
				)
			},

			expectedError: fmt.Errorf("update pipeline: some error"),
		},
		"update and deploy pipeline with specifying build property": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				mockPipelineManifest := &manifest.Pipeline{
					Name:    "pipepiper",
					Version: 1,
					Source: &manifest.Source{
						ProviderName: "GitHub",
						Properties: map[string]interface{}{
							"repository": "aws/somethingCool",
							"branch":     "main",
						},
					},
					Build: &manifest.Build{Image: "aws/codebuild/standard:3.0"},
					Stages: []manifest.PipelineStage{
						{
							Name:             "chicken",
							RequiresApproval: false,
							TestCommands:     []string{"make test", "echo 'made test'"},
						},
						{
							Name:             "wings",
							RequiresApproval: false,
							TestCommands:     []string{"echo 'bok bok bok'"},
						},
					},
				}
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(true, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(true, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployProposalStart, pipelineName)).Times(1),
					m.deployer.EXPECT().UpdatePipeline(gomock.Any(), gomock.Any()).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployProposalComplete, pipelineName)).Times(1),
				)
			},
			expectedError: nil,
		},
		"error if failed to generate the template to show diff": {
			inApp:      &app,
			inAppName:  appName,
			inRegion:   region,
			inShowDiff: true,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),

					m.pipelineStackConfig.EXPECT().Template().Return("", errors.New("some error")),
				)

			},
			expectedError: fmt.Errorf("generate the new template for diff: generate stack template: some error"),
		},
		"failed to fetch the template of deployed stack": {
			inApp:      &app,
			inAppName:  appName,
			inRegion:   region,
			inShowDiff: true,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),
					m.pipelineStackConfig.EXPECT().Template().Return("template one", nil),

					m.deployer.EXPECT().Template(gomock.Any()).Return("", fmt.Errorf("some error")))
			},
			expectedError: fmt.Errorf("retrieve the deployed template for %q: some error", pipelineName),
		},
		"failed prompt to accept diff": {
			inApp:      &app,
			inAppName:  appName,
			inRegion:   region,
			inShowDiff: true,
			callMocks: func(m deployPipelineMocks) {
				m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil)
				m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil)
				m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil)
				m.actionCmd.EXPECT().Execute().Times(2)

				// convertStages
				m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)

				// getArtifactBuckets
				m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)

				m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path")

				m.pipelineStackConfig.EXPECT().Template().Return("name: mockEnv\ntype: Environment", nil)

				m.deployer.EXPECT().Template(gomock.Any()).Return("name: mockEnv\ntype: Environment", nil)

				m.prompt.EXPECT().Confirm(continueDeploymentPrompt, "").Return(false, errors.New("some error"))

			},
			expectedError: fmt.Errorf("ask whether to continue with the deployment: some error"),
		},
		"successfully show diff and create a new pipeline": {
			inApp:      &app,
			inAppName:  appName,
			inRegion:   region,
			inShowDiff: true,
			callMocks: func(m deployPipelineMocks) {
				m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil)
				m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
				m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil)
				m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil)
				m.actionCmd.EXPECT().Execute().Times(2)

				// convertStages
				m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)

				// getArtifactBuckets
				m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)

				m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path")

				m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(false, nil)

				m.pipelineStackConfig.EXPECT().Template().Return("name: mockEnv\ntype: Environment", nil)

				m.deployer.EXPECT().Template(gomock.Any()).Return("name: mockEnv\ntype: Environment", nil)

				m.prompt.EXPECT().Confirm(continueDeploymentPrompt, "").Return(true, nil)

				m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1)
				m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1)

				// deployPipeline
				m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil)
				m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployStart, pipelineName)).Times(1)
				m.deployer.EXPECT().CreatePipeline(gomock.Any(), gomock.Any()).Return(nil)
				m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployComplete, pipelineName)).Times(1)

			},
		},
		"Successfully show diff and redeploy an existing pipeline": {
			inApp:            &app,
			inAppName:        appName,
			inRegion:         region,
			inShowDiff:       true,
			inAllowDowngrade: true,
			callMocks: func(m deployPipelineMocks) {
				m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil)
				m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil)
				m.actionCmd.EXPECT().Execute().Times(2)

				// convertStages
				m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)

				// getArtifactBuckets
				m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
				m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil)
				m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path")

				m.pipelineStackConfig.EXPECT().Template().Return("name: mockEnv\ntype: Environment", nil)
				m.deployer.EXPECT().Template(gomock.Any()).Return("name: mockEnv\ntype: Environment", nil)

				m.prompt.EXPECT().Confirm(continueDeploymentPrompt, "").Return(true, nil)

				m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1)
				m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1)

				// deployPipeline
				m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(true, nil)

				m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil)

				m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployProposalStart, pipelineName)).Times(1)
				m.deployer.EXPECT().UpdatePipeline(gomock.Any(), gomock.Any()).Return(nil)
				m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployProposalComplete, pipelineName)).Times(1)

			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := deployPipelineMocks{
				store:                  mocks.NewMockstore(ctrl),
				prompt:                 mocks.NewMockprompter(ctrl),
				prog:                   mocks.NewMockprogress(ctrl),
				deployer:               mocks.NewMockpipelineDeployer(ctrl),
				ws:                     mocks.NewMockwsPipelineReader(ctrl),
				actionCmd:              mocks.NewMockactionCommand(ctrl),
				pipelineStackConfig:    mocks.NewMockstackConfiguration(ctrl),
				deployedPipelineLister: mocks.NewMockdeployedPipelineLister(ctrl),
				versionGetter:          mocks.NewMockversionGetter(ctrl),
				mockDiffWriter:         &strings.Builder{},
			}

			tc.callMocks(mocks)

			opts := &deployPipelineOpts{
				deployPipelineVars: deployPipelineVars{
					appName:        tc.inAppName,
					name:           tc.inPipelineName,
					showDiff:       tc.inShowDiff,
					allowDowngrade: tc.inAllowDowngrade,
				},
				pipelineDeployer: mocks.deployer,
				pipelineStackConfig: func(in *deploy.CreatePipelineInput) stackConfiguration {
					return mocks.pipelineStackConfig
				},
				ws:              mocks.ws,
				app:             tc.inApp,
				region:          tc.inRegion,
				store:           mocks.store,
				prog:            mocks.prog,
				prompt:          mocks.prompt,
				templateVersion: mockTemplateVersion,
				diffWriter:      &strings.Builder{},
				newSvcListCmd: func(w io.Writer, app string) cmd {
					return mocks.actionCmd
				},
				pipelineVersionGetter: func(s1, s2 string, b bool) (versionGetter, error) {
					return mocks.versionGetter, nil
				},
				newJobListCmd: func(w io.Writer, app string) cmd {
					return mocks.actionCmd
				},
				configureDeployedPipelineLister: func() deployedPipelineLister {
					return mocks.deployedPipelineLister
				},
				pipeline: &workspace.PipelineManifest{
					Name: "pipepiper",
					Path: pipelineManifestPath,
				},
				svcBuffer: bytes.NewBufferString(`{"services":[{"app":"badgoose","name":"frontend","type":""}]}`),
				jobBuffer: bytes.NewBufferString(`{"jobs":[{"app":"badgoose","name":"backend","type":""}]}`),
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeployPipelineOpts_getArtifactBuckets(t *testing.T) {
	testCases := map[string]struct {
		mockDeployer func(m *mocks.MockpipelineDeployer)

		expectedOut []deploy.ArtifactBucket

		expectedError error
	}{
		"getsBucketInfo": {
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				mockResources := []*stack.AppRegionalResources{
					{
						S3Bucket:  "someBucket",
						KMSKeyARN: "someKey",
					},
				}
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
			},
			expectedOut: []deploy.ArtifactBucket{
				{
					BucketName: "someBucket",
					KeyArn:     "someKey",
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPipelineDeployer := mocks.NewMockpipelineDeployer(ctrl)
			tc.mockDeployer(mockPipelineDeployer)

			opts := &deployPipelineOpts{
				pipelineDeployer: mockPipelineDeployer,
			}

			// WHEN
			actual, err := opts.getArtifactBuckets()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.expectedOut, actual)
			}
		})
	}
}
