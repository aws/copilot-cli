// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template/templatetest"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	testEnvName      = "test"
	testAppName      = "phonetool"
	testImageRepoURL = "111111111111.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend"
	testImageTag     = "manual-bf3678c"
)

type mockAddons struct {
	tpl    string
	tplErr error

	params    string
	paramsErr error
}

func (m mockAddons) Template() (string, error) {
	if m.tplErr != nil {
		return "", m.tplErr
	}
	return m.tpl, nil
}

func (m mockAddons) Parameters() (string, error) {
	if m.paramsErr != nil {
		return "", m.paramsErr
	}
	return m.params, nil
}

var mockCloudFormationOverrideFunc = func(overrideRules []override.Rule, origTemp []byte) ([]byte, error) {
	return origTemp, nil
}

func TestLoadBalancedWebService_StackName(t *testing.T) {
	testCases := map[string]struct {
		inSvcName string
		inEnvName string
		inAppName string

		wantedStackName string
	}{
		"valid stack name": {
			inSvcName: "frontend",
			inEnvName: "test",
			inAppName: "phonetool",

			wantedStackName: "phonetool-test-frontend",
		},
		"longer than 128 characters": {
			inSvcName: "whatisthishorriblylongservicenamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			inEnvName: "test",
			inAppName: "phonetool",

			wantedStackName: "phonetool-test-whatisthishorriblylongservicenamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			conf := &LoadBalancedWebService{
				ecsWkld: &ecsWkld{
					wkld: &wkld{
						name: tc.inSvcName,
						env:  tc.inEnvName,
						app:  tc.inAppName,
					},
				},
			}

			// WHEN
			n := conf.StackName()

			// THEN
			require.Equal(t, tc.wantedStackName, n, "expected stack names to be equal")
		})
	}
}

