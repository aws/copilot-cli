// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/dustin/go-humanize"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type pipelineDescriberMocks struct {
	cfn            *mocks.MockstackDescriber
	pipelineGetter *mocks.MockpipelineGetter
}

var pipelineResourceName = "pipeline-dinder-badgoose-repo-RANDOMSTRING"
var pipelineName = "pipeline-dinder-badgoose-repo"
var mockTime = func() time.Time {
	t, _ := time.Parse(time.RFC3339, "2020-02-02T15:04:05+00:00")
	return t
}
var mockPipeline = &codepipeline.Pipeline{
	Name:      pipelineResourceName,
	Region:    "us-west-2",
	AccountID: "1234567890",
	Stages: []*codepipeline.Stage{
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
	CreatedAt: mockTime(),
	UpdatedAt: mockTime(),
}
var expectedResources = []*stack.Resource{
	{
		PhysicalID: "pipeline-dinder-badgoose-repo-BuildProject",
		Type:       "AWS::CodeBuild::Project",
	},
	{
		PhysicalID: "pipel-Buil-1PEASDDL44ID2",
		Type:       "AWS::IAM::Policy",
	},
	{
		PhysicalID: "pipeline-dinder-badgoose-repo-BuildProjectRole-A4V6VSG1XIIJ",
		Type:       "AWS::IAM::Role",
	},
	{
		PhysicalID: "pipeline-dinder-badgoose-repo",
		Type:       "AWS::CodePipeline::Pipeline",
	},
	{
		PhysicalID: "pipeline-dinder-badgoose-repo-PipelineRole-100SEEQN6CU0F",
		Type:       "AWS::IAM::Role",
	},
	{
		PhysicalID: "pipel-Pipe-EO4QGE10RJ8F",
		Type:       "AWS::IAM::Policy",
	},
}

func TestPipelineDescriber_Describe(t *testing.T) {
	mockResources := []*stack.Resource{
		{
			PhysicalID: "pipeline-dinder-badgoose-repo-BuildProject",
			Type:       "AWS::CodeBuild::Project",
		},
		{
			PhysicalID: "pipel-Buil-1PEASDDL44ID2",
			Type:       "AWS::IAM::Policy",
		},
		{
			PhysicalID: "pipeline-dinder-badgoose-repo-BuildProjectRole-A4V6VSG1XIIJ",
			Type:       "AWS::IAM::Role",
		},
		{
			PhysicalID: "pipeline-dinder-badgoose-repo",
			Type:       "AWS::CodePipeline::Pipeline",
		},
		{
			PhysicalID: "pipeline-dinder-badgoose-repo-PipelineRole-100SEEQN6CU0F",
			Type:       "AWS::IAM::Role",
		},
		{
			PhysicalID: "pipel-Pipe-EO4QGE10RJ8F",
			Type:       "AWS::IAM::Policy",
		},
	}
	mockError := errors.New("mockError")

	testCases := map[string]struct {
		callMocks      func(m pipelineDescriberMocks)
		inShowResource bool

		expectedError  error
		expectedOutput *Pipeline
	}{
		"happy path with resources": {
			callMocks: func(m pipelineDescriberMocks) {
				m.pipelineGetter.EXPECT().GetPipeline(pipelineResourceName).Return(mockPipeline, nil)
				m.cfn.EXPECT().Resources().Return(mockResources, nil)
			},
			inShowResource: true,
			expectedError:  nil,
			expectedOutput: &Pipeline{
				Name:      pipelineName,
				Pipeline:  *mockPipeline,
				Resources: expectedResources,
			},
		},
		"happy path without resources": {
			callMocks: func(m pipelineDescriberMocks) {
				m.pipelineGetter.EXPECT().GetPipeline(pipelineResourceName).Return(mockPipeline, nil)
			},
			inShowResource: false,
			expectedError:  nil,
			expectedOutput: &Pipeline{
				Name:      pipelineName,
				Pipeline:  *mockPipeline,
				Resources: nil,
			},
		},
		"wraps get pipeline error": {
			callMocks: func(m pipelineDescriberMocks) {
				m.pipelineGetter.EXPECT().GetPipeline(pipelineResourceName).Return(nil, mockError)
			},
			inShowResource: false,
			expectedError:  fmt.Errorf("get pipeline: %w", mockError),
			expectedOutput: nil,
		},
		"wraps stack resources error": {
			callMocks: func(m pipelineDescriberMocks) {
				m.pipelineGetter.EXPECT().GetPipeline(pipelineResourceName).Return(mockPipeline, nil)
				m.cfn.EXPECT().Resources().Return(nil, mockError)
			},
			inShowResource: true,
			expectedError:  fmt.Errorf("retrieve pipeline resources: %w", mockError),
			expectedOutput: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCFN := mocks.NewMockstackDescriber(ctrl)
			mockPipelineGetter := mocks.NewMockpipelineGetter(ctrl)

			mocks := pipelineDescriberMocks{
				cfn:            mockCFN,
				pipelineGetter: mockPipelineGetter,
			}
			tc.callMocks(mocks)

			mockDeployedPipeline := deploy.Pipeline{
				ResourceName: pipelineResourceName,
				Name:         pipelineName,
				AppName:      "mockAppName",
				IsLegacy:     false,
			}

			describer := &PipelineDescriber{
				pipeline:      mockDeployedPipeline,
				showResources: tc.inShowResource,
				pipelineSvc:   mockPipelineGetter,
				cfn:           mockCFN,
			}

			// WHEN
			pipeline, err := describer.Describe()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedOutput, pipeline, "expected output content match")
			}
		})
	}
}

