// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/aws/copilot-cli/internal/pkg/override"
	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestBackendSvcDeployer_GenerateCloudFormationTemplate(t *testing.T) {
	t.Run("ensure resulting CloudFormation template custom resource paths are empty", func(t *testing.T) {
		// GIVEN
		backend := mockBackendServiceDeployer()

		// WHEN
		out, err := backend.GenerateCloudFormationTemplate(&GenerateCloudFormationTemplateInput{})

		// THEN
		require.NoError(t, err)

		type lambdaFn struct {
			Properties struct {
				Code struct {
					S3Bucket string `yaml:"S3bucket"`
					S3Key    string `yaml:"S3Key"`
				} `yaml:"Code"`
			} `yaml:"Properties"`
		}
		dat := struct {
			Resources struct {
				EnvControllerFunction lambdaFn `yaml:"EnvControllerFunction"`
				RulePriorityFunction  lambdaFn `yaml:"RulePriorityFunction"`
			} `yaml:"Resources"`
		}{}
		require.NoError(t, yaml.Unmarshal([]byte(out.Template), &dat))
		require.Empty(t, dat.Resources.EnvControllerFunction.Properties.Code.S3Bucket)
		require.Empty(t, dat.Resources.EnvControllerFunction.Properties.Code.S3Key)

		require.Empty(t, dat.Resources.RulePriorityFunction.Properties.Code.S3Bucket)
		require.Empty(t, dat.Resources.RulePriorityFunction.Properties.Code.S3Key)
	})
}

