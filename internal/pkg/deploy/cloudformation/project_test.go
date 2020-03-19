// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCloudFormation_DeployProject(t *testing.T) {
	mockProject := &deploy.CreateProjectInput{
		Project:   "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		mockCF  func(ctrl *gomock.Controller) cfnClient
		mockSDK func() *mockCloudFormation
		want    error
	}{
		"Infrastructure Roles Stack Fails": {
			mockCF: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(errors.New("error creating stack"))
				return m
			},
			mockSDK: func() *mockCloudFormation {
				return &mockCloudFormation{
					t: t,
				}
			},
			want: errors.New("error creating stack"),
		},
		"Infrastructure Roles Stack Already Exists": {
			mockCF: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{})
				return m
			},
			mockSDK: func() *mockCloudFormation {
				return &mockCloudFormation{
					t: t,
					mockCreateStackSet: func(t *testing.T, in *sdkcloudformation.CreateStackSetInput) (*sdkcloudformation.CreateStackSetOutput, error) {
						return nil, nil
					},
				}
			},
		},
		"StackSet Already Exists": {
			mockCF: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(nil)
				return m
			},
			mockSDK: func() *mockCloudFormation {
				client := &mockCloudFormation{
					t: t,
					mockCreateStackSet: func(t *testing.T, in *sdkcloudformation.CreateStackSetInput) (*sdkcloudformation.CreateStackSetOutput, error) {
						return nil, awserr.New(sdkcloudformation.ErrCodeNameAlreadyExistsException, "StackSetAlreadyExists", nil)
					},
				}
				return client
			},
		},
		"Infrastructure Roles StackSet Created": {
			mockCF: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(nil)
				return m
			},
			mockSDK: func() *mockCloudFormation {
				client := &mockCloudFormation{
					t: t,
					mockCreateStackSet: func(t *testing.T, in *sdkcloudformation.CreateStackSetInput) (*sdkcloudformation.CreateStackSetOutput, error) {
						require.Equal(t, "ECS CLI Project Resources (ECR repos, KMS keys, S3 buckets)", *in.Description)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						require.Equal(t, "testproject-executionrole", *in.ExecutionRoleName)
						require.Equal(t, "arn:aws:iam::1234:role/testproject-adminrole", *in.AdministrationRoleARN)
						require.True(t, len(in.Tags) == 1, "There should be one tag for the project")
						require.Equal(t, "ecs-project", *in.Tags[0].Key)
						require.Equal(t, mockProject.Project, *in.Tags[0].Value)

						return nil, nil
					},
				}
				return client
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := CloudFormation{
				cfnClient: tc.mockCF(ctrl),
				sdkClient: tc.mockSDK(),
				box:       templates.Box(),
			}

			// WHEN
			got := cf.DeployProject(mockProject)

			// THEN
			if tc.want != nil {
				require.EqualError(t, tc.want, got.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_AddEnvToProject(t *testing.T) {
	mockProject := archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		cf      CloudFormation
		project *archer.Project
		env     *archer.Environment
		want    error
	}{
		"with no existing deployments and adding an env": {
			project: &mockProject,
			env:     &archer.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						require.Equal(t, "ECS CLI Project Resources (ECR repos, KMS keys, S3 buckets)", *in.Description)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						require.Equal(t, "testproject-executionrole", *in.ExecutionRoleName)
						require.Equal(t, "arn:aws:iam::1234:role/testproject-adminrole", *in.AdministrationRoleARN)
						require.True(t, len(in.Tags) == 1, "There should be one tag for the project")
						require.Equal(t, "ecs-project", *in.Tags[0].Key)
						require.Equal(t, mockProject.Name, *in.Tags[0].Value)

						require.Equal(t, "1", *in.OperationId)

						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{mockProject.AccountID}, configToDeploy.Accounts)
						require.Empty(t, configToDeploy.Apps, "There should be no new apps to deploy")
						require.Equal(t, 1, configToDeploy.Version)
						return &sdkcloudformation.UpdateStackSetOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *sdkcloudformation.CreateStackInstancesInput) (*sdkcloudformation.CreateStackInstancesOutput, error) {
						require.ElementsMatch(t, []*string{aws.String(mockProject.AccountID)}, in.Accounts)
						require.ElementsMatch(t, []*string{aws.String("us-west-2")}, in.Regions)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						return &sdkcloudformation.CreateStackInstancesOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("SUCCEEDED"),
							},
						}, nil
					},
				},
				box: templates.Box(),
			},
		},

		"with no new account ID added": {
			project: &mockProject,
			env:     &archer.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{
							Metadata: stack.ProjectResourcesConfig{
								Accounts: []string{"1234"},
							},
						})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						require.Equal(t, "ECS CLI Project Resources (ECR repos, KMS keys, S3 buckets)", *in.Description)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						require.Equal(t, "testproject-executionrole", *in.ExecutionRoleName)
						require.Equal(t, "arn:aws:iam::1234:role/testproject-adminrole", *in.AdministrationRoleARN)
						require.True(t, len(in.Tags) == 1, "There should be one tag for the project")
						require.Equal(t, "ecs-project", *in.Tags[0].Key)
						require.Equal(t, mockProject.Name, *in.Tags[0].Value)

						require.Equal(t, "1", *in.OperationId)

						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
						require.NoError(t, err)
						// Ensure there are no duplicate accounts
						require.ElementsMatch(t, []string{mockProject.AccountID}, configToDeploy.Accounts)
						require.Empty(t, configToDeploy.Apps, "There should be no new apps to deploy")
						require.Equal(t, 1, configToDeploy.Version)
						return &sdkcloudformation.UpdateStackSetOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *sdkcloudformation.CreateStackInstancesInput) (*sdkcloudformation.CreateStackInstancesOutput, error) {
						require.ElementsMatch(t, []*string{aws.String(mockProject.AccountID)}, in.Accounts)
						require.ElementsMatch(t, []*string{aws.String("us-west-2")}, in.Regions)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						return &sdkcloudformation.CreateStackInstancesOutput{
							OperationId: aws.String("1"),
						}, nil
					},

					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("SUCCEEDED"),
							},
						}, nil
					},
				},
				box: templates.Box(),
			},
		},

		"with existing stack instances in same region but different account (no new stack instances, but update stackset)": {
			project: &mockProject,
			env:     &archer.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{Metadata: stack.ProjectResourcesConfig{
							Accounts: []string{"4567"},
							Version:  1,
						}})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{mockProject.AccountID, "4567"}, configToDeploy.Accounts)
						require.Empty(t, configToDeploy.Apps, "There should be no new apps to deploy")
						require.Equal(t, 2, configToDeploy.Version)

						return &sdkcloudformation.UpdateStackSetOutput{
							OperationId: aws.String("2"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{
								{
									Region:  aws.String("us-west-2"),
									Account: aws.String(mockProject.AccountID),
								},
							},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *sdkcloudformation.CreateStackInstancesInput) (*sdkcloudformation.CreateStackInstancesOutput, error) {
						t.FailNow()
						return nil, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("SUCCEEDED"),
							},
						}, nil
					},
				},
				box: templates.Box(),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.cf.AddEnvToProject(tc.project, tc.env)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_AddPipelineResourcesToProject(t *testing.T) {
	mockProject := archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		cf                  CloudFormation
		project             *archer.Project
		getRegionFromClient func(client cloudformationiface.CloudFormationAPI) (string, error)
		expectedErr         error
	}{
		"with no existing account nor environment, add pipeline supporting resources": {
			project: &mockProject,
			getRegionFromClient: func(client cloudformationiface.CloudFormationAPI) (string, error) {
				return "us-west-2", nil
			},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{
							// no existing account used for this project
						})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						require.Equal(t, "ECS CLI Project Resources (ECR repos, KMS keys, S3 buckets)", *in.Description)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						require.Equal(t, "testproject-executionrole", *in.ExecutionRoleName)
						require.Equal(t, "arn:aws:iam::1234:role/testproject-adminrole", *in.AdministrationRoleARN)
						require.True(t, len(in.Tags) == 1, "There should be one tag for the project")
						require.Equal(t, "ecs-project", *in.Tags[0].Key)
						require.Equal(t, mockProject.Name, *in.Tags[0].Value)

						require.Equal(t, "1", *in.OperationId)

						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{mockProject.AccountID}, configToDeploy.Accounts)
						require.Empty(t, configToDeploy.Apps, "There should be no new apps to deploy")
						require.Equal(t, 1, configToDeploy.Version)
						return &sdkcloudformation.UpdateStackSetOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						// no existing environment in this region, which implies there's no stack instance in this region,
						// so return an empty slice.
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *sdkcloudformation.CreateStackInstancesInput) (*sdkcloudformation.CreateStackInstancesOutput, error) {
						require.ElementsMatch(t, []*string{aws.String(mockProject.AccountID)}, in.Accounts)
						require.ElementsMatch(t, []*string{aws.String("us-west-2")}, in.Regions)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						return &sdkcloudformation.CreateStackInstancesOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("SUCCEEDED"),
							},
						}, nil
					},
				},
				box: templates.Box(),
			},
		},

		"with existing account, no existing environment in a region, add pipeline supporting resources to that region": {
			project: &mockProject,
			getRegionFromClient: func(client cloudformationiface.CloudFormationAPI) (string, error) {
				return "us-west-2", nil
			},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{
							Metadata: stack.ProjectResourcesConfig{
								// one accountId is associated with this project
								Accounts: []string{"1234"},
							},
						})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						require.Fail(t, "UpdateStackSet should not be called because there's no nwe account")
						return nil, errors.New("should not get here")
					},
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return &sdkcloudformation.ListStackInstancesOutput{
							// even though this account has been used with this project,
							// in the particular region we are provisioning the pipeline supporting
							// resources, there's no existing archer environment.
							Summaries: []*sdkcloudformation.StackInstanceSummary{},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *sdkcloudformation.CreateStackInstancesInput) (*sdkcloudformation.CreateStackInstancesOutput, error) {
						require.ElementsMatch(t, []*string{aws.String(mockProject.AccountID)}, in.Accounts)
						require.ElementsMatch(t, []*string{aws.String("us-west-2")}, in.Regions)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						return &sdkcloudformation.CreateStackInstancesOutput{
							OperationId: aws.String("1"),
						}, nil
					},

					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("SUCCEEDED"),
							},
						}, nil
					},
				},
				box: templates.Box(),
			},
		},

		"with existing account and existing environment in a region, should not add pipeline supporting resources": {
			project: &mockProject,
			getRegionFromClient: func(client cloudformationiface.CloudFormationAPI) (string, error) {
				return "us-west-2", nil
			},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{
							Metadata: stack.ProjectResourcesConfig{
								// one accountId is associated with this project
								Accounts: []string{"1234"},
							},
						})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						require.Fail(t, "UpdateStackSet should not be called because there's no nwe account")
						return nil, errors.New("should not get here")
					},
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						// this region happened to already has an environment deployed to it
						// so there's an exsiting stack instance
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{
								{
									Region:  aws.String("us-west-2"),
									Account: aws.String(mockProject.AccountID),
								},
							},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *sdkcloudformation.CreateStackInstancesInput) (*sdkcloudformation.CreateStackInstancesOutput, error) {
						require.Fail(t, "CreateStackInstances should not be called because there's an existing stack instance in this region")
						return nil, errors.New("should not get here")
					},

					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						require.Fail(t, "DescribeStackSetOperation should not be called because there's an existing stack instance in this region")
						return nil, errors.New("should not get here")
					},
				},
				box: templates.Box(),
			},
		},
	}

	actual := getRegionFromClient // FIXME refactor using defer func
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			getRegionFromClient = tc.getRegionFromClient
			got := tc.cf.AddPipelineResourcesToProject(tc.project, "us-west-2")

			if tc.expectedErr != nil {
				require.EqualError(t, got, tc.expectedErr.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
	getRegionFromClient = actual
}

func TestCloudFormation_AddAppToProject(t *testing.T) {
	mockProject := archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		cf      CloudFormation
		project *archer.Project
		app     string
		want    error
	}{
		"with no existing deployments and adding an app": {
			project: &mockProject,
			app:     "TestApp",
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						require.Equal(t, "ECS CLI Project Resources (ECR repos, KMS keys, S3 buckets)", *in.Description)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						require.Equal(t, "testproject-executionrole", *in.ExecutionRoleName)
						require.Equal(t, "arn:aws:iam::1234:role/testproject-adminrole", *in.AdministrationRoleARN)
						require.True(t, len(in.Tags) == 1, "There should be one tag for the project")
						require.Equal(t, "ecs-project", *in.Tags[0].Key)
						require.Equal(t, mockProject.Name, *in.Tags[0].Value)
						// We should increment the version
						require.Equal(t, "1", *in.OperationId)

						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{"TestApp"}, configToDeploy.Apps)
						require.Empty(t, configToDeploy.Accounts, "There should be no new accounts to deploy")
						require.Equal(t, 1, configToDeploy.Version)
						return &sdkcloudformation.UpdateStackSetOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{},
						}, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("SUCCEEDED"),
							},
						}, nil
					},
				},
				box: templates.Box(),
			},
		},
		"with new app to existing project with existing apps": {
			project: &mockProject,
			app:     "test",
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{Metadata: stack.ProjectResourcesConfig{
							Apps:    []string{"firsttest"},
							Version: 1,
						}})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{"test", "firsttest"}, configToDeploy.Apps)
						require.Empty(t, configToDeploy.Accounts, "There should be no new apps to deploy")
						require.Equal(t, 2, configToDeploy.Version)

						return &sdkcloudformation.UpdateStackSetOutput{
							OperationId: aws.String("2"),
						}, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("SUCCEEDED"),
							},
						}, nil
					},
				},
				box: templates.Box(),
			},
		},
		"with ewxisting app to existing project with existing apps": {
			project: &mockProject,
			app:     "test",
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{Metadata: stack.ProjectResourcesConfig{
							Apps:    []string{"test"},
							Version: 1,
						}})
						require.NoError(t, err)
						return &sdkcloudformation.DescribeStackSetOutput{
							StackSet: &sdkcloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
						t.FailNow()
						return nil, nil
					},
				},
				box: templates.Box(),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.cf.AddAppToProject(tc.project, tc.app)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_RemoveAppFromProject(t *testing.T) {
	mockProject := &archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}

	tests := map[string]struct {
		app string

		mockDescribeStackSet          func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error)
		mockUpdateStackSet            func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error)
		mockDescribeStackSetOperation func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error)

		want error
	}{
		"should remove input app from the stack set": {
			app: "test",
			mockDescribeStackSet: func(t *testing.T, in *sdkcloudformation.DescribeStackSetInput) (*sdkcloudformation.DescribeStackSetOutput, error) {
				body, err := yaml.Marshal(stack.DeployedProjectMetadata{Metadata: stack.ProjectResourcesConfig{
					Apps:    []string{"test", "firsttest"},
					Version: 1,
				}})
				require.NoError(t, err)
				return &sdkcloudformation.DescribeStackSetOutput{
					StackSet: &sdkcloudformation.StackSet{
						TemplateBody: aws.String(string(body)),
					},
				}, nil
			},
			mockUpdateStackSet: func(t *testing.T, in *sdkcloudformation.UpdateStackSetInput) (*sdkcloudformation.UpdateStackSetOutput, error) {
				require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
				configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
				require.NoError(t, err)
				require.ElementsMatch(t, []string{"firsttest"}, configToDeploy.Apps)
				require.Empty(t, configToDeploy.Accounts, "config account list should be empty")
				require.Equal(t, 2, configToDeploy.Version)

				return &sdkcloudformation.UpdateStackSetOutput{
					OperationId: aws.String("2"),
				}, nil
			},
			mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
				return &sdkcloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &sdkcloudformation.StackSetOperation{
						Status: aws.String(sdkcloudformation.StackSetOperationStatusSucceeded),
					},
				}, nil
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				sdkClient: mockCloudFormation{
					t: t,

					mockDescribeStackSet:          test.mockDescribeStackSet,
					mockUpdateStackSet:            test.mockUpdateStackSet,
					mockDescribeStackSetOperation: test.mockDescribeStackSetOperation,
				},
				box: templates.Box(),
			}

			got := cf.RemoveAppFromProject(mockProject, test.app)

			require.Equal(t, test.want, got)
		})
	}
}

