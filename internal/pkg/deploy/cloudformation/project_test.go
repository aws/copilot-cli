// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCreateProjectResources(t *testing.T) {
	mockProject := &deploy.CreateProjectInput{
		Project:   "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		mockCFClient func() *mockCloudFormation
		want         error
	}{
		"Infrastructure Roles Stack Fails": {
			mockCFClient: func() *mockCloudFormation {
				return &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						return nil, fmt.Errorf("error creating stack")
					},
					mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
						return &cloudformation.DescribeStacksOutput{}, nil
					},
				}
			},
			want: fmt.Errorf("failed to create changeSet for stack testproject-infrastructure-roles: error creating stack"),
		},
		"Infrastructure Roles Stack Already Exists": {
			mockCFClient: func() *mockCloudFormation {
				return &mockCloudFormation{
					t: t,
					mockCreateStackSet: func(t *testing.T, in *cloudformation.CreateStackSetInput) (*cloudformation.CreateStackSetOutput, error) {
						return nil, nil
					},
					mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
						return &cloudformation.DescribeStacksOutput{
							Stacks: []*cloudformation.Stack{
								&cloudformation.Stack{
									StackStatus: aws.String("UPDATE_COMPLETE"),
									StackId:     aws.String(fmt.Sprintf("arn:aws:cloudformation:eu-west-3:902697171733:stack/%s", *in.StackName)),
								},
							},
						}, nil
					},
				}
			},
		},
		"StackSet Already Exists": {
			mockCFClient: func() *mockCloudFormation {
				client := getMockSuccessfulDeployCFClient(t, "stackname")
				client.mockCreateStackSet = func(t *testing.T, in *cloudformation.CreateStackSetInput) (*cloudformation.CreateStackSetOutput, error) {
					return nil, awserr.New(cloudformation.ErrCodeNameAlreadyExistsException, "StackSetAlreadyExiststs", nil)
				}
				return client
			},
		},
		"Infrastructure Roles StackSet Created": {
			mockCFClient: func() *mockCloudFormation {
				client := getMockSuccessfulDeployCFClient(t, "stackname")
				client.mockCreateStackSet = func(t *testing.T, in *cloudformation.CreateStackSetInput) (*cloudformation.CreateStackSetOutput, error) {
					require.Equal(t, "ECS CLI Project Resources (ECR repos, KMS keys, S3 buckets)", *in.Description)
					require.Equal(t, "testproject-infrastructure", *in.StackSetName)
					require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
					require.Equal(t, "testproject-executionrole", *in.ExecutionRoleName)
					require.Equal(t, "arn:aws:iam::1234:role/testproject-adminrole", *in.AdministrationRoleARN)
					require.True(t, len(in.Tags) == 1, "There should be one tag for the project")
					require.Equal(t, "ecs-project", *in.Tags[0].Key)
					require.Equal(t, mockProject.Project, *in.Tags[0].Value)

					return nil, nil
				}
				return client
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				client: tc.mockCFClient(),
				box:    templates.Box(),
			}
			got := cf.DeployProject(mockProject)

			if tc.want != nil {
				require.EqualError(t, tc.want, got.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestAddEnvToProject(t *testing.T) {
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
				client: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
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
						return &cloudformation.UpdateStackSetOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error) {
						require.ElementsMatch(t, []*string{aws.String(mockProject.AccountID)}, in.Accounts)
						require.ElementsMatch(t, []*string{aws.String("us-west-2")}, in.Regions)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						return &cloudformation.CreateStackInstancesOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{
							Metadata: stack.ProjectResourcesConfig{
								Accounts: []string{"1234"},
							},
						})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
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
						return &cloudformation.UpdateStackSetOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error) {
						require.ElementsMatch(t, []*string{aws.String(mockProject.AccountID)}, in.Accounts)
						require.ElementsMatch(t, []*string{aws.String("us-west-2")}, in.Regions)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						return &cloudformation.CreateStackInstancesOutput{
							OperationId: aws.String("1"),
						}, nil
					},

					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{Metadata: stack.ProjectResourcesConfig{
							Accounts: []string{"4567"},
							Version:  1,
						}})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{mockProject.AccountID, "4567"}, configToDeploy.Accounts)
						require.Empty(t, configToDeploy.Apps, "There should be no new apps to deploy")
						require.Equal(t, 2, configToDeploy.Version)

						return &cloudformation.UpdateStackSetOutput{
							OperationId: aws.String("2"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{
								&cloudformation.StackInstanceSummary{
									Region:  aws.String("us-west-2"),
									Account: aws.String(mockProject.AccountID),
								},
							},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error) {
						t.FailNow()
						return nil, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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

func TestAddPipelineResourcesToProject(t *testing.T) {
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
				client: &mockCloudFormation{
					t: t,
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{
							// no existing account used for this project
						})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
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
						return &cloudformation.UpdateStackSetOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						// no existing environment in this region, which implies there's no stack instance in this region,
						// so return an empty slice.
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error) {
						require.ElementsMatch(t, []*string{aws.String(mockProject.AccountID)}, in.Accounts)
						require.ElementsMatch(t, []*string{aws.String("us-west-2")}, in.Regions)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						return &cloudformation.CreateStackInstancesOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{
							Metadata: stack.ProjectResourcesConfig{
								// one accountId is associated with this project
								Accounts: []string{"1234"},
							},
						})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
						require.Fail(t, "UpdateStackSet should not be called because there's no nwe account")
						return nil, errors.New("should not get here")
					},
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return &cloudformation.ListStackInstancesOutput{
							// even though this account has been used with this project,
							// in the particular region we are provisioning the pipeline supporting
							// resources, there's no existing archer environment.
							Summaries: []*cloudformation.StackInstanceSummary{},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error) {
						require.ElementsMatch(t, []*string{aws.String(mockProject.AccountID)}, in.Accounts)
						require.ElementsMatch(t, []*string{aws.String("us-west-2")}, in.Regions)
						require.Equal(t, "testproject-infrastructure", *in.StackSetName)
						return &cloudformation.CreateStackInstancesOutput{
							OperationId: aws.String("1"),
						}, nil
					},

					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{
							Metadata: stack.ProjectResourcesConfig{
								// one accountId is associated with this project
								Accounts: []string{"1234"},
							},
						})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
						require.Fail(t, "UpdateStackSet should not be called because there's no nwe account")
						return nil, errors.New("should not get here")
					},
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						// this region happened to already has an environment deployed to it
						// so there's an exsiting stack instance
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{
								&cloudformation.StackInstanceSummary{
									Region:  aws.String("us-west-2"),
									Account: aws.String(mockProject.AccountID),
								},
							},
						}, nil
					},
					mockCreateStackInstances: func(t *testing.T, in *cloudformation.CreateStackInstancesInput) (*cloudformation.CreateStackInstancesOutput, error) {
						require.Fail(t, "CreateStackInstances should not be called because there's an existing stack instance in this region")
						return nil, errors.New("should not get here")
					},

					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
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

