// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewLoadBalancedWebService_HTTPHealthCheckOpts(t *testing.T) {
	testCases := map[string]struct {
		inputPath               *string
		inputHealthyThreshold   *int64
		inputUnhealthyThreshold *int64
		inputInterval           *time.Duration
		inputTimeout            *time.Duration

		wantedOpts template.HTTPHealthCheckOpts
	}{
		"no fields indicated in manifest": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    aws.String("/"),
				HealthyThreshold:   aws.Int64(2),
				UnhealthyThreshold: aws.Int64(2),
				Interval:           aws.Int64(10),
				Timeout:            aws.Int64(5),
			},
		},
		"just HealthyThreshold": {
			inputPath:               nil,
			inputHealthyThreshold:   aws.Int64(5),
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    aws.String("/"),
				HealthyThreshold:   aws.Int64(5),
				UnhealthyThreshold: aws.Int64(2),
				Interval:           aws.Int64(10),
				Timeout:            aws.Int64(5),
			},
		},
		"just UnhealthyThreshold": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: aws.Int64(5),
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    aws.String("/"),
				HealthyThreshold:   aws.Int64(2),
				UnhealthyThreshold: aws.Int64(5),
				Interval:           aws.Int64(10),
				Timeout:            aws.Int64(5),
			},
		},
		"just Interval": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           durationp(15 * time.Second),
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    aws.String("/"),
				HealthyThreshold:   aws.Int64(2),
				UnhealthyThreshold: aws.Int64(2),
				Interval:           aws.Int64(15),
				Timeout:            aws.Int64(5),
			},
		},
		"just Timeout": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            durationp(15 * time.Second),

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    aws.String("/"),
				HealthyThreshold:   aws.Int64(2),
				UnhealthyThreshold: aws.Int64(2),
				Interval:           aws.Int64(10),
				Timeout:            aws.Int64(15),
			},
		},
		"all values changed in manifest": {
			inputPath:               aws.String("/road/to/nowhere"),
			inputHealthyThreshold:   aws.Int64(3),
			inputUnhealthyThreshold: aws.Int64(3),
			inputInterval:           durationp(60 * time.Second),
			inputTimeout:            durationp(60 * time.Second),

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    aws.String("/road/to/nowhere"),
				HealthyThreshold:   aws.Int64(3),
				UnhealthyThreshold: aws.Int64(3),
				Interval:           aws.Int64(60),
				Timeout:            aws.Int64(60),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			lbwc := LoadBalancedWebServiceConfig{
				ImageConfig: ServiceImageWithPort{},
				RoutingRule: RoutingRule{
					Path: aws.String("path"),
					HealthCheck: HealthCheckArgsOrString{
						HealthCheckPath: tc.inputPath,
						HealthCheckArgs: HTTPHealthCheckArgs{
							Path:               tc.inputPath,
							HealthyThreshold:   tc.inputHealthyThreshold,
							UnhealthyThreshold: tc.inputUnhealthyThreshold,
							Timeout:            tc.inputTimeout,
							Interval:           tc.inputInterval,
						},
					},
				},
				TaskConfig: TaskConfig{},
				Logging:    nil,
				Sidecar:    Sidecar{},
			}
			// WHEN
			actualOpts := lbwc.HTTPHealthCheckOpts()

			// THEN
			require.Equal(t, tc.wantedOpts, actualOpts)
		})
	}
}

func TestNewLoadBalancedWebService_UnmarshalYaml(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct HealthCheckArgsOrString
		wantedError  error
	}{
		"non-args path string": {
			inContent: []byte(`  healthcheck: /testing`),

			wantedStruct: HealthCheckArgsOrString{
				HealthCheckPath: aws.String("/testing"),
			},
		},
		"args specified in health check opts": {
			inContent: []byte(`  healthcheck:
    path: /testing
    healthy_threshold: 5
    unhealthy_threshold: 6
    interval: 78s
    timeout: 9s`),
			wantedStruct: HealthCheckArgsOrString{
				HealthCheckArgs: HTTPHealthCheckArgs{
					Path:               aws.String("/testing"),
					HealthyThreshold:   aws.Int64(5),
					UnhealthyThreshold: aws.Int64(6),
					Interval:           durationp(78 * time.Second),
					Timeout:            durationp(9 * time.Second),
				},
			},
		},
		"error if unmarshalable": {
			inContent: []byte(`  healthcheck:
    bath: to ruin
    unwealthy_threshold: berry`),
			wantedError: errUnmarshalHealthCheckArgs,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var rr RoutingRule
			err := yaml.Unmarshal(tc.inContent, &rr)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct.HealthCheckPath, rr.HealthCheck.HealthCheckPath)
				require.Equal(t, tc.wantedStruct.HealthCheckArgs.Path, rr.HealthCheck.HealthCheckArgs.Path)
				require.Equal(t, tc.wantedStruct.HealthCheckArgs.HealthyThreshold, rr.HealthCheck.HealthCheckArgs.HealthyThreshold)
				require.Equal(t, tc.wantedStruct.HealthCheckArgs.UnhealthyThreshold, rr.HealthCheck.HealthCheckArgs.UnhealthyThreshold)
				require.Equal(t, tc.wantedStruct.HealthCheckArgs.Interval, rr.HealthCheck.HealthCheckArgs.Interval)
				require.Equal(t, tc.wantedStruct.HealthCheckArgs.Timeout, rr.HealthCheck.HealthCheckArgs.Timeout)
			}
		})
	}
}

