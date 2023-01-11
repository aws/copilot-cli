// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewHTTPLoadBalancedWebService(t *testing.T) {
	testCases := map[string]struct {
		props LoadBalancedWebServiceProps

		wanted *LoadBalancedWebService
	}{
		"initializes with default settings when only required configuration is provided": {
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: stringP("./Dockerfile"),
									},
								},
							},
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path: stringP("/"),
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("/"),
							},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count: Count{
							Value: aws.Int(1),
							AdvancedCount: AdvancedCount{
								workloadType: "Load Balanced Web Service",
							},
						},
						ExecuteCommand: ExecuteCommand{
							Enable: aws.Bool(false),
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{},
			},
		},
		"overrides default settings when optional configuration is provided": {
			props: LoadBalancedWebServiceProps{
				WorkloadProps: &WorkloadProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
					PrivateOnlyEnvironments: []string{
						"metrics",
					},
				},
				Path: "/",
				Port: 80,

				HTTPVersion: "gRPC",
				HealthCheck: ContainerHealthCheck{
					Command: []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
				},
				Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("windows/amd64"))},
			},

			wanted: &LoadBalancedWebService{
				Workload: Workload{
					Name: stringP("subscribers"),
					Type: stringP(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./subscribers/Dockerfile"),
									},
								},
							},
							Port: aws.Uint16(80),
						},
						HealthCheck: ContainerHealthCheck{
							Command: []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:            stringP("/"),
							ProtocolVersion: aws.String("gRPC"),
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("/"),
							},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(2048),
						Platform: PlatformArgsOrString{
							PlatformString: (*PlatformString)(aws.String("windows/amd64")),
							PlatformArgs: PlatformArgs{
								OSFamily: nil,
								Arch:     nil,
							},
						},
						Count: Count{
							Value: aws.Int(1),
							AdvancedCount: AdvancedCount{
								workloadType: "Load Balanced Web Service",
							},
						},
						ExecuteCommand: ExecuteCommand{
							Enable: aws.Bool(false),
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"metrics": {
						Network: NetworkConfig{
							VPC: vpcConfig{
								Placement: PlacementArgOrString{
									PlacementString: placementStringP(PrivateSubnetPlacement),
								},
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
	}{
		"non-args path string": {
			inContent: []byte(`  healthcheck: /testing`),

			wantedStruct: HealthCheckArgsOrString{
				Union: BasicToUnion[string, HTTPHealthCheckArgs]("/testing"),
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
				Union: AdvancedToUnion[string](HTTPHealthCheckArgs{
					Path:               aws.String("/testing"),
					HealthyThreshold:   aws.Int64(5),
					UnhealthyThreshold: aws.Int64(6),
					Interval:           durationp(78 * time.Second),
					Timeout:            durationp(9 * time.Second),
				}),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			rr := newDefaultHTTPLoadBalancedWebService().RoutingRule
			err := yaml.Unmarshal(tc.inContent, &rr)
			require.NoError(t, err)
			require.Equal(t, tc.wantedStruct.Advanced, rr.HealthCheck.Advanced)
			require.Equal(t, tc.wantedStruct.Advanced.Path, rr.HealthCheck.Advanced.Path)
			require.Equal(t, tc.wantedStruct.Advanced.HealthyThreshold, rr.HealthCheck.Advanced.HealthyThreshold)
			require.Equal(t, tc.wantedStruct.Advanced.UnhealthyThreshold, rr.HealthCheck.Advanced.UnhealthyThreshold)
			require.Equal(t, tc.wantedStruct.Advanced.Interval, rr.HealthCheck.Advanced.Interval)
			require.Equal(t, tc.wantedStruct.Advanced.Timeout, rr.HealthCheck.Advanced.Timeout)
		})
	}
}

func TestLoadBalancedWebService_ApplyEnv(t *testing.T) {
	var (
		perc       = Percentage(80)
		mockIPNet1 = IPNet("10.1.0.0/24")
		mockIPNet2 = IPNet("10.1.1.0/24")
		mockRange  = IntRangeBand("1-10")
		mockConfig = ScalingConfigOrT[Percentage]{
			Value: &perc,
		}
	)
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./Dockerfile"),
									},
								},
							},
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path: aws.String("/awards/*"),
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("/"),
							},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(1024),
						Count: Count{
							Value: aws.Int(1),
						},
						Variables: map[string]Variable{
							"VAR1": {
								stringOrFromCFN{
									Plain: stringP("var1"),
								},
							},
							"VAR2": {
								stringOrFromCFN{
									FromCFN: fromCFN{
										Name: stringP("import-var2"),
									},
								},
							},
						},
						Storage: Storage{
							Volumes: map[string]*Volume{
								"myEFSVolume": {
									MountPointOpts: MountPointOpts{
										ContainerPath: aws.String("/path/to/files"),
										ReadOnly:      aws.Bool(false),
									},
									EFS: EFSConfigOrBool{
										Advanced: EFSVolumeConfiguration{
											FileSystemID: aws.String("fs-1234"),
										},
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./Dockerfile"),
									},
								},
							},
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path: aws.String("/awards/*"),
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("/"),
							},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(1024),
						Count: Count{
							Value: aws.Int(1),
						},
						Variables: map[string]Variable{
							"VAR1": {
								stringOrFromCFN{
									Plain: stringP("var1"),
								},
							},
							"VAR2": {
								stringOrFromCFN{
									FromCFN: fromCFN{
										Name: stringP("import-var2"),
									},
								},
							},
						},
						Storage: Storage{
							Volumes: map[string]*Volume{
								"myEFSVolume": {
									MountPointOpts: MountPointOpts{
										ContainerPath: aws.String("/path/to/files"),
										ReadOnly:      aws.Bool(false),
									},
									EFS: EFSConfigOrBool{
										Advanced: EFSVolumeConfiguration{
											FileSystemID: aws.String("fs-1234"),
										},
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./Dockerfile"),
									},
								},
							},
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path: aws.String("/awards/*"),
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("/"),
							},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(1024),
						Count: Count{
							Value: aws.Int(1),
						},
						Variables: map[string]Variable{
							"LOG_LEVEL": {
								stringOrFromCFN{
									Plain: stringP("DEBUG"),
								},
							},
							"S3_TABLE_NAME": {
								stringOrFromCFN{
									Plain: stringP("doggo"),
								},
							},
							"RDS_TABLE_NAME": {
								stringOrFromCFN{
									FromCFN: fromCFN{
										Name: stringP("duckling"),
									},
								},
							},
							"DDB_TABLE_NAME": {
								stringOrFromCFN{
									FromCFN: fromCFN{
										Name: stringP("awards"),
									},
								},
							},
						},
						Secrets: map[string]Secret{
							"GITHUB_TOKEN": {
								from: stringOrFromCFN{
									Plain: aws.String("1111"),
								},
							},
							"TWILIO_TOKEN": {
								from: stringOrFromCFN{
									Plain: aws.String("1111"),
								},
							},
						},
						Storage: Storage{
							Volumes: map[string]*Volume{
								"myEFSVolume": {
									MountPointOpts: MountPointOpts{
										ContainerPath: aws.String("/path/to/files"),
										ReadOnly:      aws.Bool(false),
									},
									EFS: EFSConfigOrBool{
										Advanced: EFSVolumeConfiguration{
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
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
					Logging: Logging{
						ConfigFile: aws.String("mockConfigFile"),
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
							SecurityGroups: SecurityGroupsIDsOrConfig{
								IDs: []stringOrFromCFN{{
									Plain: aws.String("sg-123"),
								}},
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPortAndHealthcheck{
							ImageWithPort: ImageWithPort{
								Image: Image{
									Build: BuildArgsOrString{
										BuildArgs: DockerBuildArgs{
											Dockerfile: aws.String("./RealDockerfile"),
										},
									},
								},
								Port: aws.Uint16(5000),
							},
						},
						RoutingRule: RoutingRuleConfigOrBool{
							RoutingRuleConfiguration: RoutingRuleConfiguration{
								TargetContainer: aws.String("xray"),
							},
						},
						TaskConfig: TaskConfig{
							CPU: aws.Int(2046),
							Count: Count{
								Value: aws.Int(0),
							},
							Variables: map[string]Variable{
								"LOG_LEVEL": {
									stringOrFromCFN{
										Plain: stringP("ERROR"),
									},
								},
								"S3_TABLE_NAME": {
									stringOrFromCFN{
										FromCFN: fromCFN{Name: stringP("prod-doggo")},
									},
								},
								"RDS_TABLE_NAME": {
									stringOrFromCFN{Plain: stringP("duckling-prod")},
								},
								"DDB_TABLE_NAME": {
									stringOrFromCFN{
										FromCFN: fromCFN{Name: stringP("awards-prod")},
									},
								},
							},
							Storage: Storage{
								Volumes: map[string]*Volume{
									"myEFSVolume": {
										EFS: EFSConfigOrBool{
											Advanced: EFSVolumeConfiguration{
												FileSystemID: aws.String("fs-5678"),
												AuthConfig: AuthorizationConfig{
													AccessPointID: aws.String("ap-5678"),
												},
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
						Logging: Logging{
							SecretOptions: map[string]Secret{
								"FOO": {
									from: stringOrFromCFN{
										Plain: aws.String("BAR"),
									},
								},
							},
						},
						Network: NetworkConfig{
							VPC: vpcConfig{
								SecurityGroups: SecurityGroupsIDsOrConfig{
									IDs: []stringOrFromCFN{
										{
											Plain: aws.String("sg-456"),
										},
										{
											Plain: aws.String("sg-789"),
										},
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./RealDockerfile"),
									},
								},
							},
							Port: aws.Uint16(5000),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path: aws.String("/awards/*"),
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("/"),
							},
							TargetContainer: aws.String("xray"),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(2046),
						Memory: aws.Int(1024),
						Count: Count{
							Value: aws.Int(0),
						},
						Variables: map[string]Variable{
							"LOG_LEVEL": {
								stringOrFromCFN{
									Plain: stringP("ERROR"),
								},
							},
							"S3_TABLE_NAME": {
								stringOrFromCFN{
									FromCFN: fromCFN{Name: stringP("prod-doggo")},
								},
							},
							"RDS_TABLE_NAME": {
								stringOrFromCFN{
									Plain: stringP("duckling-prod"),
								},
							},
							"DDB_TABLE_NAME": {
								stringOrFromCFN{
									FromCFN: fromCFN{Name: stringP("awards-prod")},
								},
							},
						},
						Secrets: map[string]Secret{
							"GITHUB_TOKEN": {
								from: stringOrFromCFN{
									Plain: aws.String("1111"),
								},
							},
							"TWILIO_TOKEN": {
								from: stringOrFromCFN{
									Plain: aws.String("1111"),
								},
							},
						},
						Storage: Storage{
							Volumes: map[string]*Volume{
								"myEFSVolume": {
									MountPointOpts: MountPointOpts{
										ContainerPath: aws.String("/path/to/files"),
										ReadOnly:      aws.Bool(false),
									},
									EFS: EFSConfigOrBool{
										Advanced: EFSVolumeConfiguration{
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
					Logging: Logging{
						ConfigFile: aws.String("mockConfigFile"),
						SecretOptions: map[string]Secret{
							"FOO": {
								from: stringOrFromCFN{
									Plain: aws.String("BAR"),
								},
							},
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
							SecurityGroups: SecurityGroupsIDsOrConfig{
								IDs: []stringOrFromCFN{
									{
										Plain: aws.String("sg-456"),
									},
									{
										Plain: aws.String("sg-789"),
									},
								},
							},
						},
					},
				},
			},
		},
		"with empty env override": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: Range{Value: &mockRange},
								CPU:   mockConfig,
							},
						},
						Variables: map[string]Variable{
							"VAR1": {
								stringOrFromCFN{
									Plain: stringP("var1"),
								},
							},
							"VAR2": {
								stringOrFromCFN{
									FromCFN: fromCFN{Name: stringP("import-var2")},
								},
							},
						},
					},
					ImageOverride: ImageOverride{
						Command: CommandOverride{
							StringSlice: []string{"command", "default"},
						},
						EntryPoint: EntryPointOverride{
							StringSlice: []string{"entrypoint", "default"},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": nil,
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							Value: nil,
							AdvancedCount: AdvancedCount{
								Range: Range{Value: &mockRange},
								CPU:   mockConfig,
							},
						},
						Variables: map[string]Variable{
							"VAR1": {
								stringOrFromCFN{
									Plain: stringP("var1"),
								},
							},
							"VAR2": {
								stringOrFromCFN{
									FromCFN: fromCFN{Name: stringP("import-var2")},
								},
							},
						},
					},
					ImageOverride: ImageOverride{
						Command: CommandOverride{
							StringSlice: []string{"command", "default"},
						},
						EntryPoint: EntryPointOverride{
							StringSlice: []string{"entrypoint", "default"},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": nil,
				},
			},
		},
		"with range override and preserving network config": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: Range{Value: &mockRange},
								CPU:   mockConfig,
							},
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
							SecurityGroups: SecurityGroupsIDsOrConfig{
								IDs: []stringOrFromCFN{
									{
										Plain: aws.String("sg-456"),
									},
									{
										Plain: aws.String("sg-789"),
									},
								},
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
							AdvancedCount: AdvancedCount{
								Range: Range{Value: &mockRange},
								CPU:   mockConfig,
							},
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
							SecurityGroups: SecurityGroupsIDsOrConfig{
								IDs: []stringOrFromCFN{
									{
										Plain: aws.String("sg-456"),
									},
									{
										Plain: aws.String("sg-789"),
									},
								},
							},
						},
					},
				},
			},
		},
		"with network config overridden by security group config": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
							SecurityGroups: SecurityGroupsIDsOrConfig{
								AdvancedConfig: SecurityGroupsConfig{
									SecurityGroups: []stringOrFromCFN{
										{
											Plain: aws.String("sg-535"),
										},
										{
											Plain: aws.String("sg-789"),
										},
									},
								},
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						Network: NetworkConfig{
							VPC: vpcConfig{
								SecurityGroups: SecurityGroupsIDsOrConfig{
									AdvancedConfig: SecurityGroupsConfig{
										SecurityGroups: []stringOrFromCFN{
											{
												Plain: aws.String("sg-456"),
											},
											{
												Plain: aws.String("sg-700"),
											},
										},
										DenyDefault: aws.Bool(true),
									},
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
							SecurityGroups: SecurityGroupsIDsOrConfig{
								AdvancedConfig: SecurityGroupsConfig{
									SecurityGroups: []stringOrFromCFN{
										{
											Plain: aws.String("sg-456"),
										},
										{
											Plain: aws.String("sg-700"),
										},
									},
									DenyDefault: aws.Bool(true),
								},
							},
						},
					},
				},
			},
		},
		"with count value overridden by count value": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							Value: aws.Int(5),
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						TaskConfig: TaskConfig{
							Count: Count{
								Value: aws.Int(7),
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							Value: aws.Int(7),
						},
					},
				},
			},
		},
		"with count value overridden by spot count": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{Value: aws.Int(3)},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						TaskConfig: TaskConfig{
							Count: Count{
								AdvancedCount: AdvancedCount{
									Spot: aws.Int(6),
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot: aws.Int(6),
							},
						},
					},
				},
			},
		},
		"with range overridden by spot count": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: Range{Value: &mockRange},
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						TaskConfig: TaskConfig{
							Count: Count{
								AdvancedCount: AdvancedCount{
									Spot: aws.Int(5),
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot: aws.Int(5),
							},
						},
					},
				},
			},
		},
		"with range overridden by range config": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: Range{Value: &mockRange},
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						TaskConfig: TaskConfig{
							Count: Count{
								AdvancedCount: AdvancedCount{
									Range: Range{
										RangeConfig: RangeConfig{
											Min: aws.Int(2),
											Max: aws.Int(8),
										},
									},
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: Range{
									RangeConfig: RangeConfig{
										Min: aws.Int(2),
										Max: aws.Int(8),
									},
								},
							},
						},
					},
				},
			},
		},
		"with spot overridden by count value": {
			in: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot: aws.Int(5),
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						TaskConfig: TaskConfig{
							Count: Count{Value: aws.Int(15)},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{Value: aws.Int(15)},
					},
				},
			},
		},
		"with image build overridden by image location": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./Dockerfile"),
									},
								},
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPortAndHealthcheck{
							ImageWithPort: ImageWithPort{
								Image: Image{
									Location: aws.String("env-override location"),
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
		},
		"with image location overridden by image location": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Location: aws.String("default location"),
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPortAndHealthcheck{
							ImageWithPort: ImageWithPort{
								Image: Image{
									Location: aws.String("env-override location"),
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
		},
		"with image build overridden by image build": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./Dockerfile"),
									},
								},
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPortAndHealthcheck{
							ImageWithPort: ImageWithPort{
								Image: Image{
									Build: BuildArgsOrString{
										BuildString: aws.String("overridden build string"),
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("overridden build string"),
								},
							},
						},
					},
				},
			},
		},
		"with image location overridden by image build": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Location: aws.String("default location"),
							},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPortAndHealthcheck{
							ImageWithPort: ImageWithPort{
								Image: Image{
									Build: BuildArgsOrString{
										BuildString: aws.String("overridden build string"),
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("overridden build string"),
								},
							},
						},
					},
				},
			},
		},
		"with command and entrypoint overridden": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageOverride: ImageOverride{
						Command: CommandOverride{
							StringSlice: []string{"command", "default"},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						ImageOverride: ImageOverride{
							Command: CommandOverride{
								StringSlice: []string{"command", "prod"},
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
					ImageOverride: ImageOverride{
						Command: CommandOverride{
							StringSlice: []string{"command", "prod"},
						},
					},
				},
			},
		},
		"with routing rule overridden": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("path"),
							},
							AllowedSourceIps: []IPNet{mockIPNet1},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						RoutingRule: RoutingRuleConfigOrBool{
							RoutingRuleConfiguration: RoutingRuleConfiguration{
								AllowedSourceIps: []IPNet{mockIPNet2},
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
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("path"),
							},
							AllowedSourceIps: []IPNet{mockIPNet2},
						},
					},
				},
			},
		},
		"with routing rule overridden without allowed source ips": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("path"),
							},
							AllowedSourceIps: []IPNet{mockIPNet1, mockIPNet2},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						RoutingRule: RoutingRuleConfigOrBool{
							RoutingRuleConfiguration: RoutingRuleConfiguration{
								HealthCheck: HealthCheckArgsOrString{
									Union: BasicToUnion[string, HTTPHealthCheckArgs]("another-path"),
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
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("another-path"),
							},
							AllowedSourceIps: []IPNet{mockIPNet1, mockIPNet2},
						},
					},
				},
			},
		},
		"with routing rule overridden without empty allowed source ips": {
			in: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("path"),
							},
							AllowedSourceIps: []IPNet{mockIPNet1, mockIPNet2},
						},
					},
				},
				Environments: map[string]*LoadBalancedWebServiceConfig{
					"prod-iad": {
						RoutingRule: RoutingRuleConfigOrBool{
							RoutingRuleConfiguration: RoutingRuleConfiguration{
								HealthCheck: HealthCheckArgsOrString{
									Union: BasicToUnion[string, HTTPHealthCheckArgs]("another-path"),
								},
								AllowedSourceIps: []IPNet{},
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
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							HealthCheck: HealthCheckArgsOrString{
								Union: BasicToUnion[string, HTTPHealthCheckArgs]("another-path"),
							},
							AllowedSourceIps: []IPNet{},
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			conf, _ := tc.in.applyEnv(tc.envToApply)

			// THEN
			require.Equal(t, tc.wanted, conf, "returned configuration should have overrides from the environment")
		})
	}
}

