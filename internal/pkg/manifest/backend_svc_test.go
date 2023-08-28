// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewBackendSvc(t *testing.T) {
	testCases := map[string]struct {
		inProps BackendServiceProps

		wantedManifest *BackendService
	}{
		"without healthcheck and port": {
			inProps: BackendServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
					PrivateOnlyEnvironments: []string{
						"metrics",
					},
				},
			},
			wantedManifest: &BackendService{
				Workload: Workload{
					Name: aws.String("subscribers"),
					Type: aws.String(manifestinfo.BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build: BuildArgsOrString{
										BuildArgs: DockerBuildArgs{
											Dockerfile: aws.String("./subscribers/Dockerfile"),
										},
									},
								},
							},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count: Count{
							Value: aws.Int(1),
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
				Environments: map[string]*BackendServiceConfig{
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
		"with custom healthcheck command": {
			inProps: BackendServiceProps{
				WorkloadProps: WorkloadProps{
					Name:  "subscribers",
					Image: "mockImage",
				},
				HealthCheck: ContainerHealthCheck{
					Command: []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
				},
				Port: 8080,
			},
			wantedManifest: &BackendService{
				Workload: Workload{
					Name: aws.String("subscribers"),
					Type: aws.String(manifestinfo.BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Location: aws.String("mockImage"),
								},
							},
							Port: aws.Uint16(8080),
						},
						HealthCheck: ContainerHealthCheck{
							Command: []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count: Count{
							Value: aws.Int(1),
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
			},
		},
		"with windows platform": {
			inProps: BackendServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("windows/amd64"))},
			},
			wantedManifest: &BackendService{
				Workload: Workload{
					Name: aws.String("subscribers"),
					Type: aws.String(manifestinfo.BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build: BuildArgsOrString{
										BuildArgs: DockerBuildArgs{
											Dockerfile: aws.String("./subscribers/Dockerfile"),
										},
									},
								},
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
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			wantedBytes, err := yaml.Marshal(tc.wantedManifest)
			require.NoError(t, err)

			// WHEN
			actualBytes, err := yaml.Marshal(NewBackendService(tc.inProps))
			require.NoError(t, err)

			require.Equal(t, string(wantedBytes), string(actualBytes))
		})
	}
}

func TestBackendService_RequiredEnvironmentFeatures(t *testing.T) {
	testCases := map[string]struct {
		mft    func(svc *BackendService)
		wanted []string
	}{
		"no feature required by default": {
			mft: func(svc *BackendService) {},
		},
		"internal alb feature required": {
			mft: func(svc *BackendService) {
				svc.HTTP = HTTP{
					Main: RoutingRule{
						Path: aws.String("/mock_path"),
					},
				}
			},
			wanted: []string{template.InternalALBFeatureName},
		},
		"nat feature required": {
			mft: func(svc *BackendService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: PlacementArgOrString{
							PlacementString: placementStringP(PrivateSubnetPlacement),
						},
					},
				}
			},
			wanted: []string{template.NATFeatureName},
		},
		"efs feature required by enabling managed volume": {
			mft: func(svc *BackendService) {
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
									FileSystemID: StringOrFromCFN{Plain: aws.String("mock-id")},
								},
							},
						},
					},
				}
			},
			wanted: []string{template.EFSFeatureName},
		},
		"efs feature not required because storage is imported": {
			mft: func(svc *BackendService) {
				svc.Storage = Storage{
					Volumes: map[string]*Volume{
						"mock-imported-volume": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: StringOrFromCFN{Plain: aws.String("mock-id")},
								},
							},
						},
					},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			inSvc := BackendService{
				Workload: Workload{
					Name: aws.String("mock-svc"),
					Type: aws.String(manifestinfo.BackendServiceType),
				},
			}
			tc.mft(&inSvc)
			got := inSvc.requiredEnvironmentFeatures()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestBackendService_Port(t *testing.T) {
	testCases := map[string]struct {
		mft *BackendService

		wantedPort uint16
		wantedOK   bool
	}{
		"sets ok to false if no port is exposed": {
			mft: &BackendService{},
		},
		"returns the port value and sets ok to true if a port is exposed": {
			mft: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Port: uint16P(80),
						},
					},
				},
			},
			wantedPort: 80,
			wantedOK:   true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			actual, ok := tc.mft.Port()

			// THEN
			require.Equal(t, tc.wantedOK, ok)
			require.Equal(t, tc.wantedPort, actual)
		})
	}
}