func TestAddAppToProject(t *testing.T) {
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
				client: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
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
						return &cloudformation.UpdateStackSetOutput{
							OperationId: aws.String("1"),
						}, nil
					},
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{},
						}, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{Metadata: stack.ProjectResourcesConfig{
							Apps:    []string{"firsttest"},
							Version: 1,
						}})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
						require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
						configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{"test", "firsttest"}, configToDeploy.Apps)
						require.Empty(t, configToDeploy.Accounts, "There should be no new apps to deploy")
						require.Equal(t, 2, configToDeploy.Version)

						return &cloudformation.UpdateStackSetOutput{
							OperationId: aws.String("2"),
						}, nil
					},
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					// Given there hasn't been a StackSet update - the metadata in the stack body will be empty.
					mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
						body, err := yaml.Marshal(stack.DeployedProjectMetadata{Metadata: stack.ProjectResourcesConfig{
							Apps:    []string{"test"},
							Version: 1,
						}})
						require.NoError(t, err)
						return &cloudformation.DescribeStackSetOutput{
							StackSet: &cloudformation.StackSet{
								TemplateBody: aws.String(string(body)),
							},
						}, nil
					},
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
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

func TestRemoveAppFromProject(t *testing.T) {
	mockProject := &archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}

	tests := map[string]struct {
		app string

		mockDescribeStackSet          func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error)
		mockUpdateStackSet            func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error)
		mockDescribeStackSetOperation func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error)

		want error
	}{
		"should remove input app from the stack set": {
			app: "test",
			mockDescribeStackSet: func(t *testing.T, in *cloudformation.DescribeStackSetInput) (*cloudformation.DescribeStackSetOutput, error) {
				body, err := yaml.Marshal(stack.DeployedProjectMetadata{Metadata: stack.ProjectResourcesConfig{
					Apps:    []string{"test", "firsttest"},
					Version: 1,
				}})
				require.NoError(t, err)
				return &cloudformation.DescribeStackSetOutput{
					StackSet: &cloudformation.StackSet{
						TemplateBody: aws.String(string(body)),
					},
				}, nil
			},
			mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
				require.NotZero(t, *in.TemplateBody, "TemplateBody should not be empty")
				configToDeploy, err := stack.ProjectConfigFrom(in.TemplateBody)
				require.NoError(t, err)
				require.ElementsMatch(t, []string{"firsttest"}, configToDeploy.Apps)
				require.Empty(t, configToDeploy.Accounts, "config account list should be empty")
				require.Equal(t, 2, configToDeploy.Version)

				return &cloudformation.UpdateStackSetOutput{
					OperationId: aws.String("2"),
				}, nil
			},
			mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
				return &cloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &cloudformation.StackSetOperation{
						Status: aws.String(cloudformation.StackSetOperationStatusSucceeded),
					},
				}, nil
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				client: mockCloudFormation{
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
				client: &mockCloudFormation{
					t: t,
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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
				client: &mockCloudFormation{
					t: t,
					mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
						// First, say the status is running. Then during the next call, set the status to succeeded.
						status := "RUNNING"
						if !waitingForOperation {
							status = "SUCCEEDED"
						}
						waitingForOperation = false
						return &cloudformation.DescribeStackSetOperationOutput{
							StackSetOperation: &cloudformation.StackSetOperation{
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

func TestDeployProjectConfig_ErrWrapping(t *testing.T) {
	mockProject := &deploy.CreateProjectInput{Project: "project", AccountID: "12345"}

	testCases := map[string]struct {
		cf   CloudFormation
		want error
	}{
		"ErrCodeOperationIdAlreadyExistsException": {
			want: &ErrStackSetOutOfDate{projectName: mockProject.Project},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
						return nil, awserr.New(cloudformation.ErrCodeOperationIdAlreadyExistsException, "operation already exists", nil)
					},
				},
				box: boxWithTemplateFile(),
			},
		},
		"ErrCodeOperationInProgressException": {
			want: &ErrStackSetOutOfDate{projectName: mockProject.Project},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
						return nil, awserr.New(cloudformation.ErrCodeOperationInProgressException, "something is in progres", nil)
					},
				},
				box: boxWithTemplateFile(),
			},
		},
		"ErrCodeStaleRequestException": {
			want: &ErrStackSetOutOfDate{projectName: mockProject.Project},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockUpdateStackSet: func(t *testing.T, in *cloudformation.UpdateStackSetInput) (*cloudformation.UpdateStackSetOutput, error) {
						return nil, awserr.New(cloudformation.ErrCodeStaleRequestException, "something is stale", nil)
					},
				},
				box: boxWithTemplateFile(),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockProjectResources := stack.ProjectResourcesConfig{}
			got := tc.cf.deployProjectConfig(stack.NewProjectStackConfig(mockProject), &mockProjectResources)
			require.NotNil(t, got)
			require.True(t, errors.Is(tc.want, got), "Got %v but expected %v", got, tc.want)
		})
	}
}

