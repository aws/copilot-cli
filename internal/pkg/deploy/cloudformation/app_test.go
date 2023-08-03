// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/cloudformationtest"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCloudFormation_DeployApp(t *testing.T) {
	mockApp := &deploy.CreateAppInput{
		Name:      "testapp",
		AccountID: "1234",
		Version:   "v1.29.0",
	}
	testCases := map[string]struct {
		mockStack    func(ctrl *gomock.Controller) cfnClient
		mockStackSet func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		region       string
		want         error
	}{
		"should return an error if infrastructure roles stack fails": {
			mockStack: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Create(gomock.Any()).Return("", errors.New("error creating stack"))
				m.EXPECT().ErrorEvents(gomock.Any()).Return(nil, nil) // No additional error descriptions.
				return m
			},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				return nil
			},
			want: errors.New("error creating stack"),
		},
		"should return a wrapped error if region is invalid when populating the admin role arn": {
			region: "bad-region",
			mockStack: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Create(gomock.Any()).Return("", &cloudformation.ErrStackAlreadyExists{})
				return m
			},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				return nil
			},
			want: fmt.Errorf("get stack set administrator role arn: find the partition for region bad-region"),
		},
		"should return nil if there are no updates": {
			region: "us-west-2",
			mockStack: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Create(gomock.Any()).Return("", &cloudformation.ErrStackAlreadyExists{})
				return m
			},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				return m
			},
		},
		"should return nil if infrastructure roles stackset created for the first time": {
			region: "us-west-2",
			mockStack: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Create(gomock.Any()).Return("", nil)
				return m
			},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).
					Do(func(name, _ string, _ ...stackset.CreateOrUpdateOption) {
						require.Equal(t, "testapp-infrastructure", name)
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
				cfnClient:   tc.mockStack(ctrl),
				appStackSet: tc.mockStackSet(t, ctrl),
				region:      tc.region,
				console:     new(discardFile),
			}

			// WHEN
			got := cf.DeployApp(mockApp)

			// THEN
			if tc.want != nil {
				require.EqualError(t, tc.want, got.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_UpgradeApplication(t *testing.T) {
	testCases := map[string]struct {
		mockDeployer func(t *testing.T, ctrl *gomock.Controller) *CloudFormation

		wantedErr error
	}{
		"error if fail to get existing application infrastructure stack": {
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				return &CloudFormation{
					cfnClient: &cloudformationtest.Double{
						DescribeFn: func(string) (*cloudformation.StackDescription, error) {
							return nil, errors.New("some error")
						},
					},
				}
			},
			wantedErr: fmt.Errorf("get existing application infrastructure stack: some error"),
		},
		"error if fail to update app stack": {
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				return &CloudFormation{
					cfnClient: &cloudformationtest.Double{
						DescribeFn: func(string) (*cloudformation.StackDescription, error) {
							return &cloudformation.StackDescription{}, nil
						},
						UpdateFn: func(*cloudformation.Stack) (string, error) {
							return "", fmt.Errorf("some error")
						},
					},
					renderStackSet: func(input renderStackSetInput) error {
						return nil
					},
				}
			},
			wantedErr: fmt.Errorf(`upgrade stack "phonetool-infrastructure-roles": some error`),
		},
		// TODO test tags manually
		"error if fail to describe app change set": {
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				return &CloudFormation{
					cfnClient: &cloudformationtest.Double{
						DescribeFn: func(string) (*cloudformation.StackDescription, error) {
							return &cloudformation.StackDescription{}, nil
						},
						UpdateFn: func(*cloudformation.Stack) (string, error) {
							return "", nil
						},
						DescribeChangeSetFn: func(changeSetID, stackName string) (*cloudformation.ChangeSetDescription, error) {
							return nil, errors.New("some error")
						},
					},
				}
			},
			wantedErr: fmt.Errorf(`upgrade stack "phonetool-infrastructure-roles": some error`),
		},
		"error if fail to get app change set template": {
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				return &CloudFormation{
					cfnClient: &cloudformationtest.Double{
						DescribeFn: func(string) (*cloudformation.StackDescription, error) {
							return &cloudformation.StackDescription{}, nil
						},
						UpdateFn: func(*cloudformation.Stack) (string, error) {
							return "", nil
						},
						DescribeChangeSetFn: func(changeSetID, stackName string) (*cloudformation.ChangeSetDescription, error) {
							return &cloudformation.ChangeSetDescription{}, nil
						},
						TemplateBodyFromChangeSetFn: func(changeSetID, stackName string) (string, error) {
							return "", errors.New("some error")
						},
					},
				}
			},
			wantedErr: fmt.Errorf(`upgrade stack "phonetool-infrastructure-roles": some error`),
		},
		"error if fail to wait until stack set last operation complete": {
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				mockAppStackSet := mocks.NewMockstackSetClient(ctrl)
				mockAppStackSet.EXPECT().WaitForStackSetLastOperationComplete("phonetool-infrastructure").Return(errors.New("some error"))

				return &CloudFormation{
					console: mockFileWriter{Writer: &strings.Builder{}},
					cfnClient: &cloudformationtest.Double{
						DescribeFn: func(string) (*cloudformation.StackDescription, error) {
							return &cloudformation.StackDescription{}, nil
						},
						UpdateFn: func(*cloudformation.Stack) (string, error) {
							return "", nil
						},
						DescribeChangeSetFn: func(changeSetID, stackName string) (*cloudformation.ChangeSetDescription, error) {
							return &cloudformation.ChangeSetDescription{}, nil
						},
						TemplateBodyFromChangeSetFn: func(changeSetID, stackName string) (string, error) {
							return ``, nil
						},
						DescribeStackEventsFn: func(input *awscfn.DescribeStackEventsInput) (*awscfn.DescribeStackEventsOutput, error) {
							// just finish the renderer on the first Describe call
							return &awscfn.DescribeStackEventsOutput{
								StackEvents: []*awscfn.StackEvent{
									{
										Timestamp:         aws.Time(time.Now().Add(1 * time.Hour)),
										LogicalResourceId: aws.String("phonetool-infrastructure-roles"),
										ResourceStatus:    aws.String(awscfn.StackStatusUpdateComplete),
									},
								},
							}, nil
						},
					},
					appStackSet: mockAppStackSet,
				}
			},
			wantedErr: fmt.Errorf(`wait for stack set phonetool-infrastructure last operation complete: some error`),
		},
		"success": {
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				mockAppStackSet := mocks.NewMockstackSetClient(ctrl)
				mockAppStackSet.EXPECT().WaitForStackSetLastOperationComplete("phonetool-infrastructure").Return(nil)
				mockAppStackSet.EXPECT().Describe("phonetool-infrastructure").Return(stackset.Description{}, nil)
				mockAppStackSet.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil)

				return &CloudFormation{
					console: mockFileWriter{Writer: &strings.Builder{}},
					cfnClient: &cloudformationtest.Double{
						DescribeFn: func(string) (*cloudformation.StackDescription, error) {
							return &cloudformation.StackDescription{}, nil
						},
						UpdateFn: func(*cloudformation.Stack) (string, error) {
							return "", nil
						},
						DescribeChangeSetFn: func(changeSetID, stackName string) (*cloudformation.ChangeSetDescription, error) {
							return &cloudformation.ChangeSetDescription{}, nil
						},
						TemplateBodyFromChangeSetFn: func(changeSetID, stackName string) (string, error) {
							return ``, nil
						},
						DescribeStackEventsFn: func(input *awscfn.DescribeStackEventsInput) (*awscfn.DescribeStackEventsOutput, error) {
							return &awscfn.DescribeStackEventsOutput{
								StackEvents: []*awscfn.StackEvent{
									{
										Timestamp:         aws.Time(time.Now().Add(1 * time.Hour)),
										LogicalResourceId: aws.String("phonetool-infrastructure-roles"),
										ResourceStatus:    aws.String(awscfn.StackStatusUpdateComplete),
									},
								},
							}, nil
						},
					},
					appStackSet: mockAppStackSet,
					region:      "us-west-2",
					renderStackSet: func(input renderStackSetInput) error {
						_, err := input.createOpFn()
						return err
					},
				}
			},
		},
		"success with multiple tries and waitings": {
			mockDeployer: func(t *testing.T, ctrl *gomock.Controller) *CloudFormation {
				mockAppStackSet := mocks.NewMockstackSetClient(ctrl)
				mockAppStackSet.EXPECT().WaitForStackSetLastOperationComplete("phonetool-infrastructure").Return(nil)
				mockAppStackSet.EXPECT().Describe("phonetool-infrastructure").Return(stackset.Description{}, nil)
				mockAppStackSet.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", &stackset.ErrStackSetOutOfDate{})
				mockAppStackSet.EXPECT().WaitForStackSetLastOperationComplete("phonetool-infrastructure").Return(nil)
				mockAppStackSet.EXPECT().Describe("phonetool-infrastructure").Return(stackset.Description{}, nil)
				mockAppStackSet.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil)

				return &CloudFormation{
					console: mockFileWriter{Writer: &strings.Builder{}},
					cfnClient: &cloudformationtest.Double{
						DescribeFn: func(string) (*cloudformation.StackDescription, error) {
							return &cloudformation.StackDescription{}, nil
						},
						UpdateFn: func(*cloudformation.Stack) (string, error) {
							return "", nil
						},
						DescribeChangeSetFn: func(changeSetID, stackName string) (*cloudformation.ChangeSetDescription, error) {
							return &cloudformation.ChangeSetDescription{}, nil
						},
						TemplateBodyFromChangeSetFn: func(changeSetID, stackName string) (string, error) {
							return ``, nil
						},
						DescribeStackEventsFn: func(input *awscfn.DescribeStackEventsInput) (*awscfn.DescribeStackEventsOutput, error) {
							return &awscfn.DescribeStackEventsOutput{
								StackEvents: []*awscfn.StackEvent{
									{
										Timestamp:         aws.Time(time.Now().Add(1 * time.Hour)),
										LogicalResourceId: aws.String("phonetool-infrastructure-roles"),
										ResourceStatus:    aws.String(awscfn.StackStatusUpdateComplete),
									},
								},
							}, nil
						},
					},
					appStackSet: mockAppStackSet,
					region:      "us-west-2",
					renderStackSet: func(input renderStackSetInput) error {
						_, err := input.createOpFn()
						return err
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := tc.mockDeployer(t, ctrl)

			// WHEN
			err := cf.UpgradeApplication(&deploy.CreateAppInput{
				Name: "phonetool",
			})

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCloudFormation_AddEnvToApp(t *testing.T) {
	mockApp := config.Application{
		Name:      "testapp",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		mockStackSet func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		app          *config.Application
		env          *config.Environment
		want         error
	}{
		"with no existing deployments and adding an env": {
			app: &mockApp,
			env: &config.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil).
					Do(func(_, _ string, ops ...stackset.CreateOrUpdateOption) {
						actual := &awscfn.UpdateStackSetInput{}
						ops[0](actual)
						wanted := &awscfn.UpdateStackSetInput{}
						stackset.WithOperationID("1")(wanted)
						require.Equal(t, actual, wanted)
					})
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{}, nil)
				m.EXPECT().CreateInstances(gomock.Any(), []string{"1234"}, []string{"us-west-2"}).Return("", nil)
				return m
			},
		},
		"with no new account ID added": {
			app: &mockApp,
			env: &config.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{
					Metadata: stack.AppResources{
						AppResourcesConfig: stack.AppResourcesConfig{
							Accounts: []string{"1234"},
						},
					},
				})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil)
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{}, nil)
				m.EXPECT().CreateInstances(gomock.Any(), []string{"1234"}, []string{"us-west-2"}).Return("", nil)
				return m
			},
		},
		"with existing stack instances in same region but different account (no new stack instances, but update stackset)": {
			app: &mockApp,
			env: &config.Environment{Name: "test", AccountID: "1234", Region: "us-west-2"},
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{
					Metadata: stack.AppResources{
						AppResourcesConfig: stack.AppResourcesConfig{
							Accounts: []string{"1234"},
							Version:  1,
						},
					},
				})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil).
					Do(func(_, _ string, ops ...stackset.CreateOrUpdateOption) {
						actual := &awscfn.UpdateStackSetInput{}
						ops[0](actual)
						wanted := &awscfn.UpdateStackSetInput{}
						stackset.WithOperationID("2")(wanted)
						require.Equal(t, actual, wanted)
					})
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						Account: "1234",
					},
				}, nil)
				m.EXPECT().CreateInstances(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
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
				appStackSet: tc.mockStackSet(t, ctrl),
				region:      "us-west-2",
				renderStackSet: func(input renderStackSetInput) error {
					_, err := input.createOpFn()
					return err
				},
			}
			got := cf.AddEnvToApp(&AddEnvToAppOpts{
				App:          tc.app,
				EnvName:      tc.env.Name,
				EnvAccountID: tc.env.AccountID,
				EnvRegion:    tc.env.Region,
			})

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_AddPipelineResourcesToApp(t *testing.T) {
	mockApp := config.Application{
		Name:      "testapp",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		app                 *config.Application
		mockStackSet        func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		getRegionFromClient func(client cloudformationiface.CloudFormationAPI) (string, error)
		expectedErr         error
	}{
		"with no existing account nor environment, add pipeline supporting resources": {
			app: &mockApp,
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{}, nil)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().CreateInstances(gomock.Any(), []string{"1234"}, []string{"us-west-2"}).Return("1", nil)
				return m
			},
			getRegionFromClient: func(client cloudformationiface.CloudFormationAPI) (string, error) {
				return "us-west-2", nil
			},
		},
		"with existing account and existing environment in a region, should not add pipeline supporting resources": {
			app: &mockApp,
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						Account: mockApp.AccountID,
					},
				}, nil)
				body, err := yaml.Marshal(stack.DeployedAppMetadata{})
				require.NoError(t, err)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: string(body),
				}, nil)
				m.EXPECT().CreateInstances(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
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
				appStackSet: tc.mockStackSet(t, ctrl),
				renderStackSet: func(input renderStackSetInput) error {
					_, err := input.createOpFn()
					return err
				},
			}
			getRegionFromClient = tc.getRegionFromClient

			got := cf.AddPipelineResourcesToApp(tc.app, "us-west-2")

			if tc.expectedErr != nil {
				require.EqualError(t, got, tc.expectedErr.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
	getRegionFromClient = actual
}

func TestCloudFormation_AddServiceToApp(t *testing.T) {
	mockApp := config.Application{
		Name:      "testapp",
		AccountID: "1234",
	}
	testCases := map[string]struct {
		app          *config.Application
		svcName      string
		mockStackSet func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		want         error
	}{
		"with no existing deployments and adding a service": {
			app:     &mockApp,
			svcName: "TestSvc",
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: `Metadata:
  Version:
  Services: []`,
				}, nil)
				m.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil).
					Do(func(_, template string, _ ...stackset.CreateOrUpdateOption) {
						configToDeploy, err := stack.AppConfigFrom(&template)
						require.NoError(t, err)
						require.ElementsMatch(t, []stack.AppResourcesWorkload{{Name: "TestSvc", WithECR: true}}, configToDeploy.Workloads)
						require.Empty(t, configToDeploy.Accounts, "there should be no new accounts to deploy")
						require.Equal(t, 1, configToDeploy.Version)
					})
				return m
			},
		},
		"with new app to existing app with existing services": {
			app:     &mockApp,
			svcName: "test",
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: `Metadata:
  Version: 1
  Services:
  - firsttest`,
				}, nil)
				m.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil).
					Do(func(_, template string, _ ...stackset.CreateOrUpdateOption) {
						configToDeploy, err := stack.AppConfigFrom(&template)
						require.NoError(t, err)
						require.ElementsMatch(t, []stack.AppResourcesWorkload{
							{Name: "test", WithECR: true},
							{Name: "firsttest", WithECR: true},
						}, configToDeploy.Workloads)
						require.Empty(t, configToDeploy.Accounts, "there should be no new accounts to deploy")
						require.Equal(t, 2, configToDeploy.Version)

					})
				return m
			},
		},
		"with existing service to existing app with existing services": {
			app:     &mockApp,
			svcName: "test",
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: `Metadata:
  Version: 1
  Services:
  - test`,
				}, nil)
				m.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
				return m
			},
		},
		"with new app to existing app with existing Workloads": {
			app:     &mockApp,
			svcName: "test",
			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: `Metadata:
  Version: 1
  Services: "See #5140"
  Workloads:
  - Name: firsttest
    WithECR: true`,
				}, nil)
				m.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil).
					Do(func(_, template string, _ ...stackset.CreateOrUpdateOption) {
						configToDeploy, err := stack.AppConfigFrom(&template)
						require.NoError(t, err)
						require.ElementsMatch(t, []stack.AppResourcesWorkload{
							{Name: "test", WithECR: true},
							{Name: "firsttest", WithECR: true},
						}, configToDeploy.Workloads)
						require.Empty(t, configToDeploy.Accounts, "there should be no new accounts to deploy")
						require.Equal(t, 2, configToDeploy.Version)

					})
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := CloudFormation{
				appStackSet: tc.mockStackSet(t, ctrl),
				region:      "us-west-2",
				renderStackSet: func(input renderStackSetInput) error {
					_, err := input.createOpFn()
					return err
				},
			}

			got := cf.AddServiceToApp(tc.app, tc.svcName)

			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestCloudFormation_RemoveServiceFromApp(t *testing.T) {
	mockApp := &config.Application{
		Name:      "testapp",
		AccountID: "1234",
	}

	tests := map[string]struct {
		service      string
		mockStackSet func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		want         error
	}{
		"should remove input service from the stack set": {
			service: "test",

			mockStackSet: func(t *testing.T, ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(stackset.Description{
					Template: `Metadata:
  Version: 1
  Services:
  - firsttest
  - test`,
				}, nil)
				m.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", nil).
					Do(func(_, template string, opts ...stackset.CreateOrUpdateOption) {
						configToDeploy, err := stack.AppConfigFrom(&template)
						require.NoError(t, err)
						require.ElementsMatch(t, []stack.AppResourcesWorkload{{Name: "firsttest", WithECR: true}}, configToDeploy.Workloads)
						require.Empty(t, configToDeploy.Accounts, "config account list should be empty")
						require.Equal(t, 2, configToDeploy.Version)
						require.Equal(t, 5, len(opts))
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
				appStackSet: tc.mockStackSet(t, ctrl),
				region:      "us-west-2",
				renderStackSet: func(input renderStackSetInput) error {
					_, err := input.createOpFn()
					return err
				},
			}

			got := cf.RemoveServiceFromApp(mockApp, tc.service)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestCloudFormation_GetRegionalAppResources(t *testing.T) {
	mockApp := config.Application{Name: "app", AccountID: "12345"}

	testCases := map[string]struct {
		createRegionalMockClient func(ctrl *gomock.Controller) cfnClient
		mockStackSet             func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		wantedResource           stack.AppRegionalResources
		want                     error
	}{
		"should describe stack instances and convert to AppRegionalResources": {
			wantedResource: stack.AppRegionalResources{
				KMSKeyARN:      "arn:aws:kms:us-west-2:01234567890:key/0000",
				S3Bucket:       "tests3-bucket-us-west-2",
				Region:         "us-east-9",
				RepositoryURLs: map[string]string{"phonetool-svc": "123.dkr.ecr.us-west-2.amazonaws.com/phonetool-svc"},
			},
			createRegionalMockClient: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("cross-region-stack").Return(mockValidAppResourceStack(), nil)
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
						wanted := &awscfn.ListStackInstancesInput{
							StackInstanceAccount: aws.String("12345"),
						}
						actual := &awscfn.ListStackInstancesInput{}
						opt(actual)
						require.Equal(t, wanted, actual)
					})
				return m
			},
		},
		"should propagate describe errors": {
			want: fmt.Errorf("describing application resources: getting outputs for stack cross-region-stack in region us-east-9: error calling cloudformation"),
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
			want: fmt.Errorf("describing application resources: error"),
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
				appStackSet: tc.mockStackSet(t, ctrl),
			}

			// WHEN
			got, err := cf.GetRegionalAppResources(&mockApp)

			// THEN
			if tc.want != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.want.Error())
			} else {
				require.True(t, len(got) == 1, "Expected only one resource")
				// Assert that the application resources are the same.
				require.Equal(t, tc.wantedResource, *got[0])
			}
		})
	}
}

