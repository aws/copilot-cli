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

type packagePipelineMocks struct {
	store               *mocks.Mockstore
	prompt              *mocks.Mockprompter
	prog                *mocks.Mockprogress
	deployer            *mocks.MockpipelineDeployer
	pipelineStackConfig *mocks.MockpipelineStackConfig
	mockDiffWriter      *strings.Builder
	ws                  *mocks.MockwsPipelineReader
	actionCmd           *mocks.MockactionCommand
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
		inShowDiff     bool
	}{
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
		"error if failed to generate the template": {
			inApp:      &app,
			inAppName:  appName,
			inRegion:   region,
			inShowDiff: true,
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
					m.deployedPipelineLister.EXPECT().ListDeployedPipelines(appName).Return([]deploy.Pipeline{}, nil),
					m.pipelineStackConfig.EXPECT().Template().Return("", errors.New("some error")),
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
			mockPipelineStackConfig := mocks.NewMockpipelineStackConfig(ctrl)

			mocks := deployPipelineMocks{
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

			opts := &deployPipelineOpts{
				deployPipelineVars: deployPipelineVars{
					appName:  tc.inAppName,
					name:     tc.inPipelineName,
					showDiff: tc.inShowDiff,
				},
				pipelineDeployer: mockPipelineDeployer,
				pipelineStackConfig: func(in *deploy.CreatePipelineInput) pipelineStackConfig {
					return mockPipelineStackConfig
				},
				ws:         mockWorkspace,
				app:        tc.inApp,
				region:     tc.inRegion,
				store:      mockStore,
				prog:       mockProgress,
				prompt:     mockPrompt,
				diffWriter: &strings.Builder{},
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
				require.Error(t, err)
				require.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
