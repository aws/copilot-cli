// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestWorkerService_Template(t *testing.T) {
	baseProps := manifest.WorkerServiceProps{
		WorkloadProps: manifest.WorkloadProps{
			Name:       testServiceName,
			Dockerfile: testDockerfile,
		},
	}

	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, svc *WorkerService)
		setUpManifest    func(svc *WorkerService)
		wantedTemplate   string
		wantedErr        error
	}{
		"unexpected addons template parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *WorkerService) {
				svc.addons = mockAddons{tplErr: errors.New("some error")}
			},
			wantedErr: fmt.Errorf("generate addons template for %s: %w", testServiceName, errors.New("some error")),
		},
		"unexpected addons params parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *WorkerService) {
				svc.addons = mockAddons{paramsErr: errors.New("some error")}
			},
			wantedErr: fmt.Errorf("parse addons parameters for %s: %w", testServiceName, errors.New("some error")),
		},
		"failed parsing sidecars template": {
			setUpManifest: func(svc *WorkerService) {
				testWorkerSvcManifestWithBadSidecar := manifest.NewWorkerService(baseProps)
				testWorkerSvcManifestWithBadSidecar.Sidecars = map[string]*manifest.SidecarConfig{
					"xray": {
						Port: aws.String("80/80/80"),
					},
				}
				svc.manifest = testWorkerSvcManifestWithBadSidecar
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *WorkerService) {
				svc.addons = mockAddons{
					tpl: `
Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
			},
			wantedErr: fmt.Errorf("parse exposed ports in service manifest frontend: cannot parse port mapping from 80/80/80"),
		},
		"failed parsing Auto Scaling template": {
			setUpManifest: func(svc *WorkerService) {
				testWorkerSvcManifestWithBadAutoScaling := manifest.NewWorkerService(baseProps)
				badRange := manifest.IntRangeBand("badRange")
				testWorkerSvcManifestWithBadAutoScaling.Count.AdvancedCount = manifest.AdvancedCount{
					Range: manifest.Range{
						Value: &badRange,
					},
				}
				svc.manifest = testWorkerSvcManifestWithBadAutoScaling
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *WorkerService) {
				svc.addons = mockAddons{
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
			setUpManifest: func(svc *WorkerService) {
				svc.manifest = manifest.NewWorkerService(baseProps)
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *WorkerService) {
				m := mocks.NewMockworkerSvcReadParser(ctrl)
				m.EXPECT().ParseWorkerService(gomock.Any()).Return(nil, errors.New("some error"))
				svc.parser = m
				svc.addons = mockAddons{
					tpl: `
Resources:
  AdditionalResourcesPolicy:
    Type: AWS::IAM::ManagedPolicy
Outputs:
  AdditionalResourcesPolicyArn:
    Value: hello`,
				}
			},
			wantedErr: fmt.Errorf("parse worker service template: %w", errors.New("some error")),
		},
		"render template": {
			setUpManifest: func(svc *WorkerService) {
				svc.manifest = manifest.NewWorkerService(manifest.WorkerServiceProps{
					WorkloadProps: manifest.WorkloadProps{
						Name:       testServiceName,
						Dockerfile: testDockerfile,
					},
					HealthCheck: manifest.ContainerHealthCheck{
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
				svc.manifest.DeployConfig = manifest.WorkerDeploymentConfig{
					DeploymentControllerConfig: manifest.DeploymentControllerConfig{
						Rolling: aws.String("default"),
					},
					WorkerRollbackAlarms: manifest.AdvancedToUnion[[]string, manifest.WorkerAlarmArgs](
						manifest.WorkerAlarmArgs{
							MessagesDelayed: aws.Int(10),
						})}
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *WorkerService) {
				m := mocks.NewMockworkerSvcReadParser(ctrl)
				m.EXPECT().ParseWorkerService(gomock.Any()).DoAndReturn(func(actual template.WorkloadOpts) (*template.Content, error) {
					require.Equal(t, template.WorkloadOpts{
						AppName:      "phonetool",
						EnvName:      "test",
						WorkloadName: "frontend",
						WorkloadType: manifestinfo.WorkerServiceType,
						HealthCheck: &template.ContainerHealthCheck{
							Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
							Interval:    aws.Int64(5),
							Retries:     aws.Int64(3),
							StartPeriod: aws.Int64(0),
							Timeout:     aws.Int64(10),
						},
						CustomResources: make(map[string]template.S3ObjectLocation),
						ExecuteCommand:  &template.ExecuteCommandOpts{},
						NestedStack: &template.WorkloadNestedStackOpts{
							StackName:       addon.StackName,
							VariableOutputs: []string{"MyTable"},
						},
						Network: template.NetworkOpts{
							AssignPublicIP: template.DisablePublicIP,
							SubnetsType:    template.PrivateSubnetsPlacement,
							SecurityGroups: []template.SecurityGroup{},
						},
						DeploymentConfiguration: template.DeploymentConfigurationOpts{
							MinHealthyPercent: 100,
							MaxPercent:        200,
							Rollback:          template.RollingUpdateRollbackConfig{MessagesDelayed: aws.Int(10)},
						},
						EntryPoint: []string{"enter", "from"},
						Command:    []string{"here"},
					}, actual)
					return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
				})
				svc.parser = m
				svc.addons = mockAddons{
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
			conf := &WorkerService{
				ecsWkld: &ecsWkld{
					wkld: &wkld{
						name: testServiceName,
						env:  testEnvName,
						app:  testAppName,
						rc: RuntimeConfig{
							PushedImages: map[string]ECRImage{
								testServiceName: {
									RepoURL:  testImageRepoURL,
									ImageTag: testImageTag,
								},
							},
							AccountID: "0123456789012",
							Region:    "us-west-2",
						},
					},
					taskDefOverrideFunc: mockCloudFormationOverrideFunc,
				},
			}

			if tc.setUpManifest != nil {
				tc.setUpManifest(conf)
				conf.manifest.Network.VPC.Placement = manifest.PlacementArgOrString{
					PlacementString: &testPrivatePlacement,
				}
				conf.manifest.Network.VPC.SecurityGroups = manifest.SecurityGroupsIDsOrConfig{}
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

func TestWorkerService_Parameters(t *testing.T) {
	testWorkerSvcManifest := manifest.NewWorkerService(manifest.WorkerServiceProps{
		WorkloadProps: manifest.WorkloadProps{
			Name:       testServiceName,
			Dockerfile: testDockerfile,
		},
		HealthCheck: manifest.ContainerHealthCheck{
			Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
			Interval:    &testInterval,
			Retries:     &testRetries,
			Timeout:     &testTimeout,
			StartPeriod: &testStartPeriod,
		},
	})

	conf := &WorkerService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name: aws.StringValue(testWorkerSvcManifest.Name),
				env:  testEnvName,
				app:  testAppName,
				image: manifest.Image{
					ImageLocationOrBuild: manifest.ImageLocationOrBuild{
						Location: aws.String("mockLocation"),
					},
				},
			},
			tc: testWorkerSvcManifest.WorkerServiceConfig.TaskConfig,
		},
		manifest: testWorkerSvcManifest,
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
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(WorkloadArtifactKeyARNParamKey),
			ParameterValue: aws.String(""),
		},
	}, params)
}