func TestCloudFormation_GetAppResourcesByRegion(t *testing.T) {
	mockApp := config.Application{Name: "app", AccountID: "12345"}

	testCases := map[string]struct {
		createRegionalMockClient func(ctrl *gomock.Controller) cfnClient
		mockStackSet             func(t *testing.T, ctrl *gomock.Controller) stackSetClient
		wantedResource           stack.AppRegionalResources
		region                   string
		want                     error
	}{
		"should describe stack instances and convert to AppRegionalResources": {
			wantedResource: stack.AppRegionalResources{
				KMSKeyARN:      "arn:aws:kms:us-west-2:01234567890:key/0000",
				S3Bucket:       "tests3-bucket-us-west-2",
				Region:         "us-east-9",
				RepositoryURLs: map[string]string{"phonetool-svc": "123.dkr.ecr.us-west-2.amazonaws.com/phonetool-svc"},
			},
			region: "us-east-9",
			createRegionalMockClient: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe("cross-region-stack").Return(mockValidAppResourceStack(), nil)
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
					Do(func(_ string, opts ...stackset.InstanceSummariesOption) {
						wanted := &awscfn.ListStackInstancesInput{
							StackInstanceAccount: aws.String("12345"),
							StackInstanceRegion:  aws.String("us-east-9"),
						}
						actual := &awscfn.ListStackInstancesInput{}
						optAcc, optRegion := opts[0], opts[1]
						optAcc(actual)
						optRegion(actual)
						require.Equal(t, wanted, actual)
					})
				return m
			},
		},
		"should error when resources are found": {
			want:   fmt.Errorf("no regional resources for application app in region us-east-9 found"),
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
				appStackSet: tc.mockStackSet(t, ctrl),
			}

			// WHEN
			got, err := cf.GetAppResourcesByRegion(&mockApp, tc.region)

			// THEN
			if tc.want != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.want.Error())
			} else {
				require.NotNil(t, got)
				// Assert that the application resources are the same.
				require.Equal(t, tc.wantedResource, *got)
			}
		})
	}
}

