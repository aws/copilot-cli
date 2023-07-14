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
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type packagePipelineMocks struct {
	store                  *mocks.Mockstore
	prompt                 *mocks.Mockprompter
	prog                   *mocks.Mockprogress
	deployer               *mocks.MockpipelineDeployer
	pipelineStackConfig    *mocks.MockstackConfiguration
	ws                     *mocks.MockwsPipelineReader
	actionCmd              *mocks.MockactionCommand
	deployedPipelineLister *mocks.MockdeployedPipelineLister
}

func TestPipelinePackageOpts_Execute(t *testing.T) {
	const (
		appName              = "badgoose"
		region               = "us-west-2"
		accountID            = "123456789012"
		pipelineName         = "pipepiper"
		badPipelineName      = "pipeline-badgoose-honkpipes"
		pipelineManifestPath = "someStuff/someMoreStuff/aws-copilot-sample-service/copilot/pipelines/pipepiper/manifest.yml"
		relativePath         = "/copilot/pipelines/pipepiper/manifest.yml"
	)
	pipeline := workspace.PipelineManifest{
		Name: pipelineName,
		Path: pipelineManifestPath,
	}
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

	mockResources := []*stack.AppRegionalResources{
		{
			S3Bucket:  "someBucket",
			KMSKeyARN: "someKey",
		},
	}
	mockEnv := &config.Environment{
		Name:      "test",
		App:       appName,
		Region:    region,
		AccountID: accountID,
	}

	var someError = errors.New("some error")

	testCases := map[string]struct {
		callMocks     func(m packagePipelineMocks)
		expectedError error
	}{

		"returns an error if fail to get the list of pipelines": {
			callMocks: func(m packagePipelineMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ListPipelines().Return(nil, someError),
				)
			},
			expectedError: fmt.Errorf("list all pipelines in the workspace: some error"),
		},
		"returns an error if fail to read pipeline file": {
			callMocks: func(m packagePipelineMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, someError),
				)
			},
			expectedError: fmt.Errorf("read pipeline manifest: some error"),
		},
		"returns an error if unable to unmarshal pipeline file": {
			callMocks: func(m packagePipelineMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(nil, someError),
				)
			},
			expectedError: fmt.Errorf("read pipeline manifest: some error"),
		},
		"returns an error if pipeline name fails validation": {
			callMocks: func(m packagePipelineMocks) {
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
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockBadPipelineManifest, nil),
				)
			},
			expectedError: fmt.Errorf("validate pipeline manifest: pipeline name '12345678101234567820123456783012345678401234567850123456786012345678701234567880123456789012345671001' must be shorter than 100 characters"),
		},
		"returns an error if provider is not a supported type": {
			callMocks: func(m packagePipelineMocks) {
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
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockBadPipelineManifest, nil),
				)
			},
			expectedError: fmt.Errorf("read source from manifest: invalid repo source provider: NotGitHub"),
		},
		"returns an error while converting manifest path to relative path from workspace root": {
			callMocks: func(m packagePipelineMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return("", someError),
				)
			},
			expectedError: fmt.Errorf("convert manifest path to relative path: some error"),
		},
		"returns an error if unable to convert environments to deployment stage": {
			callMocks: func(m packagePipelineMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Return(someError),
				)
			},
			expectedError: fmt.Errorf("convert environments to deployment stage: get local services: some error"),
		},
		"returns an error if fails to fetch an application": {
			callMocks: func(m packagePipelineMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					m.store.EXPECT().GetApplication(appName).Return(nil, someError),
				)
			},
			expectedError: fmt.Errorf("get application %v configuration: some error", appName),
		},
		"returns an error if fails to get cross-regional resources": {
			callMocks: func(m packagePipelineMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					m.store.EXPECT().GetApplication(appName).Return(&config.Application{
						Name: appName,
					}, nil),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, someError),
				)
			},
			expectedError: fmt.Errorf("get cross-regional resources: some error"),
		},
		"error if failed to generate the template": {
			callMocks: func(m packagePipelineMocks) {
				gomock.InOrder(
					m.ws.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{pipeline}, nil),
					m.ws.EXPECT().ReadPipelineManifest(pipelineManifestPath).Return(mockPipelineManifest, nil),
					m.ws.EXPECT().Rel(pipelineManifestPath).Return(relativePath, nil),
					m.actionCmd.EXPECT().Execute().Times(2),

					// convertStages
					m.store.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.store.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					m.store.EXPECT().GetApplication(appName).Return(&config.Application{
						Name: appName,
					}, nil),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

					// check if the pipeline has been deployed using a legacy naming.
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.ws.EXPECT().PipelineOverridesPath(pipelineName).Return("path"),
					m.pipelineStackConfig.EXPECT().Template().Return("", someError),
				)

			},
			expectedError: fmt.Errorf("generate stack template: some error"),
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
			mockPipelineStackConfig := mocks.NewMockstackConfiguration(ctrl)

			mocks := packagePipelineMocks{
				store:                  mockStore,
				prompt:                 mockPrompt,
				prog:                   mockProgress,
				deployer:               mockPipelineDeployer,
				ws:                     mockWorkspace,
				actionCmd:              mockActionCmd,
				pipelineStackConfig:    mockPipelineStackConfig,
				deployedPipelineLister: mocks.NewMockdeployedPipelineLister(ctrl),
			}

			tc.callMocks(mocks)

			opts := &packagePipelineOpts{
				packagePipelineVars: packagePipelineVars{
					appName: appName,
					name:    pipelineName,
				},
				pipelineDeployer: mockPipelineDeployer,
				pipelineStackConfig: func(in *deploy.CreatePipelineInput) deploycfn.StackConfiguration {
					return mockPipelineStackConfig
				},
				ws:    mockWorkspace,
				store: mockStore,
				newSvcListCmd: func(w io.Writer, app string) cmd {
					return mockActionCmd
				},
				newJobListCmd: func(w io.Writer, app string) cmd {
					return mockActionCmd
				},
				configureDeployedPipelineLister: func() deployedPipelineLister {
					return mocks.deployedPipelineLister
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