func TestGetRegionalProjectResources(t *testing.T) {
	mockProject := archer.Project{Name: "project", AccountID: "12345"}

	testCases := map[string]struct {
		cf             CloudFormation
		clientProvider func(string) cloudformationiface.CloudFormationAPI
		wantedResource archer.ProjectRegionalResources
		want           error
	}{
		"should describe stack instances and convert to ProjectRegionalResources": {
			wantedResource: archer.ProjectRegionalResources{
				KMSKeyARN:      "arn:aws:kms:us-west-2:01234567890:key/0000",
				S3Bucket:       "tests3-bucket-us-west-2",
				Region:         "us-east-9",
				RepositoryURLs: map[string]string{},
			},
			clientProvider: func(region string) cloudformationiface.CloudFormationAPI {
				if region != "us-east-9" {
					t.FailNow()
				}
				return &mockCloudFormation{
					t: t,
					mockDescribeStacks: func(t *testing.T, input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
						require.Equal(t, "cross-region-stack", *input.StackName)
						return &cloudformation.DescribeStacksOutput{
								Stacks: []*cloudformation.Stack{mockValidProjectResourceStack()},
							},
							nil
					},
				}
			},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{
								&cloudformation.StackInstanceSummary{
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
			clientProvider: func(region string) cloudformationiface.CloudFormationAPI {
				if region != "us-east-9" {
					t.FailNow()
				}
				return &mockCloudFormation{
					t: t,
					mockDescribeStacks: func(t *testing.T, input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
						require.Equal(t, "cross-region-stack", *input.StackName)
						return nil, fmt.Errorf("error calling cloudformation")
					},
				}
			},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{
								&cloudformation.StackInstanceSummary{
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
				client: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return nil, fmt.Errorf("error")
					},
				},
				box: boxWithTemplateFile(),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.cf.regionalClientProvider = mockClientBuilder{
				mockClient: tc.clientProvider,
			}
			got, err := tc.cf.GetRegionalProjectResources(&mockProject)
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

func TestGetProjectResourcesByRegion(t *testing.T) {
	mockProject := archer.Project{Name: "project", AccountID: "12345"}

	testCases := map[string]struct {
		cf             CloudFormation
		clientProvider func(string) cloudformationiface.CloudFormationAPI
		wantedResource archer.ProjectRegionalResources
		region         string
		want           error
	}{
		"should describe stack instances and convert to ProjectRegionalResources": {
			wantedResource: archer.ProjectRegionalResources{
				KMSKeyARN:      "arn:aws:kms:us-west-2:01234567890:key/0000",
				S3Bucket:       "tests3-bucket-us-west-2",
				Region:         "us-east-9",
				RepositoryURLs: map[string]string{},
			},
			region: "us-east-9",
			clientProvider: func(region string) cloudformationiface.CloudFormationAPI {
				if region != "us-east-9" {
					t.FailNow()
				}
				return &mockCloudFormation{
					t: t,
					mockDescribeStacks: func(t *testing.T, input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
						require.Equal(t, "cross-region-stack", *input.StackName)
						return &cloudformation.DescribeStacksOutput{
								Stacks: []*cloudformation.Stack{mockValidProjectResourceStack()},
							},
							nil
					},
				}
			},
			cf: CloudFormation{
				client: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						require.Equal(t, "us-east-9", *in.StackInstanceRegion)
						return &cloudformation.ListStackInstancesOutput{
							Summaries: []*cloudformation.StackInstanceSummary{
								&cloudformation.StackInstanceSummary{
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
				client: &mockCloudFormation{
					t: t,
					mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
						return &cloudformation.ListStackInstancesOutput{}, nil
					},
				},
				box: boxWithTemplateFile(),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.cf.regionalClientProvider = mockClientBuilder{
				mockClient: tc.clientProvider,
			}
			got, err := tc.cf.GetProjectResourcesByRegion(&mockProject, tc.region)
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

func TestDelegateDNSPermissions(t *testing.T) {

	testCases := map[string]struct {
		project      *archer.Project
		accountID    string
		mockCFClient func() *mockCloudFormation
		want         error
	}{
		"Calls Update Stack": {
			project: &archer.Project{
				AccountID: "1234",
				Name:      "project",
				Domain:    "amazon.com",
			},
			mockCFClient: func() *mockCloudFormation {
				return &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						require.Equal(t, 6, len(in.Parameters))
						return &cloudformation.CreateChangeSetOutput{
							StackId: aws.String("stackname"),
						}, nil
					},
					mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error) {
						return nil, nil
					},
					mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
						stack := mockProjectRolesStack("stackname", map[string]string{
							"ProjectDNSDelegatedAccounts": "1234",
						})
						return &cloudformation.DescribeStacksOutput{Stacks: []*cloudformation.Stack{stack}}, nil
					},
					mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
						return &cloudformation.DescribeChangeSetOutput{
							ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return nil
					},
					mockWaitUntilStackUpdateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
						return nil
					},
				}
			},
		},

		"Returns error from Describe Stack": {
			project: &archer.Project{
				AccountID: "1234",
				Name:      "project",
				Domain:    "amazon.com",
			},
			want: fmt.Errorf("getting existing project infrastructure stack: error"),
			mockCFClient: func() *mockCloudFormation {
				return &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						require.Equal(t, len(in.Parameters), 4)
						return &cloudformation.CreateChangeSetOutput{
							StackId: aws.String("stackname"),
						}, nil
					},
					mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error) {
						return nil, nil
					},
					mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
						return nil, fmt.Errorf("error")
					},
				}
			},
		},
		"Returns error from Update Stack": {
			project: &archer.Project{
				AccountID: "1234",
				Name:      "project",
				Domain:    "amazon.com",
			},
			want: fmt.Errorf("updating project to allow DNS delegation: failed to execute changeSet name=, stackID=stackname: error"),
			mockCFClient: func() *mockCloudFormation {
				return &mockCloudFormation{
					t: t,
					mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
						require.Equal(t, 6, len(in.Parameters))
						return &cloudformation.CreateChangeSetOutput{
							StackId: aws.String("stackname"),
						}, nil
					},
					mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (*cloudformation.ExecuteChangeSetOutput, error) {
						return nil, fmt.Errorf("error")
					},
					mockDescribeStacks: func(t *testing.T, in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
						stack := mockProjectRolesStack("stackname", map[string]string{
							"ProjectDNSDelegatedAccounts": "1234",
						})
						return &cloudformation.DescribeStacksOutput{Stacks: []*cloudformation.Stack{stack}}, nil
					},
					mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
						return &cloudformation.DescribeChangeSetOutput{
							ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
						}, nil
					},
					mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {
						return nil
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				client: tc.mockCFClient(),
				box:    templates.Box(),
			}
			got := cf.DelegateDNSPermissions(tc.project, tc.accountID)

			if tc.want != nil {
				require.EqualError(t, tc.want, got.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

// Useful for mocking a successfully deployed stack
func getMockSuccessfulDeployCFClient(t *testing.T, stackName string) *mockCloudFormation {
	times := 0
	return &mockCloudFormation{
		t: t,
		mockCreateChangeSet: func(t *testing.T, in *cloudformation.CreateChangeSetInput) (*cloudformation.CreateChangeSetOutput, error) {
			return &cloudformation.CreateChangeSetOutput{
				Id:      aws.String("changesetID"),
				StackId: aws.String(stackName),
			}, nil
		},
		mockWaitUntilChangeSetCreateCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) error {

			return nil
		},
		mockWaitUntilStackCreateCompleteWithContext: func(t *testing.T, input *cloudformation.DescribeStacksInput) error {
			return nil
		},
		mockDescribeChangeSet: func(t *testing.T, in *cloudformation.DescribeChangeSetInput) (*cloudformation.DescribeChangeSetOutput, error) {
			return &cloudformation.DescribeChangeSetOutput{
				ExecutionStatus: aws.String(cloudformation.ExecutionStatusAvailable),
			}, nil
		},
		mockExecuteChangeSet: func(t *testing.T, in *cloudformation.ExecuteChangeSetInput) (output *cloudformation.ExecuteChangeSetOutput, e error) {
			return nil, nil
		},
		mockDescribeStacks: func(t *testing.T, input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
			if times == 0 {
				times = times + 1
				return &cloudformation.DescribeStacksOutput{}, nil
			}

			return &cloudformation.DescribeStacksOutput{
				Stacks: []*cloudformation.Stack{
					&cloudformation.Stack{
						StackId: aws.String(fmt.Sprintf("arn:aws:cloudformation:eu-west-3:902697171733:stack/%s", stackName)),
					},
				},
			}, nil
		},
	}
}

type mockClientBuilder struct {
	mockClient func(string) cloudformationiface.CloudFormationAPI
}

func (cf mockClientBuilder) Client(region string) cloudformationiface.CloudFormationAPI {
	return cf.mockClient(region)
}

func mockValidProjectResourceStack() *cloudformation.Stack {
	return mockProjectResourceStack("stack", map[string]string{
		"KMSKeyARN":      "arn:aws:kms:us-west-2:01234567890:key/0000",
		"PipelineBucket": "tests3-bucket-us-west-2",
	})
}

func mockProjectResourceStack(stackArn string, outputs map[string]string) *cloudformation.Stack {
	outputList := []*cloudformation.Output{}
	for key, val := range outputs {
		outputList = append(outputList, &cloudformation.Output{
			OutputKey:   aws.String(key),
			OutputValue: aws.String(val),
		})
	}

	return &cloudformation.Stack{
		StackId: aws.String(stackArn),
		Outputs: outputList,
	}
}

func mockProjectRolesStack(stackArn string, parameters map[string]string) *cloudformation.Stack {
	parametersList := []*cloudformation.Parameter{}
	for key, val := range parameters {
		parametersList = append(parametersList, &cloudformation.Parameter{
			ParameterKey:   aws.String(key),
			ParameterValue: aws.String(val),
		})
	}

	return &cloudformation.Stack{
		StackId:     aws.String(stackArn),
		StackStatus: aws.String("UPDATE_COMPLETE"),
		Parameters:  parametersList,
	}
}

func boxWithProjectTemplate() packd.Box {
	box := packd.NewMemoryBox()

	box.AddString("project/cf.yml", mockTemplate)

	return box
}

func TestDeleteProject(t *testing.T) {
	tests := map[string]struct {
		projectName string

		mockListStackInstances                      func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error)
		mockDeleteStackInstances                    func(t *testing.T, in *cloudformation.DeleteStackInstancesInput) (*cloudformation.DeleteStackInstancesOutput, error)
		mockDeleteStackSet                          func(t *testing.T, in *cloudformation.DeleteStackSetInput) (*cloudformation.DeleteStackSetOutput, error)
		mockDeleteStack                             func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)
		mockDescribeStackSetOperation               func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error)
		mockWaitUntilStackDeleteCompleteWithContext func(t *testing.T, in *cloudformation.DescribeStacksInput) error

		want error
	}{
		"should return nil given happy path": {
			projectName: "testProject",
			mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
				return &cloudformation.ListStackInstancesOutput{
					Summaries: []*cloudformation.StackInstanceSummary{
						&cloudformation.StackInstanceSummary{
							Region:  aws.String("us-west-2"),
							Account: aws.String("12345"),
						},
					},
				}, nil
			},
			mockDeleteStackInstances: func(t *testing.T, in *cloudformation.DeleteStackInstancesInput) (*cloudformation.DeleteStackInstancesOutput, error) {
				require.Equal(t, 1, len(in.Accounts))
				require.Equal(t, 1, len(in.Regions))
				require.Equal(t, "12345", aws.StringValue(in.Accounts[0]))
				require.Equal(t, "us-west-2", aws.StringValue(in.Regions[0]))
				return &cloudformation.DeleteStackInstancesOutput{
					OperationId: aws.String("operationId"),
				}, nil
			},
			mockDeleteStackSet: func(t *testing.T, in *cloudformation.DeleteStackSetInput) (*cloudformation.DeleteStackSetOutput, error) {
				return &cloudformation.DeleteStackSetOutput{}, nil
			},
			mockDeleteStack: func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
				return &cloudformation.DeleteStackOutput{}, nil
			},
			mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
				require.Equal(t, "operationId", aws.StringValue(in.OperationId))
				return &cloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &cloudformation.StackSetOperation{
						Status: aws.String("SUCCEEDED"),
					},
				}, nil
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				return nil
			},
			want: nil,
		},

		"should return nil if stackset has already been deleted before running": {
			projectName: "testProject",
			mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
				return nil, awserr.New(cloudformation.ErrCodeStackSetNotFoundException, "StackSetNotFoundException", nil)
			},
			mockDeleteStackInstances: func(t *testing.T, in *cloudformation.DeleteStackInstancesInput) (*cloudformation.DeleteStackInstancesOutput, error) {
				t.FailNow()
				return nil, nil
			},
			mockDeleteStackSet: func(t *testing.T, in *cloudformation.DeleteStackSetInput) (*cloudformation.DeleteStackSetOutput, error) {
				t.FailNow()
				return nil, nil
			},
			mockDeleteStack: func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
				return &cloudformation.DeleteStackOutput{}, nil
			},
			mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
				t.FailNow()
				return nil, nil
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				return nil
			},
			want: nil,
		},
		"should return nil if stackset is deleted after stack instances are created (edge case)": {
			projectName: "testProject",
			mockListStackInstances: func(t *testing.T, in *cloudformation.ListStackInstancesInput) (*cloudformation.ListStackInstancesOutput, error) {
				return &cloudformation.ListStackInstancesOutput{
					Summaries: []*cloudformation.StackInstanceSummary{
						&cloudformation.StackInstanceSummary{
							Region:  aws.String("us-west-2"),
							Account: aws.String("12345"),
						},
					},
				}, nil
			},
			mockDeleteStackInstances: func(t *testing.T, in *cloudformation.DeleteStackInstancesInput) (*cloudformation.DeleteStackInstancesOutput, error) {
				require.Equal(t, 1, len(in.Accounts))
				require.Equal(t, 1, len(in.Regions))
				require.Equal(t, "12345", aws.StringValue(in.Accounts[0]))
				require.Equal(t, "us-west-2", aws.StringValue(in.Regions[0]))
				return &cloudformation.DeleteStackInstancesOutput{
					OperationId: aws.String("operationId"),
				}, nil
			},
			mockDeleteStackSet: func(t *testing.T, in *cloudformation.DeleteStackSetInput) (*cloudformation.DeleteStackSetOutput, error) {
				return nil, awserr.New(cloudformation.ErrCodeStackSetNotFoundException, "StackSetNotFoundException", nil)
			},
			mockDeleteStack: func(t *testing.T, in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
				return &cloudformation.DeleteStackOutput{}, nil
			},
			mockDescribeStackSetOperation: func(t *testing.T, in *cloudformation.DescribeStackSetOperationInput) (*cloudformation.DescribeStackSetOperationOutput, error) {
				require.Equal(t, "operationId", aws.StringValue(in.OperationId))
				return &cloudformation.DescribeStackSetOperationOutput{
					StackSetOperation: &cloudformation.StackSetOperation{
						Status: aws.String("SUCCEEDED"),
					},
				}, nil
			},
			mockWaitUntilStackDeleteCompleteWithContext: func(t *testing.T, in *cloudformation.DescribeStacksInput) error {
				return nil
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cf := CloudFormation{
				// TODO: replace this custom mock client with gomock.
				client: &mockCloudFormation{
					t:                        t,
					mockDeleteStackInstances: test.mockDeleteStackInstances,
					mockDeleteStackSet:       test.mockDeleteStackSet,
					mockDeleteStack:          test.mockDeleteStack,
					mockListStackInstances:   test.mockListStackInstances,
					mockWaitUntilStackDeleteCompleteWithContext: test.mockWaitUntilStackDeleteCompleteWithContext,
					mockDescribeStackSetOperation:               test.mockDescribeStackSetOperation,
				},
			}

			got := cf.DeleteProject(test.projectName)

			require.Equal(t, test.want, got)
		})
	}
}