func TestWaitForStackSetOperation(t *testing.T) {
	waitingForOperation := true
	testCases := map[string]struct {
		cf   CloudFormation
		want error
	}{
		"operation succeeded": {
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("SUCCEEDED"),
							},
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
		},
		"operation failed": {
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("FAILED"),
							},
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
			want: fmt.Errorf("project operation operation in stack set stackset failed"),
		},
		"operation stopped": {
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String("STOPPED"),
							},
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
			want: fmt.Errorf("project operation operation in stack set stackset was manually stopped"),
		},
		"operation non-terminal to succeeded": {
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
						// First, say the status is running. Then during the next call, set the status to succeeded.
						status := "RUNNING"
						if !waitingForOperation {
							status = "SUCCEEDED"
						}
						waitingForOperation = false
						return &sdkcloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &sdkcloudformation.StackSetOperation{
								Status: aws.String(status),
							},
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.cf.waitForStackSetOperation("stackset", "operation")

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_GetRegionalProjectResources(t *testing.T) {
	mockProject := archer.Project{Name: "project", AccountID: "12345"}

	testCases := map[string]struct {
		cf                       CloudFormation
		createRegionalMockClient func(ctrl *gomock.Controller) cfnClient
		wantedResource           archer.ProjectRegionalResources
		want                     error
	}{
		"should describe stack instances and convert to ProjectRegionalResources": {
			wantedResource: archer.ProjectRegionalResources{
				KMSKeyARN:      "arn:aws:kms:us-west-2:01234567890:key/0000",
				S3Bucket:       "tests3-bucket-us-west-2",
				Region:         "us-east-9",
				RepositoryURLs: map[string]string{},
			},
			createRegionalMockClient: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("cross-region-stack").Return(mockValidProjectResourceStack(), nil)
				return m
			},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{
								{
									StackId: aws.String("cross-region-stack"),
									Region:  aws.String("us-east-9"),
								},
							},
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
		},

		"should propagate describe errors": {
			want: fmt.Errorf("describing project resources: getting outputs for stack cross-region-stack in region us-east-9: error calling cloudformation"),
			createRegionalMockClient: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("cross-region-stack").Return(nil, errors.New("error calling cloudformation"))
				return m
			},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{
								{
									StackId: aws.String("cross-region-stack"),
									Region:  aws.String("us-east-9"),
								},
							},
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
		},

		"should propagate list stack instances errors": {
			want: fmt.Errorf("describing project resources: listing stack instances: error"),
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return nil, fmt.Errorf("error")
					},
				},
				box: boxWithTemplateFile(),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			tc.cf.regionalClient = func(region string) cfnClient {
				return tc.createRegionalMockClient(ctrl)
			}

			// WHEN
			got, err := tc.cf.GetRegionalProjectResources(&mockProject)

			// THEN
			if tc.want != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.want.Error())
			} else {
				require.True(t, len(got) == 1, "Expected only one resource")
				// Assert that the project resources are the same.
				require.Equal(t, tc.wantedResource, *got[0])
			}
		})
	}
}