func TestCloudFormation_DelegateDNSPermissions(t *testing.T) {
	testCases := map[string]struct {
		app        *config.Application
		accountID  string
		createMock func(ctrl *gomock.Controller) cfnClient
		want       error
	}{
		"Calls Update Stack": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "app",
				Domain:    "amazon.com",
			},
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(mockAppRolesStack("stackname", map[string]string{
					"AppDNSDelegatedAccounts": "1234",
				}), nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(nil)
				return m
			},
		},

		"Returns error from Describe Stack": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "app",
				Domain:    "amazon.com",
			},
			want: fmt.Errorf("getting existing application infrastructure stack: error"),
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, errors.New("error"))
				return m
			},
		},
		"Returns nil if there are no changeset updates from deployChangeSet": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "app",
				Domain:    "amazon.com",
			},
			want: nil,
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(mockAppRolesStack("stackname", map[string]string{
					"AppDNSDelegatedAccounts": "1234",
				}), nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(&cloudformation.ErrChangeSetEmpty{})
				return m
			},
		},
		"Returns error from Update Stack": {
			app: &config.Application{
				AccountID: "1234",
				Name:      "app",
				Domain:    "amazon.com",
			},
			want: fmt.Errorf("updating application to allow DNS delegation: error"),
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(mockAppRolesStack("stackname", map[string]string{
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
			}

			// WHEN
			got := cf.DelegateDNSPermissions(tc.app, tc.accountID)

			// THEN
			if tc.want != nil {
				require.EqualError(t, tc.want, got.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func mockValidAppResourceStack() *cloudformation.StackDescription {
	return mockAppResourceStack("stack", map[string]string{
		"KMSKeyARN":               "arn:aws:kms:us-west-2:01234567890:key/0000",
		"PipelineBucket":          "tests3-bucket-us-west-2",
		"ECRRepophonetoolDASHsvc": "arn:aws:ecr:us-west-2:123:repository/phonetool-svc",
	})
}

func mockAppResourceStack(stackArn string, outputs map[string]string) *cloudformation.StackDescription {
	outputList := []*awscfn.Output{}
	for key, val := range outputs {
		outputList = append(outputList, &awscfn.Output{
			OutputKey:   aws.String(key),
			OutputValue: aws.String(val),
		})
	}

	return &cloudformation.StackDescription{
		StackId: aws.String(stackArn),
		Outputs: outputList,
	}
}

func mockAppRolesStack(stackArn string, parameters map[string]string) *cloudformation.StackDescription {
	parametersList := []*awscfn.Parameter{}
	for key, val := range parameters {
		parametersList = append(parametersList, &awscfn.Parameter{
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

func TestCloudFormation_DeleteApp(t *testing.T) {
	tests := map[string]struct {
		appName      string
		createMock   func(ctrl *gomock.Controller) cfnClient
		mockStackSet func(ctrl *gomock.Controller) stackSetClient

		want error
	}{
		"should delete stackset and then infrastructure roles": {
			appName: "testApp",
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody("testApp-infrastructure-roles").Return("", nil)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{
					StackId: aws.String("some stack"),
				}, nil)
				m.EXPECT().DeleteAndWait("testApp-infrastructure-roles").Return(&cloudformation.ErrStackNotFound{})
				m.EXPECT().DescribeStackEvents(gomock.Any()).Return(&awscfn.DescribeStackEventsOutput{}, nil).AnyTimes()
				return m
			},
			mockStackSet: func(ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().DeleteAllInstances("testApp-infrastructure").Return("1", nil)
				m.EXPECT().WaitForOperation("testApp-infrastructure", "1").Return(nil)
				m.EXPECT().Delete("testApp-infrastructure").Return(nil)
				return m
			},
		},
		"should skip waiting for delete instance operation if the stack set is already deleted": {
			appName: "testApp",
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{
					StackId: aws.String("some stack"),
				}, nil)
				m.EXPECT().DeleteAndWait(gomock.Any()).Return(&cloudformation.ErrStackNotFound{})
				m.EXPECT().DescribeStackEvents(gomock.Any()).Return(&awscfn.DescribeStackEventsOutput{}, nil).AnyTimes()
				return m
			},
			mockStackSet: func(ctrl *gomock.Controller) stackSetClient {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().DeleteAllInstances(gomock.Any()).Return("", &stackset.ErrStackSetNotFound{})
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
				cfnClient:   tc.createMock(ctrl),
				appStackSet: tc.mockStackSet(ctrl),
				console:     new(discardFile),
			}

			// WHEN
			got := cf.DeleteApp(tc.appName)

			// THEN
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCloudFormation_RenderStackSet(t *testing.T) {
	testDate := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)
	testCases := map[string]struct {
		in   renderStackSetInput
		mock func(t *testing.T, ctrl *gomock.Controller) CloudFormation

		wantedErr error
	}{
		"should return the error if a stack set operation cannot be created": {
			in: renderStackSetInput{
				hasInstanceUpdates: true,
				createOpFn: func() (string, error) {
					return "", errors.New("some error")
				},
				now: func() time.Time {
					return testDate
				},
			},
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				return CloudFormation{}
			},

			wantedErr: errors.New("some error"),
		},
		"should return a wrapped error if stack set instance streamers cannot be retrieved": {
			in: renderStackSetInput{
				name:               "demo-infra",
				hasInstanceUpdates: true,
				createOpFn: func() (string, error) {
					return "1", nil
				},
				now: func() time.Time {
					return testDate
				},
			},
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				m := mocks.NewMockstackSetClient(ctrl)
				m.EXPECT().InstanceSummaries(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				return CloudFormation{
					appStackSet: m,
				}
			},

			wantedErr: errors.New(`retrieve stack instance streamers`),
		},
		"cancel all goroutines if a streamer fails": {
			in: renderStackSetInput{
				name:               "demo-infra",
				hasInstanceUpdates: true,
				createOpFn: func() (string, error) {
					return "1", nil
				},
				now: func() time.Time {
					return testDate
				},
			},
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				mockStackSet := mocks.NewMockstackSetClient(ctrl)
				mockStackSet.EXPECT().InstanceSummaries(gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{
					{
						StackID: "stackset-instance-demo-infra",
						Account: "1111",
						Region:  "us-west-2",
						Status:  "RUNNING",
					},
				}, nil)
				mockStackSet.EXPECT().DescribeOperation(gomock.Any(), gomock.Any()).Return(stackset.Operation{
					Status: "RUNNING",
				}, nil).AnyTimes()

				mockStack := mocks.NewMockcfnClient(ctrl)
				mockStack.EXPECT().DescribeStackEvents(gomock.Any()).
					Return(nil, errors.New("some error")).AnyTimes()

				return CloudFormation{
					appStackSet: mockStackSet,
					cfnClient:   mockStack,
					regionalClient: func(_ string) cfnClient {
						return mockStack
					},
					console: mockFileWriter{
						Writer: new(strings.Builder),
					},
				}
			},

			wantedErr: errors.New(`render progress of stack set "demo-infra"`),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := tc.mock(t, ctrl)

			// WHEN
			err := client.renderStackSetImpl(tc.in)

			// THEN
			if tc.wantedErr != nil {
				require.ErrorContains(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCloudFormation_RemoveEnvFromApp(t *testing.T) {

	testCases := map[string]struct {
		mock   func(t *testing.T, ctrl *gomock.Controller) CloudFormation
		inOpts RemoveEnvFromAppOpts

		wantedErr error
	}{
		"failure describing stackset": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
				},
			},
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				s3 := mocks.NewMocks3Client(ctrl)
				ecr := mocks.NewMockimageRemover(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries("phonetool-infrastructure", gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				regionalCfn.EXPECT().Describe(gomock.Any()).Times(0)
				s3.EXPECT().EmptyBucket(gomock.Any()).Times(0)
				ecr.EXPECT().ClearRepository(gomock.Any()).Times(0)
				appStackSet.EXPECT().DeleteInstance(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				appStackSet.EXPECT().WaitForOperation(gomock.Any(), gomock.Any()).Times(0)
				cfn.EXPECT().Describe(gomock.Any()).Times(0)
				cfn.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
				appStackSet.EXPECT().Describe(gomock.Any()).Times(0)
				appStackSet.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"1234", "5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					regionalS3Client: func(region string) s3Client {
						return s3
					},
					regionalECRClient: func(region string) imageRemover {
						return ecr
					},
					regionalClient: func(region string) cfnClient { return regionalCfn },
				}
			},
			wantedErr: errors.New("some error"),
		},
		"success": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
					{
						Name:      "prod",
						AccountID: "5678",
						Region:    "us-east-2",
					},
				},
			},
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				s3 := mocks.NewMocks3Client(ctrl)
				ecr := mocks.NewMockimageRemover(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries("phonetool-infrastructure", gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						StackID: "some-stack",
					},
				}, nil)
				regionalCfn.EXPECT().Describe("some-stack").Return(mockValidAppResourceStack(), nil)
				s3.EXPECT().EmptyBucket("tests3-bucket-us-west-2").Return(nil)
				ecr.EXPECT().ClearRepository("phonetool-svc").Return(nil)
				appStackSet.EXPECT().DeleteInstance("phonetool-infrastructure", "1234", "us-west-2").Return("123", nil)
				appStackSet.EXPECT().WaitForOperation("phonetool-infrastructure", "123").Return(nil)
				cfn.EXPECT().Describe(stack.NameForAppStack("phonetool")).Return(&cloudformation.StackDescription{
					Parameters: []*awscfn.Parameter{
						{
							ParameterKey:   aws.String("AppDNSDelegatedAccounts"),
							ParameterValue: aws.String("1234,5678"),
						},
					},
				}, nil)
				cfn.EXPECT().UpdateAndWait(gomock.Any())
				appStackSet.EXPECT().Describe("phonetool-infrastructure").Return(stackset.Description{
					ID:   "",
					Name: "phonetool-infrastructure",
					Template: `Metadata:
  TemplateVersion: 'v1.2.0'
  Version: 17
  Workloads:
    - Name: ar
      WithECR: true
  Accounts:
    - 1234
    - 5678`,
				}, nil)
				appStackSet.EXPECT().Update("phonetool-infrastructure", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("123", nil)

				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"1234", "5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					regionalS3Client: func(region string) s3Client {
						return s3
					},
					regionalECRClient: func(region string) imageRemover {
						return ecr
					},
					regionalClient: func(region string) cfnClient { return regionalCfn },
				}
			},
		},
		"skips stack redeployment if we don't need to remove account": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
					{
						Name:      "prod",
						AccountID: "1234",
						Region:    "us-east-2",
					},
				},
			},
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				s3 := mocks.NewMocks3Client(ctrl)
				ecr := mocks.NewMockimageRemover(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries("phonetool-infrastructure", gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						StackID: "some-stack",
					},
				}, nil)
				regionalCfn.EXPECT().Describe("some-stack").Return(mockValidAppResourceStack(), nil)
				s3.EXPECT().EmptyBucket("tests3-bucket-us-west-2").Return(nil)
				ecr.EXPECT().ClearRepository("phonetool-svc").Return(nil)
				appStackSet.EXPECT().DeleteInstance("phonetool-infrastructure", "1234", "us-west-2").Return("123", nil)
				appStackSet.EXPECT().WaitForOperation("phonetool-infrastructure", "123").Return(nil)
				cfn.EXPECT().Describe(gomock.Any()).Times(0)
				cfn.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
				appStackSet.EXPECT().Describe(gomock.Any()).Times(0)
				appStackSet.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)

				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					regionalS3Client: func(region string) s3Client { return s3 },
					regionalECRClient: func(region string) imageRemover {
						return ecr
					},
					regionalClient: func(region string) cfnClient { return regionalCfn },
				}
			},
		},
		"skips instance delete if 'DeleteStackInstance' is false": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
					{
						Name:      "prod",
						AccountID: "5678",
						Region:    "us-west-2",
					},
				},
			},
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				s3 := mocks.NewMocks3Client(ctrl)
				ecr := mocks.NewMockimageRemover(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries(gomock.Any()).Times(0)
				regionalCfn.EXPECT().Describe(gomock.Any()).Times(0)
				s3.EXPECT().EmptyBucket(gomock.Any()).Times(0)
				ecr.EXPECT().ClearRepository(gomock.Any()).Times(0)
				appStackSet.EXPECT().DeleteInstance(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				appStackSet.EXPECT().WaitForOperation(gomock.Any(), gomock.Any()).Times(0)
				cfn.EXPECT().Describe(stack.NameForAppStack("phonetool")).Return(&cloudformation.StackDescription{
					Parameters: []*awscfn.Parameter{
						{
							ParameterKey:   aws.String("AppDNSDelegatedAccounts"),
							ParameterValue: aws.String("1234,5678"),
						},
					},
				}, nil)
				cfn.EXPECT().UpdateAndWait(gomock.Any())
				appStackSet.EXPECT().Describe("phonetool-infrastructure").Return(stackset.Description{
					ID:   "",
					Name: "phonetool-infrastructure",
					Template: `Metadata:
  TemplateVersion: 'v1.2.0'
  Version: 17
  Workloads:
    - Name: ar
      WithECR: true
  Accounts:
    - 1234
    - 5678`,
				}, nil)
				appStackSet.EXPECT().Update("phonetool-infrastructure", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("123", nil)

				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"1234", "5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					s3Client: s3,
					regionalECRClient: func(region string) imageRemover {
						return ecr
					},
					regionalClient: func(region string) cfnClient { return regionalCfn },
				}
			},
		},
		"error deleting instance": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
					{
						Name:      "prod",
						AccountID: "5678",
						Region:    "us-east-2",
					},
				},
			},
			wantedErr: errors.New("some error"),
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				s3 := mocks.NewMocks3Client(ctrl)
				ecr := mocks.NewMockimageRemover(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries("phonetool-infrastructure", gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						StackID: "some-stack",
					},
				}, nil)
				regionalCfn.EXPECT().Describe("some-stack").Return(mockValidAppResourceStack(), nil)
				s3.EXPECT().EmptyBucket("tests3-bucket-us-west-2").Return(nil)
				ecr.EXPECT().ClearRepository("phonetool-svc").Return(nil)
				// Delete stackset instance
				appStackSet.EXPECT().DeleteInstance("phonetool-infrastructure", "1234", "us-west-2").Return("", errors.New("some error"))
				appStackSet.EXPECT().WaitForOperation(gomock.Any(), gomock.Any()).Times(0)
				cfn.EXPECT().Describe(gomock.Any()).Times(0)
				cfn.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
				appStackSet.EXPECT().Describe(gomock.Any()).Times(0)
				appStackSet.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)
				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"1234", "5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					regionalClient: func(region string) cfnClient {
						return regionalCfn
					},
					regionalS3Client:  func(region string) s3Client { return s3 },
					regionalECRClient: func(region string) imageRemover { return ecr },
				}
			},
		},
		"error updating stack": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
					{
						Name:      "prod",
						AccountID: "5678",
						Region:    "us-east-2",
					},
				},
			},
			wantedErr: errors.New("some error"),
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				s3 := mocks.NewMocks3Client(ctrl)
				ecr := mocks.NewMockimageRemover(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries("phonetool-infrastructure", gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						StackID: "some-stack",
					},
				}, nil)
				regionalCfn.EXPECT().Describe("some-stack").Return(mockValidAppResourceStack(), nil)
				s3.EXPECT().EmptyBucket("tests3-bucket-us-west-2").Return(nil)
				ecr.EXPECT().ClearRepository("phonetool-svc").Return(nil)
				appStackSet.EXPECT().DeleteInstance("phonetool-infrastructure", "1234", "us-west-2").Return("123", nil)
				appStackSet.EXPECT().WaitForOperation("phonetool-infrastructure", "123").Return(nil)
				cfn.EXPECT().Describe(stack.NameForAppStack("phonetool")).Return(&cloudformation.StackDescription{
					Parameters: []*awscfn.Parameter{
						{
							ParameterKey:   aws.String("AppDNSDelegatedAccounts"),
							ParameterValue: aws.String("1234,5678"),
						},
					},
				}, nil)
				cfn.EXPECT().UpdateAndWait(gomock.Any())
				appStackSet.EXPECT().Describe("phonetool-infrastructure").Return(stackset.Description{
					ID:   "",
					Name: "phonetool-infrastructure",
					Template: `Metadata:
  TemplateVersion: 'v1.2.0'
  Version: 17
  Workloads:
    - Name: ar
      WithECR: true
  Accounts:
    - 1234
    - 5678`,
				}, nil)
				appStackSet.EXPECT().Update("phonetool-infrastructure", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))

				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"1234", "5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					regionalClient: func(region string) cfnClient {
						return regionalCfn
					},
					regionalS3Client: func(region string) s3Client {
						return s3
					},
					regionalECRClient: func(region string) imageRemover {
						return ecr
					},
				}
			},
		},
		"error emptying bucket": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
					{
						Name:      "prod",
						AccountID: "5678",
						Region:    "us-east-2",
					},
				},
			},
			wantedErr: errors.New("some error"),
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				s3 := mocks.NewMocks3Client(ctrl)
				ecr := mocks.NewMockimageRemover(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries("phonetool-infrastructure", gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						StackID: "some-stack",
					},
				}, nil)
				regionalCfn.EXPECT().Describe("some-stack").Return(mockValidAppResourceStack(), nil)
				s3.EXPECT().EmptyBucket("tests3-bucket-us-west-2").Return(errors.New("some error"))
				ecr.EXPECT().ClearRepository(gomock.Any()).Times(0)
				appStackSet.EXPECT().DeleteInstance(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				appStackSet.EXPECT().WaitForOperation(gomock.Any(), gomock.Any()).Times(0)
				cfn.EXPECT().Describe(gomock.Any()).Times(0)
				cfn.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
				appStackSet.EXPECT().Describe(gomock.Any()).Times(0)
				appStackSet.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)
				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"1234", "5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					regionalClient: func(region string) cfnClient {
						return regionalCfn
					},
					regionalS3Client: func(region string) s3Client {
						return s3
					},
					regionalECRClient: func(region string) imageRemover {
						return ecr
					},
				}
			},
		},
		"error describing regional resources": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
					{
						Name:      "prod",
						AccountID: "5678",
						Region:    "us-east-2",
					},
				},
			},
			wantedErr: errors.New("some error"),
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries("phonetool-infrastructure", gomock.Any(), gomock.Any()).Return([]stackset.InstanceSummary{
					{
						Region:  "us-west-2",
						StackID: "some-stack",
					},
				}, nil)
				regionalCfn.EXPECT().Describe("some-stack").Return(nil, errors.New("some error"))

				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"1234", "5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					regionalClient: func(region string) cfnClient {
						return regionalCfn
					},
				}
			},
		},
		"regional stack is already deleted": {
			inOpts: RemoveEnvFromAppOpts{
				App: &config.Application{
					Name:      "phonetool",
					AccountID: "1234",
					Version:   "1",
				},
				EnvToDelete: &config.Environment{
					Name:      "test",
					AccountID: "1234",
					Region:    "us-west-2",
				},
				Environments: []*config.Environment{
					{
						Name:      "test",
						AccountID: "1234",
						Region:    "us-west-2",
					},
					{
						Name:      "prod",
						AccountID: "1234",
						Region:    "us-east-2",
					},
				},
			},
			mock: func(t *testing.T, ctrl *gomock.Controller) CloudFormation {
				cfn := mocks.NewMockcfnClient(ctrl)
				appStackSet := mocks.NewMockstackSetClient(ctrl)
				s3 := mocks.NewMocks3Client(ctrl)
				ecr := mocks.NewMockimageRemover(ctrl)
				regionalCfn := mocks.NewMockcfnClient(ctrl)
				// Empty ECR and S3
				appStackSet.EXPECT().InstanceSummaries("phonetool-infrastructure", gomock.Any(), gomock.Any()).Return(nil, nil)
				// no instance summaries returned means no describe call and no EmptyBucket/ClearRepository call.
				regionalCfn.EXPECT().Describe(gomock.Any()).Times(0)
				s3.EXPECT().EmptyBucket(gomock.Any()).Times(0)
				ecr.EXPECT().ClearRepository(gomock.Any()).Times(0)
				appStackSet.EXPECT().DeleteInstance("phonetool-infrastructure", "1234", "us-west-2").Return("12345", nil)
				appStackSet.EXPECT().WaitForOperation("phonetool-infrastructure", "12345").Return(nil)
				cfn.EXPECT().Describe(gomock.Any()).Times(0)
				cfn.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
				appStackSet.EXPECT().Describe(gomock.Any()).Times(0)
				appStackSet.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)
				return CloudFormation{
					cfnClient: cfn,
					region:    "us-east-1",

					appStackSet: appStackSet,
					dnsDelegatedAccountsForStack: func(in *awscfn.Stack) []string {
						return []string{"1234", "5678"}
					},
					renderStackSet: func(in renderStackSetInput) error {
						_, err := in.createOpFn()
						return err
					},
					regionalClient: func(region string) cfnClient {
						return regionalCfn
					},
					regionalS3Client: func(region string) s3Client {
						return s3
					},
					regionalECRClient: func(region string) imageRemover {
						return ecr
					},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := tc.mock(t, ctrl)

			// WHEN
			err := client.RemoveEnvFromApp(&tc.inOpts)

			// THEN
			if tc.wantedErr != nil {
				require.ErrorContains(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
