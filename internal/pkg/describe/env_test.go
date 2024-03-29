// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/version"
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
	testJob1 := &config.Workload{
		App:  "testApp",
		Name: "testJob1",
		Type: "Scheduled Job",
	}
	testJob2 := &config.Workload{
		App:  "testApp",
		Name: "testJob2",
		Type: "Scheduled Job",
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
	envJobs := []*config.Workload{testJob1, testJob2}
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
					m.configStoreSvc.EXPECT().ListJobs(testApp).Return([]*config.Workload{
						testJob1, testJob2,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedJobs(testApp, testEnv.Name).
						Return([]string{"testJob1", "testJob2"}, nil),
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
					m.configStoreSvc.EXPECT().ListJobs(testApp).Return([]*config.Workload{
						testJob1, testJob2,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedJobs(testApp, testEnv.Name).
						Return([]string{"testJob1", "testJob2"}, nil),
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
					m.configStoreSvc.EXPECT().ListJobs(testApp).Return([]*config.Workload{
						testJob1, testJob2,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedJobs(testApp, testEnv.Name).
						Return([]string{"testJob1", "testJob2"}, nil),
					m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
						Tags:    stackTags,
						Outputs: stackOutputs,
					}, nil),
				)
			},
			wantedEnv: &EnvDescription{
				Environment: testEnv,
				Services:    envSvcs,
				Jobs:        envJobs,
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
					m.configStoreSvc.EXPECT().ListJobs(testApp).Return([]*config.Workload{
						testJob1, testJob2,
					}, nil),
					m.deployStoreSvc.EXPECT().ListDeployedJobs(testApp, testEnv.Name).
						Return([]string{"testJob1", "testJob2"}, nil),
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
				Jobs:        envJobs,
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

func TestEnvDescriber_Manifest(t *testing.T) {
	testCases := map[string]struct {
		given func(ctrl *gomock.Controller) *EnvDescriber

		wantedManifest []byte
		wantedErr      error
	}{
		"should return an error when the template Metadata cannot be retrieved": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return("", errors.New("some error"))
				return &EnvDescriber{
					cfn: m,
				}
			},
			wantedErr: errors.New("some error"),
		},
		"should unmarshal from SSM when the stack template does not have any Metadata.Manifest": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return(`
Metadata:
  Version: 1.9.0
`, nil)
				return &EnvDescriber{
					env: &config.Environment{
						Name: "test",
					},
					cfn: m,
				}
			},
			wantedManifest: []byte(`name: test
type: Environment`),
		},
		"should prioritize stack template's Metadata over SSM": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return(`{"Version":"1.9.0","Manifest":"\nname: prod\ntype: Environment"}`, nil)
				return &EnvDescriber{
					env: &config.Environment{
						Name: "test",
					},
					cfn: m,
				}
			},
			wantedManifest: []byte(`name: prod
type: Environment`),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			describer := tc.given(ctrl)

			// WHEN
			mft, err := describer.Manifest()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, string(tc.wantedManifest), string(mft), "expected manifests to match")
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
		"should return version.LegacyEnvTemplate version if legacy template": {
			given: func(ctrl *gomock.Controller) *EnvDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				m.EXPECT().StackMetadata().Return("", nil)
				return &EnvDescriber{
					app: "phonetool",
					env: &config.Environment{Name: "test"},
					cfn: m,
				}
			},
			wantedVersion: version.LegacyEnvTemplate,
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
						cfnstack.EnvParamServiceDiscoveryEndpoint: "test.phonetool.local",
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
						cfnstack.EnvParamServiceDiscoveryEndpoint: "",
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