func TestLoadBalancedWebService_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, manifest *LoadBalancedWebService)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *LoadBalancedWebService) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(lbWebSvcManifestPath, *manifest, gomock.Any()).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *LoadBalancedWebService) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(lbWebSvcManifestPath, *manifest, gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			manifest := &LoadBalancedWebService{}
			tc.mockDependencies(ctrl, manifest)

			// WHEN
			b, err := manifest.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestLoadBalancedWebService_ApplyEnv(t *testing.T) {
	mockRange := Range("1-10")
	testCases := map[string]struct {
		in         *LoadBalancedWebService
		envToApply string

		wanted *LoadBalancedWebService
	}{
		"with no existing environments": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ServiceImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
						Port: aws.Uint16(80),
					},
					RoutingRule: RoutingRule{
						Path: aws.String("/awards/*"),
						HealthCheck: HealthCheckArgsOrString{
							HealthCheckPath: aws.String("/"),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(1024),
						Count: Count{
							Value: aws.Int(1),
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ServiceImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
						Port: aws.Uint16(80),
					},
					RoutingRule: RoutingRule{
						Path: aws.String("/awards/*"),
						HealthCheck: HealthCheckArgsOrString{
							HealthCheckPath: aws.String("/"),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(1024),
						Count: Count{
							Value: aws.Int(1),
						},
					},
				},
			},
		},
		"with overrides": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ServiceImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
						Port: aws.Uint16(80),
					},
					RoutingRule: RoutingRule{
						Path: aws.String("/awards/*"),
						HealthCheck: HealthCheckArgsOrString{
							HealthCheckPath: aws.String("/"),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(1024),
						Count: Count{
							Value: aws.Int(1),
						},
						Variables: map[string]string{
							"LOG_LEVEL":      "DEBUG",
							"DDB_TABLE_NAME": "awards",
						},
						Secrets: map[string]string{
							"GITHUB_TOKEN": "1111",
							"TWILIO_TOKEN": "1111",
						},
					},
					Sidecar: Sidecar{
						Sidecars: map[string]*SidecarConfig{
							"xray": {
								Port:       aws.String("2000"),
								Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
								CredsParam: aws.String("some arn"),
							},
						},
					},
					Logging: &Logging{
						ConfigFile: aws.String("mockConfigFile"),
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						ImageConfig: ServiceImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./RealDockerfile"),
									},
								},
							},
							Port: aws.Uint16(5000),
						},
						RoutingRule: RoutingRule{
							TargetContainer: aws.String("xray"),
						},
						TaskConfig: TaskConfig{
							CPU: aws.Int(2046),
							Count: Count{
								Value: aws.Int(0),
							},
							Variables: map[string]string{
								"DDB_TABLE_NAME": "awards-prod",
							},
						},
						Sidecar: Sidecar{
							Sidecars: map[string]*SidecarConfig{
								"xray": {
									Port: aws.String("2000/udp"),
								},
							},
						},
						Logging: &Logging{
							SecretOptions: map[string]string{
								"FOO": "BAR",
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ServiceImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./RealDockerfile"),
								},
							},
						},
						Port: aws.Uint16(5000),
					},
					RoutingRule: RoutingRule{
						Path: aws.String("/awards/*"),
						HealthCheck: HealthCheckArgsOrString{
							HealthCheckPath: aws.String("/"),
						},
						TargetContainer: aws.String("xray"),
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(2046),
						Memory: aws.Int(1024),
						Count: Count{
							Value: aws.Int(0),
						},
						Variables: map[string]string{
							"LOG_LEVEL":      "DEBUG",
							"DDB_TABLE_NAME": "awards-prod",
						},
						Secrets: map[string]string{
							"GITHUB_TOKEN": "1111",
							"TWILIO_TOKEN": "1111",
						},
					},
					Sidecar: Sidecar{
						Sidecars: map[string]*SidecarConfig{
							"xray": {
								Port:       aws.String("2000/udp"),
								Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
								CredsParam: aws.String("some arn"),
							},
						},
					},
					Logging: &Logging{
						ConfigFile: aws.String("mockConfigFile"),
						SecretOptions: map[string]string{
							"FOO": "BAR",
						},
					},
				},
			},
		},
		"with range override": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							Autoscaling: Autoscaling{
								Range: &mockRange,
								CPU:   aws.Int(80),
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							Value: nil,
							Autoscaling: Autoscaling{
								Range: &mockRange,
								CPU:   aws.Int(80),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN

			// WHEN
			conf, _ := tc.in.ApplyEnv(tc.envToApply)

			// THEN
			require.Equal(t, tc.wanted, conf, "returned configuration should have overrides from the environment")
		})
	}
}

func TestLoadBalancedWebService_BuildRequired(t *testing.T) {
	testCases := map[string]struct {
		image   Image
		want    bool
		wantErr error
	}{
		"error if both build and location are set or not set": {
			image: Image{
				Build: BuildArgsOrString{
					BuildString: aws.String("mockBuildString"),
				},
				Location: aws.String("mockLocation"),
			},
			wantErr: fmt.Errorf(`either "image.build" or "image.location" needs to be specified in the manifest`),
		},
		"return true if location is not set": {
			image: Image{
				Build: BuildArgsOrString{
					BuildString: aws.String("mockBuildString"),
				},
			},
			want: true,
		},
		"return false if location is set": {
			image: Image{
				Build:    BuildArgsOrString{},
				Location: aws.String("mockLocation"),
			},
			want: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			manifest := &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ServiceImageWithPort{
						Image: tc.image,
					},
				},
			}

			// WHEN
			got, gotErr := manifest.BuildRequired()

			// THEN
			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.want, got)
			}
		})
	}
}
