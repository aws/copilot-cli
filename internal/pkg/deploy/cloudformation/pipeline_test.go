// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
)

const defaultImage = "aws/codebuild/amazonlinux2-x86_64-standard:3.0"

func TestCloudFormation_PipelineExists(t *testing.T) {
	in := &deploy.CreatePipelineInput{
		AppName: "kudos",
		Name:    "cicd",
	}
	testCases := map[string]struct {
		createMock   func(ctrl *gomock.Controller) cfnClient
		wantedExists bool
		wantedErr    error
	}{
		"return false and error on unexpected failure": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: errors.New("some error"),
		},
		"return false if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, fmt.Errorf("describe stack: %w", &cloudformation.ErrStackNotFound{}))
				return m
			},
			wantedExists: false,
		},
		"returns true if stack exists": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, nil)
				return m
			},
			wantedExists: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				cfnClient: tc.createMock(ctrl),
			}

			// WHEN
			exists, err := c.PipelineExists(in)

			// THEN
			require.Equal(t, tc.wantedExists, exists)
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_CreatePipeline(t *testing.T) {
	in := &deploy.CreatePipelineInput{
		AppName: "kudos",
		Name:    "cicd",
		Source: &deploy.BitbucketSource{
			RepositoryURL: "https://bitbucket.org/aws/somethingCool",
			ProviderName:  "Bitbucket",
			Branch:        "main",
		},
		Build: &deploy.Build{
			Image: defaultImage,
		},
		Stages:          nil,
		ArtifactBuckets: nil,
	}
	mockS3BucketName := "BitterBucket"
	mockURL := "templateURL"

	testCases := map[string]struct {
		createCfnMock func(ctrl *gomock.Controller) cfnClient
		createCsMock  func(ctrl *gomock.Controller) codeStarClient
		createCpMock  func(ctrl *gomock.Controller) codePipelineClient
		createS3Mock  func(ctrl *gomock.Controller) s3Client
		wantedErr     error
	}{
		"exits successfully with base case (no connection)": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Times(1)
				m.EXPECT().Outputs(gomock.Any()).Return(nil, nil)
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Times(0)
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution(gomock.Any(), gomock.Any()).Times(0)
				return m
			},
			wantedErr: nil,
		},
		"exits successfully with connection update to wait for": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Times(1)
				m.EXPECT().Outputs(gomock.Any()).Return(
					map[string]string{
						"PipelineConnectionARN": "mockConnectionARN",
					}, nil)
				m.EXPECT().StackResources(gomock.Any()).Return([]*cloudformation.StackResource{
					{
						LogicalResourceId:  aws.String(cfnLogicalResourceIDPipeline),
						ResourceType:       aws.String(cfnResourceTypePipeline),
						PhysicalResourceId: aws.String("mockPipelineResourceID"),
					},
				}, nil)
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Return(nil)
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution("mockPipelineResourceID", gomock.Any()).Return(nil)
				return m
			},
			wantedErr: nil,
		},
		"returns err if fail to create and wait": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(errors.New("some error"))
				m.EXPECT().Outputs(gomock.Any()).Times(0)
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Times(0)
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution(gomock.Any(), gomock.Any()).Times(0)
				return m
			},
			wantedErr: fmt.Errorf("some error"),
		},
		"returns error if fails to upload template to S3 bucket": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Times(0)
				m.EXPECT().Outputs(gomock.Any()).Times(0)
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Times(0)
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution(gomock.Any(), gomock.Any()).Times(0)
				return m
			},
			wantedErr: fmt.Errorf("upload pipeline template to S3 bucket %s: some error", "BitterBucket"),
		},
		"returns err if retrieving outputs fails": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Times(1)
				m.EXPECT().Outputs(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Times(0)
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution(gomock.Any(), gomock.Any()).Times(0)
				return m
			},
			wantedErr: fmt.Errorf("some error"),
		},
		"returns err if unsuccessful in waiting for status to become available": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Times(1)
				m.EXPECT().Outputs(gomock.Any()).Return(map[string]string{
					"PipelineConnectionARN": "mockConnectionARN",
				}, nil)
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Return(errors.New("some error"))
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution(gomock.Any(), gomock.Any()).Times(0)
				return m
			},
			wantedErr: fmt.Errorf("some error"),
		},
		"returns err if error occurs when retrieving pipeline physical id": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Times(1)
				m.EXPECT().Outputs(gomock.Any()).Return(map[string]string{
					"PipelineConnectionARN": "mockConnectionARN",
				}, nil)
				m.EXPECT().StackResources(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Return(nil)
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution(gomock.Any(), gomock.Any()).Times(0)
				return m
			},
			wantedErr: fmt.Errorf("some error"),
		},
		"returns err if unable to find pipeline resource from stack": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Times(1)
				m.EXPECT().Outputs(gomock.Any()).Return(map[string]string{
					"PipelineConnectionARN": "mockConnectionARN",
				}, nil)
				m.EXPECT().StackResources(gomock.Any()).Return([]*cloudformation.StackResource{}, nil)
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Return(nil)
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution(gomock.Any(), gomock.Any()).Times(0)
				return m
			},
			wantedErr: fmt.Errorf(`cannot find a resource in stack pipeline-kudos-cicd with logical ID "Pipeline" of type "AWS::CodePipeline::Pipeline"`),
		},
		"returns err if unsuccessful in retrying stage execution": {
			createCfnMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Times(1)
				m.EXPECT().Outputs(gomock.Any()).Return(map[string]string{
					"PipelineConnectionARN": "mockConnectionARN",
				}, nil)
				m.EXPECT().StackResources(gomock.Any()).Return([]*cloudformation.StackResource{
					{
						LogicalResourceId:  aws.String(cfnLogicalResourceIDPipeline),
						ResourceType:       aws.String(cfnResourceTypePipeline),
						PhysicalResourceId: aws.String("mockPipelineResourceID"),
					},
				}, nil)
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			createCsMock: func(ctrl *gomock.Controller) codeStarClient {
				m := mocks.NewMockcodeStarClient(ctrl)
				m.EXPECT().WaitUntilConnectionStatusAvailable(gomock.Any(), "mockConnectionARN").Return(nil)
				return m
			},
			createCpMock: func(ctrl *gomock.Controller) codePipelineClient {
				m := mocks.NewMockcodePipelineClient(ctrl)
				m.EXPECT().RetryStageExecution("mockPipelineResourceID", gomock.Any()).Return(errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				cfnClient:      tc.createCfnMock(ctrl),
				codeStarClient: tc.createCsMock(ctrl),
				cpClient:       tc.createCpMock(ctrl),
				s3Client:       tc.createS3Mock(ctrl),
			}

			// WHEN
			err := c.CreatePipeline(in, mockS3BucketName)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCloudFormation_UpdatePipeline(t *testing.T) {
	in := &deploy.CreatePipelineInput{
		AppName: "kudos",
		Name:    "cicd",
		Source: &deploy.GitHubSource{
			RepositoryURL: "aws/somethingCool",
			Branch:        "main",
		},
		Build: &deploy.Build{
			Image: defaultImage,
		},
		Stages:          nil,
		ArtifactBuckets: nil,
	}
	mockS3BucketName := "BitterBucket"
	mockURL := "templateURL"
	testCases := map[string]struct {
		createMock   func(ctrl *gomock.Controller) cfnClient
		createS3Mock func(ctrl *gomock.Controller) s3Client
		wantedErr    error
	}{
		"exits successfully if there are no updates": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(&cloudformation.ErrChangeSetEmpty{})
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return(mockURL, nil)
				return m
			},
			wantedErr: nil,
		},
		"returns an error if can't push template to S3 bucket": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				//m.EXPECT().UpdateAndWait(gomock.Any()).Return(&cloudformation.ErrChangeSetEmpty{})
				return m
			},
			createS3Mock: func(ctrl *gomock.Controller) s3Client {
				m := mocks.NewMocks3Client(ctrl)
				m.EXPECT().Upload(mockS3BucketName, gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
				return m
			},
			wantedErr: fmt.Errorf("upload pipeline template to S3 bucket %s: some error", "BitterBucket"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				cfnClient: tc.createMock(ctrl),
				s3Client:  tc.createS3Mock(ctrl),
			}

			// WHEN
			err := c.UpdatePipeline(in, mockS3BucketName)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
