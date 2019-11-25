// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	archermocks "github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestUpdatePipelineOpts_convertStages(t *testing.T) {
	testCases := map[string]struct {
		stages        []manifest.PipelineStage
		inProjectName string

		mockWorkspace func(m *archermocks.MockWorkspace)
		mockEnvStore  func(m *archermocks.MockEnvironmentStore)

		expectedStages []deploy.PipelineStage
		expectedError  error
	}{
		"converts stages": {
			stages: []manifest.PipelineStage{
				{
					Name: "test",
				},
			},
			inProjectName: "badgoose",
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return([]archer.Manifest{
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "frontend",
						},
					},
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "backend",
						},
					}}, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				mockEnv := &archer.Environment{
					Name:      "test",
					Project:   "badgoose",
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
					LocalApplications: []string{"frontend", "backend"},
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

			mockEnvStore := archermocks.NewMockEnvironmentStore(ctrl)
			mockWorkspace := archermocks.NewMockWorkspace(ctrl)

			tc.mockEnvStore(mockEnvStore)
			tc.mockWorkspace(mockWorkspace)

			opts := &UpdatePipelineOpts{
				envStore: mockEnvStore,
				ws:       mockWorkspace,

				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
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
		mockDeployer func(m *climocks.MockpipelineDeployer)

		expectedOut []deploy.ArtifactBucket

		expectedError error
	}{
		"getsBucketInfo": {
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				mockResources := []*archer.ProjectRegionalResources{
					{
						S3Bucket:  "someBucket",
						KMSKeyARN: "someKey",
					},
				}
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
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

			mockPipelineDeployer := climocks.NewMockpipelineDeployer(ctrl)
			tc.mockDeployer(mockPipelineDeployer)

			opts := &UpdatePipelineOpts{
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
		projectName  = "badgoose"
		region       = "us-west-2"
		accountID    = "123456789012"
		pipelineFile = "pipeline.yml"
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

	project := archer.Project{
		AccountID: accountID,
		Name:      projectName,
		Domain:    "amazon.com",
	}

	mockResources := []*archer.ProjectRegionalResources{
		{
			S3Bucket:  "someBucket",
			KMSKeyARN: "someKey",
		},
	}

	mockEnv := &archer.Environment{
		Name:      "test",
		Project:   projectName,
		Region:    region,
		AccountID: accountID,
		Prod:      false,
	}

	manifest := []archer.Manifest{
		&manifest.LBFargateManifest{
			AppManifest: manifest.AppManifest{
				Name: "frontend",
			},
		},
		&manifest.LBFargateManifest{
			AppManifest: manifest.AppManifest{
				Name: "backend",
			},
		},
	}

	testCases := map[string]struct {
		inProject      *archer.Project
		inProjectName  string
		inPipelineName string
		inRegion       string
		inPipelineFile string
		mockDeployer   func(m *climocks.MockpipelineDeployer)
		mockWorkspace  func(m *archermocks.MockWorkspace)
		mockEnvStore   func(m *archermocks.MockEnvironmentStore)
		mockProgress   func(m *climocks.Mockprogress)
		mockPrompt     func(m *climocks.Mockprompter)
		expectedError  error
	}{
		"create and deploy pipeline": {
			inProject:      &project,
			inProjectName:  projectName,
			inRegion:       region,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(projectName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(projectName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtCreatePipelineStart, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtCreatePipelineFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtCreatePipelineComplete, pipelineName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtUpdatePipelineStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtUpdatePipelineFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtUpdatePipelineComplete, pipelineName)).Times(0)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExist(gomock.Any()).Return(false, nil)
				m.EXPECT().CreatePipeline(gomock.Any()).Return(nil)
			},
			mockPrompt:    func(m *climocks.Mockprompter) {},
			expectedError: nil,
		},
		"update and deploy pipeline": {
			inProject:      &project,
			inProjectName:  projectName,
			inRegion:       region,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(projectName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(projectName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtCreatePipelineStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtCreatePipelineFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtCreatePipelineComplete, pipelineName)).Times(0)
				m.EXPECT().Start(fmt.Sprintf(fmtUpdatePipelineStart, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtUpdatePipelineFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtUpdatePipelineComplete, pipelineName)).Times(1)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExist(gomock.Any()).Return(true, nil)
				m.EXPECT().UpdatePipeline(gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(fmt.Sprintf(fmtUpdateEnvPrompt, pipelineName), "").Return(true, nil)
			},
			expectedError: nil,
		},
		"do not deploy pipeline if decline to update an existing pipeline": {
			inProject:      &project,
			inProjectName:  projectName,
			inRegion:       region,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(projectName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(projectName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtCreatePipelineStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtCreatePipelineFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtCreatePipelineComplete, pipelineName)).Times(0)
				m.EXPECT().Start(fmt.Sprintf(fmtUpdatePipelineStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtUpdatePipelineFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtUpdatePipelineComplete, pipelineName)).Times(0)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExist(gomock.Any()).Return(true, nil)
				m.EXPECT().UpdatePipeline(gomock.Any()).Return(nil).Times(0)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(fmt.Sprintf(fmtUpdateEnvPrompt, pipelineName), "").Return(false, nil)
			},
			expectedError: nil,
		},
		"returns an error if fails to prompt for pipeline update": {
			inProject:      &project,
			inProjectName:  projectName,
			inRegion:       region,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(projectName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(projectName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExist(gomock.Any()).Return(true, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(fmt.Sprintf(fmtUpdateEnvPrompt, pipelineName), "").Return(false, errors.New("some error"))
			},
			expectedError: fmt.Errorf("prompt for pipeline update: some error"),
		},
		"returns an error if fail to add pipeline resources to project": {
			inProject:      &project,
			inRegion:       region,
			inProjectName:  projectName,
			inPipelineFile: pipelineFile,
			mockWorkspace:  func(m *archermocks.MockWorkspace) {},
			mockEnvStore:   func(m *archermocks.MockEnvironmentStore) {},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(1)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(0)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(errors.New("some error"))
			},
			mockPrompt:    func(m *climocks.Mockprompter) {},
			expectedError: fmt.Errorf("add pipeline resources to project %s in %s: some error", projectName, region),
		},
		"returns an error if fail to read pipeline file": {
			inProject:      &project,
			inRegion:       region,
			inProjectName:  projectName,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), errors.New("some error"))
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
			},
			mockPrompt:    func(m *climocks.Mockprompter) {},
			expectedError: fmt.Errorf("read pipeline file %s: some error", pipelineFile),
		},
		"returns an error if unable to unmarshal pipeline file": {
			inProject:      &project,
			inRegion:       region,
			inProjectName:  projectName,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				content := ""
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
			},
			mockPrompt:    func(m *climocks.Mockprompter) {},
			expectedError: fmt.Errorf("unmarshal pipeline file %s: pipeline.yml contains invalid schema version: 0", pipelineFile),
		},
		"returns an error if unable to convert environments to deployment stage": {
			inProject:      &project,
			inRegion:       region,
			inProjectName:  projectName,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, errors.New("some error")).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
			},
			mockPrompt:    func(m *climocks.Mockprompter) {},
			expectedError: fmt.Errorf("convert environments to deployment stage: some error"),
		},
		"returns an error if fails to get cross-regional resources": {
			inProject:      &project,
			inRegion:       region,
			inProjectName:  projectName,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(projectName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(projectName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, errors.New("some error"))
			},
			mockPrompt:    func(m *climocks.Mockprompter) {},
			expectedError: fmt.Errorf("get cross-regional resources: some error"),
		},
		"returns an error if fails to check if pipeline exists": {
			inProject:      &project,
			inRegion:       region,
			inProjectName:  projectName,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(projectName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(projectName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExist(gomock.Any()).Return(false, errors.New("some error"))
			},
			mockPrompt:    func(m *climocks.Mockprompter) {},
			expectedError: fmt.Errorf("check if pipeline exists: some error"),
		},
		"returns an error if fails to create pipeline": {
			inProject:      &project,
			inRegion:       region,
			inProjectName:  projectName,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(projectName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(projectName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtCreatePipelineStart, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtCreatePipelineFailed, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Ssuccessf(fmtCreatePipelineComplete, pipelineName)).Times(0)
				m.EXPECT().Start(fmt.Sprintf(fmtUpdatePipelineStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtUpdatePipelineFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtUpdatePipelineComplete, pipelineName)).Times(0)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExist(gomock.Any()).Return(false, nil)
				m.EXPECT().CreatePipeline(gomock.Any()).Return(errors.New("some error"))
			},
			mockPrompt:    func(m *climocks.Mockprompter) {},
			expectedError: fmt.Errorf("create pipeline: some error"),
		},
		"returns an error if fails to update pipeline": {
			inProject:      &project,
			inRegion:       region,
			inProjectName:  projectName,
			inPipelineFile: pipelineFile,
			mockWorkspace: func(m *archermocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Return([]byte(content), nil)
				m.EXPECT().Apps().Return(manifest, nil).Times(1)
			},
			mockEnvStore: func(m *archermocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(projectName, "chicken").Return(mockEnv, nil).Times(1)
				m.EXPECT().GetEnvironment(projectName, "wings").Return(mockEnv, nil).Times(1)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddPipelineResourcesStart, projectName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtAddPipelineResourcesFailed, projectName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, projectName)).Times(1)
				m.EXPECT().Start(fmt.Sprintf(fmtCreatePipelineStart, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Serrorf(fmtCreatePipelineFailed, pipelineName)).Times(0)
				m.EXPECT().Stop(log.Ssuccessf(fmtCreatePipelineComplete, pipelineName)).Times(0)
				m.EXPECT().Start(fmt.Sprintf(fmtUpdatePipelineStart, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Serrorf(fmtUpdatePipelineFailed, pipelineName)).Times(1)
				m.EXPECT().Stop(log.Ssuccessf(fmtUpdatePipelineComplete, pipelineName)).Times(0)
			},
			mockDeployer: func(m *climocks.MockpipelineDeployer) {
				m.EXPECT().AddPipelineResourcesToProject(&project, region).Return(nil)
				m.EXPECT().GetRegionalProjectResources(gomock.Any()).Return(mockResources, nil)
				m.EXPECT().PipelineExist(gomock.Any()).Return(true, nil)
				m.EXPECT().UpdatePipeline(gomock.Any()).Return(errors.New("some error"))
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(fmt.Sprintf(fmtUpdateEnvPrompt, pipelineName), "").Return(true, nil)
			},
			expectedError: fmt.Errorf("update pipeline: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPipelineDeployer := climocks.NewMockpipelineDeployer(ctrl)
			mockEnvStore := archermocks.NewMockEnvironmentStore(ctrl)
			mockWorkspace := archermocks.NewMockWorkspace(ctrl)
			mockProgress := climocks.NewMockprogress(ctrl)
			mockPrompt := climocks.NewMockprompter(ctrl)
			tc.mockDeployer(mockPipelineDeployer)
			tc.mockEnvStore(mockEnvStore)
			tc.mockWorkspace(mockWorkspace)
			tc.mockProgress(mockProgress)
			tc.mockPrompt(mockPrompt)

			opts := &UpdatePipelineOpts{
				PipelineFile:     tc.inPipelineFile,
				PipelineName:     tc.inPipelineName,
				pipelineDeployer: mockPipelineDeployer,
				ws:               mockWorkspace,
				project:          tc.inProject,
				region:           tc.inRegion,
				envStore:         mockEnvStore,
				prog:             mockProgress,
				GlobalOpts: &GlobalOpts{
					projectName: tc.inProjectName,
					prompt:      mockPrompt,
				},
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, err.Error(), tc.expectedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
