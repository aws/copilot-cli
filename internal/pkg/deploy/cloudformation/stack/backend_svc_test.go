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

	testServiceName = "frontend"
	testDockerfile  = "./frontend/Dockerfile"
)

func TestBackendService_Template(t *testing.T) {
	baseProps := manifest.BackendServiceProps{
		WorkloadProps: manifest.WorkloadProps{
			Name:       testServiceName,
			Dockerfile: testDockerfile,
		},
		Port: 8080,
	}

	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, svc *BackendService)
		setUpManifest    func(svc *BackendService)
		wantedTemplate   string
		wantedErr        error
	}{
		"unavailable desired count lambda template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(nil, errors.New("some error"))
				svc.parser = m
			},
			wantedTemplate: "",
			wantedErr:      fmt.Errorf("read desired count lambda: some error"),
		},
		"unavailable env controller lambda template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(nil, errors.New("some error"))
				svc.parser = m
			},
			wantedTemplate: "",
			wantedErr:      fmt.Errorf("read env controller lambda: some error"),
		},
		"unexpected addons parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				svc.parser = m
				svc.addons = mockTemplater{err: errors.New("some error")}
			},
			wantedErr: fmt.Errorf("generate addons template for %s: %w", testServiceName, errors.New("some error")),
		},
		"failed parsing sidecars template": {
			setUpManifest: func(svc *BackendService) {
				testBackendSvcManifestWithBadSidecar := manifest.NewBackendService(baseProps)
				testBackendSvcManifestWithBadSidecar.Sidecars = map[string]*manifest.SidecarConfig{
					"xray": {
						Port: aws.String("80/80/80"),
					},
				}
				svc.manifest = testBackendSvcManifestWithBadSidecar
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				svc.parser = m
				svc.addons = mockTemplater{
					tpl: `
Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
			},
			wantedErr: fmt.Errorf("convert the sidecar configuration for service frontend: %w", errors.New("cannot parse port mapping from 80/80/80")),
		},
		"failed parsing Auto Scaling template": {
			setUpManifest: func(svc *BackendService) {
				testBackendSvcManifestWithBadAutoScaling := manifest.NewBackendService(baseProps)
				badRange := manifest.Range("badRange")
				testBackendSvcManifestWithBadAutoScaling.Count.AdvancedCount = manifest.AdvancedCount{
					Range: &manifest.RangeOpts{
						Range: &badRange,
					},
				}
				svc.manifest = testBackendSvcManifestWithBadAutoScaling
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				svc.parser = m
				svc.addons = mockTemplater{
					tpl: `
Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
			},
			wantedErr: fmt.Errorf("convert the advanced count configuration for service frontend: %w", errors.New("invalid range value badRange. Should be in format of ${min}-${max}")),
		},
		"failed parsing svc template": {
			setUpManifest: func(svc *BackendService) {
				svc.manifest = manifest.NewBackendService(baseProps)
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().ParseBackendService(gomock.Any()).Return(nil, errors.New("some error"))
				svc.parser = m
				svc.addons = mockTemplater{
					tpl: `
Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
			},
			wantedErr: fmt.Errorf("parse backend service template: %w", errors.New("some error")),
		},
		"render template": {
			setUpManifest: func(svc *BackendService) {
				svc.manifest = manifest.NewBackendService(manifest.BackendServiceProps{
					WorkloadProps: manifest.WorkloadProps{
						Name:       testServiceName,
						Dockerfile: testDockerfile,
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
				svc.manifest.EntryPoint = manifest.EntryPointOverride{
					String:      nil,
					StringSlice: []string{"enter", "from"},
				}
				svc.manifest.Command = manifest.CommandOverride{
					String:      nil,
					StringSlice: []string{"here"},
				}
				svc.manifest.ExecuteCommand = manifest.ExecuteCommand{Enable: aws.Bool(true)}
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().ParseBackendService(template.WorkloadOpts{
					WorkloadType: manifest.BackendServiceType,
					HealthCheck: &ecs.HealthCheck{
						Command:     aws.StringSlice([]string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}),
						Interval:    aws.Int64(5),
						Retries:     aws.Int64(3),
						StartPeriod: aws.Int64(0),
						Timeout:     aws.Int64(10),
					},
					DesiredCountLambda:  "something",
					EnvControllerLambda: "something",
					ExecuteCommand:      &template.ExecuteCommandOpts{},
					NestedStack: &template.WorkloadNestedStackOpts{
						StackName:       addon.StackName,
						VariableOutputs: []string{"MyTable"},
					},
					Network: &template.NetworkOpts{
						AssignPublicIP: template.DisablePublicIP,
						SubnetsType:    template.PrivateSubnetsPlacement,
						SecurityGroups: []string{"sg-1234"},
					},
					EntryPoint: []string{"enter", "from"},
					Command:    []string{"here"},
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				svc.parser = m
				svc.addons = mockTemplater{
					tpl: `
Resources:
  MyTable:
    Type: AWS::DynamoDB::Table
Outputs:
  MyTable:
    Value: !Ref MyTable`,
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
				wkld: &wkld{
					name: testServiceName,
					env:  testEnvName,
					app:  testAppName,
					rc: RuntimeConfig{
						Image: &ECRImage{
							RepoURL:  testImageRepoURL,
							ImageTag: testImageTag,
						},
					},
				},
			}

			if tc.setUpManifest != nil {
				tc.setUpManifest(conf)
				conf.manifest.Network.VPC.Placement = aws.String(manifest.PrivateSubnetPlacement)
				conf.manifest.Network.VPC.SecurityGroups = []string{"sg-1234"}
			}

			tc.mockDependencies(t, ctrl, conf)

			// WHEN
			template, err := conf.Template()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTemplate, template)
			}
		})
	}
}

func TestBackendService_Parameters(t *testing.T) {
	testBackendSvcManifest := manifest.NewBackendService(manifest.BackendServiceProps{
		WorkloadProps: manifest.WorkloadProps{
			Name:       testServiceName,
			Dockerfile: testDockerfile,
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

	conf := &BackendService{
		wkld: &wkld{
			name: aws.StringValue(testBackendSvcManifest.Name),
			env:  testEnvName,
			app:  testAppName,
			image: manifest.Image{
				Location: aws.String("mockLocation"),
			},
			tc: testBackendSvcManifest.BackendServiceConfig.TaskConfig,
		},
		manifest: testBackendSvcManifest,
	}

	// WHEN
	params, _ := conf.Parameters()

	// THEN
	require.ElementsMatch(t, []*cloudformation.Parameter{
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
			ParameterValue: aws.String("mockLocation"),
		},
		{
			ParameterKey:   aws.String(BackendServiceContainerPortParamKey),
			ParameterValue: aws.String("8080"),
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
			ParameterKey:   aws.String(WorkloadTaskCountParamKey),
			ParameterValue: aws.String("1"),
		},
		{
			ParameterKey:   aws.String(WorkloadLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(WorkloadAddonsTemplateURLParamKey),
			ParameterValue: aws.String(""),
		},
	}, params)
}
