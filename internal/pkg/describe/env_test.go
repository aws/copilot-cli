// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	cfstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type envDescriberMocks struct {
	configStoreSvc *mocks.MockConfigStoreSvc
	deployStoreSvc *mocks.MockDeployedEnvServicesLister
	stackDescriber *mocks.MockstackDescriber
}

var wantedResources = []*stack.Resource{
	{
		Type:       "AWS::IAM::Role",
		PhysicalID: "testApp-testEnv-CFNExecutionRole",
	},
	{
		Type:       "testApp-testEnv-Cluster",
		PhysicalID: "AWS::ECS::Cluster-jI63pYBWU6BZ",
	},
}

func TestEnvDescriber_Describe(t *testing.T) {
	testApp := "testApp"
	testEnv := &config.Environment{
		App:              "testApp",
		Name:             "testEnv",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}
	testSvc1 := &config.Workload{
		App:  "testApp",
		Name: "testSvc1",
		Type: "load-balanced",
	}
	testSvc2 := &config.Workload{
		App:  "testApp",
		Name: "testSvc2",
		Type: "load-balanced",
	}
	testSvc3 := &config.Workload{
		App:  "testApp",
		Name: "testSvc3",
		Type: "load-balanced",
	}
	stackTags := map[string]string{
		"copilot-application": "testApp",
		"copilot-environment": "testEnv",
	}
	stackOutputs := map[string]string{
		"VpcId":          "vpc-012abcd345",
		"PublicSubnets":  "subnet-0789ab,subnet-0123cd",
		"PrivateSubnets": "subnet-023ff,subnet-04af",
	}
	mockResource1 := &stack.Resource{
		PhysicalID: "testApp-testEnv-CFNExecutionRole",
		Type:       "AWS::IAM::Role",
	}
	mockResource2 := &stack.Resource{
		PhysicalID: "AWS::ECS::Cluster-jI63pYBWU6BZ",
		Type:       "testApp-testEnv-Cluster",
	}
	envSvcs := []*config.Workload{testSvc1, testSvc2}
	mockError := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks envDescriberMocks)

		wantedEnv   *EnvDescription
		wantedSvcs  []*config.Workload
		wantedError error
	}{
		"error if fail to list all services": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.configStoreSvc.EXPECT().ListServices(testApp).Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("list services for app testApp: some error"),
		},
		"error if fail to list deployed services": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.configStoreSvc.EXPECT().ListServices(testApp).Return([]*config.Workload{
						testSvc1, testSvc2, testSvc3,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedServices(testApp, testEnv.Name).
						Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("list deployed services in env testEnv: some error"),
		},
		"error if fail to get env tags": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.configStoreSvc.EXPECT().ListServices(testApp).Return([]*config.Workload{
						testSvc1, testSvc2, testSvc3,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedServices(testApp, testEnv.Name).
						Return([]string{"testSvc1", "testSvc2"}, nil),
					m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{}, mockError),
				)
			},
			wantedError: fmt.Errorf("retrieve environment stack: some error"),
		},
		"error if fail to get env resources": {
			shouldOutputResources: true,
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.configStoreSvc.EXPECT().ListServices(testApp).Return([]*config.Workload{
						testSvc1, testSvc2, testSvc3,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedServices(testApp, testEnv.Name).
						Return([]string{"testSvc1", "testSvc2"}, nil),
					m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
						Tags:    stackTags,
						Outputs: stackOutputs,
					}, nil),
					m.stackDescriber.EXPECT().Resources().Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("retrieve environment resources: some error"),
		},
		"success without resources": {
			shouldOutputResources: false,
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.configStoreSvc.EXPECT().ListServices(testApp).Return([]*config.Workload{
						testSvc1, testSvc2, testSvc3,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedServices(testApp, testEnv.Name).
						Return([]string{"testSvc1", "testSvc2"}, nil),
					m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
						Tags:    stackTags,
						Outputs: stackOutputs,
					}, nil),
				)
			},
			wantedEnv: &EnvDescription{
				Environment: testEnv,
				Services:    envSvcs,
				Tags:        map[string]string{"copilot-application": "testApp", "copilot-environment": "testEnv"},
				EnvironmentVPC: EnvironmentVPC{
					ID:               "vpc-012abcd345",
					PublicSubnetIDs:  []string{"subnet-0789ab", "subnet-0123cd"},
					PrivateSubnetIDs: []string{"subnet-023ff", "subnet-04af"},
				},
			},
		},
		"success with resources": {
			shouldOutputResources: true,
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.configStoreSvc.EXPECT().ListServices(testApp).Return([]*config.Workload{
						testSvc1, testSvc2, testSvc3,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedServices(testApp, testEnv.Name).
						Return([]string{"testSvc1", "testSvc2"}, nil),
					m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
						Tags:    stackTags,
						Outputs: stackOutputs,
					}, nil),
					m.stackDescriber.EXPECT().Resources().Return([]*stack.Resource{
						mockResource1,
						mockResource2,
					}, nil),
				)
			},
			wantedEnv: &EnvDescription{
				Environment: testEnv,
				Services:    envSvcs,
				Tags:        map[string]string{"copilot-application": "testApp", "copilot-environment": "testEnv"},
				Resources:   wantedResources,
				EnvironmentVPC: EnvironmentVPC{
					ID:               "vpc-012abcd345",
					PublicSubnetIDs:  []string{"subnet-0789ab", "subnet-0123cd"},
					PrivateSubnetIDs: []string{"subnet-023ff", "subnet-04af"},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStoreSvc := mocks.NewMockConfigStoreSvc(ctrl)
			mockDeployedEnvServicesLister := mocks.NewMockDeployedEnvServicesLister(ctrl)
			mockCFN := mocks.NewMockstackDescriber(ctrl)
			mocks := envDescriberMocks{
				configStoreSvc: mockConfigStoreSvc,
				deployStoreSvc: mockDeployedEnvServicesLister,
				stackDescriber: mockCFN,
			}

			tc.setupMocks(mocks)

			d := &EnvDescriber{
				env:             testEnv,
				app:             testApp,
				enableResources: tc.shouldOutputResources,

				configStore: mockConfigStoreSvc,
				deployStore: mockDeployedEnvServicesLister,
				cfn:         mockCFN,
			}

			// WHEN
			actual, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedEnv, actual)
			}
		})
	}
}

