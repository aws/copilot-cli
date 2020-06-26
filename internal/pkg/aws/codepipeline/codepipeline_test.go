// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package codepipeline

import (
	"errors"
	"fmt"
	"testing"
	"time"

	cpmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/codepipeline/mocks"
	rgmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/resourcegroups/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/mock/gomock"

	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/stretchr/testify/require"
)

type codepipelineMocks struct {
	cp *cpmocks.Mockapi
	rg *rgmocks.MockResourceGroupsClient
}

func TestCodePipeline_GetPipeline(t *testing.T) {
	mockPipelineName := "pipeline-dinder-badgoose-repo"
	mockError := errors.New("mockError")
	mockTime := time.Now()
	mockArn := "arn:aws:codepipeline:us-west-2:1234567890:pipeline-dinder-badgoose-repo"
	mockSourceStage := &codepipeline.StageDeclaration{
		Name: aws.String("Source"),
		Actions: []*codepipeline.ActionDeclaration{
			{
				ActionTypeId: &codepipeline.ActionTypeId{
					Category: aws.String("Source"),
					Owner:    aws.String("ThirdParty"),
					Provider: aws.String("GitHub"),
					Version:  aws.String("1"),
				},
				Configuration: map[string]*string{
					"Owner":      aws.String("badgoose"),
					"Repo":       aws.String("repo"),
					"Branch":     aws.String("master"),
					"OAuthToken": aws.String("****"),
				},
				Name: aws.String("SourceCodeFor-dinder"),
				OutputArtifacts: []*codepipeline.OutputArtifact{
					{
						Name: aws.String("SCCheckoutArtifact"),
					},
				},
				RunOrder: aws.Int64(1),
			},
		},
	}
	mockBuildStage := &codepipeline.StageDeclaration{
		Name: aws.String("Build"),
		Actions: []*codepipeline.ActionDeclaration{
			{
				ActionTypeId: &codepipeline.ActionTypeId{
					Category: aws.String("Build"),
					Owner:    aws.String("AWS"),
					Provider: aws.String("CodeBuild"),
					Version:  aws.String("1"),
				},
				Configuration: map[string]*string{
					"ProjectName": aws.String("pipeline-dinder-badgoose-repo-BuildProject"),
				},
				InputArtifacts: []*codepipeline.InputArtifact{
					{
						Name: aws.String("SCCheckoutArtifact"),
					},
				},
				Name: aws.String("Build"),
				OutputArtifacts: []*codepipeline.OutputArtifact{
					{
						Name: aws.String("BuildOutput"),
					},
				},
				RunOrder: aws.Int64(1),
			},
		},
	}
	mockTestStage := &codepipeline.StageDeclaration{
		Name: aws.String("DeployTo-test"),
		Actions: []*codepipeline.ActionDeclaration{
			{
				ActionTypeId: &codepipeline.ActionTypeId{
					Category: aws.String("Deploy"),
					Owner:    aws.String("AWS"),
					Provider: aws.String("CloudFormation"),
					Version:  aws.String("1"),
				},
				Configuration: map[string]*string{
					"TemplatePath":          aws.String("BuildOutput::infrastructure/test.stack.yml"),
					"ActionMode":            aws.String("CREATE_UPDATE"),
					"Capabilities":          aws.String("CAPABILITY_NAMED_IAM"),
					"ChangeSetName":         aws.String("dinder-test-test"),
					"RoleArn":               aws.String("arn:aws:iam::1234567890:role/trivia-test-CFNExecutionRole"),
					"StackName":             aws.String("dinder-test-test"),
					"TemplateConfiguration": aws.String("BuildOutput::infrastructure/test-test.params.json"),
				},
				InputArtifacts: []*codepipeline.InputArtifact{
					{Name: aws.String("BuildOutput")},
				},
				Name:     aws.String("CreateOrUpdate-test-test"),
				Region:   aws.String("us-west-2"),
				RoleArn:  aws.String("arn:aws:iam::12344567890:role/dinder-test-EnvManagerRole"),
				RunOrder: aws.Int64(2),
			},
		},
	}
	mockStages := []*codepipeline.StageDeclaration{mockSourceStage, mockBuildStage, mockTestStage}

	mockStageWithNoAction := &codepipeline.StageDeclaration{
		Name:    aws.String("DummyStage"),
		Actions: []*codepipeline.ActionDeclaration{},
	}
	mockOutput := &codepipeline.GetPipelineOutput{
		Pipeline: &codepipeline.PipelineDeclaration{
			Name:   aws.String(mockPipelineName),
			Stages: mockStages,
		},
		Metadata: &codepipeline.PipelineMetadata{
			Created:     &mockTime,
			Updated:     &mockTime,
			PipelineArn: aws.String(mockArn),
		},
	}

	tests := map[string]struct {
		inPipelineName string
		callMocks      func(m codepipelineMocks)

		expectedOut   *Pipeline
		expectedError error
	}{
		"happy path": {
			inPipelineName: mockPipelineName,
			callMocks: func(m codepipelineMocks) {
				m.cp.EXPECT().GetPipeline(&codepipeline.GetPipelineInput{
					Name: aws.String(mockPipelineName),
				}).Return(mockOutput, nil)

			},
			expectedOut: &Pipeline{
				Name:      mockPipelineName,
				Region:    "us-west-2",
				AccountID: "1234567890",
				Stages: []*Stage{
					{
						Name:     "Source",
						Category: "Source",
						Provider: "GitHub",
						Details:  "Repository: badgoose/repo",
					},
					{
						Name:     "Build",
						Category: "Build",
						Provider: "CodeBuild",
						Details:  "BuildProject: pipeline-dinder-badgoose-repo-BuildProject",
					},
					{
						Name:     "DeployTo-test",
						Category: "Deploy",
						Provider: "CloudFormation",
						Details:  "StackName: dinder-test-test",
					},
				},
				CreatedAt: mockTime,
				UpdatedAt: mockTime,
			},
			expectedError: nil,
		},
		"should only populate stage name if stage has no actions": {
			inPipelineName: mockPipelineName,
			callMocks: func(m codepipelineMocks) {
				m.cp.EXPECT().GetPipeline(&codepipeline.GetPipelineInput{
					Name: aws.String(mockPipelineName),
				}).Return(
					&codepipeline.GetPipelineOutput{
						Pipeline: &codepipeline.PipelineDeclaration{
							Name:   aws.String(mockPipelineName),
							Stages: []*codepipeline.StageDeclaration{mockSourceStage, mockStageWithNoAction},
						},
						Metadata: &codepipeline.PipelineMetadata{
							Created:     &mockTime,
							Updated:     &mockTime,
							PipelineArn: aws.String(mockArn),
						},
					}, nil)

			},
			expectedOut: &Pipeline{
				Name:      mockPipelineName,
				Region:    "us-west-2",
				AccountID: "1234567890",
				Stages: []*Stage{
					{
						Name:     "Source",
						Category: "Source",
						Provider: "GitHub",
						Details:  "Repository: badgoose/repo",
					},
					{
						Name:     "DummyStage",
						Category: "",
						Provider: "",
						Details:  "",
					},
				},
				CreatedAt: mockTime,
				UpdatedAt: mockTime,
			},
			expectedError: nil,
		},
		"should wrap error from codepipeline client": {
			inPipelineName: mockPipelineName,
			callMocks: func(m codepipelineMocks) {
				m.cp.EXPECT().GetPipeline(&codepipeline.GetPipelineInput{
					Name: aws.String(mockPipelineName),
				}).Return(nil, mockError)

			},
			expectedOut:   nil,
			expectedError: fmt.Errorf("get pipeline %s: %w", mockPipelineName, mockError),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := cpmocks.NewMockapi(ctrl)
			mockrgClient := rgmocks.NewMockResourceGroupsClient(ctrl)
			mocks := codepipelineMocks{
				cp: mockClient,
				rg: mockrgClient,
			}
			tc.callMocks(mocks)

			cp := CodePipeline{
				client:   mockClient,
				rgClient: mockrgClient,
			}

			// WHEN
			actualOut, err := cp.GetPipeline(tc.inPipelineName)

			// THEN
			require.Equal(t, tc.expectedError, err)
			require.Equal(t, tc.expectedOut, actualOut)
		})
	}
}

