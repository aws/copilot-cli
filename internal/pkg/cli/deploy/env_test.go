// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	cfnclient "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	cfnmocks "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	awselb "github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type envDeployerMocks struct {
	s3               *mocks.Mockuploader
	prefixListGetter *mocks.MockprefixListGetter
	appCFN           *mocks.MockappResourcesGetter
	envDeployer      *mocks.MockenvironmentDeployer
	patcher          *mocks.Mockpatcher
	stackSerializer  *cfnmocks.MockStackConfiguration
	envDescriber     *mocks.MockenvDescriber
	lbDescriber      *mocks.MocklbDescriber
	stackDescribers  map[string]*mocks.MockstackDescriber
	ws               *mocks.MockWorkspaceAddonsReaderPathGetter

	parseAddons func() (stackBuilder, error)
	addons      *mocks.MockstackBuilder
}

func TestEnvDeployer_UploadArtifacts(t *testing.T) {
	const (
		mockEnvRegion = "mockEnvRegion"
	)
	mockApp := &config.Application{}
	testCases := map[string]struct {
		setUpMocks               func(m *envDeployerMocks)
		wantedAddonsURL          string
		wantedCustomResourceURLs map[string]string
		wantedError              error
	}{
		"fail to get app resource by region": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get app resources in region %s: some error", mockEnvRegion),
		},
		"fail to find S3 bucket in the region": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{}, nil)
			},
			wantedError: fmt.Errorf("cannot find the S3 artifact bucket in region %s", mockEnvRegion),
		},
		"fail to patch the environment": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(errors.New("some error"))
			},
			wantedError: errors.New("ensure env manager role has permissions to upload: some error"),
		},
		"fail to upload custom resource scripts": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(nil)
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Any(), gomock.Any()).AnyTimes().Return("", fmt.Errorf("some error"))
			},
			wantedError: errors.New("upload custom resources to bucket mockS3Bucket"),
		},
		"fail to parse addons": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(nil)
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Any(), gomock.Any()).AnyTimes().Return("", nil)
				m.parseAddons = func() (stackBuilder, error) {
					return nil, errors.New("some error")
				}
			},
			wantedError: errors.New("parse environment addons: some error"),
		},
		"fail to package addons asset": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(nil)
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Any(), gomock.Any()).AnyTimes().Return("", nil)
				m.parseAddons = func() (stackBuilder, error) {
					return m.addons, nil
				}
				m.ws.EXPECT().Path().Return("mockPath")
				m.addons.EXPECT().Package(gomock.Any()).Return(errors.New("some error"))
			},
			wantedError: errors.New("package environment addons: some error"),
		},
		"fail to render addons template": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(nil)
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Any(), gomock.Any()).AnyTimes().Return("", nil)
				m.parseAddons = func() (stackBuilder, error) {
					return m.addons, nil
				}
				m.ws.EXPECT().Path().Return("mockPath")
				m.addons.EXPECT().Package(gomock.Any()).Return(nil)
				m.addons.EXPECT().Template().Return("", errors.New("some error"))
			},
			wantedError: errors.New("render addons template: some error"),
		},
		"fail to upload addons template": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(nil)
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Not(artifactpath.EnvironmentAddons([]byte("mockAddons"))), gomock.Any()).AnyTimes().Return("", nil)
				m.parseAddons = func() (stackBuilder, error) {
					return m.addons, nil
				}
				m.ws.EXPECT().Path().Return("mockPath")
				m.addons.EXPECT().Package(gomock.Any()).Return(nil)
				m.addons.EXPECT().Template().Return("mockAddons", nil)
				m.s3.EXPECT().Upload("mockS3Bucket", artifactpath.EnvironmentAddons([]byte("mockAddons")), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedError: errors.New("upload addons template to bucket mockS3Bucket: some error"),
		},
		"success with addons and custom resources URLs": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(nil)
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Not(artifactpath.EnvironmentAddons([]byte("mockAddons"))), gomock.Any()).AnyTimes().Return("", nil)
				m.parseAddons = func() (stackBuilder, error) {
					return m.addons, nil
				}
				m.ws.EXPECT().Path().Return("mockPath")
				m.addons.EXPECT().Package(gomock.Any()).Return(nil)
				m.addons.EXPECT().Template().Return("mockAddons", nil)
				m.s3.EXPECT().Upload("mockS3Bucket", artifactpath.EnvironmentAddons([]byte("mockAddons")), gomock.Any()).
					Return("mockAddonsURL", nil)
			},
			wantedAddonsURL: "mockAddonsURL",
			wantedCustomResourceURLs: map[string]string{
				"CertificateReplicatorFunction": "",
				"CertificateValidationFunction": "",
				"CustomDomainFunction":          "",
				"DNSDelegationFunction":         "",
				"BucketCleanerFunction":         "",
				"UniqueJSONValuesFunction":      "",
			},
		},
		"success with only custom resource URLs returned": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(nil)
				crs, err := customresource.Env(fakeTemplateFS())
				require.NoError(t, err)
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Any(), gomock.Any()).DoAndReturn(func(_, key string, _ io.Reader) (url string, err error) {
					for _, cr := range crs {
						if strings.Contains(key, strings.ToLower(cr.Name())) {
							return "", nil
						}
					}
					return "", errors.New("did not match any custom resource")
				}).Times(len(crs))
				m.parseAddons = func() (stackBuilder, error) {
					return nil, &addon.ErrAddonsNotFound{}
				}
			},
			wantedCustomResourceURLs: map[string]string{
				"CertificateReplicatorFunction": "",
				"CertificateValidationFunction": "",
				"CustomDomainFunction":          "",
				"DNSDelegationFunction":         "",
				"BucketCleanerFunction":         "",
				"UniqueJSONValuesFunction":      "",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &envDeployerMocks{
				appCFN:  mocks.NewMockappResourcesGetter(ctrl),
				s3:      mocks.NewMockuploader(ctrl),
				patcher: mocks.NewMockpatcher(ctrl),
				ws:      mocks.NewMockWorkspaceAddonsReaderPathGetter(ctrl),
				addons:  mocks.NewMockstackBuilder(ctrl),
			}
			tc.setUpMocks(m)

			mockEnv := &config.Environment{
				Name:           "mockEnv",
				ManagerRoleARN: "mockManagerRoleARN",
				Region:         mockEnvRegion,
				App:            "mockApp",
			}
			d := envDeployer{
				app:         mockApp,
				env:         mockEnv,
				appCFN:      m.appCFN,
				s3:          m.s3,
				patcher:     m.patcher,
				templateFS:  fakeTemplateFS(),
				ws:          m.ws,
				parseAddons: m.parseAddons,
			}

			got, gotErr := d.UploadArtifacts()
			if tc.wantedError != nil {
				require.Contains(t, gotErr.Error(), tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantedCustomResourceURLs, got.CustomResourceURLs)
				require.Equal(t, tc.wantedAddonsURL, got.AddonsURL)
			}
		})
	}
}

