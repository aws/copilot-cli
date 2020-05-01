// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const (
	testEnvName      = "test"
	testProjName     = "phonetool"
	testImageRepoURL = "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend"
	testImageTag     = "manual-bf3678c"
)

var testLBWebAppManifest = manifest.NewLoadBalancedWebSvc(&manifest.LoadBalancedWebSvcProps{
	SvcProps: &manifest.SvcProps{
		SvcName:    "frontend",
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

func TestLoadBalancedWebApp_StackName(t *testing.T) {
	testCases := map[string]struct {
		inAppName     string
		inEnvName     string
		inProjectName string

		wantedStackName string
	}{
		"valid stack name": {
			inAppName:     "frontend",
			inEnvName:     "test",
			inProjectName: "phonetool",

			wantedStackName: "phonetool-test-frontend",
		},
		"longer than 128 characters": {
			inAppName:     "whatisthishorriblylongapplicationnamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaaaaa",
			inEnvName:     "test",
			inProjectName: "phonetool",

			wantedStackName: "phonetool-test-whatisthishorriblylongapplicationnamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaa",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			conf := &LoadBalancedWebApp{
				app: &app{
					name:    tc.inAppName,
					env:     tc.inEnvName,
					project: tc.inProjectName,
				},
			}

			// WHEN
			n := conf.StackName()

			// THEN
			require.Equal(t, tc.wantedStackName, n, "expected stack names to be equal")
		})
	}
}

func TestLoadBalancedWebApp_Template(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebApp)

		wantedTemplate string
		wantedError    error
	}{
		"unavailable rule priority lambda template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebApp) {
				m := mocks.NewMockloadBalancedWebAppReadParser(ctrl)
				m.EXPECT().Read(lbWebAppRulePriorityGeneratorPath).Return(nil, errors.New("some error"))
				c.parser = m
			},
			wantedTemplate: "",
			wantedError:    errors.New("some error"),
		},
		"unexpected addons parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebApp) {
				m := mocks.NewMockloadBalancedWebAppReadParser(ctrl)
				m.EXPECT().Read(lbWebAppRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				addons := mockTemplater{err: errors.New("some error")}
				c.parser = m
				c.app.addons = addons
			},
			wantedTemplate: "",
			wantedError:    fmt.Errorf("generate addons template for application %s: %w", testLBWebAppManifest.Name, errors.New("some error")),
		},
		"failed parsing app template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebApp) {
				m := mocks.NewMockloadBalancedWebAppReadParser(ctrl)
				m.EXPECT().Read(lbWebAppRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().ParseLoadBalancedWebService(gomock.Any()).Return(nil, errors.New("some error"))
				addons := mockTemplater{
					tpl: `Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
				c.parser = m
				c.app.addons = addons
			},

			wantedTemplate: "",
			wantedError:    errors.New("some error"),
		},
		"render template without addons": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebApp) {
				m := mocks.NewMockloadBalancedWebAppReadParser(ctrl)
				m.EXPECT().Read(lbWebAppRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("lambda")}, nil)
				m.EXPECT().ParseLoadBalancedWebService(template.ServiceOpts{
					RulePriorityLambda: "lambda",
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)

				addons := mockTemplater{err: &addons.ErrDirNotExist{}}
				c.parser = m
				c.app.addons = addons
			},

			wantedTemplate: "template",
		},
		"render template with addons": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *LoadBalancedWebApp) {
				m := mocks.NewMockloadBalancedWebAppReadParser(ctrl)
				m.EXPECT().Read(lbWebAppRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("lambda")}, nil)
				m.EXPECT().ParseLoadBalancedWebService(template.ServiceOpts{
					NestedStack: &template.ServiceNestedStackOpts{
						StackName:       addons.StackName,
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
			conf := &LoadBalancedWebApp{
				app: &app{
					name:    testLBWebAppManifest.Name,
					env:     testEnvName,
					project: testProjName,
					rc: RuntimeConfig{
						ImageRepoURL: testImageRepoURL,
						ImageTag:     testImageTag,
					},
				},
				manifest: testLBWebAppManifest,
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

func TestLoadBalancedWebApp_Parameters(t *testing.T) {
	testCases := map[string]struct {
		httpsEnabled bool
		expectedHTTP string
	}{
		"HTTPS Enabled": {
			httpsEnabled: true,
			expectedHTTP: "true",
		},
		"HTTPS Not Enabled": {
			httpsEnabled: false,
			expectedHTTP: "false",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			conf := &LoadBalancedWebApp{
				app: &app{
					name:    testLBWebAppManifest.Name,
					env:     testEnvName,
					project: testProjName,
					tc:      testLBWebAppManifest.TaskConfig,
					rc: RuntimeConfig{
						ImageRepoURL: testImageRepoURL,
						ImageTag:     testImageTag,
					},
				},
				manifest: testLBWebAppManifest,

				httpsEnabled: tc.httpsEnabled,
			}

			// WHEN
			params := conf.Parameters()

			// THEN
			require.ElementsMatch(t, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(AppProjectNameParamKey),
					ParameterValue: aws.String("phonetool"),
				},
				{
					ParameterKey:   aws.String(AppEnvNameParamKey),
					ParameterValue: aws.String("test"),
				},
				{
					ParameterKey:   aws.String(AppNameParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(AppContainerImageParamKey),
					ParameterValue: aws.String("12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c"),
				},
				{
					ParameterKey:   aws.String(LBWebAppContainerPortParamKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(LBWebAppRulePathParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(LBWebAppHealthCheckPathParamKey),
					ParameterValue: aws.String("/"),
				},
				{
					ParameterKey:   aws.String(AppTaskCPUParamKey),
					ParameterValue: aws.String("256"),
				},
				{
					ParameterKey:   aws.String(AppTaskMemoryParamKey),
					ParameterValue: aws.String("512"),
				},
				{
					ParameterKey:   aws.String(AppTaskCountParamKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBWebAppHTTPSParamKey),
					ParameterValue: aws.String(tc.expectedHTTP),
				},
				{
					ParameterKey:   aws.String(AppLogRetentionParamKey),
					ParameterValue: aws.String("30"),
				},
				{
					ParameterKey:   aws.String(AppAddonsTemplateURLParamKey),
					ParameterValue: aws.String(""),
				},
			}, params)
		})
	}
}

func TestLoadBalancedWebApp_SerializedParameters(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, c *LoadBalancedWebApp)

		wantedParams string
		wantedError  error
	}{
		"unavailable template": {
			mockDependencies: func(ctrl *gomock.Controller, c *LoadBalancedWebApp) {
				m := mocks.NewMockloadBalancedWebAppReadParser(ctrl)
				m.EXPECT().Parse(appParamsTemplatePath, gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				c.app.parser = m
			},
			wantedParams: "",
			wantedError:  errors.New("some error"),
		},
		"render params template": {
			mockDependencies: func(ctrl *gomock.Controller, c *LoadBalancedWebApp) {
				m := mocks.NewMockloadBalancedWebAppReadParser(ctrl)
				m.EXPECT().Parse(appParamsTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("params")}, nil)
				c.app.parser = m
			},
			wantedParams: "params",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := &LoadBalancedWebApp{
				app: &app{
					name:    testLBWebAppManifest.Name,
					env:     testEnvName,
					project: testProjName,
					tc:      testLBWebAppManifest.TaskConfig,
					rc: RuntimeConfig{
						ImageRepoURL: testImageRepoURL,
						ImageTag:     testImageTag,
						AdditionalTags: map[string]string{
							"owner": "boss",
						},
					},
				},
				manifest: testLBWebAppManifest,
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

func TestLoadBalancedWebApp_Tags(t *testing.T) {
	// GIVEN
	conf := &LoadBalancedWebApp{
		app: &app{
			name:    testLBWebAppManifest.Name,
			env:     testEnvName,
			project: testProjName,
			rc: RuntimeConfig{
				ImageRepoURL: testImageRepoURL,
				ImageTag:     testImageTag,
				AdditionalTags: map[string]string{
					"owner":       "boss",
					ProjectTagKey: "overrideproject",
					EnvTagKey:     "overrideenv",
					AppTagKey:     "overrideapp",
				},
			},
		},
		manifest: testLBWebAppManifest,
	}

	// WHEN
	tags := conf.Tags()

	// THEN
	require.ElementsMatch(t, []*cloudformation.Tag{
		{
			Key:   aws.String(ProjectTagKey),
			Value: aws.String("phonetool"),
		},
		{
			Key:   aws.String(EnvTagKey),
			Value: aws.String("test"),
		},
		{
			Key:   aws.String(AppTagKey),
			Value: aws.String("frontend"),
		},
		{
			Key:   aws.String("owner"),
			Value: aws.String("boss"),
		},
	}, tags)
}