func TestCloudFormation_GetProjectResourcesByRegion(t *testing.T) {
	mockProject := archer.Project{Name: "project", AccountID: "12345"}

	testCases := map[string]struct {
		cf                       CloudFormation
		createRegionalMockClient func(ctrl *gomock.Controller) cfnClient
		wantedResource           archer.ProjectRegionalResources
		region                   string
		want                     error
	}{
		"should describe stack instances and convert to ProjectRegionalResources": {
			wantedResource: archer.ProjectRegionalResources{
				KMSKeyARN:      "arn:aws:kms:us-west-2:01234567890:key/0000",
				S3Bucket:       "tests3-bucket-us-west-2",
				Region:         "us-east-9",
				RepositoryURLs: map[string]string{},
			},
			region: "us-east-9",
			createRegionalMockClient: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("cross-region-stack").Return(mockValidProjectResourceStack(), nil)
				return m
			},
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						require.Equal(t, "us-east-9", *in.StackInstanceRegion)
						return &sdkcloudformation.ListStackInstancesOutput{
							Summaries: []*sdkcloudformation.StackInstanceSummary{
								{
									StackId: aws.String("cross-region-stack"),
									Region:  aws.String("us-east-9"),
								},
							},
						}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
		},
		"should error when resources are found": {
			want:   fmt.Errorf("no regional resources for project project in region us-east-9 found"),
			region: "us-east-9",
			cf: CloudFormation{
				sdkClient: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
						return &sdkcloudformation.ListStackInstancesOutput{}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			tc.cf.regionalClient = func(region string) cfnClient {
				return tc.createRegionalMockClient(ctrl)
			}

			// WHEN
			got, err := tc.cf.GetProjectResourcesByRegion(&mockProject, tc.region)

			// THEN
			if tc.want != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.want.Error())
			} else {
				require.NotNil(t, got)
				// Assert that the project resources are the same.
				require.Equal(t, tc.wantedResource, *got)
			}
		})
	}
}

