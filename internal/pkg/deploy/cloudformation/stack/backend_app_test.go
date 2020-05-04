// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// Test settings for container healthchecks in the backend app manifest.
var (
	testInterval    = 5 * time.Second
	testRetries     = 3
	testTimeout     = 10 * time.Second
	testStartPeriod = 0 * time.Second
)

var testBackendAppManifest = manifest.NewBackendService(manifest.BackendServiceProps{
	ServiceProps: manifest.ServiceProps{
		Name:       "frontend",
		Dockerfile: "./frontend/Dockerfile",
	},
	Port: 8080,
	HealthCheck: &manifest.ContainerHealthCheck{
		Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
		Interval:    &testInterval,
		Retries:     &testRetries,
		Timeout:     &testTimeout,
		StartPeriod: &testStartPeriod,
	},
})

func TestBackendApp_Template(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, app *BackendApp)
		wantedTemplate   string
		wantedErr        error
	}{
		"unexpected addons parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, app *BackendApp) {
				app.addons = mockTemplater{err: errors.New("some error")}
			},
			wantedErr: fmt.Errorf("generate addons template for application %s: %w", testBackendAppManifest.Name, errors.New("some error")),
		},
		"failed parsing app template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, app *BackendApp) {

				m := mocks.NewMockbackendAppReadParser(ctrl)
				m.EXPECT().ParseBackendService(gomock.Any()).Return(nil, errors.New("some error"))
				app.parser = m
				app.addons = mockTemplater{
					tpl: `Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
			},
			wantedErr: fmt.Errorf("parse backend app template: %w", errors.New("some error")),
		},
		"render template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, app *BackendApp) {
				m := mocks.NewMockbackendAppReadParser(ctrl)
				m.EXPECT().ParseBackendService(template.ServiceOpts{
					HealthCheck: &ecs.HealthCheck{
						Command:     aws.StringSlice([]string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}),
						Interval:    aws.Int64(5),
						Retries:     aws.Int64(3),
						StartPeriod: aws.Int64(0),
						Timeout:     aws.Int64(10),
					},
					NestedStack: &template.ServiceNestedStackOpts{
						StackName:       addons.StackName,
						VariableOutputs: []string{"Hello"},
					},
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				app.parser = m
				app.addons = mockTemplater{
					tpl: `Outputs:
  Hello:
    Value: hello`,
				}
			},
			wantedTemplate: "template",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			conf := &BackendApp{
				app: &app{
					name:    testBackendAppManifest.Name,
					env:     testEnvName,
					project: testProjName,
					rc: RuntimeConfig{
						ImageRepoURL: testImageRepoURL,
						ImageTag:     testImageTag,
					},
				},
				manifest: testBackendAppManifest,
			}
			tc.mockDependencies(t, ctrl, conf)

			// WHEN
			template, err := conf.Template()

			// THEN
			require.Equal(t, tc.wantedErr, err)
			require.Equal(t, tc.wantedTemplate, template)
		})
	}
}

func TestBackendApp_Parameters(t *testing.T) {
	// GIVEN
	conf := &BackendApp{
		app: &app{
			name:    testBackendAppManifest.Name,
			env:     testEnvName,
			project: testProjName,
			tc:      testBackendAppManifest.TaskConfig,
			rc: RuntimeConfig{
				ImageRepoURL: testImageRepoURL,
				ImageTag:     testImageTag,
			},
		},
		manifest: testBackendAppManifest,
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
			ParameterKey:   aws.String(BackendAppContainerPortParamKey),
			ParameterValue: aws.String("8080"),
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
			ParameterKey:   aws.String(AppLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(AppAddonsTemplateURLParamKey),
			ParameterValue: aws.String(""),
		},
	}, params)
}
