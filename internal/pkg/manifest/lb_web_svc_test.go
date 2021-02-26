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

func TestNewLoadBalancedWebService(t *testing.T) {
	testCases := map[string]struct {
		props LoadBalancedWebServiceProps

		wanted *LoadBalancedWebService
	}{
		"translates to default load balanced web service": {
			props: LoadBalancedWebServiceProps{
				WorkloadProps: &WorkloadProps{
					Name:       "frontend",
					Dockerfile: "./Dockerfile",
				},
				Path: "/",
				Port: 80,
			},

			wanted: &LoadBalancedWebService{
				Workload: Workload{
					Name: stringP("frontend"),
					Type: stringP("Load Balanced Web Service"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ServiceImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: stringP("./Dockerfile"),
								},
							},
						},
						Port: aws.Uint16(80),
					},
					RoutingRule: RoutingRule{
						Path: stringP("/"),
						HealthCheck: HealthCheckArgsOrString{
							HealthCheckPath: stringP("/"),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count: Count{
							Value: aws.Int(1),
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: stringP("public"),
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			manifest := NewLoadBalancedWebService(&tc.props)

			// THEN
			require.Equal(t, tc.wanted.Workload, manifest.Workload)
			require.Equal(t, tc.wanted.LoadBalancedWebServiceConfig, manifest.LoadBalancedWebServiceConfig)
			require.Equal(t, tc.wanted.Environments, manifest.Environments)
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
		"should use custom healthcheck configuration when provided and set default path to nil": {
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
				HealthCheckPath: nil,
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
			rr := newDefaultLoadBalancedWebService().RoutingRule
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
						Storage: &Storage{
							Volumes: map[string]Volume{
								"myEFSVolume": {
									MountPointOpts: MountPointOpts{
										ContainerPath: aws.String("/path/to/files"),
										ReadOnly:      aws.Bool(false),
									},
									EFS: EFSVolumeConfiguration{
										FileSystemID: aws.String("fs-1234"),
									},
								},
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
						Storage: &Storage{
							Volumes: map[string]Volume{
								"myEFSVolume": {
									MountPointOpts: MountPointOpts{
										ContainerPath: aws.String("/path/to/files"),
										ReadOnly:      aws.Bool(false),
									},
									EFS: EFSVolumeConfiguration{
										FileSystemID: aws.String("fs-1234"),
									},
								},
							},
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
						Storage: &Storage{
							Volumes: map[string]Volume{
								"myEFSVolume": {
									MountPointOpts: MountPointOpts{
										ContainerPath: aws.String("/path/to/files"),
										ReadOnly:      aws.Bool(false),
									},
									EFS: EFSVolumeConfiguration{
										FileSystemID: aws.String("fs-1234"),
										AuthConfig: AuthorizationConfig{
											IAM:           aws.Bool(true),
											AccessPointID: aws.String("ap-1234"),
										},
									},
								},
							},
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
					Logging: &Logging{
						ConfigFile: aws.String("mockConfigFile"),
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement:      stringP("public"),
							SecurityGroups: []string{"sg-123"},
						},
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
							Storage: &Storage{
								Volumes: map[string]Volume{
									"myEFSVolume": {
										EFS: EFSVolumeConfiguration{
											FileSystemID: aws.String("fs-5678"),
											AuthConfig: AuthorizationConfig{
												AccessPointID: aws.String("ap-5678"),
											},
										},
									},
								},
							},
						},
						Sidecars: map[string]*SidecarConfig{
							"xray": {
								Port: aws.String("2000/udp"),
								MountPoints: []SidecarMountPoint{
									{
										SourceVolume: aws.String("myEFSVolume"),
										MountPointOpts: MountPointOpts{
											ReadOnly:      aws.Bool(true),
											ContainerPath: aws.String("/var/www"),
										},
									},
								},
							},
						},
						Logging: &Logging{
							SecretOptions: map[string]string{
								"FOO": "BAR",
							},
						},
						Network: NetworkConfig{
							VPC: vpcConfig{
								SecurityGroups: []string{"sg-456", "sg-789"},
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
						Storage: &Storage{
							Volumes: map[string]Volume{
								"myEFSVolume": {
									MountPointOpts: MountPointOpts{
										ContainerPath: aws.String("/path/to/files"),
										ReadOnly:      aws.Bool(false),
									},
									EFS: EFSVolumeConfiguration{
										FileSystemID: aws.String("fs-5678"),
										AuthConfig: AuthorizationConfig{
											IAM:           aws.Bool(true),
											AccessPointID: aws.String("ap-5678"),
										},
									},
								},
							},
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000/udp"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
							MountPoints: []SidecarMountPoint{
								{
									SourceVolume: aws.String("myEFSVolume"),
									MountPointOpts: MountPointOpts{
										ReadOnly:      aws.Bool(true),
										ContainerPath: aws.String("/var/www"),
									},
								},
							},
						},
					},
					Logging: &Logging{
						ConfigFile: aws.String("mockConfigFile"),
						SecretOptions: map[string]string{
							"FOO": "BAR",
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement:      stringP("public"),
							SecurityGroups: []string{"sg-456", "sg-789"},
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