func TestPipelineDescriber_String(t *testing.T) {
	oldHumanize := humanizeTime
	humanizeTime = func(then time.Time) string {
		now, _ := time.Parse(time.RFC3339, "2020-06-19T00:00:00+00:00")
		return humanize.RelTime(then, now, "ago", "from now")
	}
	defer func() {
		humanizeTime = oldHumanize
	}()
	testCases := map[string]struct {
		inPipeline          *Pipeline
		expectedHumanString string
		expectedJSONString  string
	}{
		"correct output with resources": {
			inPipeline: &Pipeline{
				Name:      pipelineName,
				Pipeline:  *mockPipeline,
				Resources: expectedResources,
			},
			expectedHumanString: `About

  Name        pipeline-dinder-badgoose-repo
  Region      us-west-2
  AccountID   1234567890
  Created At  4 months ago
  Updated At  4 months ago

Stages

  Name           Category  Provider        Details
  ----           --------  --------        -------
  Source         Source    GitHub          Repository: badgoose/repo
  Build          Build     CodeBuild       BuildProject: pipeline-dinder-badgoose-repo-BuildProject
  DeployTo-test  Deploy    CloudFormation  StackName: dinder-test-test

Resources
    AWS::CodeBuild::Project      pipeline-dinder-badgoose-repo-BuildProject
    AWS::IAM::Policy             pipel-Buil-1PEASDDL44ID2
    AWS::IAM::Role               pipeline-dinder-badgoose-repo-BuildProjectRole-A4V6VSG1XIIJ
    AWS::CodePipeline::Pipeline  pipeline-dinder-badgoose-repo
    AWS::IAM::Role               pipeline-dinder-badgoose-repo-PipelineRole-100SEEQN6CU0F
    AWS::IAM::Policy             pipel-Pipe-EO4QGE10RJ8F
`,
			expectedJSONString: "{\"name\":\"pipeline-dinder-badgoose-repo\",\"pipelineName\":\"pipeline-dinder-badgoose-repo-RANDOMSTRING\",\"region\":\"us-west-2\",\"accountId\":\"1234567890\",\"stages\":[{\"name\":\"Source\",\"category\":\"Source\",\"provider\":\"GitHub\",\"details\":\"Repository: badgoose/repo\"},{\"name\":\"Build\",\"category\":\"Build\",\"provider\":\"CodeBuild\",\"details\":\"BuildProject: pipeline-dinder-badgoose-repo-BuildProject\"},{\"name\":\"DeployTo-test\",\"category\":\"Deploy\",\"provider\":\"CloudFormation\",\"details\":\"StackName: dinder-test-test\"}],\"createdAt\":\"2020-02-02T15:04:05Z\",\"updatedAt\":\"2020-02-02T15:04:05Z\",\"resources\":[{\"type\":\"AWS::CodeBuild::Project\",\"physicalID\":\"pipeline-dinder-badgoose-repo-BuildProject\"},{\"type\":\"AWS::IAM::Policy\",\"physicalID\":\"pipel-Buil-1PEASDDL44ID2\"},{\"type\":\"AWS::IAM::Role\",\"physicalID\":\"pipeline-dinder-badgoose-repo-BuildProjectRole-A4V6VSG1XIIJ\"},{\"type\":\"AWS::CodePipeline::Pipeline\",\"physicalID\":\"pipeline-dinder-badgoose-repo\"},{\"type\":\"AWS::IAM::Role\",\"physicalID\":\"pipeline-dinder-badgoose-repo-PipelineRole-100SEEQN6CU0F\"},{\"type\":\"AWS::IAM::Policy\",\"physicalID\":\"pipel-Pipe-EO4QGE10RJ8F\"}]}\n",
		},
		"correct output without resources": {
			inPipeline: &Pipeline{
				Name:      pipelineName,
				Pipeline:  *mockPipeline,
				Resources: nil,
			},
			expectedHumanString: `About

  Name        pipeline-dinder-badgoose-repo
  Region      us-west-2
  AccountID   1234567890
  Created At  4 months ago
  Updated At  4 months ago

Stages

  Name           Category  Provider        Details
  ----           --------  --------        -------
  Source         Source    GitHub          Repository: badgoose/repo
  Build          Build     CodeBuild       BuildProject: pipeline-dinder-badgoose-repo-BuildProject
  DeployTo-test  Deploy    CloudFormation  StackName: dinder-test-test
`,
			expectedJSONString: "{\"name\":\"pipeline-dinder-badgoose-repo\",\"pipelineName\":\"pipeline-dinder-badgoose-repo-RANDOMSTRING\",\"region\":\"us-west-2\",\"accountId\":\"1234567890\",\"stages\":[{\"name\":\"Source\",\"category\":\"Source\",\"provider\":\"GitHub\",\"details\":\"Repository: badgoose/repo\"},{\"name\":\"Build\",\"category\":\"Build\",\"provider\":\"CodeBuild\",\"details\":\"BuildProject: pipeline-dinder-badgoose-repo-BuildProject\"},{\"name\":\"DeployTo-test\",\"category\":\"Deploy\",\"provider\":\"CloudFormation\",\"details\":\"StackName: dinder-test-test\"}],\"createdAt\":\"2020-02-02T15:04:05Z\",\"updatedAt\":\"2020-02-02T15:04:05Z\"}\n",
		},
	}
	for _, tc := range testCases {
		human := tc.inPipeline.HumanString()
		json, _ := tc.inPipeline.JSONString()

		require.Equal(t, tc.expectedHumanString, human, "expected human output to match")
		require.Equal(t, tc.expectedJSONString, json, "expected JSON output to match")
	}
}
