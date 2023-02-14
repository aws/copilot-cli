// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"

	"github.com/aws/copilot-cli/internal/pkg/override"
	"gopkg.in/yaml.v3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRdwsDeployer_GenerateCloudFormationTemplate(t *testing.T) {
	t.Run("ensure resulting CloudFormation template custom resource paths are empty", func(t *testing.T) {
		// GIVEN
		rdws := mockRDWSDeployer()

		// WHEN
		out, err := rdws.GenerateCloudFormationTemplate(&GenerateCloudFormationTemplateInput{})

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
			} `yaml:"Resources"`
		}{}
		require.NoError(t, yaml.Unmarshal([]byte(out.Template), &dat))
		require.Empty(t, dat.Resources.EnvControllerFunction.Properties.Code.S3Bucket)
		require.Empty(t, dat.Resources.EnvControllerFunction.Properties.Code.S3Key)
	})
}

type deployRDSvcMocks struct {
	mockAppVersionGetter *mocks.MockversionGetter
	mockEnvVersionGetter *mocks.MockversionGetter
	mockEndpointGetter   *mocks.MockendpointGetter
}

func TestSvcDeployOpts_rdWebServiceStackConfiguration(t *testing.T) {
	const (
		mockAppName   = "mockApp"
		mockEnvName   = "mockEnv"
		mockName      = "mockWkld"
		mockAddonsURL = "mockAddonsURL"
		mockBucket    = "mockBucket"
	)
	mockResources := &stack.AppRegionalResources{
		S3Bucket: mockBucket,
	}
	tests := map[string]struct {
		inAlias       string
		inApp         *config.Application
		inEnvironment *config.Environment

		mock func(m *deployRDSvcMocks)

		wantAlias string
		wantErr   error
	}{
		"alias used while app is not associated with a domain": {
			inAlias: "v1.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},

			wantErr: errors.New("alias specified when application is not associated with a domain"),
		},
		"invalid alias with unknown domain": {
			inAlias: "v1.someRandomDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},

			wantErr: fmt.Errorf("alias is not supported in hosted zones that are not managed by Copilot"),
		},
		"invalid environment level alias": {
			inAlias: "mockEnv.mockApp.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},

			wantErr: fmt.Errorf("mockEnv.mockApp.mockDomain is an environment-level alias, which is not supported yet"),
		},
		"invalid application level alias": {
			inAlias: "someSub.mockApp.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},

			wantErr: fmt.Errorf("someSub.mockApp.mockDomain is an application-level alias, which is not supported yet"),
		},
		"invalid root level alias": {
			inAlias: "mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},

			wantErr: fmt.Errorf("mockDomain is a root domain alias, which is not supported yet"),
		},
		"success": {
			inAlias: "v1.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantAlias: "v1.mockDomain",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployRDSvcMocks{
				mockAppVersionGetter: mocks.NewMockversionGetter(ctrl),
				mockEnvVersionGetter: mocks.NewMockversionGetter(ctrl),
				mockEndpointGetter:   mocks.NewMockendpointGetter(ctrl),
			}
			tc.mock(m)

			deployer := rdwsDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						name:             mockName,
						app:              tc.inApp,
						env:              tc.inEnvironment,
						resources:        mockResources,
						endpointGetter:   m.mockEndpointGetter,
						envVersionGetter: m.mockEnvVersionGetter,
					},
					newSvcUpdater: func(f func(*session.Session) serviceForceUpdater) serviceForceUpdater {
						return nil
					},
				},
				appVersionGetter: m.mockAppVersionGetter,
				rdwsMft: &manifest.RequestDrivenWebService{
					Workload: manifest.Workload{
						Name: aws.String(mockName),
					},
					RequestDrivenWebServiceConfig: manifest.RequestDrivenWebServiceConfig{
						ImageConfig: manifest.ImageWithPort{
							Image: manifest.Image{
								ImageLocationOrBuild: manifest.ImageLocationOrBuild{
									Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
								},
							},
							Port: aws.Uint16(80),
						},
						RequestDrivenWebServiceHttpConfig: manifest.RequestDrivenWebServiceHttpConfig{
							Alias: aws.String(tc.inAlias),
						},
					},
				},
				newStack: func() cloudformation.StackConfiguration {
					return new(stubCloudFormationStack)
				},
			}

			got, gotErr := deployer.stackConfiguration(&StackRuntimeConfiguration{
				AddonsURL: mockAddonsURL,
			})

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantAlias, got.rdSvcAlias)
			}
		})
	}
}

func mockRDWSDeployer(opts ...func(*rdwsDeployer)) *rdwsDeployer {
	deployer := &rdwsDeployer{
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
		rdwsMft: &manifest.RequestDrivenWebService{
			Workload: manifest.Workload{
				Name: aws.String("example"),
			},
			RequestDrivenWebServiceConfig: manifest.RequestDrivenWebServiceConfig{
				ImageConfig: manifest.ImageWithPort{
					Image: manifest.Image{
						ImageLocationOrBuild: manifest.ImageLocationOrBuild{
							Location: aws.String("111111111111.dkr.ecr.us-west-2.amazonaws.com/nginx:latest"),
						},
					},
					Port: aws.Uint16(80),
				},
				InstanceConfig: manifest.AppRunnerInstanceConfig{
					CPU:    aws.Int(1024),
					Memory: aws.Int(2048),
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