func TestLoadBalancedWebService_Port(t *testing.T) {
	// GIVEN
	mft := LoadBalancedWebService{
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
					Port: uint16P(80),
				},
			},
		},
	}

	// WHEN
	actual, ok := mft.Port()

	// THEN
	require.True(t, ok)
	require.Equal(t, uint16(80), actual)
}

func TestLoadBalancedWebService_Publish(t *testing.T) {
	testCases := map[string]struct {
		mft *LoadBalancedWebService

		wantedTopics []Topic
	}{
		"returns nil if there are no topics set": {
			mft: &LoadBalancedWebService{},
		},
		"returns the list of topics if manifest publishes notifications": {
			mft: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					PublishConfig: PublishConfig{
						Topics: []Topic{
							{
								Name: stringP("hello"),
							},
						},
					},
				},
			},
			wantedTopics: []Topic{
				{
					Name: stringP("hello"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			actual := tc.mft.Publish()

			// THEN
			require.Equal(t, tc.wantedTopics, actual)
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: tc.image,
						},
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

func TestNetworkLoadBalancerConfiguration_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     NetworkLoadBalancerConfiguration
		wanted bool
	}{
		"empty": {
			in:     NetworkLoadBalancerConfiguration{},
			wanted: true,
		},
		"non empty": {
			in: NetworkLoadBalancerConfiguration{
				Port: aws.String("443"),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.in.IsEmpty()

			// THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestLoadBalancedWebService_RequiredEnvironmentFeatures(t *testing.T) {
	testCases := map[string]struct {
		mft    func(svc *LoadBalancedWebService)
		wanted []string
	}{
		"no feature required": {
			mft: func(svc *LoadBalancedWebService) {
				svc.RoutingRule = RoutingRuleConfigOrBool{
					Enabled: aws.Bool(false),
				}
			},
		},
		"alb feature required by default": {
			mft:    func(svc *LoadBalancedWebService) {},
			wanted: []string{template.ALBFeatureName},
		},
		"nat feature required": {
			mft: func(svc *LoadBalancedWebService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: PlacementArgOrString{
							PlacementString: placementStringP(PrivateSubnetPlacement),
						},
					},
				}
			},
			wanted: []string{template.ALBFeatureName, template.NATFeatureName},
		},
		"efs feature required by enabling managed volume with bool": {
			mft: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Volumes: map[string]*Volume{
						"mock-managed-volume-1": {
							EFS: EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
						"mock-imported-volume": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mock-id"),
								},
							},
						},
					},
				}
			},
			wanted: []string{template.ALBFeatureName, template.EFSFeatureName},
		},
		"efs feature required by enabling managed volume with uid or gid": {
			mft: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Volumes: map[string]*Volume{
						"mock-managed-volume-1": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(1),
								},
							},
						},
						"mock-imported-volume": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mock-id"),
								},
							},
						},
					},
				}
			},
			wanted: []string{template.ALBFeatureName, template.EFSFeatureName},
		},
		"efs feature not required because storage is imported": {
			mft: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Volumes: map[string]*Volume{
						"mock-imported-volume": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mock-id"),
								},
							},
						},
					},
				}
			},
			wanted: []string{template.ALBFeatureName},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			inSvc := LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("mock-svc"),
					Type: aws.String(LoadBalancedWebServiceType),
				},
			}
			tc.mft(&inSvc)
			got := inSvc.requiredEnvironmentFeatures()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestLoadBalancedWebService_ExposedPorts(t *testing.T) {
	testCases := map[string]struct {
		mft                *LoadBalancedWebService
		wantedExposedPorts []ExposedPort
	}{
		"expose new sidecar container port through alb target_port and target_container": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:            aws.String("/"),
							TargetContainer: aws.String("xray"),
							TargetPort:      aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          81,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
				{
					Port:          2000,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"expose new primary container port through alb target_port": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:       aws.String("/"),
							TargetPort: aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          81,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          2000,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"expose new primary container port through alb target_port and target_container": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:            aws.String("/"),
							TargetContainer: aws.String("frontend"),
							TargetPort:      aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          81,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          2000,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"expose sidecar container port through alb target_port": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:            aws.String("/"),
							TargetContainer: aws.String("xray"),
							TargetPort:      aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          81,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"reference existing sidecar container port through alb target_port": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:       aws.String("/"),
							TargetPort: aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("81"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          81,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"reference existing primary container port through alb target_port": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:       aws.String("/"),
							TargetPort: aws.Uint16(80),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("81"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          81,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"ALB and NLB exposes the same additional port on the main container": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:       aws.String("/"),
							TargetPort: aws.Uint16(81),
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Port:       aws.String("85"),
						TargetPort: aws.Int(81),
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          81,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          2000,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"ALB and NLB exposes two different ports on the main container": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							TargetPort: aws.Uint16(81),
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Port: aws.String("82"),
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          81,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          82,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          2000,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"expose new primary container port through NLB config": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Port: aws.String("82"),
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          82,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          2000,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
		"alb and nlb pointing to the same primary container port": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(8080),
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Port: aws.String("8080"),
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:       aws.String("/"),
							TargetPort: aws.Uint16(8080),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("80"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
				{
					Port:          8080,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
			},
		},
		"alb and nlb exposing new ports of the main and sidecar containers": {
			mft: &LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(8080),
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Port:            aws.String("8082/tcp"),
						TargetContainer: aws.String("xray"),
					},
					RoutingRule: RoutingRuleConfigOrBool{
						RoutingRuleConfiguration: RoutingRuleConfiguration{
							Path:       aws.String("/"),
							TargetPort: aws.Uint16(8081),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("80"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: []ExposedPort{
				{
					Port:          80,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
				{
					Port:          8080,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          8081,
					ContainerName: "frontend",
					Protocol:      "tcp",
				},
				{
					Port:          8082,
					ContainerName: "xray",
					Protocol:      "tcp",
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			actual, err := tc.mft.ExposedPorts()

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedExposedPorts, actual)
		})
	}
}