func TestEnvDeployer_DeployDiff(t *testing.T) {
	testCases := map[string]struct {
		inTemplate string
		setUpMocks func(m *deployDiffMocks)
		wanted     string
		checkErr   func(t *testing.T, gotErr error)
	}{
		"error getting the deployed template": {
			setUpMocks: func(m *deployDiffMocks) {
				m.mockDeployedTmplGetter.
					EXPECT().Template(gomock.Eq(cfnstack.NameForEnv("mockApp", "mockEnv"))).
					Return("", errors.New("some error"))
			},
			checkErr: func(t *testing.T, gotErr error) {
				require.EqualError(t, gotErr, `retrieve the deployed template for "mockEnv": some error`)
			},
		},
		"error parsing the diff against the deployed template": {
			inTemplate: `!!!???what a weird template`,
			setUpMocks: func(m *deployDiffMocks) {
				m.mockDeployedTmplGetter.EXPECT().
					Template(gomock.Eq(cfnstack.NameForEnv("mockApp", "mockEnv"))).
					Return("wow such template", nil)
			},
			checkErr: func(t *testing.T, gotErr error) {
				require.ErrorContains(t, gotErr, `parse the diff against the deployed env stack "mockEnv"`)
			},
		},
		"get the correct diff": {
			inTemplate: `peace: and love`,
			setUpMocks: func(m *deployDiffMocks) {
				m.mockDeployedTmplGetter.EXPECT().
					Template(gomock.Eq(cfnstack.NameForEnv("mockApp", "mockEnv"))).
					Return("peace: und Liebe", nil)
			},
			wanted: `~ peace: und Liebe -> and love
`,
		},
		"get the correct diff when there is no deployed diff": {
			inTemplate: `peace: and love`,
			setUpMocks: func(m *deployDiffMocks) {
				m.mockDeployedTmplGetter.EXPECT().
					Template(gomock.Eq(cfnstack.NameForEnv("mockApp", "mockEnv"))).
					Return("", &cfnclient.ErrStackNotFound{})
			},
			wanted: `+ peace: and love
`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployDiffMocks{
				mockDeployedTmplGetter: mocks.NewMockdeployedTemplateGetter(ctrl),
			}
			tc.setUpMocks(m)
			deployer := envDeployer{
				app: &config.Application{
					Name: "mockApp",
				},
				env: &config.Environment{
					Name: "mockEnv",
				},
				tmplGetter: m.mockDeployedTmplGetter,
			}
			got, gotErr := deployer.DeployDiff(tc.inTemplate)
			if tc.checkErr != nil {
				tc.checkErr(t, gotErr)
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wanted, got)
			}
		})
	}
}