func TestBackendSvcDeployer_stackConfiguration(t *testing.T) {
	const (
		mockAppName = "mock-app"
		mockEnvName = "mock-env"
		mockSvcName = "mock-svc"
	)

	tests := map[string]struct {
		App      *config.Application
		Env      *config.Environment
		Manifest *manifest.BackendService

		// Cached variables.
		inEnvironmentConfig func() *manifest.Environment

		setupMocks func(m *deployMocks)

		expectedErr string
	}{
		"success if alb not configured": {
			App: &config.Application{
				Name: mockAppName,
			},
			Env: &config.Environment{
				Name: mockEnvName,
			},
			Manifest: &manifest.BackendService{},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
		},
		"failure if alias configured, no env certs": {
			App: &config.Application{
				Name: mockAppName,
			},
			Env: &config.Environment{
				Name: mockEnvName,
			},
			Manifest: &manifest.BackendService{
				BackendServiceConfig: manifest.BackendServiceConfig{
					HTTP: manifest.HTTP{
						Main: manifest.RoutingRule{
							Alias: manifest.Alias{
								AdvancedAliases: []manifest.AdvancedAlias{
									{Alias: aws.String("go.dev")},
								},
							},
						},
					},
				},
			},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			expectedErr: `validate ALB runtime configuration for "http": cannot specify "alias" in an environment without imported certs`,
		},
		"failure if cert validation fails": {
			App: &config.Application{
				Name: mockAppName,
			},
			Env: &config.Environment{
				Name: mockEnvName,
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Private.Certificates = []string{"mockCertARN"}
				return envConfig
			},
			Manifest: &manifest.BackendService{
				BackendServiceConfig: manifest.BackendServiceConfig{
					HTTP: manifest.HTTP{
						Main: manifest.RoutingRule{
							Alias: manifest.Alias{
								AdvancedAliases: []manifest.AdvancedAlias{
									{Alias: aws.String("go.dev")},
								},
							},
						},
					},
				},
			},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"go.dev"}, []string{"mockCertARN"}).Return(errors.New("some error"))
			},
			expectedErr: "validate ALB runtime configuration for \"http\": validate aliases against the imported certificate for env mock-env: some error",
		},
		"success if cert validation succeeds": {
			App: &config.Application{
				Name: mockAppName,
			},
			Env: &config.Environment{
				Name: mockEnvName,
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Private.Certificates = []string{"mockCertARN"}
				return envConfig
			},
			Manifest: &manifest.BackendService{
				BackendServiceConfig: manifest.BackendServiceConfig{
					HTTP: manifest.HTTP{
						Main: manifest.RoutingRule{
							Alias: manifest.Alias{
								AdvancedAliases: []manifest.AdvancedAlias{
									{Alias: aws.String("go.dev")},
								},
							},
						},
						AdditionalRoutingRules: []manifest.RoutingRule{
							{
								Alias: manifest.Alias{
									AdvancedAliases: []manifest.AdvancedAlias{
										{Alias: aws.String("go.test")},
									},
								},
							},
						},
					},
				},
			},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"go.dev"}, []string{"mockCertARN"}).Return(nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"go.test"}, []string{"mockCertARN"}).Return(nil)
			},
		},
		"failure if env has imported certs but no alias set": {
			App: &config.Application{
				Name: mockAppName,
			},
			Env: &config.Environment{
				Name: mockEnvName,
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Private.Certificates = []string{"mockCertARN"}
				return envConfig
			},
			Manifest: &manifest.BackendService{
				BackendServiceConfig: manifest.BackendServiceConfig{
					HTTP: manifest.HTTP{
						Main: manifest.RoutingRule{
							Path: aws.String("/"),
						},
					},
				},
			},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			expectedErr: `validate ALB runtime configuration for "http": cannot deploy service mock-svc without "alias" to environment mock-env with certificate imported`,
		},
		"failure if env has imported certs but no alias set in additional rules": {
			App: &config.Application{
				Name: mockAppName,
			},
			Env: &config.Environment{
				Name: mockEnvName,
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Private.Certificates = []string{"mockCertARN"}
				return envConfig
			},
			Manifest: &manifest.BackendService{
				BackendServiceConfig: manifest.BackendServiceConfig{
					HTTP: manifest.HTTP{
						Main: manifest.RoutingRule{
							Path: aws.String("/"),
							Alias: manifest.Alias{
								AdvancedAliases: []manifest.AdvancedAlias{
									{Alias: aws.String("go.test")},
								},
							},
						},
						AdditionalRoutingRules: []manifest.RoutingRule{
							{
								Path: aws.String("/admin"),
							},
						},
					},
				},
			},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"go.test"}, []string{"mockCertARN"}).Return(nil)
			},
			expectedErr: `validate ALB runtime configuration for "http.additional_rules[0]": cannot deploy service mock-svc without "alias" to environment mock-env with certificate imported`,
		},
		"success if env has imported certs but alb not configured": {
			App: &config.Application{
				Name: mockAppName,
			},
			Env: &config.Environment{
				Name: mockEnvName,
				CustomConfig: &config.CustomizeEnv{
					ImportCertARNs: []string{"mockCertARN"},
				},
			},
			Manifest: &manifest.BackendService{},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployMocks{
				mockEndpointGetter:   mocks.NewMockendpointGetter(ctrl),
				mockValidator:        mocks.NewMockaliasCertValidator(ctrl),
				mockEnvVersionGetter: mocks.NewMockversionGetter(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}
			if tc.inEnvironmentConfig == nil {
				tc.inEnvironmentConfig = func() *manifest.Environment {
					return &manifest.Environment{}
				}
			}
			deployer := &backendSvcDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						name:             mockSvcName,
						app:              tc.App,
						env:              tc.Env,
						endpointGetter:   m.mockEndpointGetter,
						resources:        &stack.AppRegionalResources{},
						envConfig:        tc.inEnvironmentConfig(),
						envVersionGetter: m.mockEnvVersionGetter,
					},
					newSvcUpdater: func(f func(*session.Session) serviceForceUpdater) serviceForceUpdater {
						return nil
					},
				},
				backendMft:         tc.Manifest,
				aliasCertValidator: m.mockValidator,
				newStack: func() cloudformation.StackConfiguration {
					return new(stubCloudFormationStack)
				},
			}

			_, err := deployer.stackConfiguration(&StackRuntimeConfiguration{})
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func mockBackendServiceDeployer(opts ...func(*backendSvcDeployer)) *backendSvcDeployer {
	deployer := &backendSvcDeployer{
		svcDeployer: &svcDeployer{
			workloadDeployer: &workloadDeployer{
				name: "example",
				app: &config.Application{
					Name: "demo",
				},
				env: &config.Environment{
					App:  "demo",
					Name: "test",
				},
				resources:        &stack.AppRegionalResources{},
				envConfig:        new(manifest.Environment),
				endpointGetter:   &mockEndpointGetter{endpoint: "demo.test.local"},
				envVersionGetter: &mockEnvVersionGetter{version: "v1.0.0"},
				overrider:        new(override.Noop),
			},
			newSvcUpdater: func(f func(*session.Session) serviceForceUpdater) serviceForceUpdater {
				return nil
			},
			now: func() time.Time {
				return time.Date(2020, 11, 23, 0, 0, 0, 0, time.UTC)
			},
		},
		backendMft: &manifest.BackendService{
			Workload: manifest.Workload{
				Name: aws.String("example"),
			},
			BackendServiceConfig: manifest.BackendServiceConfig{
				TaskConfig: manifest.TaskConfig{
					Count: manifest.Count{
						Value: aws.Int(1),
					},
				},
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							ImageLocationOrBuild: manifest.ImageLocationOrBuild{
								Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
							},
						},
						Port: aws.Uint16(80),
					},
				},
			},
		},
		newStack: func() cloudformation.StackConfiguration {
			return new(stubCloudFormationStack)
		},
	}
	for _, opt := range opts {
		opt(deployer)
	}
	return deployer
}