func TestLoadBalancedWebService_Template(t *testing.T) {
	t.Cleanup(func() {
		fs = realEmbedFS
	})
	fs = templatetest.Stub{}

	t.Run("returns a wrapped error when addons template parsing fails", func(t *testing.T) {
		// GIVEN
		lbws, err := NewLoadBalancedWebService(LoadBalancedWebServiceConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			ArtifactBucketName: "mockBucket",
			Manifest: &manifest.LoadBalancedWebService{
				Workload: manifest.Workload{
					Name: aws.String("frontend"),
				},
			},
			Addons: mockAddons{tplErr: errors.New("some error")},
		})
		require.NoError(t, err)

		// WHEN
		_, err = lbws.Template()

		// THEN
		require.EqualError(t, err, "generate addons template for frontend: some error")
	})

	t.Run("returns a wrapped error when addons parameter parsing fails", func(t *testing.T) {
		// GIVEN
		lbws, err := NewLoadBalancedWebService(LoadBalancedWebServiceConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			ArtifactBucketName: "mockBucket",
			Manifest: &manifest.LoadBalancedWebService{
				Workload: manifest.Workload{
					Name: aws.String("frontend"),
				},
			},
			Addons: mockAddons{paramsErr: errors.New("some error")},
		})
		require.NoError(t, err)

		// WHEN
		_, err = lbws.Template()

		// THEN
		require.EqualError(t, err, "parse addons parameters for frontend: some error")
	})

	t.Run("returns the error when parsing the service template fails", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		parser := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
		parser.EXPECT().ParseLoadBalancedWebService(gomock.Any()).Return(nil, errors.New("some error"))
		addons := mockAddons{
			tpl: `
Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
		}
		lbws, err := NewLoadBalancedWebService(LoadBalancedWebServiceConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			ArtifactBucketName: "mockBucket",
			Manifest:           &manifest.LoadBalancedWebService{},
			Addons:             addons,
		}, func(s *LoadBalancedWebService) {
			s.parser = parser
		})
		require.NoError(t, err)

		// WHEN
		_, err = lbws.Template()

		// THEN
		require.EqualError(t, err, "some error")
	})

	t.Run("renders all manifest fields into template without any addons", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mft := manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
			WorkloadProps: &manifest.WorkloadProps{
				Name:       "frontend",
				Dockerfile: "frontend/Dockerfile",
			},
			Path: "frontend",
			Port: 80,
		})
		mft.ImageConfig.HealthCheck = manifest.ContainerHealthCheck{
			Retries: aws.Int(5),
		}
		mft.HTTPOrBool.Main.Alias = manifest.Alias{AdvancedAliases: []manifest.AdvancedAlias{
			{
				Alias:      aws.String("mockAlias"),
				HostedZone: aws.String("mockHostedZone"),
			},
		}}
		mft.EntryPoint = manifest.EntryPointOverride{
			String:      nil,
			StringSlice: []string{"/bin/echo", "hello"},
		}
		mft.Command = manifest.CommandOverride{
			String:      nil,
			StringSlice: []string{"world"},
		}
		mft.Network.Connect = manifest.ServiceConnectBoolOrArgs{
			ServiceConnectArgs: manifest.ServiceConnectArgs{
				Alias: aws.String("frontend"),
			},
		}

		var actual template.WorkloadOpts
		parser := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
		parser.EXPECT().ParseLoadBalancedWebService(gomock.Any()).DoAndReturn(func(in template.WorkloadOpts) (*template.Content, error) {
			actual = in // Capture the translated object.
			return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
		})

		lbws, err := NewLoadBalancedWebService(LoadBalancedWebServiceConfig{
			App: &config.Application{
				Name:   "phonetool",
				Domain: "phonetool.com",
			},
			EnvManifest: &manifest.Environment{
				Workload: manifest.Workload{
					Name: aws.String("test"),
				},
			},
			Manifest: mft,
			RuntimeConfig: RuntimeConfig{
				PushedImages: map[string]ECRImage{
					"test": {
						RepoURL:  testImageRepoURL,
						ImageTag: testImageTag,
					},
				},
				AccountID: "0123456789012",
				Region:    "us-west-2",
			},
			Addons:             mockAddons{},
			ArtifactBucketName: "bucket",
		}, func(s *LoadBalancedWebService) {
			s.parser = parser
		})
		require.NoError(t, err)

		// WHEN
		_, err = lbws.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, template.WorkloadOpts{
			AppName:      "phonetool",
			EnvName:      "test",
			WorkloadName: "frontend",
			WorkloadType: manifestinfo.LoadBalancedWebServiceType,
			ServiceConnectOpts: template.ServiceConnectOpts{
				Server: &template.ServiceConnectServer{
					Name:  "frontend",
					Port:  "80",
					Alias: "frontend",
				},
				Client: true,
			},
			HealthCheck: &template.ContainerHealthCheck{
				Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
				Interval:    aws.Int64(10),
				StartPeriod: aws.Int64(0),
				Timeout:     aws.Int64(5),
				Retries:     aws.Int64(5),
			},
			CustomResources: map[string]template.S3ObjectLocation{
				"DynamicDesiredCountFunction": {
					Bucket: "bucket",
					Key:    "manual/scripts/custom-resources/dynamicdesiredcountfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"EnvControllerFunction": {
					Bucket: "bucket",
					Key:    "manual/scripts/custom-resources/envcontrollerfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"NLBCertValidatorFunction": {
					Bucket: "bucket",
					Key:    "manual/scripts/custom-resources/nlbcertvalidatorfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"NLBCustomDomainFunction": {
					Bucket: "bucket",
					Key:    "manual/scripts/custom-resources/nlbcustomdomainfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"RulePriorityFunction": {
					Bucket: "bucket",
					Key:    "manual/scripts/custom-resources/rulepriorityfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
			},
			Network: template.NetworkOpts{
				AssignPublicIP: template.EnablePublicIP,
				SubnetsType:    template.PublicSubnetsPlacement,
				SecurityGroups: []template.SecurityGroup{},
			},
			DeploymentConfiguration: template.DeploymentConfigurationOpts{
				MinHealthyPercent: 100,
				MaxPercent:        200,
			},
			EntryPoint: []string{"/bin/echo", "hello"},
			Command:    []string{"world"},
			ALBListener: &template.ALBListener{
				Rules: []template.ALBListenerRule{
					{
						Path:            "/frontend",
						TargetContainer: "frontend",
						TargetPort:      "80",
						Aliases: []string{
							"mockAlias",
						},
						HTTPHealthCheck: template.HTTPHealthCheckOpts{
							HealthCheckPath: "/",
							GracePeriod:     60,
							Port:            "",
							SuccessCodes:    "",
						},
						Stickiness:          "false",
						RedirectToHTTPS:     true,
						DeregistrationDelay: aws.Int64(int64(60)),
					},
				},
				IsHTTPS: true,
				HostedZoneAliases: template.AliasesForHostedZone{
					"mockHostedZone": {
						"mockAlias",
					},
				},
			},
			GracePeriod: aws.Int64(int64(60)),
			ALBEnabled:  true,
			PortMappings: []*template.PortMapping{
				{
					Protocol:      "tcp",
					ContainerPort: 80,
					ContainerName: "frontend",
				},
			},
		}, actual)
	})

	t.Run("renders a template with addons", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mft := manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
			WorkloadProps: &manifest.WorkloadProps{
				Name:       "frontend",
				Dockerfile: "frontend/Dockerfile",
			},
			Path: "frontend",
			Port: 80,
		})

		var actual template.WorkloadOpts
		parser := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
		parser.EXPECT().ParseLoadBalancedWebService(gomock.Any()).DoAndReturn(func(in template.WorkloadOpts) (*template.Content, error) {
			actual = in // Capture the translated object.
			return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
		})
		addons := mockAddons{
			tpl: `Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Statement:
        - Effect: Allow
          Action: '*'
          Resource: '*'
  MySecret:
    Type: AWS::SecretsManager::Secret
    Properties:
      Description: 'This is my rds instance secret'
      GenerateSecretString:
        SecretStringTemplate: '{"username": "admin"}'
        GenerateStringKey: 'password'
        PasswordLength: 16
        ExcludeCharacters: '"@/\'
Outputs:
  AdditionalResourcesPolicyArn:
    Value: !Ref AdditionalResourcesPolicy
  MySecretArn:
    Value: !Ref MySecret
  Hello:
    Value: hello`,
			params: "ServiceName: !GetAtt Service.Name",
		}

		lbws, err := NewLoadBalancedWebService(LoadBalancedWebServiceConfig{
			App: &config.Application{
				Name: "phonetool",
			},
			EnvManifest: &manifest.Environment{
				Workload: manifest.Workload{
					Name: aws.String("test"),
				},
			},
			ArtifactBucketName: "mockBucket",
			Manifest:           mft,
			Addons:             addons,
		}, func(s *LoadBalancedWebService) {
			s.parser = parser
		})
		require.NoError(t, err)

		// WHEN
		_, err = lbws.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, "ServiceName: !GetAtt Service.Name", actual.AddonsExtraParams)
		require.Equal(t, &template.WorkloadNestedStackOpts{
			StackName:       addon.StackName,
			VariableOutputs: []string{"Hello"},
			SecretOutputs:   []string{"MySecretArn"},
			PolicyOutputs:   []string{"AdditionalResourcesPolicyArn"},
		}, actual.NestedStack)
	})

	t.Run("should set the target group container to sidecar", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mft := manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
			WorkloadProps: &manifest.WorkloadProps{
				Name:       "frontend",
				Dockerfile: "frontend/Dockerfile",
			},
			Path: "frontend",
			Port: 80,
		})
		mft.Network.Connect.EnableServiceConnect = aws.Bool(true)
		mft.HTTPOrBool.Main.TargetContainer = aws.String("envoy")
		mft.Sidecars = map[string]*manifest.SidecarConfig{
			"envoy": {
				Port: aws.String("443"),
			},
		}

		var actual template.WorkloadOpts
		parser := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
		parser.EXPECT().ParseLoadBalancedWebService(gomock.Any()).DoAndReturn(func(in template.WorkloadOpts) (*template.Content, error) {
			actual = in // Capture the translated object.
			return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
		})

		lbws, err := NewLoadBalancedWebService(LoadBalancedWebServiceConfig{
			App: &config.Application{
				Name: "phonetool",
			},
			EnvManifest: &manifest.Environment{
				Workload: manifest.Workload{
					Name: aws.String("test"),
				},
			},
			ArtifactBucketName: "mockBucket",
			Manifest:           mft,
			Addons:             mockAddons{},
		}, func(s *LoadBalancedWebService) {
			s.parser = parser
		})
		require.NoError(t, err)

		// WHEN
		_, err = lbws.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, template.ServiceConnectOpts{
			Server: &template.ServiceConnectServer{
				Name: "envoy",
				Port: "443",
			},
			Client: true,
		}, actual.ServiceConnectOpts)
	})
}

func TestLoadBalancedWebService_Parameters(t *testing.T) {
	baseProps := &manifest.LoadBalancedWebServiceProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       "frontend",
			Dockerfile: "frontend/Dockerfile",
		},
		Path: "frontend",
		Port: 80,
	}
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadAppNameParamKey),
			ParameterValue: aws.String("phonetool"),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvNameParamKey),
			ParameterValue: aws.String("test"),
		},
		{
			ParameterKey:   aws.String(WorkloadNameParamKey),
			ParameterValue: aws.String("frontend"),
		},
		{
			ParameterKey:   aws.String(WorkloadContainerImageParamKey),
			ParameterValue: aws.String("111111111111.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c"),
		},
		{
			ParameterKey:   aws.String(WorkloadContainerPortParamKey),
			ParameterValue: aws.String("80"),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskCPUParamKey),
			ParameterValue: aws.String("256"),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskMemoryParamKey),
			ParameterValue: aws.String("512"),
		},
		{
			ParameterKey:   aws.String(WorkloadLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(WorkloadAddonsTemplateURLParamKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(WorkloadArtifactKeyARNParamKey),
			ParameterValue: aws.String(""),
		},
	}
	testCases := map[string]struct {
		httpsEnabled         bool
		dnsDelegationEnabled bool
		setupManifest        func(*manifest.LoadBalancedWebService)

		expectedParams []*cloudformation.Parameter
		expectedErr    error
	}{
		"HTTPS Enabled": {
			httpsEnabled: true,
			setupManifest: func(service *manifest.LoadBalancedWebService) {
				countRange := manifest.IntRangeBand("2-100")
				service.Count = manifest.Count{
					Value: aws.Int(1),
					AdvancedCount: manifest.AdvancedCount{
						Range: manifest.Range{
							Value: &countRange,
						},
					},
				}
			},
			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadHTTPSParamKey),
					ParameterValue: aws.String("true"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(WorkloadTaskCountParamKey),
					ParameterValue: aws.String("2"),
				},
				{
					ParameterKey:   aws.String(WorkloadRulePathParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
					ParameterValue: aws.String("false"),
				},
			}...),
		},
		"HTTPS Not Enabled": {
			httpsEnabled: false,
			setupManifest: func(service *manifest.LoadBalancedWebService) {
				countRange := manifest.IntRangeBand("2-100")
				service.Count = manifest.Count{
					Value: aws.Int(1),
					AdvancedCount: manifest.AdvancedCount{
						Range: manifest.Range{
							Value: &countRange,
						},
					},
				}
			},
			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadRulePathParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadHTTPSParamKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(WorkloadTaskCountParamKey),
					ParameterValue: aws.String("2"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
					ParameterValue: aws.String("false"),
				},
			}...),
		},
		"with sidecar container": {
			httpsEnabled: true,
			setupManifest: func(service *manifest.LoadBalancedWebService) {
				service.HTTPOrBool.Main.TargetContainer = aws.String("xray")
				service.Sidecars = map[string]*manifest.SidecarConfig{
					"xray": {
						Port: aws.String("5000"),
					},
				}
			},
			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadRulePathParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadHTTPSParamKey),
					ParameterValue: aws.String("true"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
					ParameterValue: aws.String("xray"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetPortParamKey),
					ParameterValue: aws.String("5000"),
				},
				{
					ParameterKey:   aws.String(WorkloadTaskCountParamKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
					ParameterValue: aws.String("false"),
				},
			}...),
		},
		"exec enabled": {
			httpsEnabled: false,
			setupManifest: func(service *manifest.LoadBalancedWebService) {
				service.ExecuteCommand = manifest.ExecuteCommand{
					Enable: aws.Bool(false),
					Config: manifest.ExecuteCommandConfig{
						Enable: aws.Bool(true),
					},
				}
			},
			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadRulePathParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadHTTPSParamKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(WorkloadTaskCountParamKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
					ParameterValue: aws.String("false"),
				},
			}...),
		},
		"dns delegation enabled": {
			httpsEnabled:         false,
			dnsDelegationEnabled: true,

			setupManifest: func(service *manifest.LoadBalancedWebService) {
				service.ExecuteCommand = manifest.ExecuteCommand{
					Enable: aws.Bool(false),
					Config: manifest.ExecuteCommandConfig{
						Enable: aws.Bool(true),
					},
				}
			},
			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadRulePathParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadHTTPSParamKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(WorkloadTaskCountParamKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
					ParameterValue: aws.String("true"),
				},
			}...),
		},
		"nlb enabled": {
			setupManifest: func(service *manifest.LoadBalancedWebService) {
				service.NLBConfig = manifest.NetworkLoadBalancerConfiguration{
					Listener: manifest.NetworkLoadBalancerListener{
						Port: aws.String("443/tcp"),
					},
				}
			},
			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadRulePathParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadHTTPSParamKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(WorkloadTaskCountParamKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceNLBPortParamKey),
					ParameterValue: aws.String("443"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceNLBAliasesParamKey),
					ParameterValue: aws.String(""),
				},
			}...),
		},
		"nlb alias enabled": {
			setupManifest: func(service *manifest.LoadBalancedWebService) {
				service.NLBConfig = manifest.NetworkLoadBalancerConfiguration{
					Listener: manifest.NetworkLoadBalancerListener{
						Port: aws.String("443/tcp"),
					},
					Aliases: manifest.Alias{
						AdvancedAliases: []manifest.AdvancedAlias{
							{Alias: aws.String("example.com")},
							{Alias: aws.String("v1.example.com")},
						},
					},
				}
			},
			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadRulePathParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadHTTPSParamKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(WorkloadTaskCountParamKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceNLBPortParamKey),
					ParameterValue: aws.String("443"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceNLBAliasesParamKey),
					ParameterValue: aws.String("example.com,v1.example.com"),
				},
			}...),
		},
		"do not render http params when disabled": {
			setupManifest: func(service *manifest.LoadBalancedWebService) {
				service.HTTPOrBool = manifest.HTTPOrBool{
					Enabled: aws.Bool(false),
				}
			},
			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(WorkloadTaskCountParamKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceDNSDelegatedParamKey),
					ParameterValue: aws.String("false"),
				},
			}...),
		},
		"with bad count": {
			httpsEnabled: true,
			setupManifest: func(service *manifest.LoadBalancedWebService) {
				badCountRange := manifest.IntRangeBand("badCount")
				service.Count = manifest.Count{
					AdvancedCount: manifest.AdvancedCount{
						Range: manifest.Range{
							Value: &badCountRange,
						},
					},
				}
			},
			expectedErr: fmt.Errorf("parse task count value badCount: invalid range value badCount. Should be in format of ${min}-${max}"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			testManifest := manifest.NewLoadBalancedWebService(baseProps)
			tc.setupManifest(testManifest)

			// GIVEN
			conf := &LoadBalancedWebService{
				ecsWkld: &ecsWkld{
					wkld: &wkld{
						name: aws.StringValue(testManifest.Name),
						env:  testEnvName,
						app:  testAppName,
						rc: RuntimeConfig{
							PushedImages: map[string]ECRImage{
								aws.StringValue(testManifest.Name): {
									RepoURL:  testImageRepoURL,
									ImageTag: testImageTag,
								},
							},
						},
					},
					tc: testManifest.TaskConfig,
				},
				manifest:             testManifest,
				httpsEnabled:         tc.httpsEnabled,
				dnsDelegationEnabled: tc.dnsDelegationEnabled,
			}

			// WHEN
			params, err := conf.Parameters()

			// THEN
			if err == nil {
				require.ElementsMatch(t, tc.expectedParams, params)
			} else {
				require.EqualError(t, tc.expectedErr, err.Error())
			}
		})
	}
}

func TestLoadBalancedWebService_SerializedParameters(t *testing.T) {
	testLBWebServiceManifest := manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       "frontend",
			Dockerfile: "frontend/Dockerfile",
		},
		Path: "frontend",
		Port: 80,
	})

	c := &LoadBalancedWebService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name: "frontend",
				env:  testEnvName,
				app:  testAppName,
				rc: RuntimeConfig{
					PushedImages: map[string]ECRImage{
						"frontend": {
							RepoURL:  testImageRepoURL,
							ImageTag: testImageTag,
						},
					},
					AdditionalTags: map[string]string{
						"owner": "boss",
					},
				},
			},
			tc: testLBWebServiceManifest.TaskConfig,
		},
		manifest: testLBWebServiceManifest,
	}

	params, err := c.SerializedParameters()
	require.NoError(t, err)
	require.Equal(t, params, `{
  "Parameters": {
    "AddonsTemplateURL": "",
    "AppName": "phonetool",
    "ArtifactKeyARN": "",
    "ContainerImage": "111111111111.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c",
    "ContainerPort": "80",
    "DNSDelegated": "false",
    "EnvFileARN": "",
    "EnvName": "test",
    "HTTPSEnabled": "false",
    "LogRetention": "30",
    "RulePath": "frontend",
    "TargetContainer": "frontend",
    "TargetPort": "80",
    "TaskCPU": "256",
    "TaskCount": "1",
    "TaskMemory": "512",
    "WorkloadName": "frontend"
  },
  "Tags": {
    "copilot-application": "phonetool",
    "copilot-environment": "test",
    "copilot-service": "frontend",
    "owner": "boss"
  }
}`)
}

func TestLoadBalancedWebService_Tags(t *testing.T) {
	// GIVEN
	var testLBWebServiceManifest = manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       "frontend",
			Dockerfile: "frontend/Dockerfile",
		},
		Path: "frontend",
		Port: 80,
	})
	conf := &LoadBalancedWebService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name: aws.StringValue(testLBWebServiceManifest.Name),
				env:  testEnvName,
				app:  testAppName,
				rc: RuntimeConfig{
					PushedImages: map[string]ECRImage{
						"frontend": {
							RepoURL:  testImageRepoURL,
							ImageTag: testImageTag,
						},
					},
					AdditionalTags: map[string]string{
						"owner":              "boss",
						deploy.AppTagKey:     "overrideapp",
						deploy.EnvTagKey:     "overrideenv",
						deploy.ServiceTagKey: "overridesvc",
					},
				},
			},
		},
		manifest: testLBWebServiceManifest,
	}

	// WHEN
	tags := conf.Tags()

	// THEN
	require.Equal(t, []*cloudformation.Tag{
		{
			Key:   aws.String(deploy.AppTagKey),
			Value: aws.String("phonetool"),
		},
		{
			Key:   aws.String(deploy.EnvTagKey),
			Value: aws.String("test"),
		},
		{
			Key:   aws.String(deploy.ServiceTagKey),
			Value: aws.String("frontend"),
		},
		{
			Key:   aws.String("owner"),
			Value: aws.String("boss"),
		},
	}, tags)
}