func TestEnvDeployer_AddonsTemplate(t *testing.T) {
	testCases := map[string]struct {
		setUpMocks  func(m *envDeployerMocks)
		wanted      string
		wantedError error
	}{
		"error rendering addons template": {
			setUpMocks: func(m *envDeployerMocks) {
				m.parseAddons = func() (stackBuilder, error) {
					return m.addons, nil
				}
				m.addons.EXPECT().Template().Return("", errors.New("some error"))
			},
			wantedError: errors.New("render addons template: some error"),
		},
		"return empty string when no addons is found": {
			setUpMocks: func(m *envDeployerMocks) {
				m.parseAddons = func() (stackBuilder, error) {
					return nil, &addon.ErrAddonsNotFound{}
				}
			},
		},
		"return the addon template": {
			setUpMocks: func(m *envDeployerMocks) {
				m.parseAddons = func() (stackBuilder, error) {
					return m.addons, nil
				}
				m.addons.EXPECT().Template().Return("mockAddonsTemplate", nil)
			},
			wanted: "mockAddonsTemplate",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &envDeployerMocks{
				addons: mocks.NewMockstackBuilder(ctrl),
			}
			tc.setUpMocks(m)
			d := envDeployer{
				parseAddons: m.parseAddons,
			}
			got, gotErr := d.AddonsTemplate()
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, got, tc.wanted)
			}
		})

	}
}

