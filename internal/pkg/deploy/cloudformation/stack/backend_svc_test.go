// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// Test settings for container healthchecks in the backend service manifest.
var (
	testInterval    = 5 * time.Second
	testRetries     = 3
	testTimeout     = 10 * time.Second
	testStartPeriod = 0 * time.Second
)

var testBackendSvcManifest = manifest.NewBackendService(manifest.BackendServiceProps{
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

func TestBackendService_Template(t *testing.T) {
	badTestBackendSvcManifest := manifest.NewBackendService(manifest.BackendServiceProps{
		ServiceProps: manifest.ServiceProps{
			Name:       "frontend",
			Dockerfile: "./frontend/Dockerfile",
		},
		Port: 8080,
	})
	badTestBackendSvcManifest.Sidecar = manifest.Sidecar{Sidecars: map[string]*manifest.SidecarConfig{
		"xray": {
			Port: aws.String("80/80/80"),
		},
	}}
	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, svc *BackendService)
		manifest         *manifest.BackendService
		wantedTemplate   string
		wantedErr        error
	}{
		"unexpected addons parsing error": {
			manifest: testBackendSvcManifest,
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				svc.addons = mockTemplater{err: errors.New("some error")}
			},
			wantedErr: fmt.Errorf("generate addons template for service %s: %w", aws.StringValue(testBackendSvcManifest.Name), errors.New("some error")),
		},
		"failed parsing sidecars template": {
			manifest: badTestBackendSvcManifest,
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				svc.addons = mockTemplater{
					tpl: `Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
			},
			wantedErr: fmt.Errorf("convert the sidecar configuration for service frontend: %w", errors.New("cannot parse port mapping from 80/80/80")),
		},
		"failed parsing svc template": {
			manifest: testBackendSvcManifest,
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {

				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().ParseBackendService(gomock.Any()).Return(nil, errors.New("some error"))
				svc.parser = m
				svc.addons = mockTemplater{
					tpl: `Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
			},
			wantedErr: fmt.Errorf("parse backend service template: %w", errors.New("some error")),
		},
		"render template": {
			manifest: testBackendSvcManifest,
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().ParseBackendService(template.ServiceOpts{
					HealthCheck: &ecs.HealthCheck{
						Command:     aws.StringSlice([]string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}),
						Interval:    aws.Int64(5),
						Retries:     aws.Int64(3),
						StartPeriod: aws.Int64(0),
						Timeout:     aws.Int64(10),
					},
					NestedStack: &template.ServiceNestedStackOpts{
						StackName:       addon.StackName,
						VariableOutputs: []string{"Hello"},
					},
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				svc.parser = m
				svc.addons = mockTemplater{
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
			conf := &BackendService{
				svc: &svc{
					name: aws.StringValue(testBackendSvcManifest.Name),
					env:  testEnvName,
					app:  testAppName,
					rc: RuntimeConfig{
						ImageRepoURL: testImageRepoURL,
						ImageTag:     testImageTag,
					},
				},
				manifest: tc.manifest,
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

func TestBackendService_Parameters(t *testing.T) {
	// GIVEN
	conf := &BackendService{
		svc: &svc{
			name: aws.StringValue(testBackendSvcManifest.Name),
			env:  testEnvName,
			app:  testAppName,
			tc:   testBackendSvcManifest.BackendServiceConfig.TaskConfig,
			rc: RuntimeConfig{
				ImageRepoURL: testImageRepoURL,
				ImageTag:     testImageTag,
			},
		},
		manifest: testBackendSvcManifest,
	}

	// WHEN
	params, _ := conf.Parameters()

	// THEN
	require.ElementsMatch(t, []*cloudformation.Parameter{
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
			ParameterKey:   aws.String(BackendServiceContainerPortParamKey),
			ParameterValue: aws.String("8080"),
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
	}, params)
}
