// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func durationPointer(d time.Duration) *time.Duration {
	return &d
}

func TestNewRequestDrivenWebService(t *testing.T) {
	testCases := map[string]struct {
		input *RequestDrivenWebServiceProps

		wantedStruct *RequestDrivenWebService
	}{
		"should return an instance of RequestDrivenWebService": {
			input: &RequestDrivenWebServiceProps{
				WorkloadProps: &WorkloadProps{
					Name:       "frontend",
					Dockerfile: "./Dockerfile",
				},
				Port: uint16(80),
			},

			wantedStruct: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
					Type: aws.String(RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
						Port: aws.Uint16(80),
					},
					InstanceConfig: AppRunnerInstanceConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(2048),
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			svc := NewRequestDrivenWebService(tc.input)

			require.Equal(t, tc.wantedStruct.Name, svc.Name)
			require.Equal(t, tc.wantedStruct.Type, svc.Type)
			require.Equal(t, tc.wantedStruct.Environments, svc.Environments)
			require.Equal(t, tc.wantedStruct.InstanceConfig, svc.InstanceConfig)
			require.Equal(t, tc.wantedStruct.ImageConfig, svc.ImageConfig)
			require.Equal(t, tc.wantedStruct.Tags, svc.Tags)
			require.Equal(t, tc.wantedStruct.Variables, svc.Variables)

		})
	}
}

func TestRequestDrivenWebService_UnmarshalYaml(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct RequestDrivenWebService
		wantedError  error
	}{
		"should unmarshal basic yaml configuration": {
			inContent: []byte(
				"name: test-service\n" +
					"type: Request-Driven Web Service\n" +
					"cpu: 512\n" +
					"memory: 1024\n" +
					"image:\n" +
					"  build: ./Dockerfile\n" +
					"  port: 80\n",
			),

			wantedStruct: RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("test-service"),
					Type: aws.String(RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("./Dockerfile"),
							},
						},
						Port: aws.Uint16(80),
					},
					InstanceConfig: AppRunnerInstanceConfig{
						CPU:    aws.Int(512),
						Memory: aws.Int(1024),
					},
				},
			},
		},
		"should unmarshal image location": {
			inContent: []byte(
				"image:\n" +
					"  location: test-repository/image@digest\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Location: aws.String("test-repository/image@digest"),
						},
					},
				},
			},
		},
		"should unmarshal image build configuration": {
			inContent: []byte(
				"image:\n" +
					"  build:\n" +
					"    dockerfile: ./Dockerfile\n" +
					"    context: context/dir\n" +
					"    target: build-stage\n" +
					"    cache_from:\n" +
					"      - image:tag\n" +
					"    args:\n" +
					"      a: 1\n" +
					"      b: 2\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Context:    aws.String("context/dir"),
									Dockerfile: aws.String("./Dockerfile"),
									Target:     aws.String("build-stage"),
									CacheFrom:  []string{"image:tag"},
									Args:       map[string]string{"a": "1", "b": "2"},
								},
							},
						},
					},
				},
			},
		},
		"should unmarshal environment variables": {
			inContent: []byte(
				"variables:\n" +
					"  LOG_LEVEL: info\n" +
					"  NODE_ENV: development\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					Variables: map[string]string{
						"LOG_LEVEL": "info",
						"NODE_ENV":  "development",
					},
				},
			},
		},
		"should unmarshal tags": {
			inContent: []byte(
				"tags:\n" +
					"  owner: account-id\n" +
					"  project: my-project\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					Tags: map[string]string{
						"owner":   "account-id",
						"project": "my-project",
					},
				},
			},
		},
		"should unmarshal healthcheck": {
			inContent: []byte(
				"http:\n" +
					"  healthcheck:\n" +
					"    path: /healthcheck\n" +
					"    healthy_threshold: 3\n" +
					"    unhealthy_threshold: 5\n" +
					"    interval: 10s\n" +
					"    timeout: 5s\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					RequestDrivenWebServiceHttpConfig: RequestDrivenWebServiceHttpConfig{
						HealthCheckConfiguration: HealthCheckArgsOrString{
							HealthCheckArgs: HTTPHealthCheckArgs{
								Path:               aws.String("/healthcheck"),
								HealthyThreshold:   aws.Int64(3),
								UnhealthyThreshold: aws.Int64(5),
								Interval:           durationPointer(10 * time.Second),
								Timeout:            durationPointer(5 * time.Second),
							},
						},
					},
				},
			},
		},
		"should unmarshal healthcheck shorthand": {
			inContent: []byte(
				"http:\n" +
					"  healthcheck: /healthcheck\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					RequestDrivenWebServiceHttpConfig: RequestDrivenWebServiceHttpConfig{
						HealthCheckConfiguration: HealthCheckArgsOrString{
							HealthCheckPath: aws.String("/healthcheck"),
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var svc RequestDrivenWebService
			err := yaml.Unmarshal(tc.inContent, &svc)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct.Type, svc.Type)
				require.Equal(t, tc.wantedStruct.Name, svc.Name)
				require.Equal(t, tc.wantedStruct.HealthCheckConfiguration, svc.HealthCheckConfiguration)
				require.Equal(t, tc.wantedStruct.ImageConfig, svc.ImageConfig)
				require.Equal(t, tc.wantedStruct.Variables, svc.Variables)
				require.Equal(t, tc.wantedStruct.InstanceConfig, svc.InstanceConfig)
				require.Equal(t, tc.wantedStruct.Tags, svc.Tags)
			}
		})
	}
}

func TestRequestDrivenWebService_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		inManifest *RequestDrivenWebService

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			inManifest: &RequestDrivenWebService{},

			wantedError: errors.New("test error"),
		},
		"returns rendered content": {
			inManifest: &RequestDrivenWebService{},

			wantedBinary: []byte("test content"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockParser := mocks.NewMockParser(ctrl)
			tc.inManifest.parser = mockParser
			var wantedTemplContent *template.Content = nil

			if tc.wantedBinary != nil {
				wantedTemplContent = &template.Content{Buffer: bytes.NewBufferString(string(tc.wantedBinary))}
			}

			mockParser.
				EXPECT().
				Parse(requestDrivenWebSvcManifestPath, *tc.inManifest, gomock.Any()).
				Return(wantedTemplContent, tc.wantedError)

			b, err := tc.inManifest.MarshalBinary()

			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestRequestDrivenWebService_ApplyEnv(t *testing.T) {
	testCases := map[string]struct {
		in         *RequestDrivenWebService
		envToApply string

		wanted *RequestDrivenWebService
	}{
		"with image build overridden by image location": {
			in: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
					},
				},
				Environments: map[string]*RequestDrivenWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPort{
							Image: Image{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Location: aws.String("env-override location"),
						},
					},
				},
			},
		},
		"with image location overridden by image location": {
			in: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Location: aws.String("default location"),
						},
					},
				},
				Environments: map[string]*RequestDrivenWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPort{
							Image: Image{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Location: aws.String("env-override location"),
						},
					},
				},
			},
		},
		"with image build overridden by image build": {
			in: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
					},
				},
				Environments: map[string]*RequestDrivenWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("overridden build string"),
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("overridden build string"),
							},
						},
					},
				},
			},
		},
		"with image location overridden by image build": {
			in: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Location: aws.String("default location"),
						},
					},
				},
				Environments: map[string]*RequestDrivenWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("overridden build string"),
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("overridden build string"),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			conf, _ := tc.in.ApplyEnv(tc.envToApply)

			// THEN
			require.Equal(t, tc.wanted, conf, "returned configuration should have overrides from the environment")
		})
	}
}