func TestEnvDeployer_GenerateCloudFormationTemplate(t *testing.T) {
	const (
		mockEnvRegion = "us-west-2"
		mockAppName   = "mockApp"
		mockEnvName   = "mockEnv"
	)
	mockError := errors.New("some error")
	mockApp := &config.Application{
		Name: mockAppName,
	}
	testCases := map[string]struct {
		inManifest manifest.Environment
		setUpMocks func(m *envDeployerMocks, ctrl *gomock.Controller)

		wantedTemplate string
		wantedParams   string
		wantedError    error
	}{
		"fail to get app resources by region": {
			setUpMocks: func(m *envDeployerMocks, _ *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).
					Return(nil, mockError)
			},
			wantedError: errors.New("get app resources in region us-west-2: some error"),
		},
		"fail to get existing parameters": {
			setUpMocks: func(m *envDeployerMocks, _ *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) {
					return nil, &addon.ErrAddonsNotFound{}
				}
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, mockError)
			},
			wantedError: errors.New("describe environment stack parameters: some error"),
		},
		"fail to get existing force update ID": {
			setUpMocks: func(m *envDeployerMocks, _ *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) {
					return nil, &addon.ErrAddonsNotFound{}
				}
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", mockError)
			},
			wantedError: errors.New("retrieve environment stack force update ID: some error"),
		},
		"fail to generate stack template": {
			setUpMocks: func(m *envDeployerMocks, _ *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) {
					return nil, &addon.ErrAddonsNotFound{}
				}
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.stackSerializer.EXPECT().Template().Return("", mockError)
			},
			wantedError: errors.New("generate stack template: some error"),
		},
		"fail to generate stack parameters": {
			setUpMocks: func(m *envDeployerMocks, _ *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) {
					return nil, &addon.ErrAddonsNotFound{}
				}
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.stackSerializer.EXPECT().Template().Return("", nil)
				m.stackSerializer.EXPECT().SerializedParameters().Return("", mockError)
			},
			wantedError: errors.New("generate stack template parameters: some error"),
		},
		"return an error when addons cannot be parsed due to unknown reasons": {
			setUpMocks: func(m *envDeployerMocks, _ *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) {
					return nil, mockError
				}
			},
			wantedError: errors.New("some error"),
		},
		"return an error if the URL isn't provided and addons template cannot be retrieved": {
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) {
					mockStack := mocks.NewMockstackBuilder(ctrl)
					mockStack.EXPECT().Template().Return("", mockError)
					return mockStack, nil
				}
			},
			wantedError: errors.New("render addons template: some error"),
		},
		"successfully return environment template without addons": {
			setUpMocks: func(m *envDeployerMocks, _ *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) {
					return nil, &addon.ErrAddonsNotFound{}
				}
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(mockAppName, mockEnvName).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.stackSerializer.EXPECT().Template().Return("aloo", nil)
				m.stackSerializer.EXPECT().SerializedParameters().Return("gobi", nil)
			},

			wantedTemplate: "aloo",
			wantedParams:   "gobi",
		},
		"successfully return environment template with addons": {
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) {
					mockStack := mocks.NewMockstackBuilder(ctrl)
					mockStack.EXPECT().Template().Return("template", nil)
					return mockStack, nil
				}
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(mockAppName, mockEnvName).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.stackSerializer.EXPECT().Template().Return("aloo", nil)
				m.stackSerializer.EXPECT().SerializedParameters().Return("gobi", nil)
			},

			wantedTemplate: "aloo",
			wantedParams:   "gobi",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &envDeployerMocks{
				appCFN:          mocks.NewMockappResourcesGetter(ctrl),
				envDeployer:     mocks.NewMockenvironmentDeployer(ctrl),
				stackSerializer: cfnmocks.NewMockStackConfiguration(ctrl),
			}
			tc.setUpMocks(m, ctrl)
			d := envDeployer{
				app: mockApp,
				env: &config.Environment{
					Name:   mockEnvName,
					Region: mockEnvRegion,
				},
				appCFN:      m.appCFN,
				envDeployer: m.envDeployer,
				newStack: func(_ *cfnstack.EnvConfig, _ string, _ []*awscfn.Parameter) (cloudformation.StackConfiguration, error) {
					return m.stackSerializer, nil
				},
				parseAddons: m.parseAddons,
			}
			actual, err := d.GenerateCloudFormationTemplate(&DeployEnvironmentInput{
				Manifest: &tc.inManifest,
			})
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTemplate, actual.Template)
				require.Equal(t, tc.wantedParams, actual.Parameters)
			}
		})
	}
}

