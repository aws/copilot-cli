// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deployPipelineMocks struct {
	envStore *mocks.MockenvironmentStore
	prompt   *mocks.Mockprompter
	prog     *mocks.Mockprogress
	deployer *mocks.MockpipelineDeployer
	ws       *mocks.MockwsPipelineReader
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
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment("badgoose", "test").Return(mockEnv, nil).Times(1),
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
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment("badgoose", "test").Return(mockEnv, nil).Times(1),
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
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment("badgoose", "test").Return(mockEnv, nil).Times(1),
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

			mockEnvStore := mocks.NewMockenvironmentStore(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mocks := deployPipelineMocks{
				envStore: mockEnvStore,
				ws:       mockWorkspace,
			}

			tc.callMocks(mocks)

			opts := &deployPipelineOpts{
				deployPipelineVars: deployPipelineVars{
					appName: tc.inAppName,
				},
				envStore: mockEnvStore,
				ws:       mockWorkspace,
			}

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

func TestDeployPipelineOpts_Execute(t *testing.T) {
	const (
		appName      = "badgoose"
		region       = "us-west-2"
		accountID    = "123456789012"
		pipelineName = "pipepiper"
		content      = `
name: pipepiper
version: 1

source:
  provider: GitHub
  properties:
    repository: aws/somethingCool
    access_token_secret: "github-token-badgoose-backend"
    branch: main

stages:
    -
      name: chicken
      test_commands:
        - make test
        - echo "made test"
    -
      name: wings
      test_commands:
        - echo "bok bok bok"
`
	)

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
		Prod:      false,
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

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

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
		"update and deploy pipeline": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				gomock.InOrder(
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

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
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

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

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

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

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), errors.New("some error")),
				)
			},
			expectedError: fmt.Errorf("read pipeline manifest: some error"),
		},
		"returns an error if unable to unmarshal pipeline file": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			callMocks: func(m deployPipelineMocks) {
				content := ""
				gomock.InOrder(
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
				)
			},
			expectedError: fmt.Errorf("unmarshal pipeline manifest: pipeline.yml contains invalid schema version: 0"),
		},
		"returns an error if pipeline name exceeds 100 characters": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				content := `
name: 12345678101234567820123456783012345678401234567850123456786012345678701234567880123456789012345671001
version: 1
`
				gomock.InOrder(
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
				)
			},
			expectedError: fmt.Errorf("pipeline name '12345678101234567820123456783012345678401234567850123456786012345678701234567880123456789012345671001' must be shorter than 100 characters"),
		},
		"returns an error if provider is not a supported type": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			callMocks: func(m deployPipelineMocks) {
				content := `
name: pipepiper
version: 1

source:
  provider: NotGitHub
  properties:
    repository: aws/somethingCool
    access_token_secret: "github-token-badgoose-backend"
    branch: main
`
				gomock.InOrder(
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),
					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
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

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return(nil, errors.New("some error")).Times(1),
				)
			},
			expectedError: fmt.Errorf("convert environments to deployment stage: get workload names from workspace: some error"),
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

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

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

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

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

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

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

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

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
				content := `
name: pipepiper
version: 1

source:
  provider: GitHub
  properties:
    repository: aws/somethingCool
    access_token_secret: "github-token-badgoose-backend"
    branch: main

build:
  image: aws/codebuild/standard:3.0

stages:
    -
      name: chicken
      test_commands:
        - make test
        - echo "made test"
    -
      name: wings
      test_commands:
        - echo "bok bok bok"
`
				gomock.InOrder(
					m.prog.EXPECT().Start(fmt.Sprintf(fmtPipelineDeployResourcesStart, appName)).Times(1),
					m.deployer.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil),
					m.prog.EXPECT().Stop(log.Ssuccessf(fmtPipelineDeployResourcesComplete, appName)).Times(1),

					m.ws.EXPECT().ReadPipelineManifest().Return([]byte(content), nil),
					m.ws.EXPECT().ListWorkloads().Return([]string{"frontend", "backend"}, nil).Times(1),

					// convertStages
					m.envStore.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1),
					m.envStore.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1),

					// getArtifactBuckets
					m.deployer.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil),

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
			mockEnvStore := mocks.NewMockenvironmentStore(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)

			mocks := deployPipelineMocks{
				envStore: mockEnvStore,
				prompt:   mockPrompt,
				prog:     mockProgress,
				deployer: mockPipelineDeployer,
				ws:       mockWorkspace,
			}

			tc.callMocks(mocks)

			opts := &deployPipelineOpts{
				deployPipelineVars: deployPipelineVars{
					appName: tc.inAppName,
				},
				pipelineDeployer: mockPipelineDeployer,
				ws:               mockWorkspace,
				app:              tc.inApp,
				region:           tc.inRegion,
				envStore:         mockEnvStore,
				prog:             mockProgress,
				prompt:           mockPrompt,

				pipelineName: tc.inPipelineName,
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
