// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

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
					RoutingRule: manifest.RoutingRuleConfiguration{
						Alias: manifest.Alias{
							AdvancedAliases: []manifest.AdvancedAlias{
								{Alias: aws.String("go.dev")},
							},
						},
					},
				},
			},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			expectedErr: `cannot specify "alias" in an environment without imported certs`,
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
					RoutingRule: manifest.RoutingRuleConfiguration{
						Alias: manifest.Alias{
							AdvancedAliases: []manifest.AdvancedAlias{
								{Alias: aws.String("go.dev")},
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
			expectedErr: "validate aliases against the imported certificate for env mock-env: some error",
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
					RoutingRule: manifest.RoutingRuleConfiguration{
						Alias: manifest.Alias{
							AdvancedAliases: []manifest.AdvancedAlias{
								{Alias: aws.String("go.dev")},
							},
						},
					},
				},
			},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"go.dev"}, []string{"mockCertARN"}).Return(nil)
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
					RoutingRule: manifest.RoutingRuleConfiguration{
						Path: aws.String("/"),
					},
				},
			},
			setupMocks: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return(mockAppName+".local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			expectedErr: `cannot deploy service mock-svc without http.alias to environment mock-env with certificate imported`,
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
						envConfig:        tc.inEnvironmentConfig(),
						envVersionGetter: m.mockEnvVersionGetter,
					},
					newSvcUpdater: func(f func(*session.Session) serviceForceUpdater) serviceForceUpdater {
						return nil
					},
				},
				backendMft:         tc.Manifest,
				aliasCertValidator: m.mockValidator,
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
