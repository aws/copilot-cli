// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestUpdatePipelineOpts_convertStages(t *testing.T) {
	testCases := map[string]struct {
		stages    []manifest.PipelineStage
		inAppName string

		mockWorkspace func(m *mocks.MockwsPipelineReader)
		mockEnvStore  func(m *mocks.MockenvironmentStore)

		expectedStages []deploy.PipelineStage
		expectedError  error
	}{
		"converts stages": {
			stages: []manifest.PipelineStage{
				{
					Name: "test",
				},
			},
			inAppName: "badgoose",
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				mockEnv := &config.Environment{
					Name:      "test",
					App:       "badgoose",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Prod:      false,
				}

				m.EXPECT().GetEnvironment("badgoose", "test").Return(mockEnv, nil).Times(1)
			},

			expectedStages: []deploy.PipelineStage{
				{
					AssociatedEnvironment: &deploy.AssociatedEnvironment{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789012",
						Prod:      false,
					},
					LocalServices: []string{"frontend", "backend"},
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

			tc.mockEnvStore(mockEnvStore)
			tc.mockWorkspace(mockWorkspace)

			opts := &updatePipelineOpts{
				updatePipelineVars: updatePipelineVars{
					GlobalOpts: &GlobalOpts{appName: tc.inAppName},
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
				require.Nil(t, err)
				require.ElementsMatch(t, tc.expectedStages, actualStages)
			}
		})
	}

}

func TestUpdatePipelineOpts_getArtifactBuckets(t *testing.T) {
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

			opts := &updatePipelineOpts{
				pipelineDeployer: mockPipelineDeployer,
			}

			// WHEN
			actual, err := opts.getArtifactBuckets()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.Nil(t, err)
				require.ElementsMatch(t, tc.expectedOut, actual)
			}
		})
	}
}