func TestEnvDeployer_DeployEnvironment(t *testing.T) {
	const (
		mockManagerRoleARN = "mockManagerRoleARN"
		mockEnvRegion      = "us-west-2"
		mockAppName        = "mockApp"
		mockEnvName        = "mockEnv"
	)
	mockApp := &config.Application{
		Name: mockAppName,
	}
	testCases := map[string]struct {
		setUpMocks        func(m *envDeployerMocks)
		inManifest        *manifest.Environment
		inDisableRollback bool
		wantedError       error
	}{
		"fail to get app resources by region": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).
					Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get app resources in region %s: some error", mockEnvRegion),
		},
		"fail to get prefix list id": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return("", errors.New("some error"))
			},
			inManifest: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
					HTTPConfig: manifest.EnvironmentHTTPConfig{
						Public: manifest.PublicHTTPConfig{
							DeprecatedSG: manifest.DeprecatedALBSecurityGroupsConfig{
								DeprecatedIngress: manifest.DeprecatedIngress{
									RestrictiveIngress: manifest.RestrictiveIngress{
										CDNIngress: aws.Bool(true),
									},
								},
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf("retrieve CloudFront managed prefix list id: some error"),
		},
		"prefix list not retrieved when manifest not present": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return("mockPrefixListID", nil).Times(0)
				m.parseAddons = func() (stackBuilder, error) { return nil, &addon.ErrAddonsNotFound{} }
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			inManifest: nil,
		},
		"fail to get existing parameters": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) { return nil, &addon.ErrAddonsNotFound{} }
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("describe environment stack parameters: some error"),
		},
		"fail to get existing force update ID": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.parseAddons = func() (stackBuilder, error) { return nil, &addon.ErrAddonsNotFound{} }
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedError: errors.New("retrieve environment stack force update ID: some error"),
		},
		"fail to deploy environment": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return("mockPrefixListID", nil).Times(0)
				m.parseAddons = func() (stackBuilder, error) { return nil, &addon.ErrAddonsNotFound{} }
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"successful environment deployment": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return("mockPrefixListID", nil).Times(0)
				m.parseAddons = func() (stackBuilder, error) { return nil, &addon.ErrAddonsNotFound{} }
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		"successful environment deployment, no rollback": {
			inDisableRollback: true,
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return("mockPrefixListID", nil).Times(0)
				m.parseAddons = func() (stackBuilder, error) { return nil, &addon.ErrAddonsNotFound{} }
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Len(2)).Return(nil)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &envDeployerMocks{
				appCFN:           mocks.NewMockappResourcesGetter(ctrl),
				envDeployer:      mocks.NewMockenvironmentDeployer(ctrl),
				prefixListGetter: mocks.NewMockprefixListGetter(ctrl),
				stackSerializer:  cfnmocks.NewMockStackConfiguration(ctrl),
			}
			tc.setUpMocks(m)
			d := envDeployer{
				app: mockApp,
				env: &config.Environment{
					Name:           mockEnvName,
					ManagerRoleARN: mockManagerRoleARN,
					Region:         mockEnvRegion,
				},
				appCFN:           m.appCFN,
				envDeployer:      m.envDeployer,
				prefixListGetter: m.prefixListGetter,
				parseAddons:      m.parseAddons,
				newStack: func(_ *cfnstack.EnvConfig, _ string, _ []*awscfn.Parameter) (cloudformation.StackConfiguration, error) {
					return m.stackSerializer, nil
				},
			}
			mockIn := &DeployEnvironmentInput{
				RootUserARN: "mockRootUserARN",
				CustomResourcesURLs: map[string]string{
					"mockResource": "mockURL",
				},
				Manifest:        tc.inManifest,
				DisableRollback: tc.inDisableRollback,
			}
			gotErr := d.DeployEnvironment(mockIn)
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEnvDeployer_Validate(t *testing.T) {
	listenerRuleNoRedirect := elbv2.Rule{
		Actions: []*awselb.Action{
			{
				Type: aws.String(awselb.ActionTypeEnumForward),
			},
		},
	}
	listenerRuleWithRedirect := elbv2.Rule{
		Actions: []*awselb.Action{
			{
				Type: aws.String(awselb.ActionTypeEnumRedirect),
			},
		},
	}
	mftCDNTerminateTLSAndHTTPCert := &manifest.Environment{
		EnvironmentConfig: manifest.EnvironmentConfig{
			HTTPConfig: manifest.EnvironmentHTTPConfig{
				Public: manifest.PublicHTTPConfig{
					Certificates: []string{"mockCertARN"},
				},
			},
			CDNConfig: manifest.EnvironmentCDNConfig{
				Config: manifest.AdvancedCDNConfig{
					TerminateTLS: aws.Bool(true),
				},
			},
		},
	}
	mftCDNTerminateTLS := &manifest.Environment{
		EnvironmentConfig: manifest.EnvironmentConfig{
			CDNConfig: manifest.EnvironmentCDNConfig{
				Config: manifest.AdvancedCDNConfig{
					Certificate:  aws.String("mockCDNCertARN"),
					TerminateTLS: aws.Bool(true),
				},
			},
		},
	}
	tests := map[string]struct {
		app            *config.Application
		mft            *manifest.Environment
		setUpMocks     func(*envDeployerMocks, *gomock.Controller)
		expected       string
		expectedStdErr string
	}{
		"cdn enabled, domain imported, no public http certs, and validate aliases fails": {
			app: &config.Application{
				Domain: "example.com",
			},
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
					CDNConfig: manifest.EnvironmentCDNConfig{
						Enabled: aws.Bool(true),
					},
				},
			},
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.envDescriber.EXPECT().ValidateCFServiceDomainAliases().Return(errors.New("some error"))
			},
			expected: "some error",
		},
		"cdn enabled, domain imported, no public http certs, and validate aliases succeeds": {
			app: &config.Application{
				Domain: "example.com",
			},
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
					CDNConfig: manifest.EnvironmentCDNConfig{
						Enabled: aws.Bool(true),
					},
				},
			},
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.envDescriber.EXPECT().ValidateCFServiceDomainAliases().Return(nil)
			},
		},
		"cdn tls termination enabled, fail to get env stack params": {
			app: &config.Application{},
			mft: mftCDNTerminateTLSAndHTTPCert,
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.envDescriber.EXPECT().Params().Return(nil, errors.New("some error"))
			},
			expected: "enable TLS termination on CDN: get env params: some error",
		},
		"cdn tls termination enabled, skip if no services deployed": {
			app: &config.Application{},
			mft: mftCDNTerminateTLSAndHTTPCert,
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.stackDescribers = map[string]*mocks.MockstackDescriber{
					"svc1": mocks.NewMockstackDescriber(ctrl),
				}

				m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil)
			},
		},
		"cdn tls termination enabled, fail to get service resources": {
			app: &config.Application{},
			mft: mftCDNTerminateTLSAndHTTPCert,
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.stackDescribers = map[string]*mocks.MockstackDescriber{
					"svc1": mocks.NewMockstackDescriber(ctrl),
				}

				m.envDescriber.EXPECT().Params().Return(map[string]string{
					"ALBWorkloads": "svc1",
				}, nil)
				m.stackDescribers["svc1"].EXPECT().Resources().Return(nil, errors.New("some error"))
			},
			expected: `enable TLS termination on CDN: verify service "svc1": get stack resources: some error`,
		},
		"cdn tls termination enabled, fail to check listener rule": {
			app: &config.Application{},
			mft: mftCDNTerminateTLSAndHTTPCert,
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.stackDescribers = map[string]*mocks.MockstackDescriber{
					"svc1": mocks.NewMockstackDescriber(ctrl),
				}

				m.envDescriber.EXPECT().Params().Return(map[string]string{
					"ALBWorkloads": "svc1",
				}, nil)
				m.stackDescribers["svc1"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc1RuleARN",
					},
				}, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc1RuleARN").Return(elbv2.Rule{}, errors.New("some error"))
			},
			expected: `enable TLS termination on CDN: verify service "svc1": describe listener rule "svc1RuleARN": some error`,
		},
		"cdn tls termination enabled, warn with one service that doesn't redirect, two that do redirect": {
			app: &config.Application{},
			mft: mftCDNTerminateTLSAndHTTPCert,
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.stackDescribers = map[string]*mocks.MockstackDescriber{
					"svc1": mocks.NewMockstackDescriber(ctrl),
					"svc2": mocks.NewMockstackDescriber(ctrl),
					"svc3": mocks.NewMockstackDescriber(ctrl),
				}

				m.envDescriber.EXPECT().Params().Return(map[string]string{
					"ALBWorkloads": "svc1,svc2,svc3",
				}, nil)
				m.stackDescribers["svc1"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc1RuleARN",
					},
				}, nil)
				m.stackDescribers["svc2"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc2RuleARN",
					},
				}, nil)
				m.stackDescribers["svc3"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc3RuleARN",
					},
				}, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc1RuleARN").Return(listenerRuleWithRedirect, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc2RuleARN").Return(listenerRuleNoRedirect, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc3RuleARN").Return(listenerRuleWithRedirect, nil)
			},
			expectedStdErr: fmt.Sprintf(`Note: Services "svc1" and "svc3" redirect HTTP traffic to HTTPS.
These services will not be reachable through the CDN.
To fix this, set the following field in each manifest:
%s
http:
  redirect_to_https: false
%s
and run %scopilot svc deploy%s.
If you'd like to use these services without a CDN, ensure each service's A record is pointed to the ALB.
`,
				"```", "```", "`", "`"), // ugh
		},
		"cdn tls termination enabled, warn with one service that doesn't redirect, two that do redirect, alb ingress restricted to cdn": {
			app: &config.Application{},
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
					HTTPConfig: manifest.EnvironmentHTTPConfig{
						Public: manifest.PublicHTTPConfig{
							Certificates: []string{"mockCertARN"},
							DeprecatedSG: manifest.DeprecatedALBSecurityGroupsConfig{
								DeprecatedIngress: manifest.DeprecatedIngress{
									RestrictiveIngress: manifest.RestrictiveIngress{
										CDNIngress: aws.Bool(true),
									},
								},
							},
						},
					},
					CDNConfig: manifest.EnvironmentCDNConfig{
						Config: manifest.AdvancedCDNConfig{
							TerminateTLS: aws.Bool(true),
						},
					},
				},
			},
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.stackDescribers = map[string]*mocks.MockstackDescriber{
					"svc1": mocks.NewMockstackDescriber(ctrl),
					"svc2": mocks.NewMockstackDescriber(ctrl),
					"svc3": mocks.NewMockstackDescriber(ctrl),
				}

				m.envDescriber.EXPECT().Params().Return(map[string]string{
					"ALBWorkloads": "svc1,svc2,svc3",
				}, nil)
				m.stackDescribers["svc1"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc1RuleARN",
					},
				}, nil)
				m.stackDescribers["svc2"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc2RuleARN",
					},
				}, nil)
				m.stackDescribers["svc3"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc3RuleARN",
					},
				}, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc1RuleARN").Return(listenerRuleWithRedirect, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc2RuleARN").Return(listenerRuleNoRedirect, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc3RuleARN").Return(listenerRuleWithRedirect, nil)
			},
			expected: "2 services redirect HTTP to HTTPS",
		},
		"cdn tls termination enabled, success with three services that don't redirect": {
			app: &config.Application{},
			mft: mftCDNTerminateTLSAndHTTPCert,
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.stackDescribers = map[string]*mocks.MockstackDescriber{
					"svc1": mocks.NewMockstackDescriber(ctrl),
					"svc2": mocks.NewMockstackDescriber(ctrl),
					"svc3": mocks.NewMockstackDescriber(ctrl),
				}

				m.envDescriber.EXPECT().Params().Return(map[string]string{
					"ALBWorkloads": "svc1,svc2,svc3",
				}, nil)
				m.stackDescribers["svc1"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc1RuleARN",
					},
				}, nil)
				m.stackDescribers["svc2"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc2RuleARN",
					},
				}, nil)
				m.stackDescribers["svc3"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc3RuleARN",
					},
				}, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc1RuleARN").Return(listenerRuleNoRedirect, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc2RuleARN").Return(listenerRuleNoRedirect, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc3RuleARN").Return(listenerRuleNoRedirect, nil)
			},
		},
		"cdn tls termination enabled, one http only service deployed": {
			app: &config.Application{},
			mft: mftCDNTerminateTLS,
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.stackDescribers = map[string]*mocks.MockstackDescriber{
					"svc1": mocks.NewMockstackDescriber(ctrl),
				}
				m.envDescriber.EXPECT().Params().Return(map[string]string{
					"ALBWorkloads": "svc1",
				}, nil)
				m.stackDescribers["svc1"].EXPECT().Resources().Return([]*stack.Resource{
					{
						LogicalID:  "HTTPListenerRuleWithDomain",
						PhysicalID: "svc1RuleARN",
					},
				}, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc1RuleARN").Return(listenerRuleNoRedirect, nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &envDeployerMocks{
				envDescriber: mocks.NewMockenvDescriber(ctrl),
				lbDescriber:  mocks.NewMocklbDescriber(ctrl),
			}
			if tc.setUpMocks != nil {
				tc.setUpMocks(m, ctrl)
			}

			d := &envDeployer{
				app: tc.app,
				env: &config.Environment{
					Name: aws.StringValue(tc.mft.Name),
				},
				envDescriber: m.envDescriber,
				lbDescriber:  m.lbDescriber,
				newServiceStackDescriber: func(svc string) stackDescriber {
					return m.stackDescribers[svc]
				},
			}

			buf := &bytes.Buffer{}
			log.DiagnosticWriter = buf

			err := d.Validate(tc.mft)
			if tc.expected != "" {
				require.EqualError(t, err, tc.expected)
			} else {
				require.NoError(t, err)
			}

			if tc.expectedStdErr != "" {
				require.Equal(t, tc.expectedStdErr, buf.String())
			}
		})
	}
}