func TestBackendService_Publish(t *testing.T) {
	testCases := map[string]struct {
		mft *BackendService

		wantedTopics []Topic
	}{
		"returns nil if there are no topics set": {
			mft: &BackendService{},
		},
		"returns the list of topics if manifest publishes notifications": {
			mft: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
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

func TestBackendSvc_ApplyEnv(t *testing.T) {
	perc := Percentage(70)
	mockConfig := ScalingConfigOrT[Percentage]{
		Value: &perc,
	}
	mockBackendServiceWithNoEnvironments := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(manifestinfo.BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: ImageWithOptionalPort{
					Image: Image{
						ImageLocationOrBuild: ImageLocationOrBuild{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
					},
					Port: aws.Uint16(8080),
				},
				HealthCheck: ContainerHealthCheck{
					Command:     []string{"hello", "world"},
					Interval:    durationp(1 * time.Second),
					Retries:     aws.Int(100),
					Timeout:     durationp(100 * time.Minute),
					StartPeriod: durationp(5 * time.Second),
				},
			},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(256),
				Count: Count{
					Value: aws.Int(1),
				},
			},
		},
	}
	mockBackendServiceWithNilEnvironment := BackendService{
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: ImageWithOptionalPort{
					Port: aws.Uint16(80),
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"test": nil,
		},
	}
	mockBackendServiceWithMinimalOverride := BackendService{
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: ImageWithOptionalPort{
					Port: aws.Uint16(80),
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"test": {
				ImageConfig: ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: ImageWithOptionalPort{
						Port: aws.Uint16(5000),
					},
				},
			},
		},
	}
	mockBackendServiceWithAllOverride := BackendService{
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: ImageWithOptionalPort{
					Port: aws.Uint16(80),
					Image: Image{
						DockerLabels: map[string]string{
							"com.amazonaws.ecs.copilot.description": "Hello world!",
						},
					},
				},
			},

			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(256),
				Count: Count{
					Value: aws.Int(1),
				},
			},
			Sidecars: map[string]*SidecarConfig{
				"xray": {
					Port: aws.String("2000/udp"),
					Image: Union[*string, ImageLocationOrBuild]{
						Basic: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
					},
				},
			},
			Logging: Logging{
				Destination: map[string]string{
					"Name":            "datadog",
					"exclude-pattern": "*",
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"test": {
				ImageConfig: ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: ImageWithOptionalPort{
						Image: Image{
							DockerLabels: map[string]string{
								"com.amazonaws.ecs.copilot.description": "Overridden!",
							},
						},
					},
				},
				TaskConfig: TaskConfig{
					Count: Count{
						AdvancedCount: AdvancedCount{
							CPU: mockConfig,
						},
					},
					CPU: aws.Int(512),
					Variables: map[string]Variable{
						"LOG_LEVEL": {
							StringOrFromCFN{
								Plain: stringP(""),
							},
						},
					},
				},
				Sidecars: map[string]*SidecarConfig{
					"xray": {
						CredsParam: aws.String("some arn"),
					},
				},
				Logging: Logging{
					Destination: map[string]string{
						"include-pattern": "*",
						"exclude-pattern": "fe/",
					},
				},
			},
		},
	}
	mockBackendServiceWithImageOverrideBuildByLocation := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(manifestinfo.BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: ImageWithOptionalPort{
					Image: Image{
						ImageLocationOrBuild: ImageLocationOrBuild{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
					},
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: ImageWithOptionalPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
		},
	}
	mockBackendServiceWithImageOverrideLocationByLocation := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(manifestinfo.BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: ImageWithOptionalPort{
					Image: Image{
						ImageLocationOrBuild: ImageLocationOrBuild{
							Location: aws.String("original location"),
						},
					},
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: ImageWithOptionalPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
		},
	}
	mockBackendServiceWithImageOverrideBuildByBuild := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(manifestinfo.BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: ImageWithOptionalPort{
					Image: Image{
						ImageLocationOrBuild: ImageLocationOrBuild{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("original dockerfile"),
									Context:    aws.String("original context"),
								},
							},
						},
					},
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: ImageWithOptionalPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildString: aws.String("env overridden dockerfile"),
								},
							},
						},
					},
				},
			},
		},
	}
	mockBackendServiceWithImageOverrideLocationByBuild := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(manifestinfo.BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: ImageWithOptionalPort{
					Image: Image{
						ImageLocationOrBuild: ImageLocationOrBuild{
							Location: aws.String("original location"),
						},
					},
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: ImageWithOptionalPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildString: aws.String("env overridden dockerfile"),
								},
							},
						},
					},
				},
			},
		},
	}
	testCases := map[string]struct {
		svc       *BackendService
		inEnvName string

		wanted   *BackendService
		original *BackendService
	}{
		"no env override": {
			svc:       &mockBackendServiceWithNoEnvironments,
			inEnvName: "test",

			wanted:   &mockBackendServiceWithNoEnvironments,
			original: &mockBackendServiceWithNoEnvironments,
		},
		"with nil env override": {
			svc:       &mockBackendServiceWithNilEnvironment,
			inEnvName: "test",

			wanted:   &mockBackendServiceWithNilEnvironment,
			original: &mockBackendServiceWithNilEnvironment,
		},
		"uses env minimal overrides": {
			svc:       &mockBackendServiceWithMinimalOverride,
			inEnvName: "test",

			wanted: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Port: aws.Uint16(5000),
						},
					},
				},
			},
			original: &mockBackendServiceWithMinimalOverride,
		},
		"uses env all overrides": {
			svc:       &mockBackendServiceWithAllOverride,
			inEnvName: "test",

			wanted: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Port: aws.Uint16(80),
							Image: Image{
								DockerLabels: map[string]string{
									"com.amazonaws.ecs.copilot.description": "Overridden!",
								},
							},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(512),
						Memory: aws.Int(256),
						Count: Count{
							AdvancedCount: AdvancedCount{
								CPU: mockConfig,
							},
						},
						Variables: map[string]Variable{
							"LOG_LEVEL": {
								StringOrFromCFN{
									Plain: stringP(""),
								},
							},
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port: aws.String("2000/udp"),
							Image: Union[*string, ImageLocationOrBuild]{
								Basic: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							},
							CredsParam: aws.String("some arn"),
						},
					},
					Logging: Logging{
						Destination: map[string]string{
							"Name":            "datadog",
							"include-pattern": "*",
							"exclude-pattern": "fe/",
						},
					},
				},
			},
			original: &mockBackendServiceWithAllOverride,
		},
		"with image build overridden by image location": {
			svc:       &mockBackendServiceWithImageOverrideBuildByLocation,
			inEnvName: "prod-iad",

			wanted: &BackendService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Location: aws.String("env-override location"),
								},
							},
						},
					},
				},
			},
			original: &mockBackendServiceWithImageOverrideBuildByLocation,
		},
		"with image location overridden by image location": {
			svc:       &mockBackendServiceWithImageOverrideLocationByLocation,
			inEnvName: "prod-iad",

			wanted: &BackendService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Location: aws.String("env-override location"),
								},
							},
						},
					},
				},
			},
			original: &mockBackendServiceWithImageOverrideLocationByLocation,
		},
		"with image build overridden by image build": {
			svc:       &mockBackendServiceWithImageOverrideBuildByBuild,
			inEnvName: "prod-iad",
			wanted: &BackendService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build: BuildArgsOrString{
										BuildString: aws.String("env overridden dockerfile"),
									},
								},
							},
						},
					},
				},
			},
			original: &mockBackendServiceWithImageOverrideBuildByBuild,
		},
		"with image location overridden by image build": {
			svc:       &mockBackendServiceWithImageOverrideLocationByBuild,
			inEnvName: "prod-iad",
			wanted: &BackendService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build: BuildArgsOrString{
										BuildString: aws.String("env overridden dockerfile"),
									},
								},
							},
						},
					},
				},
			},
			original: &mockBackendServiceWithImageOverrideLocationByBuild,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, _ := tc.svc.applyEnv(tc.inEnvName)

			// Should override properly.
			require.Equal(t, tc.wanted, got)
			// Should not impact the original manifest struct.
			require.Equal(t, tc.svc, tc.original)
		})
	}
}

