// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	ws                     *mocks.MockwsPipelineReader
	actionCmd              *mocks.MockactionCommand
	deployedPipelineLister *mocks.MockpipelineLister
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
		inApp          *config.Application
		inAppName      string
		inPipelineName string
		inRegion       string
		inPipelineFile string
		callMocks      func(m deployPipelineMocks)
		expectedError  error
	}{
		"create and deploy pipeline": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {

				gomock.InOrder(
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{}, nil),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).DoAndReturn(func(in *deploy.CreatePipelineInput) (bool, error) {
						if in.IsLegacy {
							return false, errors.New("should not be a legacy pipeline")
						}
						return false, nil
					}),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployStart, pipelineName)).Times(1),
					m.deployer.EXPECT().CreatePipeline(gomock.Any(), gomock.Any()).DoAndReturn(func(in *deploy.CreatePipelineInput, _ string) error {
						if in.IsLegacy {
							return errors.New("should not be a legacy pipeline")
						}
						return nil
					}),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{}, nil),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).DoAndReturn(func(in *deploy.CreatePipelineInput) (bool, error) {
						if in.IsLegacy {
							return false, errors.New("should not be a legacy pipeline")
						}
						return true, nil
					}),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(true, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployProposalStart, pipelineName)).Times(1),
					m.deployer.EXPECT().UpdatePipeline(gomock.Any(), gomock.Any()).DoAndReturn(func(in *deploy.CreatePipelineInput, _ string) error {
						if in.IsLegacy {
							return errors.New("should not be a legacy pipeline")
						}
						return nil
					}),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{
						{
							ResourceName: pipelineName,
							IsLegacy:     true,
						},
					}, nil),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).DoAndReturn(func(in *deploy.CreatePipelineInput) (bool, error) {
						if !in.IsLegacy {
							return false, errors.New("should be a legacy pipeline")
						}
						return true, nil
					}),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(true, nil),
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployProposalStart, pipelineName)).Times(1),
					m.deployer.EXPECT().UpdatePipeline(gomock.Any(), gomock.Any()).DoAndReturn(func(in *deploy.CreatePipelineInput, _ string) error {
						if !in.IsLegacy {
							return errors.New("should be a legacy pipeline")
						}
						return nil
					}),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{}, nil),

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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{}, nil),

					// deployPipeline
					m.deployer.EXPECT().PipelineExists(gomock.Any()).Return(true, nil),
					m.deployer.EXPECT().GetAppResourcesByRegion(&app, region).Return(mockResource, nil),
					m.prompt.EXPECT().Confirm(fmt.Sprintf(fmtPipelineDeployExistPrompt, pipelineName), "").Return(false, errors.New("some error")),
				)
			},
			expectedError: fmt.Errorf("prompt for pipeline deploy: some error"),
		},
		"returns an error if fail to add pipeline resources to app": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{}, nil),

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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{}, nil),

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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{}, nil),

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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines().Return([]deploy.Pipeline{}, nil),

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
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPipelineDeployer := mocks.NewMockpipelineDeployer(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockActionCmd := mocks.NewMockactionCommand(ctrl)

			mocks := deployPipelineMocks{
				store:                  mockStore,
				prompt:                 mockPrompt,
				prog:                   mockProgress,
				deployer:               mockPipelineDeployer,
				ws:                     mockWorkspace,
				actionCmd:              mockActionCmd,
				deployedPipelineLister: mocks.NewMockpipelineLister(ctrl),
			}

			tc.callMocks(mocks)

			opts := &deployPipelineOpts{
				deployPipelineVars: deployPipelineVars{
					appName: tc.inAppName,
					name:    tc.inPipelineName,
				},
				pipelineDeployer: mockPipelineDeployer,
				ws:               mockWorkspace,
				app:              tc.inApp,
				region:           tc.inRegion,
				store:            mockStore,
				prog:             mockProgress,
				prompt:           mockPrompt,
				newSvcListCmd: func(w io.Writer, app string) cmd {
					return mockActionCmd
				},
				newJobListCmd: func(w io.Writer, app string) cmd {
					return mockActionCmd
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
				require.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeployPipelineOpts_convertStages(t *testing.T) {
	testCases := map[string]struct {
		stages    []manifest.PipelineStage
		inAppName string
		callMocks func(m deployPipelineMocks)

		expectedStages []deploy.PipelineStage
		expectedError  error
	}{
		"converts stages with test commands": {
			stages: []manifest.PipelineStage{
				{
					Name:         "test",
					TestCommands: []string{"make test", "echo \"made test\""},
				},
			},
			inAppName: "badgoose",
			callMocks: func(m deployPipelineMocks) {
				mockEnv := &config.Environment{
					Name:      "test",
					App:       "badgoose",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Prod:      false,
				}
				gomock.InOrder(
					m.actionCmd.EXPECT().Execute().Times(2),
					m.store.EXPECT().GetEnvironment("badgoose", "test").Return(mockEnv, nil).Times(1),
				)
			},

			expectedStages: []deploy.PipelineStage{
				{
					AssociatedEnvironment: &deploy.AssociatedEnvironment{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789012",
					},
					LocalWorkloads:   []string{"frontend", "backend"},
					RequiresApproval: false,
					TestCommands:     []string{"make test", "echo \"made test\""},
				},
			},
			expectedError: nil,
		},
		"converts stages with only stage name": {
			stages: []manifest.PipelineStage{
				{
					Name: "test",
				},
			},
			inAppName: "badgoose",
			callMocks: func(m deployPipelineMocks) {
				mockEnv := &config.Environment{
					Name:      "test",
					App:       "badgoose",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Prod:      false,
				}
				gomock.InOrder(
					m.actionCmd.EXPECT().Execute().Times(2),
					m.store.EXPECT().GetEnvironment("badgoose", "test").Return(mockEnv, nil).Times(1),
				)
			},

			expectedStages: []deploy.PipelineStage{
				{
					AssociatedEnvironment: &deploy.AssociatedEnvironment{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789012",
					},
					LocalWorkloads:   []string{"frontend", "backend"},
					RequiresApproval: false,
					TestCommands:     []string(nil),
				},
			},
			expectedError: nil,
		},
		"converts stages with requires approval": {
			stages: []manifest.PipelineStage{
				{
					Name:             "test",
					RequiresApproval: true,
				},
			},
			inAppName: "badgoose",
			callMocks: func(m deployPipelineMocks) {
				mockEnv := &config.Environment{
					Name:      "test",
					App:       "badgoose",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Prod:      true,
				}
				gomock.InOrder(
					m.actionCmd.EXPECT().Execute().Times(2),
					m.store.EXPECT().GetEnvironment("badgoose", "test").Return(mockEnv, nil).Times(1),
				)
			},

			expectedStages: []deploy.PipelineStage{
				{
					AssociatedEnvironment: &deploy.AssociatedEnvironment{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789012",
					},
					LocalWorkloads:   []string{"frontend", "backend"},
					RequiresApproval: true,
					TestCommands:     []string(nil),
				},
			},
			expectedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mockActionCmd := mocks.NewMockactionCommand(ctrl)
			mocks := deployPipelineMocks{
				store:     mockStore,
				ws:        mockWorkspace,
				actionCmd: mockActionCmd,
			}

			tc.callMocks(mocks)

			opts := &deployPipelineOpts{
				deployPipelineVars: deployPipelineVars{
					appName: tc.inAppName,
				},
				store: mockStore,
				ws:    mockWorkspace,
				newSvcListCmd: func(w io.Writer, app string) cmd {
					return mockActionCmd
				},
				newJobListCmd: func(w io.Writer, app string) cmd {
					return mockActionCmd
				},
				svcBuffer: bytes.NewBufferString(`{"services":[{"app":"badgoose","name":"frontend","type":""}]}`),
				jobBuffer: bytes.NewBufferString(`{"jobs":[{"app":"badgoose","name":"backend","type":""}]}`)}

			// WHEN
			actualStages, err := opts.convertStages(tc.stages)

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.expectedStages, actualStages)
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