func TestEnvDescriber_Version(t *testing.T) {
	testCases := map[string]struct {
		given func(ctrl *gomock.Controller) *EnvDescriber

		wantedVersion string
		wantedErr     error
	}{
		"should return deploy.LegacyEnvTemplateVersion version if legacy template": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return("", nil)
				return &EnvDescriber{
					app: "phonetool",
					env: &config.Environment{Name: "test"},
					cfn: m,
				}
			},
			wantedVersion: deploy.LegacyEnvTemplateVersion,
		},
		"should read the version from the Metadata field": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return(`{"Version":"1.0.0"}`, nil)
				return &EnvDescriber{
					app: "phonetool",
					env: &config.Environment{Name: "test"},
					cfn: m,
				}
			},

			wantedVersion: "1.0.0",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			d := tc.given(ctrl)

			// WHEN
			actual, err := d.Version()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedVersion, actual)
			}
		})
	}
}

func TestEnvDescriber_ServiceDiscoveryEndpoint(t *testing.T) {
	testCases := map[string]struct {
		given func(ctrl *gomock.Controller) *EnvDescriber

		wantedEndpoint string
		wantedErr      error
	}{
		"should return app.local if legacy, unupgraded environment": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().Describe().Return(stack.StackDescription{Parameters: map[string]string{}}, nil)
				return &EnvDescriber{
					app: "phonetool",
					env: &config.Environment{Name: "test"},
					cfn: m,
				}
			},
			wantedEndpoint: "phonetool.local",
		},
		"should return the new env template if the parameter is set": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: map[string]string{
						cfstack.EnvParamServiceDiscoveryEndpoint: "test.phonetool.local",
					}}, nil)
				return &EnvDescriber{
					app: "phonetool",
					env: &config.Environment{Name: "test"},
					cfn: m,
				}
			},
			wantedEndpoint: "test.phonetool.local",
		},
		"should return the old env template if the parameter is empty": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: map[string]string{
						cfstack.EnvParamServiceDiscoveryEndpoint: "",
					}}, nil)
				return &EnvDescriber{
					app: "phonetool",
					env: &config.Environment{Name: "test"},
					cfn: m,
				}
			},
			wantedEndpoint: "phonetool.local",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			d := tc.given(ctrl)

			// WHEN
			actual, err := d.ServiceDiscoveryEndpoint()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedEndpoint, actual)
			}
		})
	}
}