func TestUpdatePipelineOpts_Execute(t *testing.T) {
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
    branch: master

stages:
    -
      name: chicken
    -
      name: wings
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
		mockDeployer   func(m *mocks.MockpipelineDeployer)
		mockWorkspace  func(m *mocks.MockwsPipelineReader)
		mockEnvStore   func(m *mocks.MockenvironmentStore)
		mockProgress   func(m *mocks.Mockprogress)
		mockPrompt     func(m *mocks.Mockprompter)
		expectedError  error
	}{
		"create and deploy pipeline": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				m.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateStart, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateComplete, pipelineName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateProposalStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateProposalFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateProposalComplete, pipelineName)).Times(0)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExists(gomock.Any()).Return(false, nil)
				m.EXPECT().CreatePipeline(gomock.Any()).Return(nil)
			},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			expectedError: nil,
		},
		"update and deploy pipeline": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				m.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateComplete, pipelineName)).Times(0)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateProposalStart, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateProposalFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateProposalComplete, pipelineName)).Times(1)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExists(gomock.Any()).Return(true, nil)
				m.EXPECT().UpdatePipeline(gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(fmt.Sprintf(fmtPipelineUpdateExistPrompt, pipelineName), "").Return(true, nil)
			},
			expectedError: nil,
		},
		"do not deploy pipeline if decline to update an existing pipeline": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				m.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateComplete, pipelineName)).Times(0)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateProposalStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateProposalFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateProposalComplete, pipelineName)).Times(0)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExists(gomock.Any()).Return(true, nil)
				m.EXPECT().UpdatePipeline(gomock.Any()).Return(nil).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(fmt.Sprintf(fmtPipelineUpdateExistPrompt, pipelineName), "").Return(false, nil)
			},
			expectedError: nil,
		},
		"returns an error if fails to prompt for pipeline update": {
			inApp:     &app,
			inAppName: appName,
			inRegion:  region,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				m.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExists(gomock.Any()).Return(true, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(fmt.Sprintf(fmtPipelineUpdateExistPrompt, pipelineName), "").Return(false, errors.New("some error"))
			},
			expectedError: fmt.Errorf("prompt for pipeline update: some error"),
		},
		"returns an error if fail to add pipeline resources to app": {
			inApp:         &app,
			inRegion:      region,
			inAppName:     appName,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {},
			mockEnvStore:  func(m *mocks.MockenvironmentStore) {},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(1)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(0)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(errors.New("some error"))
			},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			expectedError: fmt.Errorf("add pipeline resources to application %s in %s: some error", appName, region),
		},
		"returns an error if fail to read pipeline file": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), errors.New("some error"))
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
			},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			expectedError: fmt.Errorf("read pipeline manifest: some error"),
		},
		"returns an error if unable to unmarshal pipeline file": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				content := ""
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
			},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			expectedError: fmt.Errorf("unmarshal pipeline manifest: pipeline.yml contains invalid schema version: 0"),
		},
		"returns an error if unable to convert environments to deployment stage": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return(nil, errors.New("some error")).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
			},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			expectedError: fmt.Errorf("convert environments to deployment stage: service names from workspace: some error"),
		},
		"returns an error if fails to get cross-regional resources": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				m.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, errors.New("some error"))
			},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			expectedError: fmt.Errorf("get cross-regional resources: some error"),
		},
		"returns an error if fails to check if pipeline exists": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				m.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExists(gomock.Any()).Return(false, errors.New("some error"))
			},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			expectedError: fmt.Errorf("check if pipeline exists: some error"),
		},
		"returns an error if fails to create pipeline": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				m.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateStart, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateFailed, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateComplete, pipelineName)).Times(0)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateProposalStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateProposalFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateProposalComplete, pipelineName)).Times(0)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExists(gomock.Any()).Return(false, nil)
				m.EXPECT().CreatePipeline(gomock.Any()).Return(errors.New("some error"))
			},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			expectedError: fmt.Errorf("create pipeline: some error"),
		},
		"returns an error if fails to update pipeline": {
			inApp:     &app,
			inRegion:  region,
			inAppName: appName,
			mockWorkspace: func(m *mocks.MockwsPipelineReader) {
				m.EXPECT().ReadPipelineManifest().Return([]byte(content), nil)
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil).Times(1)
			},
			mockEnvStore: func(m *mocks.MockenvironmentStore) {
				m.EXPECT().GetEnvironment(appName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(appName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, appName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, appName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, appName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateComplete, pipelineName)).Times(0)
				m.EXPECT().Start(fmt.Sprintf(fmtPipelineUpdateProposalStart, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtPipelineUpdateProposalFailed, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Ssuccessf(fmtPipelineUpdateProposalComplete, pipelineName)).Times(0)
			},
			mockDeployer: func(m *mocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToApp(&app, region).Return(nil)
				m.EXPECT().GetRegionalAppResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExists(gomock.Any()).Return(true, nil)
				m.EXPECT().UpdatePipeline(gomock.Any()).Return(errors.New("some error"))
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(fmt.Sprintf(fmtPipelineUpdateExistPrompt, pipelineName), "").Return(true, nil)
			},
			expectedError: fmt.Errorf("update pipeline: some error"),
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
			tc.mockDeployer(mockPipelineDeployer)
			tc.mockEnvStore(mockEnvStore)
			tc.mockWorkspace(mockWorkspace)
			tc.mockProgress(mockProgress)
			tc.mockPrompt(mockPrompt)

			opts := &updatePipelineOpts{
				updatePipelineVars: updatePipelineVars{
					PipelineName: tc.inPipelineName,
					GlobalOpts: &GlobalOpts{
						appName: tc.inAppName,
						prompt:  mockPrompt,
					},
				},
				pipelineDeployer: mockPipelineDeployer,
				ws:               mockWorkspace,
				app:              tc.inApp,
				region:           tc.inRegion,
				envStore:         mockEnvStore,
				prog:             mockProgress,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
