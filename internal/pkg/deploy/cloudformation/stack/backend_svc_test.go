// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
		"unavailable rule priority lambda template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(nil, errors.New("some error"))
				svc.parser = m
			},
			wantedTemplate: "",
			wantedErr:      fmt.Errorf("read rule priority lambda: some error"),
		},
		"unavailable desired count lambda template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(nil, errors.New("some error"))
				svc.parser = m
			},
			wantedTemplate: "",
			wantedErr:      fmt.Errorf("read desired count lambda: some error"),
		},
		"unavailable env controller lambda template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
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
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				svc.parser = m
				svc.addons = mockAddons{tplErr: errors.New("some error")}
			},
			wantedErr: fmt.Errorf("generate addons template for %s: %w", testServiceName, errors.New("some error")),
		},
		"unexpected addons parameter parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				svc.parser = m
				svc.addons = mockAddons{paramsErr: errors.New("some error")}
			},
			wantedErr: fmt.Errorf("parse addons parameters for %s: %w", testServiceName, errors.New("some error")),
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
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
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
			wantedErr: fmt.Errorf("convert the sidecar configuration for service frontend: %w", errors.New("cannot parse port mapping from 80/80/80")),
		},
		"failed parsing Auto Scaling template": {
			setUpManifest: func(svc *BackendService) {
				testBackendSvcManifestWithBadAutoScaling := manifest.NewBackendService(baseProps)
				badRange := manifest.IntRangeBand("badRange")
				testBackendSvcManifestWithBadAutoScaling.Count.AdvancedCount = manifest.AdvancedCount{
					Range: manifest.Range{
						Value: &badRange,
					},
				}
				svc.manifest = testBackendSvcManifestWithBadAutoScaling
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
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
			wantedErr: fmt.Errorf("convert the advanced count configuration for service frontend: %w", errors.New("invalid range value badRange. Should be in format of ${min}-${max}")),
		},
		"failed parsing svc template": {
			setUpManifest: func(svc *BackendService) {
				svc.manifest = manifest.NewBackendService(baseProps)
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().ParseBackendService(gomock.Any()).Return(nil, errors.New("some error"))
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
				svc.manifest.DeployConfig = manifest.DeploymentConfiguration{
					Rolling: aws.String("recreate"),
				}
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().ParseBackendService(template.WorkloadOpts{
					WorkloadType: manifest.BackendServiceType,
					HealthCheck: &template.ContainerHealthCheck{
						Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
						Interval:    aws.Int64(5),
						Retries:     aws.Int64(3),
						StartPeriod: aws.Int64(0),
						Timeout:     aws.Int64(10),
					},
					HTTPHealthCheck: template.HTTPHealthCheckOpts{
						HealthCheckPath: manifest.DefaultHealthCheckPath,
						GracePeriod:     aws.Int64(manifest.DefaultHealthCheckGracePeriod),
					},
					DeregistrationDelay: aws.Int64(60), // defaults to 60
					RulePriorityLambda:  "something",
					DesiredCountLambda:  "something",
					EnvControllerLambda: "something",
					ExecuteCommand:      &template.ExecuteCommandOpts{},
					NestedStack: &template.WorkloadNestedStackOpts{
						StackName:       addon.StackName,
						VariableOutputs: []string{"MyTable"},
					},
					Network: template.NetworkOpts{
						AssignPublicIP: template.DisablePublicIP,
						SubnetsType:    template.PrivateSubnetsPlacement,
						SecurityGroups: []string{"sg-1234"},
					},
					DeploymentConfiguration: template.DeploymentConfigurationOpts{
						MinHealthyPercent: 0,
						MaxPercent:        100,
					},
					EntryPoint: []string{"enter", "from"},
					Command:    []string{"here"},
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				svc.parser = m
				svc.addons = mockAddons{
					tpl: `
Resources:
  MyTable:
    Type: AWS::DynamoDB::Table
Outputs:
  MyTable:
    Value: !Ref MyTable`,
					params: "",
				}
			},
			wantedTemplate: "template",
		},
		"render template with internal alb": {
			setUpManifest: func(svc *BackendService) {
				svc.manifest = manifest.NewBackendService(manifest.BackendServiceProps{
					WorkloadProps: manifest.WorkloadProps{
						Name:       testServiceName,
						Dockerfile: testDockerfile,
					},
					Port: 8080,
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
				svc.manifest.DeployConfig = manifest.DeploymentConfiguration{
					Rolling: aws.String("recreate"),
				}
				svc.manifest.RoutingRule = manifest.RoutingRuleConfigOrBool{
					RoutingRuleConfiguration: manifest.RoutingRuleConfiguration{
						Path: aws.String("/albPath"),
						HealthCheck: manifest.HealthCheckArgsOrString{
							HealthCheckArgs: manifest.HTTPHealthCheckArgs{
								Path:               aws.String("/healthz"),
								Port:               aws.Int(4200),
								SuccessCodes:       aws.String("418"),
								HealthyThreshold:   aws.Int64(64),
								UnhealthyThreshold: aws.Int64(63),
								Timeout:            (*time.Duration)(aws.Int64(int64(62 * time.Second))),
								Interval:           (*time.Duration)(aws.Int64(int64(61 * time.Second))),
								GracePeriod:        (*time.Duration)(aws.Int64(int64(1 * time.Minute))),
							},
						},
						Stickiness:          aws.Bool(true),
						DeregistrationDelay: (*time.Duration)(aws.Int64(int64(59 * time.Second))),
						AllowedSourceIps:    []manifest.IPNet{"10.0.1.0/24"},
					},
				}
			},
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, svc *BackendService) {
				m := mocks.NewMockbackendSvcReadParser(ctrl)
				m.EXPECT().Read(albRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(desiredCountGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Read(envControllerPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().ParseBackendService(template.WorkloadOpts{
					WorkloadType: manifest.BackendServiceType,
					HealthCheck: &template.ContainerHealthCheck{
						Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
						Interval:    aws.Int64(5),
						Retries:     aws.Int64(3),
						StartPeriod: aws.Int64(0),
						Timeout:     aws.Int64(10),
					},
					HTTPHealthCheck: template.HTTPHealthCheckOpts{
						HealthCheckPath:    "/healthz",
						Port:               "4200",
						SuccessCodes:       "418",
						HealthyThreshold:   aws.Int64(64),
						UnhealthyThreshold: aws.Int64(63),
						Timeout:            aws.Int64(62),
						Interval:           aws.Int64(61),
						GracePeriod:        aws.Int64(60),
					},
					DeregistrationDelay: aws.Int64(59),
					AllowedSourceIps:    []string{"10.0.1.0/24"},
					RulePriorityLambda:  "something",
					DesiredCountLambda:  "something",
					EnvControllerLambda: "something",
					ExecuteCommand:      &template.ExecuteCommandOpts{},
					NestedStack: &template.WorkloadNestedStackOpts{
						StackName:       addon.StackName,
						VariableOutputs: []string{"MyTable"},
					},
					Network: template.NetworkOpts{
						AssignPublicIP: template.DisablePublicIP,
						SubnetsType:    template.PrivateSubnetsPlacement,
						SecurityGroups: []string{"sg-1234"},
					},
					DeploymentConfiguration: template.DeploymentConfigurationOpts{
						MinHealthyPercent: 0,
						MaxPercent:        100,
					},
					EntryPoint: []string{"enter", "from"},
					Command:    []string{"here"},
					ALBEnabled: true,
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				svc.parser = m
				svc.addons = mockAddons{
					tpl: `
Resources:
  MyTable:
    Type: AWS::DynamoDB::Table
Outputs:
  MyTable:
    Value: !Ref MyTable`,
					params: "",
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
				ecsWkld: &ecsWkld{
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
					taskDefOverrideFunc: mockCloudFormationOverrideFunc,
				},
			}

			if tc.setUpManifest != nil {
				tc.setUpManifest(conf)
				privatePlacement := manifest.PlacementString(manifest.PrivateSubnetPlacement)
				conf.manifest.Network.VPC.Placement = manifest.PlacementArgOrString{
					PlacementString: &privatePlacement,
				}
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
		HealthCheck: manifest.ContainerHealthCheck{
			Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
			Interval:    &testInterval,
			Retries:     &testRetries,
			Timeout:     &testTimeout,
			StartPeriod: &testStartPeriod,
		},
	})

	conf := &BackendService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name: aws.StringValue(testBackendSvcManifest.Name),
				env:  testEnvName,
				app:  testAppName,
				image: manifest.Image{
					Location: aws.String("mockLocation"),
				},
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
			ParameterKey:   aws.String(WorkloadContainerPortParamKey),
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
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(""),
		},
	}, params)
}

func TestBackendService_TemplateAndParamsGeneration(t *testing.T) {
	const (
		appName        = "my-app"
		envName        = "my-env"
		manifestSuffix = "-manifest.yml"
		templateSuffix = "-template.yml"
		paramsSuffix   = "-params.json"
	)

	// discover test cases
	tests := make(map[string]string) // name -> path prefix
	dir := filepath.Join("testdata", "workloads", "backend")
	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), manifestSuffix) {
			name := strings.TrimSuffix(info.Name(), manifestSuffix)
			tests[name] = strings.TrimSuffix(path, manifestSuffix)
		}

		return nil
	})

	// run tests
	for name, pathPrefix := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// parse files
			manifestBytes, err := ioutil.ReadFile(pathPrefix + manifestSuffix)
			require.NoError(t, err)
			tmplBytes, err := ioutil.ReadFile(pathPrefix + templateSuffix)
			require.NoError(t, err)
			paramsBytes, err := ioutil.ReadFile(pathPrefix + paramsSuffix)
			require.NoError(t, err)

			mft, err := manifest.UnmarshalWorkload([]byte(manifestBytes))
			require.NoError(t, err)
			require.NoError(t, mft.Validate())

			serializer, err := NewBackendService(mft.(*manifest.BackendService), envName, appName,
				RuntimeConfig{
					ServiceDiscoveryEndpoint: fmt.Sprintf("%s.%s.local", envName, appName),
				})
			require.NoError(t, err)

			// mock parser for lambda functions
			realParser := serializer.parser
			mockParser := mocks.NewMockbackendSvcReadParser(ctrl)
			mockParser.EXPECT().Read(albRulePriorityGeneratorPath).Return(templateContent("albRulePriorityGenerator"), nil)
			mockParser.EXPECT().Read(desiredCountGeneratorPath).Return(templateContent("desiredCountGenerator"), nil)
			mockParser.EXPECT().Read(envControllerPath).Return(templateContent("envController"), nil)
			mockParser.EXPECT().ParseBackendService(gomock.Any()).DoAndReturn(func(data template.WorkloadOpts) (*template.Content, error) {
				// pass call to real parser
				return realParser.ParseBackendService(data)
			})
			serializer.parser = mockParser

			// validate generated template
			tmpl, err := serializer.Template()
			require.NoError(t, err)
			var actualTmpl map[any]any
			require.NoError(t, yaml.Unmarshal([]byte(tmpl), &actualTmpl))

			var expectedTmpl map[any]any
			require.NoError(t, yaml.Unmarshal(tmplBytes, &expectedTmpl))

			require.Equal(t, expectedTmpl, actualTmpl, "template mismatch")

			// validate generated params
			params, err := serializer.SerializedParameters()
			require.NoError(t, err)
			var actualParams map[string]any
			require.NoError(t, json.Unmarshal([]byte(params), &actualParams))

			var expectedParams map[string]any
			require.NoError(t, json.Unmarshal(paramsBytes, &expectedParams))

			require.Equal(t, expectedParams, actualParams, "param mismatch")
		})
	}
}

func templateContent(str string) *template.Content {
	return &template.Content{
		Buffer: bytes.NewBuffer([]byte(str)),
	}
}