func TestEnvDescription_JSONString(t *testing.T) {
	testApp := &config.Application{
		Name: "testApp",
		Tags: map[string]string{"key1": "value1", "key2": "value2"},
	}
	testEnv := &config.Environment{
		App:              "testApp",
		Name:             "testEnv",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
		CustomConfig:     &config.CustomizeEnv{},
	}
	testSvc1 := &config.Workload{
		App:  "testApp",
		Name: "testSvc1",
		Type: "load-balanced",
	}
	testSvc2 := &config.Workload{
		App:  "testApp",
		Name: "testSvc2",
		Type: "load-balanced",
	}
	testSvc3 := &config.Workload{
		App:  "testApp",
		Name: "testSvc3",
		Type: "load-balanced",
	}
	allSvcs := []*config.Workload{testSvc1, testSvc2, testSvc3}
	wantedContent := "{\"environment\":{\"app\":\"testApp\",\"name\":\"testEnv\",\"region\":\"us-west-2\",\"accountID\":\"123456789012\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\",\"customConfig\":{}},\"services\":[{\"app\":\"testApp\",\"name\":\"testSvc1\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc2\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc3\",\"type\":\"load-balanced\"}],\"tags\":{\"key1\":\"value1\",\"key2\":\"value2\"},\"resources\":[{\"type\":\"AWS::IAM::Role\",\"physicalID\":\"testApp-testEnv-CFNExecutionRole\"},{\"type\":\"testApp-testEnv-Cluster\",\"physicalID\":\"AWS::ECS::Cluster-jI63pYBWU6BZ\"}],\"environmentVPC\":{\"id\":\"\",\"publicSubnetIDs\":null,\"privateSubnetIDs\":null}}\n"

	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := &EnvDescription{
		Environment:    testEnv,
		EnvironmentVPC: EnvironmentVPC{},
		Services:       allSvcs,
		Tags:           testApp.Tags,
		Resources:      wantedResources,
	}

	// WHEN
	actual, _ := d.JSONString()

	// THEN
	require.Equal(t, wantedContent, actual)
}

func TestEnvDescription_HumanString(t *testing.T) {
	testApp := &config.Application{
		Name: "testApp",
		Tags: map[string]string{"key1": "value1", "key2": "value2"},
	}
	testEnv := &config.Environment{
		App:              "testApp",
		Name:             "testEnv",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}
	testSvc1 := &config.Workload{
		App:  "testApp",
		Name: "testSvc1",
		Type: "load-balanced",
	}
	testSvc2 := &config.Workload{
		App:  "testApp",
		Name: "testSvc2",
		Type: "load-balanced",
	}
	testSvc3 := &config.Workload{
		App:  "testApp",
		Name: "testSvc3",
		Type: "load-balanced",
	}
	allSvcs := []*config.Workload{testSvc1, testSvc2, testSvc3}

	wantedContent := `About

  Name              testEnv
  Production        false
  Region            us-west-2
  Account ID        123456789012

Services

  Name              Type
  ----              ----
  testSvc1          load-balanced
  testSvc2          load-balanced
  testSvc3          load-balanced

Tags

  Key               Value
  ---               -----
  key1              value1
  key2              value2

Resources

  AWS::IAM::Role           testApp-testEnv-CFNExecutionRole
  testApp-testEnv-Cluster  AWS::ECS::Cluster-jI63pYBWU6BZ
`
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := &EnvDescription{
		Environment: testEnv,
		Services:    allSvcs,
		Tags:        testApp.Tags,
		Resources:   wantedResources,
	}

	// WHEN
	actual := d.HumanString()

	// THEN
	require.Equal(t, wantedContent, actual)
}
