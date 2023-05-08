// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestStaticSiteDeployer_UploadArtifacts(t *testing.T) {
	tests := map[string]struct {
		mock func(m *mocks.MockfileUploader)

		expected *UploadArtifactsOutput
		wantErr  error
	}{
		"error if failed to upload": {
			mock: func(m *mocks.MockfileUploader) {
				m.EXPECT().UploadFiles(gomock.Any()).Return("", errors.New("some error"))
			},
			wantErr: fmt.Errorf("upload static files: some error"),
		},
		"success": {
			mock: func(m *mocks.MockfileUploader) {
				m.EXPECT().UploadFiles([]manifest.FileUpload{
					{
						Source:      "assets",
						Context:     "frontend",
						Destination: "static",
						Recursive:   true,
						Exclude: manifest.StringSliceOrString{
							String: aws.String("*.manifest"),
						},
					},
				}).Return("asdf", nil)
			},
			expected: &UploadArtifactsOutput{
				CustomResourceURLs:             map[string]string{},
				StaticSiteAssetMappingLocation: "s3://mockArtifactBucket/asdf",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockfileUploader(ctrl)
			if tc.mock != nil {
				tc.mock(m)
			}

			deployer := &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						customResources: func(fs template.Reader) ([]*customresource.CustomResource, error) {
							return nil, nil
						},
						mft: &mockWorkloadMft{},
						resources: &stack.AppRegionalResources{
							S3Bucket: "mockArtifactBucket",
						},
					},
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						FileUploads: []manifest.FileUpload{
							{
								Source:      "assets",
								Context:     "frontend",
								Destination: "static",
								Recursive:   true,
								Exclude: manifest.StringSliceOrString{
									String: aws.String("*.manifest"),
								},
							},
						},
					},
				},
				uploader: m,
			}

			actual, err := deployer.UploadArtifacts()
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}

func TestStaticSiteDeployer_stackConfiguration(t *testing.T) {
	tests := map[string]struct {
		deployer *staticSiteDeployer
		wantErr  string
	}{
		"error getting service discovery endpoint": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", errors.New("some error")
							},
						},
					},
				},
			},
			wantErr: "get service discovery endpoint: some error",
		},
		"error getting env version": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						env: &config.Environment{
							Name: "mockEnv",
						},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", errors.New("some error")
							},
						},
					},
				},
			},
			wantErr: `get version of environment "mockEnv": some error`,
		},
		"error getting app version": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app: &config.Application{
							Name: "mockApp",
						},
						env: &config.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", nil
							},
						},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: func() (string, error) {
						return "", errors.New("some error")
					},
				},
				staticSiteMft: &manifest.StaticSite{},
			},
			wantErr: `static sites not supported: get version for app "mockApp": some error`,
		},
		"error bc app version out of date": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app: &config.Application{},
						env: &config.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", nil
							},
						},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: func() (string, error) {
						return "v1.1.0", nil
					},
				},
				staticSiteMft: &manifest.StaticSite{},
			},
			wantErr: `static sites not supported: app version must be >= v1.2.0`,
		},
		"error bc alias specified and env has imported certs": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app: &config.Application{},
						env: &config.Environment{
							Name: "mockEnv",
						},
						envConfig: &manifest.Environment{
							EnvironmentConfig: manifest.EnvironmentConfig{
								HTTPConfig: manifest.EnvironmentHTTPConfig{
									Public: manifest.PublicHTTPConfig{
										Certificates: []string{"mockCert"},
									},
								},
							},
						},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", nil
							},
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: func() (string, error) {
						return "v1.2.0", nil
					},
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						Alias: "hi.com",
					},
				},
			},
			wantErr: `cannot specify alias when env "mockEnv" imports one or more certificates`,
		},
		"error bc alias specified no domain imported": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app:       &config.Application{},
						env:       &config.Environment{},
						envConfig: &manifest.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", nil
							},
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: func() (string, error) {
						return "v1.2.0", nil
					},
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						Alias: "hi.com",
					},
				},
			},
			wantErr: `cannot specify alias when application is not associated with a domain`,
		},
		"error bc invalid alias": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app: &config.Application{
							Name:   "mockApp",
							Domain: "example.com",
						},
						env: &config.Environment{
							Name: "mockEnv",
						},
						envConfig: &manifest.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", nil
							},
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: func() (string, error) {
						return "v1.2.0", nil
					},
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						Alias: "hi.com",
					},
				},
			},
			wantErr: `alias "hi.com" is not supported in hosted zones managed by Copilot`,
		},
		"error creating stack": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app: &config.Application{},
						env: &config.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", nil
							},
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: func() (string, error) {
						return "v1.2.0", nil
					},
				},
				staticSiteMft: &manifest.StaticSite{},
				newStack: func(*stack.StaticSiteConfig) (*stack.StaticSite, error) {
					return nil, errors.New("some error")
				},
			},
			wantErr: `create stack configuration: some error`,
		},
		"success": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app: &config.Application{},
						env: &config.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", nil
							},
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: func() (string, error) {
						return "v1.2.0", nil
					},
				},
				staticSiteMft: &manifest.StaticSite{},
				newStack: func(*stack.StaticSiteConfig) (*stack.StaticSite, error) {
					return nil, nil
				},
			},
		},
		"success with alias": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app: &config.Application{
							Name:   "mockApp",
							Domain: "example.com",
						},
						env: &config.Environment{
							Name: "mockEnv",
						},
						envConfig: &manifest.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: func() (string, error) {
								return "", nil
							},
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: func() (string, error) {
								return "", nil
							},
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: func() (string, error) {
						return "v1.2.0", nil
					},
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						Alias: "hi.mockApp.example.com",
					},
				},
				newStack: func(*stack.StaticSiteConfig) (*stack.StaticSite, error) {
					return nil, nil
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, gotErr := tc.deployer.stackConfiguration(&StackRuntimeConfiguration{})
			if tc.wantErr != "" {
				require.EqualError(t, gotErr, tc.wantErr)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func ThingAndError[A any]() (A, error) {
	var zero A
	return zero, errors.New("some error")
}