func TestCodePipeline_ListPipelinesForProject(t *testing.T) {
	mockProjectName := "dinder"
	mockPipelineName := "pipeline-dinder-badgoose-repo"
	mockError := errors.New("mockError")
	mockOutput := []string{
		"arn:aws:codepipeline:us-west-2:1234567890:" + mockPipelineName,
	}
	testTags := map[string]string{
		"copilot-application": mockProjectName,
	}
	badArn := "badArn"

	tests := map[string]struct {
		inProjectName string
		callMocks     func(m codepipelineMocks)
		expectedOut   []string

		expectedError error
	}{
		"happy path": {
			inProjectName: mockProjectName,
			callMocks: func(m codepipelineMocks) {
				m.rg.EXPECT().GetResourcesByTags(pipelineResourceType, testTags).Return(mockOutput, nil)
			},
			expectedOut:   []string{mockPipelineName},
			expectedError: nil,
		},
		"should return error from resourcegroups client": {
			inProjectName: mockProjectName,
			callMocks: func(m codepipelineMocks) {
				m.rg.EXPECT().GetResourcesByTags(pipelineResourceType, testTags).Return(nil, mockError)
			},
			expectedOut:   nil,
			expectedError: mockError,
		},
		"should return error for bad arns": {
			inProjectName: mockProjectName,
			callMocks: func(m codepipelineMocks) {
				m.rg.EXPECT().GetResourcesByTags(pipelineResourceType, testTags).Return([]string{badArn}, nil)
			},
			expectedOut:   nil,
			expectedError: fmt.Errorf("parse pipeline ARN: %s", badArn),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := cpmocks.NewMockapi(ctrl)
			mockrgClient := rgmocks.NewMockResourceGroupsClient(ctrl)
			mocks := codepipelineMocks{
				cp: mockClient,
				rg: mockrgClient,
			}
			tc.callMocks(mocks)

			cp := CodePipeline{
				client:   mockClient,
				rgClient: mockrgClient,
			}

			// WHEN
			actualOut, actualErr := cp.ListPipelineNamesByTags(testTags)

			// THEN
			if actualErr != nil {
				require.EqualError(t, tc.expectedError, actualErr.Error())
			} else {
				require.Equal(t, tc.expectedOut, actualOut)
			}
		})
	}
}