func TestCloudFormation_DelegateDNSPermissions(t *testing.T) {
	testCases := map[string]struct {
		project    *archer.Project
		accountID  string
		createMock func(ctrl *gomock.Controller) cfnClient
		want       error
	}{
		"Calls Update Stack": {
			project: &archer.Project{
				AccountID: "1234",
				Name:      "project",
				Domain:    "amazon.com",
			},
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(mockProjectRolesStack("stackname", map[string]string{
					"ProjectDNSDelegatedAccounts": "1234",
				}), nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(nil)
				return m
			},
		},

		"Returns error from Describe Stack": {
			project: &archer.Project{
				AccountID: "1234",
				Name:      "project",
				Domain:    "amazon.com",
			},
			want: fmt.Errorf("getting existing project infrastructure stack: error"),
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, errors.New("error"))
				return m
			},
		},
		"Returns nil if there are no changeset updates from deployChangeSet": {
			project: &archer.Project{
				AccountID: "1234",
				Name:      "project",
				Domain:    "amazon.com",
			},
			want: nil,
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(mockProjectRolesStack("stackname", map[string]string{
					"ProjectDNSDelegatedAccounts": "1234",
				}), nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(&cloudformation.ErrChangeSetEmpty{})
				return m
			},
		},
		"Returns error from Update Stack": {
			project: &archer.Project{
				AccountID: "1234",
				Name:      "project",
				Domain:    "amazon.com",
			},
			want: fmt.Errorf("updating project to allow DNS delegation: error"),
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(mockProjectRolesStack("stackname", map[string]string{
					"ProjectDNSDelegatedAccounts": "1234",
				}), nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(errors.New("error"))
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := CloudFormation{
				cfnClient: tc.createMock(ctrl),
				box:       templates.Box(),
			}

			// WHEN
			got := cf.DelegateDNSPermissions(tc.project, tc.accountID)

			// THEN
			if tc.want != nil {
				require.EqualError(t, tc.want, got.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func mockValidProjectResourceStack() *cloudformation.StackDescription {
	return mockProjectResourceStack("stack", map[string]string{
		"KMSKeyARN":      "arn:aws:kms:us-west-2:01234567890:key/0000",
		"PipelineBucket": "tests3-bucket-us-west-2",
	})
}

func mockProjectResourceStack(stackArn string, outputs map[string]string) *cloudformation.StackDescription {
	outputList := []*sdkcloudformation.Output{}
	for key, val := range outputs {
		outputList = append(outputList, &sdkcloudformation.Output{
			OutputKey:   aws.String(key),
			OutputValue: aws.String(val),
		})
	}

	return &cloudformation.StackDescription{
		StackId: aws.String(stackArn),
		Outputs: outputList,
	}
}

func mockProjectRolesStack(stackArn string, parameters map[string]string) *cloudformation.StackDescription {
	parametersList := []*sdkcloudformation.Parameter{}
	for key, val := range parameters {
		parametersList = append(parametersList, &sdkcloudformation.Parameter{
			ParameterKey:   aws.String(key),
			ParameterValue: aws.String(val),
		})
	}

	return &cloudformation.StackDescription{
		StackId:     aws.String(stackArn),
		StackStatus: aws.String("UPDATE_COMPLETE"),
		Parameters:  parametersList,
	}
}

func TestCloudFormation_DeleteProject(t *testing.T) {
	tests := map[string]struct {
		projectName string
		createMock  func(ctrl *gomock.Controller) cfnClient

		mockListStackInstances                      func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error)
		mockDeleteStackInstances                    func(t *testing.T, in *sdkcloudformation.DeleteStackInstancesInput) (*sdkcloudformation.DeleteStackInstancesOutput, error)
		mockDeleteStackSet                          func(t *testing.T, in *sdkcloudformation.DeleteStackSetInput) (*sdkcloudformation.DeleteStackSetOutput, error)
		mockDescribeStackSetOperation               func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error)
		mockWaitUntilStackDeleteCompleteWithContext func(t *testing.T, in *sdkcloudformation.DescribeStacksInput) error

		want error
	}{
		"should return nil given happy path": {
			projectName: "testProject",
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().DeleteAndWait("testProject-infrastructure-roles").Return(nil)
				return m
			},
			mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
				return &sdkcloudformation.ListStackInstancesOutput{
					Summaries: []*sdkcloudformation.StackInstanceSummary{
						{
							Region:  aws.String("us-west-2"),
							Account: aws.String("12345"),
						},
					},
				}, nil
			},
			mockDeleteStackInstances: func(t *testing.T, in *sdkcloudformation.DeleteStackInstancesInput) (*sdkcloudformation.DeleteStackInstancesOutput, error) {
				require.Equal(t, 1, len(in.Accounts))
				require.Equal(t, 1, len(in.Regions))
				require.Equal(t, "12345", aws.StringValue(in.Accounts[0]))
				require.Equal(t, "us-west-2", aws.StringValue(in.Regions[0]))
				return &sdkcloudformation.DeleteStackInstancesOutput{
					OperationId: aws.String("operationId"),
				}, nil
			},
			mockDeleteStackSet: func(t *testing.T, in *sdkcloudformation.DeleteStackSetInput) (*sdkcloudformation.DeleteStackSetOutput, error) {
				return &sdkcloudformation.DeleteStackSetOutput{}, nil
			},
			mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
				require.Equal(t, "operationId", aws.StringValue(in.OperationId))
				return &sdkcloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &sdkcloudformation.StackSetOperation{
						Status: aws.String("SUCCEEDED"),
					},
				}, nil
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *sdkcloudformation.DescribeStacksInput) error {
				return nil
			},
			want: nil,
		},
		"should return nil if stackset has already been deleted before running": {
			projectName: "testProject",
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().DeleteAndWait("testProject-infrastructure-roles").Return(nil)
				return m
			},
			mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
				return nil, awserr.New(sdkcloudformation.ErrCodeStackSetNotFoundException, "StackSetNotFoundException", nil)
			},
			mockDeleteStackInstances: func(t *testing.T, in *sdkcloudformation.DeleteStackInstancesInput) (*sdkcloudformation.DeleteStackInstancesOutput, error) {
				t.FailNow()
				return nil, nil
			},
			mockDeleteStackSet: func(t *testing.T, in *sdkcloudformation.DeleteStackSetInput) (*sdkcloudformation.DeleteStackSetOutput, error) {
				t.FailNow()
				return nil, nil
			},
			mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
				t.FailNow()
				return nil, nil
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *sdkcloudformation.DescribeStacksInput) error {
				return nil
			},
			want: nil,
		},
		"should return nil if stackset is deleted after stack instances are created (edge case)": {
			projectName: "testProject",
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().DeleteAndWait("testProject-infrastructure-roles").Return(nil)
				return m
			},
			mockListStackInstances: func(t *testing.T, in *sdkcloudformation.ListStackInstancesInput) (*sdkcloudformation.ListStackInstancesOutput, error) {
				return &sdkcloudformation.ListStackInstancesOutput{
					Summaries: []*sdkcloudformation.StackInstanceSummary{
						{
							Region:  aws.String("us-west-2"),
							Account: aws.String("12345"),
						},
					},
				}, nil
			},
			mockDeleteStackInstances: func(t *testing.T, in *sdkcloudformation.DeleteStackInstancesInput) (*sdkcloudformation.DeleteStackInstancesOutput, error) {
				require.Equal(t, 1, len(in.Accounts))
				require.Equal(t, 1, len(in.Regions))
				require.Equal(t, "12345", aws.StringValue(in.Accounts[0]))
				require.Equal(t, "us-west-2", aws.StringValue(in.Regions[0]))
				return &sdkcloudformation.DeleteStackInstancesOutput{
					OperationId: aws.String("operationId"),
				}, nil
			},
			mockDeleteStackSet: func(t *testing.T, in *sdkcloudformation.DeleteStackSetInput) (*sdkcloudformation.DeleteStackSetOutput, error) {
				return nil, awserr.New(sdkcloudformation.ErrCodeStackSetNotFoundException, "StackSetNotFoundException", nil)
			},
			mockDescribeStackSetOperation: func(t *testing.T, in *sdkcloudformation.DescribeStackSetOperationInput) (*sdkcloudformation.DescribeStackSetOperationOutput, error) {
				require.Equal(t, "operationId", aws.StringValue(in.OperationId))
				return &sdkcloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &sdkcloudformation.StackSetOperation{
						Status: aws.String("SUCCEEDED"),
					},
				}, nil
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *sdkcloudformation.DescribeStacksInput) error {
				return nil
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cf := CloudFormation{
				// TODO: replace this custom mock client with gomock.
				sdkClient: &mockCloudFormation{
					t:                             t,
					mockDeleteStackInstances:      test.mockDeleteStackInstances,
					mockDeleteStackSet:            test.mockDeleteStackSet,
					mockListStackInstances:        test.mockListStackInstances,
					mockDescribeStackSetOperation: test.mockDescribeStackSetOperation,
				},
				cfnClient: test.createMock(ctrl),
			}

			// WHEN
			got := cf.DeleteProject(test.projectName)

			// THEN
			require.Equal(t, test.want, got)
		})
	}
}
