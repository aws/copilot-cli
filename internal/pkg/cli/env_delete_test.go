// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	stackdescr "github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var noopInitRuntimeClients = func(opts *deleteEnvOpts) error {
	return nil
}

func TestDeleteEnvOpts_Validate(t *testing.T) {
	const (
		testAppName = "phonetool"
		testEnvName = "test"
	)
	testCases := map[string]struct {
		inAppName string
		inEnv     string
		mockStore func(ctrl *gomock.Controller) *mocks.MockenvironmentStore

		wantedError error
	}{
		"failed to retrieve environment from store": {
			inAppName: testAppName,
			inEnv:     testEnvName,
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvironmentStore {
				envStore := mocks.NewMockenvironmentStore(ctrl)
				envStore.EXPECT().GetEnvironment(testAppName, testEnvName).Return(nil, errors.New("some error"))
				return envStore
			},
			wantedError: errors.New("get environment test configuration from app phonetool: some error"),
		},
		"environment exists": {
			inAppName: testAppName,
			inEnv:     testEnvName,
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvironmentStore {
				envStore := mocks.NewMockenvironmentStore(ctrl)
				envStore.EXPECT().GetEnvironment(testAppName, testEnvName).Return(&config.Environment{}, nil)
				return envStore
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := &deleteEnvOpts{
				deleteEnvVars: deleteEnvVars{
					name:    tc.inEnv,
					appName: tc.inAppName,
				},
				store: tc.mockStore(ctrl),
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestDeleteEnvOpts_Ask(t *testing.T) {
	const (
		testApp = "phonetool"
		testEnv = "test"
	)
	testCases := map[string]struct {
		inAppName          string
		inEnvName          string
		inSkipConfirmation bool

		mockDependencies func(ctrl *gomock.Controller, o *deleteEnvOpts)

		wantedEnvName string
		wantedError   error
	}{
		"prompts for all required flags": {
			inSkipConfirmation: false,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockSelector := mocks.NewMockconfigSelector(ctrl)
				mockSelector.EXPECT().Application(envDeleteAppNamePrompt, envDeleteAppNameHelpPrompt, gomock.Any()).
					Return(testApp, nil)
				mockSelector.EXPECT().Environment(envDeleteNamePrompt, "", testApp).Return(testEnv, nil)

				mockPrompter := mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, testEnv, testApp), gomock.Any(), gomock.Any()).Return(true, nil)

				o.sel = mockSelector
				o.prompt = mockPrompter
			},
			wantedEnvName: testEnv,
		},
		"error if fail to select applications": {
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {
				mockSelector := mocks.NewMockconfigSelector(ctrl)
				mockSelector.EXPECT().Application(envDeleteAppNamePrompt, envDeleteAppNameHelpPrompt, gomock.Any()).
					Return("", errors.New("some error"))

				o.sel = mockSelector
			},
			wantedError: fmt.Errorf("ask for application: some error"),
		},
		"wraps error from prompting for confirmation": {
			inSkipConfirmation: false,
			inAppName:          testApp,
			inEnvName:          testEnv,
			mockDependencies: func(ctrl *gomock.Controller, o *deleteEnvOpts) {

				mockPrompter := mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, testEnv, testApp), gomock.Any(), gomock.Any()).Return(false, errors.New("some error"))

				o.prompt = mockPrompter
			},

			wantedError: errors.New("confirm to delete environment test: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := &deleteEnvOpts{
				deleteEnvVars: deleteEnvVars{
					name:             tc.inEnvName,
					appName:          tc.inAppName,
					skipConfirmation: tc.inSkipConfirmation,
				},
			}
			tc.mockDependencies(ctrl, opts)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.Equal(t, tc.wantedEnvName, opts.name)
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestDeleteEnvOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		given func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts

		mockRG             func(ctrl *gomock.Controller) *mocks.MockresourceGetter
		mockDescriber      func(ctrl *gomock.Controller) *mocks.MockstackDescriber
		mockProg           func(ctrl *gomock.Controller) *mocks.Mockprogress
		mockDeploy         func(ctrl *gomock.Controller) *mocks.MockenvironmentDeployer
		mockStore          func(ctrl *gomock.Controller) *mocks.MockenvironmentStore
		mockPipelineLister func(ctrl *gomock.Controller) *mocks.MockdeployedPipelineLister
		mockPipelineGetter func(ctrl *gomock.Controller) *mocks.MockpipelineGetter

		wantedError error
	}{
		"returns wrapped errors when failed to retrieve running services in the environment": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				m := mocks.NewMockresourceGetter(ctrl)
				m.EXPECT().GetResources(gomock.Any()).Return(nil, errors.New("some error"))

				return &deleteEnvOpts{
					rg:                 m,
					initRuntimeClients: noopInitRuntimeClients,
				}
			},
			wantedError: errors.New("find service cloudformation stacks: some error"),
		},
		"returns error when there are running services": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				m := mocks.NewMockresourceGetter(ctrl)
				m.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{
						{
							Tags: []*resourcegroupstaggingapi.Tag{
								{
									Key:   aws.String(deploy.ServiceTagKey),
									Value: aws.String("frontend"),
								},
								{
									Key:   aws.String(deploy.ServiceTagKey),
									Value: aws.String("backend"),
								},
							},
						},
					},
				}, nil)

				return &deleteEnvOpts{
					deleteEnvVars: deleteEnvVars{
						appName: "phonetool",
						name:    "test",
					},
					rg:                 m,
					initRuntimeClients: noopInitRuntimeClients,
				}
			},

			wantedError: errors.New(`service "frontend, backend" still exist within the environment test`),
		},
		"returns error when more pipelines are using the env": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{},
				}, nil)

				lister := mocks.NewMockdeployedPipelineLister(ctrl)
				lister.EXPECT().ListDeployedPipelines("phonetool").Return([]deploy.Pipeline{
					{
						ResourceName: "mockResourceName",
						Name:         "mockName",
					},
				}, nil)

				getter := mocks.NewMockpipelineGetter(ctrl)
				getter.EXPECT().GetPipeline("mockResourceName").Return(&codepipeline.Pipeline{
					Stages: []*codepipeline.Stage{
						{
							Name: "DeployTo-test",
						},
					},
				}, nil)

				return &deleteEnvOpts{
					deleteEnvVars: deleteEnvVars{
						appName: "phonetool",
						name:    "test",
					},
					rg:                     rg,
					deployedPipelineLister: lister,
					pipelineGetter:         getter,
					initRuntimeClients:     noopInitRuntimeClients,
				}
			},

			wantedError: errors.New(`environment "test" cannot be deleted because pipeline "mockName" depends on it`),
		},
		"returns wrapped error when environment stack cannot be updated to retain roles": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)

				lister := mocks.NewMockdeployedPipelineLister(ctrl)
				lister.EXPECT().ListDeployedPipelines("phonetool").Return([]deploy.Pipeline{}, nil)

				prog := mocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(gomock.Any())

				deployer := mocks.NewMockenvironmentDeployer(ctrl)
				deployer.EXPECT().Template(gomock.Any()).Return(`
Resources:
  EnableLongARNFormatAction:
    Type: Custom::EnableLongARNFormatFunction
    DependsOn:
    - EnableLongARNFormatFunction
    Properties:
    ServiceToken: !GetAtt EnableLongARNFormatFunction.Arn
  CloudformationExecutionRole:
    Type: AWS::IAM::Role
  EnvironmentManagerRole:
    Type: AWS::IAM::Role
`, nil)
				deployer.EXPECT().UpdateEnvironmentTemplate(
					"phonetool",
					"test",
					`
Resources:
  EnableLongARNFormatAction:
    Type: Custom::EnableLongARNFormatFunction
    DependsOn:
    - EnableLongARNFormatFunction
    Properties:
    ServiceToken: !GetAtt EnableLongARNFormatFunction.Arn
  CloudformationExecutionRole:
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
  EnvironmentManagerRole:
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
`, "arn").Return(errors.New("some error"))

				prog.EXPECT().Stop(log.Serror("Failed to retain IAM roles for the \"test\" environment\n"))

				return &deleteEnvOpts{
					deleteEnvVars: deleteEnvVars{
						appName: "phonetool",
						name:    "test",
					},
					rg:                     rg,
					deployer:               deployer,
					prog:                   prog,
					deployedPipelineLister: lister,
					envConfig: &config.Environment{
						ExecutionRoleARN: "arn",
					},
					initRuntimeClients: noopInitRuntimeClients,
				}
			},
			wantedError: errors.New("update environment stack to retain environment roles: some error"),
		},
		"returns wrapped error when stack cannot be deleted": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)

				lister := mocks.NewMockdeployedPipelineLister(ctrl)
				lister.EXPECT().ListDeployedPipelines("phonetool").Return([]deploy.Pipeline{}, nil)

				prog := mocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(gomock.Any()).Times(1)

				deployer := mocks.NewMockenvironmentDeployer(ctrl)
				deployer.EXPECT().Template(gomock.Any()).Return(`
Resources:
  CloudformationExecutionRole:
    DeletionPolicy: Retain
  EnvironmentManagerRole:
    # An IAM Role to manage resources in your environment
    DeletionPolicy: Retain`, nil)

				descr := mocks.NewMockstackDescriber(ctrl)
				descr.EXPECT().Resources().Return([]*stackdescr.Resource{}, nil)

				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)

				deployer.EXPECT().DeleteEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))

				prog.EXPECT().Stop(gomock.Any()).Times(1)

				return &deleteEnvOpts{
					deleteEnvVars: deleteEnvVars{
						appName: "phonetool",
						name:    "test",
					},
					rg:                     rg,
					envStackDescriber:      descr,
					deployedPipelineLister: lister,
					deployer:               deployer,
					prog:                   prog,
					envConfig:              &config.Environment{},
					initRuntimeClients:     noopInitRuntimeClients,
				}
			},

			wantedError: errors.New("delete environment test stack: some error"),
		},
		"returns wrapped error removing env from app": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				app := &config.Application{
					Name: "phonetool",
				}
				mockEnv := config.Environment{
					App:              "phonetool",
					Name:             "test",
					Region:           "us-west-2",
					ExecutionRoleARN: "execARN",
					ManagerRoleARN:   "managerRoleARN",
					AccountID:        "1234",
				}
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)

				lister := mocks.NewMockdeployedPipelineLister(ctrl)
				lister.EXPECT().ListDeployedPipelines("phonetool").Return([]deploy.Pipeline{}, nil)

				prog := mocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(gomock.Any()).AnyTimes()

				deployer := mocks.NewMockenvironmentDeployer(ctrl)
				deployer.EXPECT().Template(stack.NameForEnv("phonetool", "test")).Return(`
Resources:
  CloudformationExecutionRole:
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
  EnvironmentManagerRole:
    # An IAM Role to manage resources in your environment
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
`, nil)

				descr := mocks.NewMockstackDescriber(ctrl)
				descr.EXPECT().Resources().Return([]*stackdescr.Resource{}, nil)

				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)

				deployer.EXPECT().DeleteEnvironment("phonetool", "test", "execARN").Return(nil)

				store := mocks.NewMockenvironmentStore(ctrl)
				store.EXPECT().ListEnvironments("phonetool").Return([]*config.Environment{
					&mockEnv,
					{
						Name:      "prod",
						Region:    "us-west-2",
						AccountID: "5678",
					},
				}, nil)
				store.EXPECT().GetEnvironment("phonetool", "test").Return(&mockEnv, nil)
				store.EXPECT().GetApplication("phonetool").Return(app, nil)

				envDeleter := mocks.NewMockenvDeleterFromApp(ctrl)
				envDeleter.EXPECT().RemoveEnvFromApp(&cloudformation.RemoveEnvFromAppOpts{
					App:         app,
					EnvToDelete: &mockEnv,
					Environments: []*config.Environment{
						&mockEnv,
						{
							Name:      "prod",
							Region:    "us-west-2",
							AccountID: "5678",
						},
					},
				}).Return(errors.New("some error"))

				prog.EXPECT().Stop(gomock.Any()).AnyTimes()

				return &deleteEnvOpts{
					deleteEnvVars: deleteEnvVars{
						appName: "phonetool",
						name:    "test",
					},
					rg:                     rg,
					envStackDescriber:      descr,
					deployer:               deployer,
					prog:                   prog,
					deployedPipelineLister: lister,
					store:                  store,
					envDeleterFromApp:      envDeleter,
					initRuntimeClients:     noopInitRuntimeClients,
				}
			},
			wantedError: errors.New("remove environment test from application phonetool: some error"),
		},
		"success, but handles error when failed to retrieve environment stack": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				app := &config.Application{
					Name: "phonetool",
				}
				mockEnv := config.Environment{
					App:              "phonetool",
					Name:             "test",
					Region:           "us-west-2",
					ExecutionRoleARN: "execARN",
					ManagerRoleARN:   "managerRoleARN",
					AccountID:        "1234",
				}
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)

				lister := mocks.NewMockdeployedPipelineLister(ctrl)
				lister.EXPECT().ListDeployedPipelines("phonetool").Return([]deploy.Pipeline{}, nil)

				iam := mocks.NewMockroleDeleter(ctrl)

				prog := mocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(gomock.Any()).AnyTimes()

				deployer := mocks.NewMockenvironmentDeployer(ctrl)
				deployer.EXPECT().Template(stack.NameForEnv("phonetool", "test")).Return(`
Resources:
  CloudformationExecutionRole:
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
  EnvironmentManagerRole:
    # An IAM Role to manage resources in your environment
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
`, nil)

				rg.EXPECT().GetResources(gomock.Any()).Return(nil, errors.New("some error"))

				deployer.EXPECT().DeleteEnvironment("phonetool", "test", "execARN").Return(nil)

				store := mocks.NewMockenvironmentStore(ctrl)
				store.EXPECT().ListEnvironments("phonetool").Return([]*config.Environment{
					&mockEnv,
					{
						Name:      "prod",
						Region:    "us-west-2",
						AccountID: "5678",
					},
				}, nil)
				store.EXPECT().GetEnvironment("phonetool", "test").Return(&mockEnv, nil)
				store.EXPECT().GetApplication("phonetool").Return(app, nil)

				envDeleter := mocks.NewMockenvDeleterFromApp(ctrl)
				envDeleter.EXPECT().RemoveEnvFromApp(&cloudformation.RemoveEnvFromAppOpts{
					App:         app,
					EnvToDelete: &mockEnv,
					Environments: []*config.Environment{
						&mockEnv,
						{
							Name:      "prod",
							Region:    "us-west-2",
							AccountID: "5678",
						},
					},
				}).Return(nil)

				prog.EXPECT().Stop(gomock.Any()).AnyTimes()
				iam.EXPECT().DeleteRole(mockEnv.ExecutionRoleARN).Return(nil)
				iam.EXPECT().DeleteRole(mockEnv.ManagerRoleARN).Return(nil)

				store.EXPECT().DeleteEnvironment(mockEnv.App, mockEnv.Name).Return(nil)

				return &deleteEnvOpts{
					deleteEnvVars: deleteEnvVars{
						appName: "phonetool",
						name:    "test",
					},
					rg:                     rg,
					deployer:               deployer,
					prog:                   prog,
					deployedPipelineLister: lister,
					store:                  store,
					iam:                    iam,
					envDeleterFromApp:      envDeleter,
					initRuntimeClients:     noopInitRuntimeClients,
				}
			},
		},
		"success, but handles error caused by failure to empty bucket": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				app := &config.Application{
					Name: "phonetool",
				}
				mockEnv := config.Environment{
					App:              "phonetool",
					Name:             "test",
					Region:           "us-west-2",
					ExecutionRoleARN: "execARN",
					ManagerRoleARN:   "managerRoleARN",
					AccountID:        "1234",
				}
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)

				lister := mocks.NewMockdeployedPipelineLister(ctrl)
				lister.EXPECT().ListDeployedPipelines("phonetool").Return([]deploy.Pipeline{}, nil)

				iam := mocks.NewMockroleDeleter(ctrl)

				prog := mocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(gomock.Any()).AnyTimes()

				deployer := mocks.NewMockenvironmentDeployer(ctrl)
				deployer.EXPECT().Template(stack.NameForEnv("phonetool", "test")).Return(`
Resources:
  CloudformationExecutionRole:
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
  EnvironmentManagerRole:
    # An IAM Role to manage resources in your environment
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
`, nil)
				rg.EXPECT().GetResources(gomock.Any()).Return(nil, errors.New("some error"))

				deployer.EXPECT().DeleteEnvironment("phonetool", "test", "execARN").Return(nil)

				store := mocks.NewMockenvironmentStore(ctrl)
				store.EXPECT().ListEnvironments("phonetool").Return([]*config.Environment{
					&mockEnv,
					{
						Name:      "prod",
						Region:    "us-west-2",
						AccountID: "5678",
					},
				}, nil)
				store.EXPECT().GetEnvironment("phonetool", "test").Return(&mockEnv, nil)
				store.EXPECT().GetApplication("phonetool").Return(app, nil)

				envDeleter := mocks.NewMockenvDeleterFromApp(ctrl)
				envDeleter.EXPECT().RemoveEnvFromApp(&cloudformation.RemoveEnvFromAppOpts{
					App:         app,
					EnvToDelete: &mockEnv,
					Environments: []*config.Environment{
						&mockEnv,
						{
							Name:      "prod",
							Region:    "us-west-2",
							AccountID: "5678",
						},
					},
				}).Return(nil)

				prog.EXPECT().Stop(gomock.Any()).AnyTimes()
				iam.EXPECT().DeleteRole(mockEnv.ExecutionRoleARN).Return(nil)
				iam.EXPECT().DeleteRole(mockEnv.ManagerRoleARN).Return(nil)

				store.EXPECT().DeleteEnvironment(mockEnv.App, mockEnv.Name).Return(nil)

				return &deleteEnvOpts{
					deleteEnvVars: deleteEnvVars{
						appName: "phonetool",
						name:    "test",
					},
					rg:                     rg,
					deployer:               deployer,
					prog:                   prog,
					store:                  store,
					deployedPipelineLister: lister,
					iam:                    iam,
					envDeleterFromApp:      envDeleter,
					initRuntimeClients:     noopInitRuntimeClients,
				}
			},
		},
		"success": {
			given: func(t *testing.T, ctrl *gomock.Controller) *deleteEnvOpts {
				app := &config.Application{
					Name: "phonetool",
				}
				mockEnv := config.Environment{
					App:              "phonetool",
					Name:             "test",
					Region:           "us-west-2",
					ExecutionRoleARN: "execARN",
					ManagerRoleARN:   "managerRoleARN",
					AccountID:        "1234",
				}
				rg := mocks.NewMockresourceGetter(ctrl)
				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{}}, nil)

				lister := mocks.NewMockdeployedPipelineLister(ctrl)
				lister.EXPECT().ListDeployedPipelines("phonetool").Return([]deploy.Pipeline{}, nil)

				iam := mocks.NewMockroleDeleter(ctrl)

				prog := mocks.NewMockprogress(ctrl)
				prog.EXPECT().Start(gomock.Any()).AnyTimes()

				deployer := mocks.NewMockenvironmentDeployer(ctrl)
				deployer.EXPECT().Template(stack.NameForEnv("phonetool", "test")).Return(`
Resources:
  CloudformationExecutionRole:
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
  EnvironmentManagerRole:
    # An IAM Role to manage resources in your environment
    DeletionPolicy: Retain
    Type: AWS::IAM::Role
`, nil)

				descr := mocks.NewMockstackDescriber(ctrl)
				descr.EXPECT().Resources().Return(
					[]*stackdescr.Resource{
						{
							Type:       "AWS::S3::Bucket",
							LogicalID:  "ELBAccessLogsBucket",
							PhysicalID: "arn:aws:s3:::mockapp-mockenv-mockbucket",
						},
					}, nil,
				)

				rg.EXPECT().GetResources(gomock.Any()).Return(&resourcegroupstaggingapi.GetResourcesOutput{
					ResourceTagMappingList: []*resourcegroupstaggingapi.ResourceTagMapping{
						{
							ResourceARN: aws.String("arn:aws:s3:::mockapp-mockenv-mockbucket"),
							Tags: []*resourcegroupstaggingapi.Tag{
								{
									Key:   aws.String(stack.StackNameTagKey),
									Value: aws.String("mockapp-mockenv"),
								},
								{
									Key:   aws.String(deploy.EnvTagKey),
									Value: aws.String("mockenv"),
								},
								{
									Key:   aws.String(deploy.AppTagKey),
									Value: aws.String("mockapp"),
								},
							},
						},
					},
				}, nil)

				s3 := mocks.NewMockbucketEmptier(ctrl)
				s3.EXPECT().EmptyBucket(gomock.Any()).Return(nil)

				deployer.EXPECT().DeleteEnvironment("phonetool", "test", "execARN").Return(nil)

				store := mocks.NewMockenvironmentStore(ctrl)
				store.EXPECT().ListEnvironments("phonetool").Return([]*config.Environment{
					&mockEnv,
					{
						Name:      "prod",
						Region:    "us-west-2",
						AccountID: "5678",
					},
				}, nil)
				store.EXPECT().GetEnvironment("phonetool", "test").Return(&mockEnv, nil)
				store.EXPECT().GetApplication("phonetool").Return(app, nil)

				envDeleter := mocks.NewMockenvDeleterFromApp(ctrl)
				envDeleter.EXPECT().RemoveEnvFromApp(&cloudformation.RemoveEnvFromAppOpts{
					App:         app,
					EnvToDelete: &mockEnv,
					Environments: []*config.Environment{
						&mockEnv,
						{
							Name:      "prod",
							Region:    "us-west-2",
							AccountID: "5678",
						},
					},
				}).Return(nil)

				prog.EXPECT().Stop(gomock.Any()).AnyTimes()
				iam.EXPECT().DeleteRole(mockEnv.ExecutionRoleARN).Return(nil)
				iam.EXPECT().DeleteRole(mockEnv.ManagerRoleARN).Return(nil)

				store.EXPECT().DeleteEnvironment(mockEnv.App, mockEnv.Name).Return(nil)

				return &deleteEnvOpts{
					deleteEnvVars: deleteEnvVars{
						appName: "phonetool",
						name:    "test",
					},
					rg:                     rg,
					envStackDescriber:      descr,
					s3:                     s3,
					deployer:               deployer,
					prog:                   prog,
					store:                  store,
					deployedPipelineLister: lister,
					iam:                    iam,
					envDeleterFromApp:      envDeleter,
					initRuntimeClients:     noopInitRuntimeClients,
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			opts := tc.given(t, ctrl)

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