func TestEnvDescriber_Features(t *testing.T) {
	testCases := map[string]struct {
		setupMock func(m *envDescriberMocks)

		wanted    []string
		wantedErr error
	}{
		"error describing stack": {
			setupMock: func(m *envDescriberMocks) {
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{}, errors.New("some error"))
			},
			wantedErr: errors.New("some error"),
		},
		"return outdated features": {
			setupMock: func(m *envDescriberMocks) {
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: map[string]string{
						"AppName":                   "mock-app",
						"EnvironmentName":           "mock-env",
						"ToolsAccountPrincipalARN":  "mock-arn",
						"AppDNSName":                "mock-dns",
						"AppDNSDelegationRole":      "mock-role",
						template.ALBFeatureName:     "workload1,workload2",
						template.EFSFeatureName:     "",
						template.NATFeatureName:     "",
						template.AliasesFeatureName: "",
					},
				}, nil)
			},
			wanted: []string{template.ALBFeatureName, template.EFSFeatureName, template.NATFeatureName, template.AliasesFeatureName},
		},
		"return up-to-date features": {
			setupMock: func(m *envDescriberMocks) {
				mockParams := map[string]string{
					"AppName":                  "mock-app",
					"EnvironmentName":          "mock-env",
					"ToolsAccountPrincipalARN": "mock-arn",
					"AppDNSName":               "mock-dns",
					"AppDNSDelegationRole":     "mock-role",
				}
				for _, f := range template.AvailableEnvFeatures() {
					mockParams[f] = ""
				}
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: mockParams,
				}, nil)
			},
			wanted: template.AvailableEnvFeatures(),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &envDescriberMocks{
				stackDescriber: mocks.NewMockstackDescriber(ctrl),
			}
			tc.setupMock(m)
			d := &EnvDescriber{
				cfn: m.stackDescriber,
			}

			// WHEN
			got, err := d.AvailableFeatures()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, got, "expected features to match")
			}
		})
	}
}

