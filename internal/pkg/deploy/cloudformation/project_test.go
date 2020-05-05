// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/aws/aws-sdk-go/aws"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCloudFormation_DeployProject(t *testing.T) {
	mockApp := &deploy.CreateProjectInput{
		Project:   "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		mockStack    func(ctrl *gomock.Controller) cfnClient
		mockStackSet func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		want         error
	}{
		"Infrastructure Roles Stack Fails": {
			mockStack: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(errors.New("error creating stack"))
				return m
			},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				return nil
			},
			want: errors.New("error creating stack"),
		},
		"Infrastructure Roles Stack Already Exists": {
			mockStack: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{})
				return m
			},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				return m
			},
		},
		"Infrastructure Roles StackSet Created": {
			mockStack: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(nil)
				return m
			},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Do(func(name, _ string, _, _, _, _ stackset.CreateOrUpdateOption) {
						require.Equal(t, "testproject-infrastructure", name)
					})
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
				cfnClient:       tc.mockStack(ctrl),
				projectStackSet: tc.mockStackSet(t, ctrl),
				box:             templates.Box(),
			}

			// WHEN
			got := cf.DeployProject(mockApp)

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
	mockApp := archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		mockStackSet func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		project      *archer.Project
		env          *archer.Environment
		want         error
	}{
		"with no existing deployments and adding an env": {
			project: &mockApp,
			env:     &archer.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Do(func(_, _ string, op, _, _, _, _ stackset.CreateOrUpdateOption) {
						actual := &sdkcloudformation.UpdateStackSetInput{}
						op(actual)
						wanted := &sdkcloudformation.UpdateStackSetInput{}
						stackset.WithOperationID("1")(wanted)
						require.Equal(t, actual, wanted)
					})
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{}, nil)
				m.EXPECT().CreateInstancesAndWait(gomock.Any(), []string{"1234"}, []string{"us-west-2"})
				return m
			},
		},
		"with no new account ID added": {
			project: &mockApp,
			env:     &archer.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{
					Metadata: stack.AppResourcesConfig{
						Accounts: []string{"1234"},
					},
				})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{}, nil)
				m.EXPECT().CreateInstancesAndWait(gomock.Any(), []string{"1234"}, []string{"us-west-2"}).Return(nil)
				return m
			},
		},
		"with existing stack instances in same region but different account (no new stack instances, but update stackset)": {
			project: &mockApp,
			env:     &archer.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{
					Metadata: stack.AppResourcesConfig{
						Accounts: []string{"1234"},
						Version:  1,
					},
				})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Do(func(_, _ string, op, _, _, _, _ stackset.CreateOrUpdateOption) {
						actual := &sdkcloudformation.UpdateStackSetInput{}
						op(actual)
						wanted := &sdkcloudformation.UpdateStackSetInput{}
						stackset.WithOperationID("2")(wanted)
						require.Equal(t, actual, wanted)
					})
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						Account: "1234",
					},
				}, nil)
				m.EXPECT().CreateInstancesAndWait(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
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
				projectStackSet: tc.mockStackSet(t, ctrl),
				box:             templates.Box(),
			}
			got := cf.AddEnvToProject(tc.project, tc.env)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_AddPipelineResourcesToProject(t *testing.T) {
	mockApp := archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		project             *archer.Project
		mockStackSet        func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		getRegionFromClient func(client cloudformationiface.CloudFormationAPI) (string, error)
		expectedErr         error
	}{
		"with no existing account nor environment, add pipeline supporting resources": {
			project: &mockApp,
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{}, nil)
				m.EXPECT().CreateInstancesAndWait(gomock.Any(), []string{"1234"}, []string{"us-west-2"}).Return(nil)
				return m
			},
			getRegionFromClient: func(client cloudformationiface.CloudFormationAPI) (string, error) {
				return "us-west-2", nil
			},
		},
		"with existing account and existing environment in a region, should not add pipeline supporting resources": {
			project: &mockApp,
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						Account: mockApp.AccountID,
					},
				}, nil)
				m.EXPECT().CreateInstancesAndWait(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				return m
			},
			getRegionFromClient: func(client cloudformationiface.CloudFormationAPI) (string, error) {
				return "us-west-2", nil
			},
		},
	}

	actual := getRegionFromClient // FIXME refactor using defer func
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := CloudFormation{
				projectStackSet: tc.mockStackSet(t, ctrl),
				box:             templates.Box(),
			}
			getRegionFromClient = tc.getRegionFromClient

			got := cf.AddPipelineResourcesToProject(tc.project, "us-west-2")

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
	mockApp := archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		project      *archer.Project
		app          string
		mockStackSet func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		want         error
	}{
		"with no existing deployments and adding an app": {
			project: &mockApp,
			app:     "TestApp",
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Do(func(_, template string, _, _, _, _, _ stackset.CreateOrUpdateOption) {
						configToDeploy, err := stack.AppConfigFrom(&template)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{"TestApp"}, configToDeploy.Services)
						require.Empty(t, configToDeploy.Accounts, "there should be no new accounts to deploy")
						require.Equal(t, 1, configToDeploy.Version)
					})
				return m
			},
		},
		"with new app to existing project with existing apps": {
			project: &mockApp,
			app:     "test",
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{Metadata: stack.AppResourcesConfig{
					Services: []string{"firsttest"},
					Version:  1,
				}})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Do(func(_, template string, _, _, _, _, _ stackset.CreateOrUpdateOption) {
						configToDeploy, err := stack.AppConfigFrom(&template)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{"test", "firsttest"}, configToDeploy.Services)
						require.Empty(t, configToDeploy.Accounts, "there should be no new accounts to deploy")
						require.Equal(t, 2, configToDeploy.Version)

					})
				return m
			},
		},
		"with existing app to existing project with existing apps": {
			project: &mockApp,
			app:     "test",
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{Metadata: stack.AppResourcesConfig{
					Services: []string{"test"},
					Version:  1,
				}})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := CloudFormation{
				projectStackSet: tc.mockStackSet(t, ctrl),
				box:             templates.Box(),
			}

			got := cf.AddAppToProject(tc.project, tc.app)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_RemoveAppFromProject(t *testing.T) {
	mockApp := &archer.Project{
		Name:      "testproject",
		AccountID: "1234",
	}

	tests := map[string]struct {
		app          string
		mockStackSet func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		want         error
	}{
		"should remove input app from the stack set": {
			app: "test",

			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{Metadata: stack.AppResourcesConfig{
					Services: []string{"test", "firsttest"},
					Version:  1,
				}})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().UpdateAndWait(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Do(func(_, template string, _, _, _, _, _ stackset.CreateOrUpdateOption) {
						configToDeploy, err := stack.AppConfigFrom(&template)
						require.NoError(t, err)
						require.ElementsMatch(t, []string{"firsttest"}, configToDeploy.Services)
						require.Empty(t, configToDeploy.Accounts, "config account list should be empty")
						require.Equal(t, 2, configToDeploy.Version)
					})
				return m
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := CloudFormation{
				projectStackSet: tc.mockStackSet(t, ctrl),
				box:             templates.Box(),
			}

			got := cf.RemoveAppFromProject(mockApp, tc.app)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestCloudFormation_GetRegionalProjectResources(t *testing.T) {
	mockApp := archer.Project{Name: "project", AccountID: "12345"}

	testCases := map[string]struct {
		createRegionalMockClient func(ctrl *gomock.Controller) cfnClient
		mockStackSet             func(t *testing.T, ctrl *gomock.Controller) stackSetClient
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
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any(), gomock.Any()).
					Return([]stackset.InstanceSummary{
						{
							StackID: "cross-region-stack",
							Region:  "us-east-9",
						},
					}, nil).
					Do(func(_ string, opt stackset.InstanceSummariesOption) {
						wanted := &sdkcloudformation.ListStackInstancesInput{
							StackInstanceAccount: aws.String("12345"),
						}
						actual := &sdkcloudformation.ListStackInstancesInput{}
						opt(actual)
						require.Equal(t, wanted, actual)
					})
				return m
			},
		},
		"should propagate describe errors": {
			want: fmt.Errorf("describing project resources: getting outputs for stack cross-region-stack in region us-east-9: error calling cloudformation"),
			createRegionalMockClient: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("cross-region-stack").Return(nil, errors.New("error calling cloudformation"))
				return m
			},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{
					{
						StackID: "cross-region-stack",
						Region:  "us-east-9",
					},
				}, nil)
				return m
			},
		},

		"should propagate list stack instances errors": {
			want: fmt.Errorf("describing project resources: error"),
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
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
				regionalClient: func(region string) cfnClient {
					return tc.createRegionalMockClient(ctrl)
				},
				projectStackSet: tc.mockStackSet(t, ctrl),
				box:             boxWithTemplateFile(),
			}

			// WHEN
			got, err := cf.GetRegionalProjectResources(&mockApp)

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
	mockApp := archer.Project{Name: "project", AccountID: "12345"}

	testCases := map[string]struct {
		createRegionalMockClient func(ctrl *gomock.Controller) cfnClient
		mockStackSet             func(t *testing.T, ctrl *gomock.Controller) stackSetClient
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
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]stackset.InstanceSummary{
						{
							StackID: "cross-region-stack",
							Region:  "us-east-9",
						},
					}, nil).
					Do(func(_ string, optAcc, optRegion stackset.InstanceSummariesOption) {
						wanted := &sdkcloudformation.ListStackInstancesInput{
							StackInstanceAccount: aws.String("12345"),
							StackInstanceRegion:  aws.String("us-east-9"),
						}
						actual := &sdkcloudformation.ListStackInstancesInput{}
						optAcc(actual)
						optRegion(actual)
						require.Equal(t, wanted, actual)
					})
				return m
			},
		},
		"should error when resources are found": {
			want:   fmt.Errorf("no regional resources for project project in region us-east-9 found"),
			region: "us-east-9",
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any(), gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{}, nil)
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
				regionalClient: func(region string) cfnClient {
					return tc.createRegionalMockClient(ctrl)
				},
				projectStackSet: tc.mockStackSet(t, ctrl),
				box:             boxWithTemplateFile(),
			}

			// WHEN
			got, err := cf.GetProjectResourcesByRegion(&mockApp, tc.region)

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
					"AppDNSDelegatedAccounts": "1234",
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
					"AppDNSDelegatedAccounts": "1234",
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
					"AppDNSDelegatedAccounts": "1234",
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
		projectName  string
		createMock   func(ctrl *gomock.Controller) cfnClient
		mockStackSet func(ctrl *gomock.Controller) stackSetClient

		want error
	}{
		"should delete stackset and then infrastructure roles": {
			projectName: "testProject",
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().DeleteAndWait("testProject-infrastructure-roles").Return(nil)
				return m
			},
			mockStackSet: func(ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Delete(gomock.Any()).Return(nil)
				return m
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cf := CloudFormation{
				cfnClient:       tc.createMock(ctrl),
				projectStackSet: tc.mockStackSet(ctrl),
			}

			// WHEN
			got := cf.DeleteProject(tc.projectName)

			// THEN
			require.Equal(t, tc.want, got)
		})
	}
}
