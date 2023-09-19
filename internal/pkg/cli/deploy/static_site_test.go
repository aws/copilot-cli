// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	deployCFN "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestStaticSiteDeployer_UploadArtifacts(t *testing.T) {
	type mockDeps struct {
		uploader     *mocks.MockfileUploader
		fs           func() afero.Fs
		cachedWSRoot string
	}
	tests := map[string]struct {
		mock func(m *mockDeps)

		expected *UploadArtifactsOutput
		wantErr  error
	}{
		"error if failed to upload": {
			mock: func(m *mockDeps) {
				m.cachedWSRoot = "mockRoot"
				m.fs = func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("mockRoot/assets/", 0755)
					return fs
				}
				m.uploader.EXPECT().UploadFiles(gomock.Any()).Return("", errors.New("some error"))
			},
			wantErr: fmt.Errorf("upload static files: some error"),
		},
		"error if source path does not exist": {
			mock: func(m *mockDeps) {
				m.cachedWSRoot = "mockRoot"
				m.fs = func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("mockrOOt/assets/", 0755)
					return fs
				}
			},
			wantErr: errors.New(`source "assets" must be a valid path relative to the workspace root "mockRoot": open mockRoot/assets: file does not exist`),
		},
		"success": {
			mock: func(m *mockDeps) {
				m.cachedWSRoot = "mockRoot"
				m.fs = func() afero.Fs {
					fs := afero.NewMemMapFs()
					_ = fs.Mkdir("mockRoot/assets/", 0755)
					return fs
				}
				m.uploader.EXPECT().UploadFiles([]manifest.FileUpload{
					{
						Source:      "mockRoot/assets",
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

			m := &mockDeps{
				uploader: mocks.NewMockfileUploader(ctrl),
			}
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
								Destination: "static",
								Recursive:   true,
								Exclude: manifest.StringSliceOrString{
									String: aws.String("*.manifest"),
								},
							},
						},
					},
				},
				uploader: m.uploader,
				fs:       m.fs(),
				wsRoot:   m.cachedWSRoot,
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
		deployer     *staticSiteDeployer
		wantErr      string
		wantTemplate string
	}{
		"error getting service discovery endpoint": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: ReturnsValues("", errors.New("some error")),
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
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", errors.New("some error")),
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
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("", errors.New("some error")),
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
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("v1.1.0", error(nil)),
				},
				staticSiteMft: &manifest.StaticSite{},
			},
			wantErr: `static sites not supported: app version must be >= v1.2.0`,
		},
		"error bc alias specified no domain imported and no imported cert": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app:       &config.Application{},
						env:       &config.Environment{},
						envConfig: &manifest.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("v1.2.0", error(nil)),
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						HTTP: manifest.StaticSiteHTTP{
							Alias: "hi.com",
						},
					},
				},
			},
			wantErr: `cannot specify alias when application is not associated with a domain or "http.certificate" is not set`,
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
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("v1.2.0", error(nil)),
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						HTTP: manifest.StaticSiteHTTP{
							Alias: "hi.com",
						},
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
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("v1.2.0", error(nil)),
				},
				staticSiteMft: &manifest.StaticSite{},
				newStack: func(*stack.StaticSiteConfig) (deployCFN.StackConfiguration, error) {
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
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("v1.2.0", error(nil)),
				},
				staticSiteMft: &manifest.StaticSite{},
				newStack: func(*stack.StaticSiteConfig) (deployCFN.StackConfiguration, error) {
					return nil, nil
				},
			},
		},
		"success with app alias": {
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
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("v1.2.0", error(nil)),
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						HTTP: manifest.StaticSiteHTTP{
							Alias: "hi.mockApp.example.com",
						},
					},
				},
				newStack: func(*stack.StaticSiteConfig) (deployCFN.StackConfiguration, error) {
					return nil, nil
				},
			},
		},
		"success with cert alias": {
			deployer: &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						app: &config.Application{
							Name: "mockApp",
						},
						env: &config.Environment{
							Name: "mockEnv",
						},
						envConfig: &manifest.Environment{},
						endpointGetter: &endpointGetterDouble{
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
						resources: &stack.AppRegionalResources{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("v1.2.0", error(nil)),
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						HTTP: manifest.StaticSiteHTTP{
							Alias:       "random.example.com",
							Certificate: "mockCert",
						},
					},
				},
				newStack: func(*stack.StaticSiteConfig) (deployCFN.StackConfiguration, error) {
					return nil, nil
				},
			},
		},
		"success with overrider": {
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
							ServiceDiscoveryEndpointFn: ReturnsValues("", error(nil)),
						},
						envVersionGetter: &versionGetterDouble{
							VersionFn: ReturnsValues("", error(nil)),
						},
						resources: &stack.AppRegionalResources{},
						overrider: &mockOverrider{},
					},
				},
				appVersionGetter: &versionGetterDouble{
					VersionFn: ReturnsValues("v1.2.0", error(nil)),
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						HTTP: manifest.StaticSiteHTTP{
							Alias: "hi.mockApp.example.com",
						},
					},
				},
				newStack: func(*stack.StaticSiteConfig) (deployCFN.StackConfiguration, error) {
					return &mockStaticSite{}, nil
				},
			},
			wantTemplate: "mockOverride",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			out, gotErr := tc.deployer.stackConfiguration(&StackRuntimeConfiguration{})
			if tc.wantErr != "" {
				require.EqualError(t, gotErr, tc.wantErr)
				return
			}
			require.NoError(t, gotErr)
			if tc.wantTemplate != "" {
				s, err := out.Template()
				require.NoError(t, err)
				require.Equal(t, tc.wantTemplate, s)
			}
		})
	}
}

func ReturnsValues[A, B any](a A, b B) func() (A, B) {
	return func() (A, B) {
		return a, b
	}
}

type mockOverrider struct{}

func (o *mockOverrider) Override(body []byte) (out []byte, err error) {
	return []byte("mockOverride"), nil
}

type mockStaticSite struct{}

func (s *mockStaticSite) Template() (string, error)                        { return "", nil }
func (s *mockStaticSite) StackName() string                                { return "" }
func (s *mockStaticSite) Parameters() ([]*cloudformation.Parameter, error) { return nil, nil }
func (s *mockStaticSite) Tags() []*cloudformation.Tag                      { return nil }
func (s *mockStaticSite) SerializedParameters() (string, error)            { return "", nil }