func TestCodePipeline_GetPipelineState(t *testing.T) {
	mockPipelineName := "pipeline-dinder-badgoose-repo"
	mockTime := time.Now()
	mockOutput := &codepipeline.GetPipelineStateOutput{
		PipelineName: aws.String(mockPipelineName),
		StageStates: []*codepipeline.StageState{
			{
				ActionStates: []*codepipeline.ActionState{
					{
						ActionName:      aws.String("action1"),
						LatestExecution: &codepipeline.ActionExecution{Status: aws.String(codepipeline.ActionExecutionStatusSucceeded)},
					},
					{
						ActionName:      aws.String("action2"),
						LatestExecution: &codepipeline.ActionExecution{Status: aws.String(codepipeline.ActionExecutionStatusSucceeded)},
					},
				},
				StageName: aws.String("Source"),
			},
			{
				InboundTransitionState: &codepipeline.TransitionState{Enabled: aws.Bool(true)},
				ActionStates: []*codepipeline.ActionState{
					{
						ActionName:      aws.String("action1"),
						LatestExecution: &codepipeline.ActionExecution{Status: aws.String(codepipeline.ActionExecutionStatusFailed)},
					},
					{
						ActionName:      aws.String("action2"),
						LatestExecution: &codepipeline.ActionExecution{Status: aws.String(codepipeline.ActionExecutionStatusInProgress)},
					},
					{
						ActionName:      aws.String("action3"),
						LatestExecution: &codepipeline.ActionExecution{Status: aws.String(codepipeline.ActionExecutionStatusSucceeded)},
					},
				},
				StageName: aws.String("Build"),
			},
			{
				InboundTransitionState: &codepipeline.TransitionState{Enabled: aws.Bool(true)},
				ActionStates: []*codepipeline.ActionState{
					{
						ActionName:      aws.String("action1"),
						LatestExecution: &codepipeline.ActionExecution{Status: aws.String(codepipeline.ActionExecutionStatusFailed)},
					},
				},
				StageName: aws.String("DeployTo-test"),
			},
			{
				InboundTransitionState: &codepipeline.TransitionState{Enabled: aws.Bool(false)},
				StageName:              aws.String("DeployTo-prod"),
			},
		},
		Updated: &mockTime,
	}
	mockError := errors.New("mockError")

	tests := map[string]struct {
		inPipelineName string
		callMocks      func(m codepipelineMocks)

		expectedOut   *PipelineState
		expectedError error
	}{
		"happy path": {
			inPipelineName: mockPipelineName,
			callMocks: func(m codepipelineMocks) {
				m.cp.EXPECT().GetPipelineState(&codepipeline.GetPipelineStateInput{
					Name: aws.String(mockPipelineName),
				}).Return(mockOutput, nil)

			},
			expectedOut: &PipelineState{
				PipelineName: mockPipelineName,
				StageStates: []*StageState{
					{
						StageName: "Source",
						Actions: []StageAction{
							{
								Name:   "action1",
								Status: "Succeeded",
							},
							{
								Name:   "action2",
								Status: "Succeeded",
							},
						},
						Transition: "",
					},
					{
						StageName: "Build",
						Actions: []StageAction{
							{
								Name:   "action1",
								Status: "Failed",
							},
							{
								Name:   "action2",
								Status: "InProgress",
							},
							{
								Name:   "action3",
								Status: "Succeeded",
							},
						},
						Transition: "ENABLED",
					},
					{
						StageName: "DeployTo-test",
						Actions: []StageAction{
							{
								Name:   "action1",
								Status: "Failed",
							},
						},
						Transition: "ENABLED",
					},
					{
						StageName:  "DeployTo-prod",
						Transition: "DISABLED",
					},
				},
				UpdatedAt: mockTime,
			},
			expectedError: nil,
		},
		"should wrap error from CodePipeline client": {
			inPipelineName: mockPipelineName,
			callMocks: func(m codepipelineMocks) {
				m.cp.EXPECT().GetPipelineState(&codepipeline.GetPipelineStateInput{
					Name: aws.String(mockPipelineName),
				}).Return(nil, mockError)

			},
			expectedOut:   nil,
			expectedError: fmt.Errorf("get pipeline state %s: %w", mockPipelineName, mockError),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := cpmocks.NewMockapi(ctrl)

			mocks := codepipelineMocks{
				cp: mockClient,
			}
			tc.callMocks(mocks)

			cp := CodePipeline{
				client: mockClient,
			}

			// WHEN
			actualOut, err := cp.GetPipelineState(tc.inPipelineName)

			// THEN
			require.Equal(t, tc.expectedError, err)
			require.Equal(t, tc.expectedOut, actualOut)
		})
	}
}