func TestBackendSvc_ApplyEnv_CountOverrides(t *testing.T) {
	mockRange := IntRangeBand("1-10")
	perc := Percentage(80)
	mockConfig := ScalingConfigOrT[Percentage]{
		Value: &perc,
	}
	testCases := map[string]struct {
		svcCount Count
		envCount Count

		expected *BackendService
	}{
		"empty env advanced count override": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Range: Range{Value: &mockRange},
					CPU:   mockConfig,
				},
			},
			envCount: Count{},
			expected: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: Range{Value: &mockRange},
								CPU:   mockConfig,
							},
						},
					},
				},
			},
		},
		"with count value overriden by count value": {
			svcCount: Count{Value: aws.Int(5)},
			envCount: Count{Value: aws.Int(8)},
			expected: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{Value: aws.Int(8)},
					},
				},
			},
		},
		"with count value overriden by spot count": {
			svcCount: Count{Value: aws.Int(4)},
			envCount: Count{
				AdvancedCount: AdvancedCount{
					Spot: aws.Int(6),
				},
			},
			expected: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
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
		"with range overriden by spot count": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Range: Range{Value: &mockRange},
				},
			},
			envCount: Count{
				AdvancedCount: AdvancedCount{
					Spot: aws.Int(6),
				},
			},
			expected: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
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
		"with range overriden by range config": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Range: Range{Value: &mockRange},
				},
			},
			envCount: Count{
				AdvancedCount: AdvancedCount{
					Range: Range{
						RangeConfig: RangeConfig{
							Min: aws.Int(2),
							Max: aws.Int(8),
						},
					},
				},
			},
			expected: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
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
		"with spot overriden by count value": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Spot: aws.Int(5),
				},
			},
			envCount: Count{Value: aws.Int(12)},
			expected: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{Value: aws.Int(12)},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		// GIVEN
		svc := BackendService{
			BackendServiceConfig: BackendServiceConfig{
				TaskConfig: TaskConfig{
					Count: tc.svcCount,
				},
			},
			Environments: map[string]*BackendServiceConfig{
				"test": {
					TaskConfig: TaskConfig{
						Count: tc.envCount,
					},
				},
				"staging": {
					TaskConfig: TaskConfig{},
				},
			},
		}
		t.Run(name, func(t *testing.T) {
			// WHEN
			actual, _ := svc.applyEnv("test")

			// THEN
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestBackendService_ExposedPorts(t *testing.T) {
	testCases := map[string]struct {
		mft                *BackendService
		wantedExposedPorts map[string][]ExposedPort
	}{
		"expose primary container port through target_port": {
			mft: &BackendService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{},
					HTTP: HTTP{
						Main: RoutingRule{
							TargetPort: aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port: aws.String("2000"),
							Image: Union[*string, ImageLocationOrBuild]{
								Basic: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							},
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: map[string][]ExposedPort{
				"frontend": {
					{
						Port:          81,
						ContainerName: "frontend",
						Protocol:      "tcp",
					},
				},
				"xray": {
					{
						Port:                 2000,
						ContainerName:        "xray",
						Protocol:             "tcp",
						isDefinedByContainer: true,
					},
				},
			},
		},
		"expose two primary container port internally through image.port and target_port": {
			mft: &BackendService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Port: aws.Uint16(80),
						},
					},
					HTTP: HTTP{
						Main: RoutingRule{
							TargetPort: aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port: aws.String("2000"),
							Image: Union[*string, ImageLocationOrBuild]{
								Basic: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							},
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: map[string][]ExposedPort{
				"frontend": {
					{
						Port:                 80,
						ContainerName:        "frontend",
						Protocol:             "tcp",
						isDefinedByContainer: true,
					},
					{
						Port:          81,
						ContainerName: "frontend",
						Protocol:      "tcp",
					},
				},
				"xray": {
					{
						Port:                 2000,
						ContainerName:        "xray",
						Protocol:             "tcp",
						isDefinedByContainer: true,
					},
				},
			},
		},
		"expose two primary container port internally through image.port and target_port and target_container": {
			mft: &BackendService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Port: aws.Uint16(80),
						},
					},
					HTTP: HTTP{
						Main: RoutingRule{
							TargetContainer: aws.String("frontend"),
							TargetPort:      aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port: aws.String("2000"),
							Image: Union[*string, ImageLocationOrBuild]{
								Basic: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							},
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},
			wantedExposedPorts: map[string][]ExposedPort{
				"frontend": {
					{
						Port:                 80,
						ContainerName:        "frontend",
						Protocol:             "tcp",
						isDefinedByContainer: true,
					},
					{
						Port:          81,
						ContainerName: "frontend",
						Protocol:      "tcp",
					},
				},
				"xray": {
					{
						Port:                 2000,
						ContainerName:        "xray",
						Protocol:             "tcp",
						isDefinedByContainer: true,
					},
				},
			},
		},
		"expose primary container port through image.port and sidecar container port through target_port and target_container": {
			mft: &BackendService{
				Workload: Workload{
					Name: aws.String("frontend"),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Port: aws.Uint16(80),
						},
					},
					HTTP: HTTP{
						Main: RoutingRule{
							TargetContainer: aws.String("xray"),
							TargetPort:      aws.Uint16(81),
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Image: Union[*string, ImageLocationOrBuild]{
								Advanced: ImageLocationOrBuild{
									Location: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
								},
							},
							CredsParam: aws.String("some arn"),
						},
					},
				},
			},

			wantedExposedPorts: map[string][]ExposedPort{
				"frontend": {
					{
						Port:                 80,
						ContainerName:        "frontend",
						Protocol:             "tcp",
						isDefinedByContainer: true,
					},
				},
				"xray": {
					{
						Port:          81,
						ContainerName: "xray",
						Protocol:      "tcp",
					},
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
			require.Equal(t, tc.wantedExposedPorts, actual.PortsForContainer)
		})
	}
}
