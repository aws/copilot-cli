// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awscfn "github.com/aws/aws-sdk-go/service/cloudformation"
	awselb "github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type envDeployerMocks struct {
	s3               *mocks.Mockuploader
	prefixListGetter *mocks.MockprefixListGetter
	appCFN           *mocks.MockappResourcesGetter
	envDeployer      *mocks.MockenvironmentDeployer
	patcher          *mocks.Mockpatcher
	stackSerializer  *mocks.MockstackSerializer
	envDescriber     *mocks.MockenvDescriber
	lbDescriber      *mocks.MocklbDescriber
	stackDescribers  map[string]*mocks.MockstackDescriber
}

func TestEnvDeployer_UploadArtifacts(t *testing.T) {
	const (
		mockEnvRegion = "mockEnvRegion"
	)
	mockApp := &config.Application{}
	testCases := map[string]struct {
		setUpMocks  func(m *envDeployerMocks)
		wantedOut   map[string]string
		wantedError error
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
		"fail to upload artifacts": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.patcher.EXPECT().EnsureManagerRoleIsAllowedToUpload("mockS3Bucket").Return(nil)
				m.s3.EXPECT().Upload("mockS3Bucket", gomock.Any(), gomock.Any()).AnyTimes().Return("", fmt.Errorf("some error"))
			},
			wantedError: errors.New("upload custom resources to bucket mockS3Bucket"),
		},
		"success with URL returned": {
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
			},
			wantedOut: map[string]string{
				"CertificateReplicatorFunction": "",
				"CertificateValidationFunction": "",
				"CustomDomainFunction":          "",
				"DNSDelegationFunction":         "",
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
			}
			tc.setUpMocks(m)

			mockEnv := &config.Environment{
				Name:           "mockEnv",
				ManagerRoleARN: "mockManagerRoleARN",
				Region:         mockEnvRegion,
				App:            "mockApp",
			}
			d := envDeployer{
				app:        mockApp,
				env:        mockEnv,
				appCFN:     m.appCFN,
				s3:         m.s3,
				patcher:    m.patcher,
				templateFS: fakeTemplateFS(),
			}

			got, gotErr := d.UploadArtifacts()
			if tc.wantedError != nil {
				require.Contains(t, gotErr.Error(), tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantedOut, got)
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
	mockApp := &config.Application{
		Name: mockAppName,
	}
	testCases := map[string]struct {
		setUpMocks func(m *envDeployerMocks)

		wantedTemplate string
		wantedParams   string
		wantedError    error
	}{
		"fail to get app resources by region": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get app resources in region us-west-2: some error"),
		},
		"fail to get existing parameters": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("describe environment stack parameters: some error"),
		},
		"fail to get existing force update ID": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedError: errors.New("retrieve environment stack force update ID: some error"),
		},
		"fail to generate stack template": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.stackSerializer.EXPECT().Template().Return("", errors.New("some error"))
			},
			wantedError: errors.New("generate stack template: some error"),
		},
		"fail to generate stack parameters": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.stackSerializer.EXPECT().Template().Return("", nil)
				m.stackSerializer.EXPECT().SerializedParameters().Return("", errors.New("some error"))
			},
			wantedError: errors.New("generate stack template parameters: some error"),
		},
		"successfully return templates environment deployment": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
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
				stackSerializer: mocks.NewMockstackSerializer(ctrl),
			}
			tc.setUpMocks(m)
			d := envDeployer{
				app: mockApp,
				env: &config.Environment{
					Name:   mockEnvName,
					Region: mockEnvRegion,
				},
				appCFN:      m.appCFN,
				envDeployer: m.envDeployer,
				newStackSerializer: func(_ *deploy.CreateEnvironmentInput, _ string, _ []*awscfn.Parameter) stackSerializer {
					return m.stackSerializer
				},
			}
			actual, err := d.GenerateCloudFormationTemplate(&DeployEnvironmentInput{})
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
		setUpMocks  func(m *envDeployerMocks)
		inManifest  *manifest.Environment
		wantedError error
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
							SecurityGroupConfig: manifest.ALBSecurityGroupsConfig{
								Ingress: manifest.Ingress{
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
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			inManifest: nil,
		},
		"fail to get existing parameters": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("describe environment stack parameters: some error"),
		},
		"fail to get existing force update ID": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
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
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"successful environment deployment": {
			setUpMocks: func(m *envDeployerMocks) {
				m.appCFN.EXPECT().GetAppResourcesByRegion(mockApp, mockEnvRegion).Return(&cfnstack.AppRegionalResources{
					S3Bucket: "mockS3Bucket",
				}, nil)
				m.prefixListGetter.EXPECT().CloudFrontManagedPrefixListID().Return("mockPrefixListID", nil).Times(0)
				m.envDeployer.EXPECT().DeployedEnvironmentParameters(gomock.Any(), gomock.Any()).Return(nil, nil)
				m.envDeployer.EXPECT().ForceUpdateOutputID(gomock.Any(), gomock.Any()).Return("", nil)
				m.envDeployer.EXPECT().UpdateAndRenderEnvironment(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
			}
			mockIn := &DeployEnvironmentInput{
				RootUserARN: "mockRootUserARN",
				CustomResourcesURLs: map[string]string{
					"mockResource": "mockURL",
				},
				Manifest: tc.inManifest,
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
	tests := map[string]struct {
		app        *config.Application
		mft        *manifest.Environment
		setUpMocks func(*envDeployerMocks, *gomock.Controller)
		expected   string
	}{
		"cdn enabled, domain imported, no public http certs and validate aliases fails": {
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
		"cdn enabled, domain imported, no public http certs and validate aliases succeeds": {
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
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
					CDNConfig: manifest.EnvironmentCDNConfig{
						Config: manifest.AdvancedCDNConfig{
							TerminateTLS: aws.Bool(true),
						},
					},
				},
			},
			setUpMocks: func(m *envDeployerMocks, ctrl *gomock.Controller) {
				m.envDescriber.EXPECT().Params().Return(nil, errors.New("some error"))
			},
			expected: "can't enable TLS termination on CDN: get env params: some error",
		},
		"cdn tls termination enabled, fail to get service resources": {
			app: &config.Application{},
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
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
				}

				m.envDescriber.EXPECT().Params().Return(map[string]string{
					"ALBWorkloads": "svc1",
				}, nil)
				m.stackDescribers["svc1"].EXPECT().Resources().Return(nil, errors.New("some error"))
			},
			expected: `can't enable TLS termination on CDN: verify service "svc1": get stack resources: some error`,
		},
		"cdn tls termination enabled, fail to check listener rule": {
			app: &config.Application{},
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
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
			expected: `can't enable TLS termination on CDN: verify service "svc1": get listener rule "svc1RuleARN": some error`,
		},
		"cdn tls termination enabled, fail with one service that redirects": {
			app: &config.Application{},
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
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
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc1RuleARN").Return(listenerRuleWithRedirect, nil)
			},
			expected: "can't enable TLS termination on CDN: HTTP traffic redirects to HTTPS in service svc1.\nSet http.redirect_to_https to false for that service and redeploy it.",
		},
		"cdn tls termination enabled, fail with one service that doesn't redirects, two that do redirect": {
			app: &config.Application{},
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
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
			expected: "can't enable TLS termination on CDN: HTTP traffic redirects to HTTPS in services svc1 and svc3.\nSet http.redirect_to_https to false for those services and redeploy them.",
		},
		"cdn tls termination enabled, success with three services that don't redirect": {
			app: &config.Application{},
			mft: &manifest.Environment{
				EnvironmentConfig: manifest.EnvironmentConfig{
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
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc1RuleARN").Return(listenerRuleNoRedirect, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc2RuleARN").Return(listenerRuleNoRedirect, nil)
				m.lbDescriber.EXPECT().DescribeRule(gomock.Any(), "svc3RuleARN").Return(listenerRuleNoRedirect, nil)
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

			err := d.Validate(context.Background(), tc.mft)
			if tc.expected != "" {
				require.EqualError(t, err, tc.expected)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
