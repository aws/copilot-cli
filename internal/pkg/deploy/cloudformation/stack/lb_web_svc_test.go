// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	testEnvName      = "test"
	testAppName      = "phonetool"
	testImageRepoURL = "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend"
	testImageTag     = "manual-bf3678c"
)

var testLBWebServiceManifest = manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
	ServiceProps: &manifest.ServiceProps{
		Name:       "frontend",
		Dockerfile: "frontend/Dockerfile",
	},
	Path: "frontend",
	Port: 80,
})

type mockTemplater struct {
	tpl string
	err error
}

func (m mockTemplater) Template() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.tpl, nil
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
				svc: &svc{
					name: tc.inSvcName,
					env:  tc.inEnvName,
					app:  tc.inAppName,
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
	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebService)

		wantedTemplate string
		wantedError    error
	}{
		"unavailable rule priority lambda template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebService) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
				m.EXPECT().Read(lbWebSvcRulePriorityGeneratorPath).Return(nil, errors.New("some error"))
				c.parser = m
			},
			wantedTemplate: "",
			wantedError:    errors.New("some error"),
		},
		"unexpected addons parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebService) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
				m.EXPECT().Read(lbWebSvcRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				addons := mockTemplater{err: errors.New("some error")}
				c.parser = m
				c.svc.addons = addons
			},
			wantedTemplate: "",
			wantedError:    fmt.Errorf("generate addons template for service %s: %w", aws.StringValue(testLBWebServiceManifest.Name), errors.New("some error")),
		},
		"failed parsing svc template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebService) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
				m.EXPECT().Read(lbWebSvcRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().ParseLoadBalancedWebService(gomock.Any()).Return(nil, errors.New("some error"))
				addons := mockTemplater{
					tpl: `Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
				c.parser = m
				c.svc.addons = addons
			},

			wantedTemplate: "",
			wantedError:    errors.New("some error"),
		},
		"render template without addons": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebService) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
				m.EXPECT().Read(lbWebSvcRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("lambda")}, nil)
				m.EXPECT().ParseLoadBalancedWebService(template.ServiceOpts{
					RulePriorityLambda: "lambda",
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)

				addons := mockTemplater{err: &addon.ErrDirNotExist{}}
				c.parser = m
				c.svc.addons = addons
			},

			wantedTemplate: "template",
		},
		"render template with addons": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebService) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
				m.EXPECT().Read(lbWebSvcRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("lambda")}, nil)
				m.EXPECT().ParseLoadBalancedWebService(template.ServiceOpts{
					NestedStack: &template.ServiceNestedStackOpts{
						StackName:       addon.StackName,
						VariableOutputs: []string{"Hello"},
						SecretOutputs:   []string{"MySecretArn"},
						PolicyOutputs:   []string{"AdditionalResourcesPolicyArn"},
					},
					RulePriorityLambda: "lambda",
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				addons := mockTemplater{
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
				}
				c.parser = m
				c.addons = addons
			},
			wantedTemplate: "template",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			conf := &LoadBalancedWebService{
				svc: &svc{
					name: aws.StringValue(testLBWebServiceManifest.Name),
					env:  testEnvName,
					app:  testAppName,
					rc: RuntimeConfig{
						ImageRepoURL: testImageRepoURL,
						ImageTag:     testImageTag,
					},
				},
				manifest: testLBWebServiceManifest,
			}
			tc.mockDependencies(t, ctrl, conf)

			// WHEN
			template, err := conf.Template()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedTemplate, template)
		})
	}
}

func TestLoadBalancedWebService_Parameters(t *testing.T) {
	testLBWebServiceManifest := manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
		ServiceProps: &manifest.ServiceProps{
			Name:       "frontend",
			Dockerfile: "frontend/Dockerfile",
		},
		Path: "frontend",
		Port: 80,
	})
	testLBWebServiceManifestWithSidecar := manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
		ServiceProps: &manifest.ServiceProps{
			Name:       "frontend",
			Dockerfile: "frontend/Dockerfile",
		},
		Path: "frontend",
		Port: 80,
	})
	testLBWebServiceManifestWithSidecar.TargetContainer = aws.String("xray")
	testLBWebServiceManifestWithSidecar.Sidecar = manifest.Sidecar{Sidecars: map[string]*manifest.SidecarConfig{
		"xray": {
			Port: aws.String("5000"),
		},
	}}
	testLBWebServiceManifestWithBadSidecar := manifest.NewLoadBalancedWebService(&manifest.LoadBalancedWebServiceProps{
		ServiceProps: &manifest.ServiceProps{
			Name:       "frontend",
			Dockerfile: "frontend/Dockerfile",
		},
		Path: "frontend",
		Port: 80,
	})
	testLBWebServiceManifestWithBadSidecar.TargetContainer = aws.String("xray")
	expectedParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(ServiceAppNameParamKey),
			ParameterValue: aws.String("phonetool"),
		},
		{
			ParameterKey:   aws.String(ServiceEnvNameParamKey),
			ParameterValue: aws.String("test"),
		},
		{
			ParameterKey:   aws.String(ServiceNameParamKey),
			ParameterValue: aws.String("frontend"),
		},
		{
			ParameterKey:   aws.String(ServiceContainerImageParamKey),
			ParameterValue: aws.String("12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c"),
		},
		{
			ParameterKey:   aws.String(LBWebServiceContainerPortParamKey),
			ParameterValue: aws.String("80"),
		},
		{
			ParameterKey:   aws.String(LBWebServiceRulePathParamKey),
			ParameterValue: aws.String("frontend"),
		},
		{
			ParameterKey:   aws.String(LBWebServiceHealthCheckPathParamKey),
			ParameterValue: aws.String("/"),
		},
		{
			ParameterKey:   aws.String(ServiceTaskCPUParamKey),
			ParameterValue: aws.String("256"),
		},
		{
			ParameterKey:   aws.String(ServiceTaskMemoryParamKey),
			ParameterValue: aws.String("512"),
		},
		{
			ParameterKey:   aws.String(ServiceTaskCountParamKey),
			ParameterValue: aws.String("1"),
		},
		{
			ParameterKey:   aws.String(ServiceLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(ServiceAddonsTemplateURLParamKey),
			ParameterValue: aws.String(""),
		},
	}
	testCases := map[string]struct {
		httpsEnabled bool
		manifest     *manifest.LoadBalancedWebService

		expectedParams []*cloudformation.Parameter
		expectedErr    error
	}{
		"HTTPS Enabled": {
			httpsEnabled: true,
			manifest:     testLBWebServiceManifest,

			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(LBWebServiceHTTPSParamKey),
					ParameterValue: aws.String("true"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
			}...),
		},
		"HTTPS Not Enabled": {
			httpsEnabled: false,
			manifest:     testLBWebServiceManifest,

			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(LBWebServiceHTTPSParamKey),
					ParameterValue: aws.String("false"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceTargetContainerParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceTargetPortParamKey),
					ParameterValue: aws.String("80"),
				},
			}...),
		},
		"with sidecar container": {
			httpsEnabled: true,
			manifest:     testLBWebServiceManifestWithSidecar,

			expectedParams: append(expectedParams, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(LBWebServiceHTTPSParamKey),
					ParameterValue: aws.String("true"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceTargetContainerParamKey),
					ParameterValue: aws.String("xray"),
				},
				{
					ParameterKey:   aws.String(LBWebServiceTargetPortParamKey),
					ParameterValue: aws.String("5000"),
				},
			}...),
		},
		"with bad sidecar container": {
			httpsEnabled: true,
			manifest:     testLBWebServiceManifestWithBadSidecar,

			expectedErr: fmt.Errorf("target container xray doesn't exist"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			conf := &LoadBalancedWebService{
				svc: &svc{
					name: aws.StringValue(tc.manifest.Name),
					env:  testEnvName,
					app:  testAppName,
					tc:   tc.manifest.TaskConfig,
					rc: RuntimeConfig{
						ImageRepoURL: testImageRepoURL,
						ImageTag:     testImageTag,
					},
				},
				manifest: tc.manifest,

				httpsEnabled: tc.httpsEnabled,
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
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, c *LoadBalancedWebService)

		wantedParams string
		wantedError  error
	}{
		"unavailable template": {
			mockDependencies: func(ctrl *gomock.Controller, c *LoadBalancedWebService) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
				m.EXPECT().Parse(svcParamsTemplatePath, gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				c.svc.parser = m
			},
			wantedParams: "",
			wantedError:  errors.New("some error"),
		},
		"render params template": {
			mockDependencies: func(ctrl *gomock.Controller, c *LoadBalancedWebService) {
				m := mocks.NewMockloadBalancedWebSvcReadParser(ctrl)
				m.EXPECT().Parse(svcParamsTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("params")}, nil)
				c.svc.parser = m
			},
			wantedParams: "params",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := &LoadBalancedWebService{
				svc: &svc{
					name: aws.StringValue(testLBWebServiceManifest.Name),
					env:  testEnvName,
					app:  testAppName,
					tc:   testLBWebServiceManifest.TaskConfig,
					rc: RuntimeConfig{
						ImageRepoURL: testImageRepoURL,
						ImageTag:     testImageTag,
						AdditionalTags: map[string]string{
							"owner": "boss",
						},
					},
				},
				manifest: testLBWebServiceManifest,
			}
			tc.mockDependencies(ctrl, c)

			// WHEN
			params, err := c.SerializedParameters()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedParams, params)
		})
	}
}

func TestLoadBalancedWebService_Tags(t *testing.T) {
	// GIVEN
	conf := &LoadBalancedWebService{
		svc: &svc{
			name: aws.StringValue(testLBWebServiceManifest.Name),
			env:  testEnvName,
			app:  testAppName,
			rc: RuntimeConfig{
				ImageRepoURL: testImageRepoURL,
				ImageTag:     testImageTag,
				AdditionalTags: map[string]string{
					"owner":       "boss",
					AppTagKey:     "overrideapp",
					EnvTagKey:     "overrideenv",
					ServiceTagKey: "overridesvc",
				},
			},
		},
		manifest: testLBWebServiceManifest,
	}

	// WHEN
	tags := conf.Tags()

	// THEN
	require.ElementsMatch(t, []*cloudformation.Tag{
		{
			Key:   aws.String(AppTagKey),
			Value: aws.String("phonetool"),
		},
		{
			Key:   aws.String(EnvTagKey),
			Value: aws.String("test"),
		},
		{
			Key:   aws.String(ServiceTagKey),
			Value: aws.String("frontend"),
		},
		{
			Key:   aws.String("owner"),
			Value: aws.String("boss"),
		},
	}, tags)
}