func TestEnvDescriber_ValidateCFServiceDomainAliases(t *testing.T) {
	const (
		mockAppName                  = "mock-app"
		mockEnvName                  = "mock-env"
		mockALBWorkloads             = "svc-1,svc-2"
		mockAliasesJsonString        = `{"svc-1":["test.copilot.com"],"svc-2":["test.copilot.com"]}`
		mockInvalidAliasesJsonString = `{"svc-1":["test.copilot.com"]}`
	)
	mockEnvConfig := config.Environment{
		App:  mockAppName,
		Name: mockEnvName,
	}

	testCases := map[string]struct {
		setupMock func(m *envDescriberMocks)

		wantedErr error
	}{
		"error describing stack": {
			setupMock: func(m *envDescriberMocks) {
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{}, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("describe stack: some error"),
		},
		"no load balanced services": {
			setupMock: func(m *envDescriberMocks) {
				mockParams := map[string]string{
					"AppName":         mockAppName,
					"EnvironmentName": mockEnvName,
				}
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: mockParams,
				}, nil)
			},
		},
		"no load balanced services with empty value for the ALBWorkloads parameter": {
			setupMock: func(m *envDescriberMocks) {
				mockParams := map[string]string{
					"AppName":         mockAppName,
					"EnvironmentName": mockEnvName,
					"ALBWorkloads":    "",
				}
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: mockParams,
				}, nil)
			},
		},
		"missing aliases parameter in env stack": {
			setupMock: func(m *envDescriberMocks) {
				mockParams := map[string]string{
					"AppName":         mockAppName,
					"EnvironmentName": mockEnvName,
					"ALBWorkloads":    mockALBWorkloads,
				}
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: mockParams,
				}, nil)
			},
			wantedErr: fmt.Errorf("cannot find %s in env stack parameter set", cfnstack.EnvParamAliasesKey),
		},
		"error unmarshalling json string": {
			setupMock: func(m *envDescriberMocks) {
				mockParams := map[string]string{
					"AppName":         mockAppName,
					"EnvironmentName": mockEnvName,
					"ALBWorkloads":    mockALBWorkloads,
					"Aliases":         "mock-invalid-aliases",
				}
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: mockParams,
				}, nil)
			},
			wantedErr: fmt.Errorf("unmarshal \"mock-invalid-aliases\": invalid character 'm' looking for beginning of value"),
		},
		"no alb workloads have aliases parameter": {
			setupMock: func(m *envDescriberMocks) {
				mockParams := map[string]string{
					"AppName":         mockAppName,
					"EnvironmentName": mockEnvName,
					"ALBWorkloads":    mockALBWorkloads,
					"Aliases":         "",
				}
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: mockParams,
				}, nil)
			},
			wantedErr: fmt.Errorf("services \"svc-1\" and \"svc-2\" must have \"http.alias\" specified when CloudFront is enabled"),
		},
		"not all valid services have an alias": {
			setupMock: func(m *envDescriberMocks) {
				mockParams := map[string]string{
					"AppName":         mockAppName,
					"EnvironmentName": mockEnvName,
					"ALBWorkloads":    mockALBWorkloads,
					"Aliases":         mockInvalidAliasesJsonString,
				}
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: mockParams,
				}, nil)
			},
			wantedErr: fmt.Errorf("service \"svc-2\" must have \"http.alias\" specified when CloudFront is enabled"),
		},
		"all valid services have an alias": {
			setupMock: func(m *envDescriberMocks) {
				mockParams := map[string]string{
					"AppName":         mockAppName,
					"EnvironmentName": mockEnvName,
					"ALBWorkloads":    mockALBWorkloads,
					"Aliases":         mockAliasesJsonString,
				}
				m.stackDescriber.EXPECT().Describe().Return(stack.StackDescription{
					Parameters: mockParams,
				}, nil)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &envDescriberMocks{
				stackDescriber: mocks.NewMockstackDescriber(ctrl),
			}
			tc.setupMock(m)
			d := &EnvDescriber{
				app:         mockAppName,
				env:         &mockEnvConfig,
				cfn:         m.stackDescriber,
				deployStore: m.deployStoreSvc,
			}

			// WHEN
			err := d.ValidateCFServiceDomainAliases()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
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
	testJob1 := &config.Workload{
		App:  "testApp",
		Name: "testJob1",
		Type: "Scheduled Job",
	}
	allSvcs := []*config.Workload{testSvc1, testSvc2, testSvc3}
	allJobs := []*config.Workload{testJob1}
	wantedContent := "{\"environment\":{\"app\":\"testApp\",\"name\":\"testEnv\",\"region\":\"us-west-2\",\"accountID\":\"123456789012\",\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\",\"customConfig\":{}},\"services\":[{\"app\":\"testApp\",\"name\":\"testSvc1\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc2\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc3\",\"type\":\"load-balanced\"}],\"jobs\":[{\"app\":\"testApp\",\"name\":\"testJob1\",\"type\":\"Scheduled Job\"}],\"tags\":{\"key1\":\"value1\",\"key2\":\"value2\"},\"resources\":[{\"type\":\"AWS::IAM::Role\",\"physicalID\":\"testApp-testEnv-CFNExecutionRole\"},{\"type\":\"testApp-testEnv-Cluster\",\"physicalID\":\"AWS::ECS::Cluster-jI63pYBWU6BZ\"}],\"environmentVPC\":{\"id\":\"\",\"publicSubnetIDs\":null,\"privateSubnetIDs\":null}}\n"

	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := &EnvDescription{
		Environment:    testEnv,
		EnvironmentVPC: EnvironmentVPC{},
		Services:       allSvcs,
		Jobs:           allJobs,
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
	testJob1 := &config.Workload{
		App:  "testApp",
		Name: "testJob1",
		Type: "Scheduled Job",
	}
	allSvcs := []*config.Workload{testSvc1, testSvc2, testSvc3}
	allJobs := []*config.Workload{testJob1}

	wantedContent := `About

  Name        testEnv
  Region      us-west-2
  Account ID  123456789012

Workloads

  Name      Type
  ----      ----
  testSvc1  load-balanced
  testSvc2  load-balanced
  testSvc3  load-balanced
  testJob1  Scheduled Job

Tags

  Key     Value
  ---     -----
  key1    value1
  key2    value2

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
		Jobs:        allJobs,
		Tags:        testApp.Tags,
		Resources:   wantedResources,
	}

	// WHEN
	actual := d.HumanString()

	// THEN
	require.Equal(t, wantedContent, actual)
}
