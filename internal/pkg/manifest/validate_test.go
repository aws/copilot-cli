// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancedWebService_validate(t *testing.T) {
	testImageConfig := ImageWithPortAndHealthcheck{
		ImageWithPort: ImageWithPort{
			Image: Image{
				ImageLocationOrBuild: ImageLocationOrBuild{
					Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
				},
			},
			Port: uint16P(80),
		},
	}
	testCases := map[string]struct {
		lbConfig LoadBalancedWebService

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"error if fail to validate image": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
									Location: aws.String("mockLocation"),
								},
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "image": `,
		},
		"error if fail to validate grace_period when specified in the additional listener rules of ALB": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
								HealthCheck: HealthCheckArgsOrString{
									Union: AdvancedToUnion[string](HTTPHealthCheckArgs{
										Path:               aws.String("/testing"),
										HealthyThreshold:   aws.Int64(5),
										UnhealthyThreshold: aws.Int64(6),
										Interval:           durationp(78 * time.Second),
										Timeout:            durationp(9 * time.Second),
									}),
								},
							},
							AdditionalRoutingRules: []RoutingRule{
								{
									Path: stringP("/"),
									HealthCheck: HealthCheckArgsOrString{
										Union: AdvancedToUnion[string](HTTPHealthCheckArgs{
											Path:               aws.String("/testing"),
											HealthyThreshold:   aws.Int64(5),
											UnhealthyThreshold: aws.Int64(6),
											Interval:           durationp(78 * time.Second),
											Timeout:            durationp(9 * time.Second),
											GracePeriod:        durationp(9 * time.Second),
										}),
									},
								},
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "grace_period": %w`, &errGracePeriodSpecifiedInAdditionalRule{0}),
		},
		"error if fail to validate grace_period when specified the additional listener of NLB": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						Enabled: aws.Bool(false),
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port:        stringP("80"),
							HealthCheck: NLBHealthCheckArgs{GracePeriod: durationp(9 * time.Second)},
						},
						AdditionalListeners: []NetworkLoadBalancerListener{
							{
								Port:        stringP("80"),
								HealthCheck: NLBHealthCheckArgs{GracePeriod: durationp(9 * time.Second)},
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "grace_period": %w`, &errGracePeriodSpecifiedInAdditionalListener{0}),
		},
		"error if fail to validate grace_period when specified in ALB and NLB at the same time": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
								HealthCheck: HealthCheckArgsOrString{
									Union: AdvancedToUnion[string](HTTPHealthCheckArgs{
										Path:               aws.String("/testing"),
										HealthyThreshold:   aws.Int64(5),
										UnhealthyThreshold: aws.Int64(6),
										Interval:           durationp(78 * time.Second),
										Timeout:            durationp(9 * time.Second),
										GracePeriod:        durationp(9 * time.Second),
									}),
								},
							},
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port:        stringP("80"),
							HealthCheck: NLBHealthCheckArgs{GracePeriod: durationp(9 * time.Second)},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "grace_period": %w`, &errGracePeriodsInBothALBAndNLB{errFieldMutualExclusive{firstField: "http.healthcheck.grace_period", secondField: "nlb.healthcheck.grace_period"}}),
		},
		"error if fail to validate http": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								TargetContainer: aws.String("mockTargetContainer"),
							},
							TargetContainerCamelCase: aws.String("mockTargetContainer"),
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "http": `,
		},
		"error if fail to validate sidecars": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					Sidecars: map[string]*SidecarConfig{
						"foo": {
							DependsOn: DependsOn{
								"foo": "bar",
							},
						},
					},
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "sidecars[foo]": `,
		},
		"error if fail to validate network": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: (*PlacementString)(aws.String("")),
							},
						},
					},
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "network": `,
		},
		"error if fail to validate publish config": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					PublishConfig: PublishConfig{
						Topics: []Topic{
							{},
						},
					},
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "publish": `,
		},
		"error if fail to validate taskdef override": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					TaskDefOverrides: []OverrideRule{
						{
							Path: "Family",
						},
					},
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "taskdef_overrides[0]": `,
		},
		"error if name is not set": {
			lbConfig: LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`"name" must be specified`),
		},
		"error if http field is empty": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{Name: aws.String("mockName")},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
				},
			},
			wantedErrorMsgPrefix: `"http" must be specified`,
		},
		"error if fail to validate HTTP load balancer target": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{Name: aws.String("mockName")},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path:            stringP("/"),
								TargetContainer: aws.String("foo"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate load balancer target for "http":`,
		},
		"error if fail to validate network load balancer target": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{Name: aws.String("mockName")},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path:            stringP("/"),
								TargetContainer: aws.String("mockName"),
							},
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port:            aws.String("443"),
							TargetContainer: aws.String("foo"),
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate target for "nlb": `,
		},
		"error if fail to validate network load balancer target for additional listener": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{Name: aws.String("mockName")},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path:            stringP("/"),
								TargetContainer: aws.String("mockName"),
							},
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port:            aws.String("443"),
							TargetContainer: aws.String("mockName"),
						},
						AdditionalListeners: []NetworkLoadBalancerListener{
							{
								Port:            aws.String("444"),
								TargetContainer: aws.String("foo"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate target for "nlb.additional_listeners[0]": `,
		},
		"error if fail to validate dependencies": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{Name: aws.String("mockName")},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					Sidecars: map[string]*SidecarConfig{
						"foo": {
							Image:     BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: map[string]string{"bar": "healthy"},
							Essential: aws.Bool(false),
						},
						"bar": {
							Image:     BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: map[string]string{"foo": "healthy"},
							Essential: aws.Bool(false),
						},
					},
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate container dependencies: `,
		},
		"error if fail to validate windows": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{Name: aws.String("mockName")},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("windows/amd64"))},
						Storage: Storage{Volumes: map[string]*Volume{
							"foo": {
								EFS: EFSConfigOrBool{
									Enabled: aws.Bool(true),
								},
								MountPointOpts: MountPointOpts{
									ContainerPath: aws.String("mockPath"),
								},
							},
						},
						},
					},
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate Windows: `,
		},
		"error if fail to validate ARM": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("linux/arm64"))},
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot:         aws.Int(123),
								workloadType: manifestinfo.LoadBalancedWebServiceType,
							},
						},
					},
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate ARM: `,
		},
		"error if neither of http or nlb is enabled": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						Enabled: aws.Bool(false),
					},
				},
			},
			wantedError: errors.New(`must specify at least one of "http" or "nlb"`),
		},
		"error if scaling based on nlb requests": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Requests: ScalingConfigOrT[int]{
									Value: aws.Int(3),
								},
							},
						},
					},
					HTTPOrBool: HTTPOrBool{
						Enabled: aws.Bool(false),
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port: aws.String("80"),
						},
					},
				},
			},
			wantedError: errors.New(`scaling based on "nlb" requests or response time is not supported`),
		},
		"error if scaling based on nlb response time": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								ResponseTime: ScalingConfigOrT[time.Duration]{
									Value: durationp(10 * time.Second),
								},
							},
						},
					},
					HTTPOrBool: HTTPOrBool{
						Enabled: aws.Bool(false),
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port: aws.String("80"),
						},
					},
				},
			},
			wantedError: errors.New(`scaling based on "nlb" requests or response time is not supported`),
		},
		"error if healthcheck points to nlb port using udp": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						Enabled: aws.Bool(false),
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port: aws.String("80/udp"),
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate load balancer health check ports: container "mockName" exposes port 80 using protocol udp invalid for health checks. Valid protocol is "TCP".`),
		},
		"error if fail to validate deployment": {
			lbConfig: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: testImageConfig,
					HTTPOrBool: HTTPOrBool{
						HTTP: HTTP{
							Main: RoutingRule{
								Path: stringP("/"),
							},
						},
					},
					DeployConfig: DeploymentConfig{
						DeploymentControllerConfig: DeploymentControllerConfig{
							Rolling: aws.String("mockName"),
						}},
				},
			},
			wantedErrorMsgPrefix: `validate "deployment"`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.lbConfig.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestBackendService_validate(t *testing.T) {
	testImageConfig := ImageWithHealthcheckAndOptionalPort{
		ImageWithOptionalPort: ImageWithOptionalPort{
			Image: Image{
				ImageLocationOrBuild: ImageLocationOrBuild{
					Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
				},
			},
		},
	}
	testCases := map[string]struct {
		config BackendService

		wantedErrorMsgPrefix string
		wantedError          error
	}{
		"error if fail to validate image": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithHealthcheckAndOptionalPort{
						ImageWithOptionalPort: ImageWithOptionalPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
									Location: aws.String("mockLocation"),
								},
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "image": `,
		},
		"error if fail to validate sidecars": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					Sidecars: map[string]*SidecarConfig{
						"foo": {
							Image: BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: DependsOn{
								"foo": "bar",
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "sidecars[foo]": `,
		},
		"error if fail to validate network": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: (*PlacementString)(aws.String("")),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "network": `,
		},
		"error if fail to validate publish config": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					PublishConfig: PublishConfig{
						Topics: []Topic{
							{},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "publish": `,
		},
		"error if fail to validate taskdef override": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					TaskDefOverrides: []OverrideRule{
						{
							Path: "Family",
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "taskdef_overrides[0]": `,
		},
		"error if name is not set": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
				},
			},
			wantedError: fmt.Errorf(`"name" must be specified`),
		},
		"error if fail to validate dependencies": {
			config: BackendService{
				Workload: Workload{Name: aws.String("mockName")},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					Sidecars: map[string]*SidecarConfig{
						"foo": {
							Image:     BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: map[string]string{"bar": "start"},
						},
						"bar": {
							Image:     BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: map[string]string{"foo": "start"},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate container dependencies: `,
		},
		"error if fail to validate Windows": {
			config: BackendService{
				Workload: Workload{Name: aws.String("mockName")},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("windows/amd64"))},
						Storage: Storage{Volumes: map[string]*Volume{
							"foo": {
								EFS: EFSConfigOrBool{
									Enabled: aws.Bool(true),
								},
								MountPointOpts: MountPointOpts{
									ContainerPath: aws.String("mockPath"),
								},
							},
						},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate Windows: `,
		},
		"error if fail to validate ARM": {
			config: BackendService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("linux/arm64"))},
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot:         aws.Int(123),
								workloadType: manifestinfo.BackendServiceType,
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate ARM: `,
		},
		"error if fail to validate deployment": {
			config: BackendService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("linux/arm64"))},
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot:         aws.Int(123),
								workloadType: manifestinfo.BackendServiceType,
							},
						},
					},
					DeployConfig: DeploymentConfig{
						DeploymentControllerConfig: DeploymentControllerConfig{
							Rolling: aws.String("mockName"),
						}},
				},
			},
			wantedErrorMsgPrefix: `validate "deployment":`,
		},
		"error if fail to validate http": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					HTTP: HTTP{
						Main: RoutingRule{
							ProtocolVersion: aws.String("GRPC"),
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "http": "path" must be specified`,
		},
		"error if request scaling without http": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								workloadType: manifestinfo.BackendServiceType,
								Requests: ScalingConfigOrT[int]{
									Value: aws.Int(128),
								},
							},
						},
					},
				},
			},
			wantedError: errors.New(`"http" must be specified if "count.requests" or "count.response_time" are specified`),
		},
		"error if invalid topic is defined": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					PublishConfig: PublishConfig{
						Topics: []Topic{
							{
								Name: aws.String("mytopic.fifo"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "publish": `,
		},
		"error if target container not found": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					HTTP: HTTP{
						Main: RoutingRule{
							TargetContainer: aws.String("api"),
							Path:            aws.String("/"),
						},
					},
				},
				Workload: Workload{
					Name: aws.String("api"),
				},
			},
			wantedError: fmt.Errorf(`validate load balancer target for "http": target container "api" doesn't expose a port`),
		},
		"error if service connect is enabled without any port exposed": {
			config: BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: testImageConfig,
					Network: NetworkConfig{
						Connect: ServiceConnectBoolOrArgs{
							ServiceConnectArgs: ServiceConnectArgs{
								Alias: aws.String("some alias"),
							},
						},
					},
				},
				Workload: Workload{
					Name: aws.String("api"),
				},
			},
			wantedError: fmt.Errorf(`cannot set "network.connect.alias" when no ports are exposed`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestRequestDrivenWebService_validate(t *testing.T) {
	testCases := map[string]struct {
		config RequestDrivenWebService

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"error if fail to validate image": {
			config: RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
								Location: aws.String("mockLocation"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "image": `,
		},
		"error if fail to validate instance": {
			config: RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
							},
						},
						Port: uint16P(80),
					},
					InstanceConfig: AppRunnerInstanceConfig{
						CPU:    nil,
						Memory: nil,
						Platform: PlatformArgsOrString{
							PlatformString: (*PlatformString)(aws.String("mockPlatform")),
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "platform": `,
		},
		"error if fail to validate network": {
			config: RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
							},
						},
						Port: uint16P(80),
					},
					Network: RequestDrivenWebServiceNetworkConfig{
						VPC: rdwsVpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: (*PlacementString)(aws.String("")),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "network": `,
		},
		"error if fail to validate observability": {
			config: RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: stringP("mockLocation"),
							},
						},
						Port: uint16P(80),
					},
					Observability: Observability{
						Tracing: aws.String("unknown-vendor"),
					},
				},
			},
			wantedErrorMsgPrefix: `validate "observability": `,
		},
		"error if name is not set": {
			config: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
							},
						},
						Port: uint16P(80),
					},
				},
			},
			wantedError: fmt.Errorf(`"name" must be specified`),
		},
		"error if placement is not private": {
			config: RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: stringP("mockLocation"),
							},
						},
						Port: uint16P(80),
					},
					Network: RequestDrivenWebServiceNetworkConfig{
						VPC: rdwsVpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`placement "public" is not supported for Request-Driven Web Service`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestWorkerService_validate(t *testing.T) {
	testImageConfig := ImageWithHealthcheck{
		Image: Image{
			ImageLocationOrBuild: ImageLocationOrBuild{
				Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
			},
		},
	}
	testCases := map[string]struct {
		config WorkerService

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"error if fail to validate image": {
			config: WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
								Location: aws.String("mockLocation"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "image": `,
		},
		"error if fail to validate sidecars": {
			config: WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					Sidecars: map[string]*SidecarConfig{
						"foo": {
							DependsOn: DependsOn{
								"foo": "bar",
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "sidecars[foo]": `,
		},
		"error if fail to validate network": {
			config: WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: (*PlacementString)(aws.String("")),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "network": `,
		},
		"error if fail to validate subscribe": {
			config: WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					Subscribe: SubscribeConfig{
						Topics: []TopicSubscription{
							{
								Name: aws.String("mockTopic"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "subscribe": `,
		},
		"error if fail to validate publish": {
			config: WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					PublishConfig: PublishConfig{
						Topics: []Topic{
							{},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "publish": `,
		},
		"error if fail to validate taskdef override": {
			config: WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					TaskDefOverrides: []OverrideRule{
						{
							Path: "Family",
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "taskdef_overrides[0]": `,
		},
		"error if name is not set": {
			config: WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
				},
			},
			wantedError: fmt.Errorf(`"name" must be specified`),
		},
		"error if fail to validate dependencies": {
			config: WorkerService{
				Workload: Workload{Name: aws.String("mockWorkload")},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					Sidecars: map[string]*SidecarConfig{
						"foo": {
							Image:     BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: map[string]string{"bar": "start"},
						},
						"bar": {
							Image:     BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: map[string]string{"foo": "start"},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate container dependencies: `,
		},
		"error if fail to validate windows": {
			config: WorkerService{
				Workload: Workload{Name: aws.String("mockName")},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("windows/amd64"))},
						Storage: Storage{Volumes: map[string]*Volume{
							"foo": {
								EFS: EFSConfigOrBool{
									Enabled: aws.Bool(true),
								},
								MountPointOpts: MountPointOpts{
									ContainerPath: aws.String("mockPath"),
								},
							},
						},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate Windows: `,
		},
		"error if fail to validate ARM": {
			config: WorkerService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("linux/arm64"))},
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot:         aws.Int(123),
								workloadType: manifestinfo.WorkerServiceType,
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate ARM: `,
		},
		"error if fail to validate deployment": {
			config: WorkerService{
				Workload: Workload{
					Name: aws.String("mockName"),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("linux/arm64"))},
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot:         aws.Int(123),
								workloadType: manifestinfo.WorkerServiceType,
							},
						},
					},
					DeployConfig: WorkerDeploymentConfig{
						DeploymentControllerConfig: DeploymentControllerConfig{
							Rolling: aws.String("mockName"),
						}},
				},
			},
			wantedErrorMsgPrefix: `validate "deployment":`,
		},
		"error if service connect is enabled without any port exposed": {
			config: WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: testImageConfig,
					Network: NetworkConfig{
						Connect: ServiceConnectBoolOrArgs{
							ServiceConnectArgs: ServiceConnectArgs{
								Alias: aws.String("some alias"),
							},
						},
					},
				},
				Workload: Workload{
					Name: aws.String("api"),
				},
			},
			wantedError: fmt.Errorf(`cannot set "network.connect.alias" when no ports are exposed`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestScheduledJob_validate(t *testing.T) {
	testImageConfig := ImageWithHealthcheck{
		Image: Image{
			ImageLocationOrBuild: ImageLocationOrBuild{
				Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
			},
		},
	}
	testCases := map[string]struct {
		config ScheduledJob

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"error if fail to validate image": {
			config: ScheduledJob{
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
								Location: aws.String("mockLocation"),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "image": `,
		},
		"error if fail to validate sidecars": {
			config: ScheduledJob{
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: testImageConfig,
					Sidecars: map[string]*SidecarConfig{
						"foo": {
							DependsOn: DependsOn{
								"foo": "bar",
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "sidecars[foo]": `,
		},
		"error if fail to validate network": {
			config: ScheduledJob{
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: testImageConfig,
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: (*PlacementString)(aws.String("")),
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "network": `,
		},
		"error if fail to validate on": {
			config: ScheduledJob{
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: testImageConfig,
					On:          JobTriggerConfig{},
				},
			},
			wantedErrorMsgPrefix: `validate "on": `,
		},
		"error if fail to validate publish config": {
			config: ScheduledJob{
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: testImageConfig,
					On: JobTriggerConfig{
						Schedule: aws.String("mockSchedule"),
					},
					PublishConfig: PublishConfig{
						Topics: []Topic{
							{},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "publish": `,
		},
		"error if fail to validate taskdef override": {
			config: ScheduledJob{
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: testImageConfig,
					On: JobTriggerConfig{
						Schedule: aws.String("mockSchedule"),
					},
					TaskDefOverrides: []OverrideRule{
						{
							Path: "Family",
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "taskdef_overrides[0]": `,
		},
		"error if name is not set": {
			config: ScheduledJob{
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: testImageConfig,
					On: JobTriggerConfig{
						Schedule: aws.String("mockSchedule"),
					},
				},
			},
			wantedError: fmt.Errorf(`"name" must be specified`),
		},
		"error if fail to validate dependencies": {
			config: ScheduledJob{
				Workload: Workload{Name: aws.String("mockWorkload")},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: testImageConfig,
					On: JobTriggerConfig{
						Schedule: aws.String("mockSchedule"),
					},
					Sidecars: map[string]*SidecarConfig{
						"foo": {
							Image:     BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: map[string]string{"bar": "start"},
						},
						"bar": {
							Image:     BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
							DependsOn: map[string]string{"foo": "start"},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate container dependencies: `,
		},
		"error if fail to validate windows": {
			config: ScheduledJob{
				Workload: Workload{Name: aws.String("mockName")},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: testImageConfig,
					On: JobTriggerConfig{
						Schedule: aws.String("mockSchedule"),
					},
					TaskConfig: TaskConfig{
						Platform: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("windows/amd64"))},
						Storage: Storage{Volumes: map[string]*Volume{
							"foo": {
								EFS: EFSConfigOrBool{
									Enabled: aws.Bool(true),
								},
								MountPointOpts: MountPointOpts{
									ContainerPath: aws.String("mockPath"),
								},
							},
						},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate Windows: `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestPipelineManifest_validate(t *testing.T) {
	testCases := map[string]struct {
		Pipeline Pipeline

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"error if name exceeds 100 characters": {
			Pipeline: Pipeline{
				Name: "12345678902234567890323456789042345678905234567890623456789072345678908234567890923456789010234567890",
			},
			wantedError: errors.New("pipeline name '12345678902234567890323456789042345678905234567890623456789072345678908234567890923456789010234567890' must be shorter than 100 characters"),
		},
		"should validate pipeline stages": {
			Pipeline: Pipeline{
				Name: "release",
				Stages: []PipelineStage{
					{
						Name: "test",
						PostDeployments: PrePostDeployments{
							"first_action": &PrePostDeployment{
								BuildspecPath: "copilot/pipelines/my-pipeline/buildspecs/migration.yml",
							},
						},
						TestCommands: []string{"testing", "testing123"},
					},
				},
			},
			wantedError: errors.New(`validate stage "test" for pipeline "release": must specify one, not both, of "post_deployments" and "test_commands"`),
		},
		"should validate buildspec exists for pre/post-deployments": {
			Pipeline: Pipeline{
				Name: "release",
				Stages: []PipelineStage{
					{
						Name: "test",
						PostDeployments: PrePostDeployments{
							"first_action": &PrePostDeployment{
								BuildspecPath: "copilot/pipelines/my-pipeline/buildspecs/migration.yml",
							},
							"second_action": &PrePostDeployment{},
						},
					},
				},
			},
			wantedError: errors.New(`validate stage "test" for pipeline "release": "buildspec" must be specified`),
		},
		"should validate pipeline deployments": {
			Pipeline: Pipeline{
				Name: "release",
				Stages: []PipelineStage{
					{
						Name: "test",
						Deployments: map[string]*Deployment{
							"frontend": {
								DependsOn: []string{"backend"},
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "deployments" for pipeline stage test:`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.Pipeline.Validate()

			switch {
			case tc.wantedError != nil:
				require.EqualError(t, gotErr, tc.wantedError.Error())
			case tc.wantedErrorMsgPrefix != "":
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
			default:
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestDeployments_validate(t *testing.T) {
	testCases := map[string]struct {
		in     Deployments
		wanted error
	}{
		"should return nil on empty deployments": {},
		"should return an error when a dependency does not exist": {
			in: map[string]*Deployment{
				"frontend": {
					DependsOn: []string{"backend"},
				},
			},
			wanted: errors.New("dependency deployment named 'backend' of 'frontend' does not exist"),
		},
		"should return nil when all dependencies are present": {
			in: map[string]*Deployment{
				"frontend": {
					DependsOn: []string{"backend"},
				},
				"backend": nil,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := tc.in.validate()

			if tc.wanted == nil {
				require.NoError(t, actual)
			} else {
				require.EqualError(t, actual, tc.wanted.Error())
			}
		})
	}
}

func TestImageWithPort_validate(t *testing.T) {
	testCases := map[string]struct {
		ImageWithPort ImageWithPort

		wantedError error
	}{
		"error if port is not specified": {
			ImageWithPort: ImageWithPort{
				Image: Image{
					ImageLocationOrBuild: ImageLocationOrBuild{
						Location: aws.String("mockLocation"),
					},
				},
			},
			wantedError: fmt.Errorf(`"port" must be specified`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.ImageWithPort.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestImage_validate(t *testing.T) {
	testCases := map[string]struct {
		Image Image

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"error if build and location both specified": {
			Image: Image{
				ImageLocationOrBuild: ImageLocationOrBuild{
					Build: BuildArgsOrString{
						BuildString: aws.String("mockBuild"),
					},
					Location: aws.String("mockLocation"),
				},
			},
			wantedError: fmt.Errorf(`must specify one of "build" and "location"`),
		},
		"error if neither build nor location is specified": {
			Image:       Image{},
			wantedError: fmt.Errorf(`must specify one of "build" and "location"`),
		},
		"error if fail to validate depends_on": {
			Image: Image{
				ImageLocationOrBuild: ImageLocationOrBuild{
					Location: aws.String("mockLocation"),
				},
				DependsOn: DependsOn{
					"foo": "bar",
				},
			},

			wantedErrorMsgPrefix: `validate "depends_on":`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.Image.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestDependsOn_validate(t *testing.T) {
	testCases := map[string]struct {
		in     DependsOn
		wanted error
	}{
		"should return an error if dependency status is invalid": {
			in: DependsOn{
				"foo": "bar",
			},
			wanted: errors.New("container dependency status must be one of START, COMPLETE, SUCCESS or HEALTHY"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRoutingRule_validate(t *testing.T) {
	testCases := map[string]struct {
		RoutingRule RoutingRule

		wantedErrorMsgPrefix string
		wantedError          error
	}{
		"error if one of allowed_source_ips is not valid": {
			RoutingRule: RoutingRule{
				Path: stringP("/"),
				AllowedSourceIps: []IPNet{
					IPNet("10.1.0.0/24"),
					IPNet("badIP"),
					IPNet("10.1.1.0/24"),
				},
			},
			wantedErrorMsgPrefix: `validate "allowed_source_ips[1]": `,
		},
		"error if protocol version is not valid": {
			RoutingRule: RoutingRule{
				Path:            stringP("/"),
				ProtocolVersion: aws.String("quic"),
			},
			wantedErrorMsgPrefix: `"version" field value 'quic' must be one of GRPC, HTTP1 or HTTP2`,
		},
		"error if path is missing": {
			RoutingRule: RoutingRule{
				ProtocolVersion: aws.String("GRPC"),
			},
			wantedErrorMsgPrefix: `"path" must be specified`,
		},
		"should not error if protocol version is not uppercase": {
			RoutingRule: RoutingRule{
				Path:            stringP("/"),
				ProtocolVersion: aws.String("gRPC"),
			},
		},
		"error if hosted zone set without alias": {
			RoutingRule: RoutingRule{
				Path:       stringP("/"),
				HostedZone: aws.String("ABCD1234"),
			},
			wantedErrorMsgPrefix: `"alias" must be specified if "hosted_zone" is specified`,
		},
		"error if one of alias is not valid": {
			RoutingRule: RoutingRule{
				Path: stringP("/"),
				Alias: Alias{
					AdvancedAliases: []AdvancedAlias{
						{
							HostedZone: aws.String("mockHostedZone"),
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "alias":`,
		},
		"error if fail to valiadte condition values per listener rule": {
			RoutingRule: RoutingRule{
				Path: stringP("/"),
				Alias: Alias{
					StringSliceOrString: StringSliceOrString{
						StringSlice: []string{
							"example.com",
							"v1.example.com",
							"v2.example.com",
							"v3.example.com",
							"v4.example.com",
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate condition values per listener rule: listener rule has more than five conditions example.com, v1.example.com, v2.example.com, v3.example.com and v4.example.com `),
		},
		"error if fail to validate condition values for advanced aliases": {
			RoutingRule: RoutingRule{
				Path: stringP("/"),
				Alias: Alias{
					AdvancedAliases: []AdvancedAlias{
						{
							Alias: aws.String("example.com"),
						},
						{
							Alias: aws.String("v1.example.com"),
						},
						{
							Alias: aws.String("v2.example.com"),
						},
						{
							Alias: aws.String("v3.example.com"),
						},
						{
							Alias: aws.String("v4.example.com"),
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate condition values per listener rule: listener rule has more than five conditions example.com, v1.example.com, v2.example.com, v3.example.com and v4.example.com `),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.RoutingRule.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestHTTP_validate(t *testing.T) {
	testCases := map[string]struct {
		HTTP                 HTTP
		wantedErrorMsgPrefix string
		wantedError          error
	}{
		"return if routing rule configuration is nil": {},
		"error if both target_container and targetContainer are specified": {
			HTTP: HTTP{
				Main: RoutingRule{
					Path:            stringP("/"),
					TargetContainer: aws.String("mockContainer"),
				},
				TargetContainerCamelCase: aws.String("mockContainer"),
			},
			wantedError: fmt.Errorf(`must specify one, not both, of "target_container" and "targetContainer"`),
		},
		"error if the main routing rule is invalid": {
			HTTP: HTTP{
				Main: RoutingRule{
					TargetContainer: aws.String("mockContainer"),
				},
			},
			wantedError: fmt.Errorf(`"path" must be specified`),
		},
		"error if the additional routing rule is invalid": {
			HTTP: HTTP{
				Main: RoutingRule{
					Path:            stringP("/"),
					TargetContainer: aws.String("mockContainer"),
				},
				AdditionalRoutingRules: []RoutingRule{
					{
						TargetContainer: aws.String("mockContainer"),
					},
				},
			},
			wantedError: fmt.Errorf(`validate "additional_rules[0]": "path" must be specified`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.HTTP.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestNetworkLoadBalancerConfiguration_validate(t *testing.T) {
	testCases := map[string]struct {
		nlb NetworkLoadBalancerConfiguration

		wantedErrorMsgPrefix string
		wantedError          error
	}{
		"success if empty": {
			nlb: NetworkLoadBalancerConfiguration{},
		},
		"error if port unspecified": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					TargetContainer: aws.String("main"),
				},
			},
			wantedError: fmt.Errorf(`"port" must be specified`),
		},
		"error if port unspecified in additional listeners": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port:            aws.String("80/tcp"),
					TargetContainer: aws.String("main"),
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						TargetContainer: aws.String("main"),
					},
				},
			},
			wantedError: fmt.Errorf(`validate "additional_listeners[0]": "port" must be specified`),
		},
		"error parsing port": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("sabotage/this/string"),
				},
			},
			wantedErrorMsgPrefix: `validate "nlb": `,
			wantedError:          fmt.Errorf(`validate "port": cannot parse port mapping from sabotage/this/string`),
		},
		"error parsing port for additional listeners": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("80/tcp"),
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("81/tcp"),
					},
					{
						Port: aws.String("sabotage/this/string"),
					},
				},
			},
			wantedErrorMsgPrefix: `validate "nlb": `,
			wantedError:          fmt.Errorf(`validate "additional_listeners[1]": validate "port": cannot parse port mapping from sabotage/this/string`),
		},
		"success if port is specified without protocol": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443"),
				},
			},
		},
		"success if port is specified without protocol in additional listeners": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tcp"),
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("443"),
					},
				},
			},
		},
		"fail if protocol is not recognized": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tps"),
				},
			},
			wantedErrorMsgPrefix: `validate "nlb": `,
			wantedError:          fmt.Errorf(`validate "port": invalid protocol tps; valid protocols include TCP, UDP and TLS`),
		},
		"fail if protocol is not recognized in additional listeners": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tcp"),
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("443/tps"),
					},
				},
			},
			wantedErrorMsgPrefix: `validate "nlb": `,
			wantedError:          fmt.Errorf(`validate "additional_listeners[0]": validate "port": invalid protocol tps; valid protocols include TCP, UDP and TLS`),
		},
		"success if tcp": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tcp"),
				},
			},
		},
		"success if udp": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("161/udp"),
				},
			},
		},
		"success if udp in additional listeners": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("161/tcp"),
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("161/udp"),
					},
				},
			},
		},
		"error if additional listeners are defined before main listener": {
			nlb: NetworkLoadBalancerConfiguration{
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("161/udp"),
					},
				},
			},
			wantedError: fmt.Errorf(`"port" must be specified`),
		},
		"success if tls": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tls"),
				},
			},
		},
		"success if tls in additional listeners": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tcp"),
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("443/tls"),
					},
				},
			},
		},
		"error if tcp_udp": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/TCP_udp"),
				},
			},
			wantedError: fmt.Errorf(`validate "port": invalid protocol TCP_udp; valid protocols include TCP, UDP and TLS`),
		},
		"error if tcp_udp in additional listeners": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tcp"),
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("443/TCP_udp"),
					},
				},
			},
			wantedError: fmt.Errorf(`validate "additional_listeners[0]": validate "port": invalid protocol TCP_udp; valid protocols include TCP, UDP and TLS`),
		},
		"error if hosted zone is set": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tcp"),
				},
				Aliases: Alias{
					AdvancedAliases: []AdvancedAlias{
						{
							Alias:      aws.String("mockAlias"),
							HostedZone: aws.String("mockHostedZone"),
						},
					},
				},
			},
			wantedError: fmt.Errorf(`"hosted_zone" is not supported for Network Load Balancer`),
		},
		"error if hosted zone is set in additional listeners": {
			nlb: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("443/tcp"),
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("80/tcp"),
					},
				},
				Aliases: Alias{
					AdvancedAliases: []AdvancedAlias{
						{
							Alias:      aws.String("mockAlias"),
							HostedZone: aws.String("mockHostedZone"),
						},
					},
				},
			},
			wantedError: fmt.Errorf(`"hosted_zone" is not supported for Network Load Balancer`),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.nlb.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestAdvancedAlias_validate(t *testing.T) {
	testCases := map[string]struct {
		in     AdvancedAlias
		wanted error
	}{
		"should return an error if name is not specified": {
			in: AdvancedAlias{
				HostedZone: aws.String("ABCD123"),
			},
			wanted: errors.New(`"name" must be specified`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIPNet_validate(t *testing.T) {
	testCases := map[string]struct {
		in     IPNet
		wanted error
	}{
		"should return an error if IPNet is not valid": {
			in:     IPNet("badIPNet"),
			wanted: errors.New("parse IPNet badIPNet: invalid CIDR address: badIPNet"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTaskConfig_validate(t *testing.T) {
	perc := Percentage(70)
	mockConfig := ScalingConfigOrT[Percentage]{
		Value: &perc,
	}
	testCases := map[string]struct {
		TaskConfig TaskConfig

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"error if fail to validate platform": {
			TaskConfig: TaskConfig{
				Platform: PlatformArgsOrString{
					PlatformString: (*PlatformString)(aws.String("")),
				},
			},
			wantedErrorMsgPrefix: `validate "platform": `,
		},
		"error if fail to validate count": {
			TaskConfig: TaskConfig{
				Count: Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(123),
						CPU:  mockConfig,
					},
				},
			},
			wantedErrorMsgPrefix: `validate "count": `,
		},
		"error if fail to validate storage": {
			TaskConfig: TaskConfig{
				Storage: Storage{
					Volumes: map[string]*Volume{
						"foo": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID:          aws.Uint32(123),
									FileSystemID: StringOrFromCFN{Plain: aws.String("mock-ID")},
								},
							},
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "storage": `,
		},
		"error if invalid env file": {
			TaskConfig: TaskConfig{
				EnvFile: aws.String("foo"),
			},
			wantedError: fmt.Errorf("environment file foo must have a .env file extension"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.TaskConfig.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestPlatformArgsOrString_validate(t *testing.T) {
	testCases := map[string]struct {
		in     PlatformArgsOrString
		wanted error
	}{
		"error if platform string is invalid": {
			in:     PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("foobar/amd64"))},
			wanted: fmt.Errorf("platform 'foobar/amd64' is invalid; valid platforms are: linux/amd64, linux/x86_64, linux/arm, linux/arm64, windows/amd64 and windows/x86_64"),
		},
		"error if only half of platform string is specified": {
			in:     PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("linux"))},
			wanted: fmt.Errorf("platform 'linux' must be in the format [OS]/[Arch]"),
		},
		"error if only osfamily is specified": {
			in: PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("linux"),
				},
			},
			wanted: fmt.Errorf(`fields "osfamily" and "architecture" must either both be specified or both be empty`),
		},
		"error if only architecture is specified": {
			in: PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					Arch: aws.String("X86_64"),
				},
			},
			wanted: fmt.Errorf(`fields "osfamily" and "architecture" must either both be specified or both be empty`),
		},
		"error if osfamily is invalid": {
			in: PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("foo"),
					Arch:     aws.String("amd64"),
				},
			},
			wanted: fmt.Errorf("platform pair ('foo', 'amd64') is invalid: fields ('osfamily', 'architecture') must be one of ('linux', 'x86_64'), ('linux', 'amd64'), ('linux', 'arm'), ('linux', 'arm64'), ('windows', 'x86_64'), ('windows', 'amd64'), ('windows_server_2019_core', 'x86_64'), ('windows_server_2019_core', 'amd64'), ('windows_server_2019_full', 'x86_64'), ('windows_server_2019_full', 'amd64'), ('windows_server_2022_core', 'x86_64'), ('windows_server_2022_core', 'amd64'), ('windows_server_2022_full', 'x86_64'), ('windows_server_2022_full', 'amd64')"),
		},
		"error if arch is invalid": {
			in: PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("linux"),
					Arch:     aws.String("bar"),
				},
			},
			wanted: fmt.Errorf("platform pair ('linux', 'bar') is invalid: fields ('osfamily', 'architecture') must be one of ('linux', 'x86_64'), ('linux', 'amd64'), ('linux', 'arm'), ('linux', 'arm64'), ('windows', 'x86_64'), ('windows', 'amd64'), ('windows_server_2019_core', 'x86_64'), ('windows_server_2019_core', 'amd64'), ('windows_server_2019_full', 'x86_64'), ('windows_server_2019_full', 'amd64'), ('windows_server_2022_core', 'x86_64'), ('windows_server_2022_core', 'amd64'), ('windows_server_2022_full', 'x86_64'), ('windows_server_2022_full', 'amd64')"),
		},
		"return nil if platform string valid": {
			in: PlatformArgsOrString{PlatformString: (*PlatformString)(aws.String("linux/amd64"))},
		},
		"return nil if platform args valid": {
			in: PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("linux"),
					Arch:     aws.String("amd64"),
				},
			},
			wanted: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScalingConfigOrT_validate(t *testing.T) {

	var (
		time = 60 * time.Second
		perc = Percentage(70)
	)

	testCases := map[string]struct {
		ScalingConfig ScalingConfigOrT[Percentage]

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"valid if only value is specified": {
			ScalingConfig: ScalingConfigOrT[Percentage]{
				Value: &perc,
			},
		},
		"valid if only scaling config is specified": {
			ScalingConfig: ScalingConfigOrT[Percentage]{
				ScalingConfig: AdvancedScalingConfig[Percentage]{
					Value: &perc,
					Cooldown: Cooldown{
						ScaleInCooldown:  &time,
						ScaleOutCooldown: &time,
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.ScalingConfig.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestAdvancedCount_validate(t *testing.T) {
	var (
		perc        = Percentage(70)
		invalidPerc = Percentage(-1)
		timeMinute  = time.Second * 60
		mockConfig  = ScalingConfigOrT[Percentage]{
			Value: &perc,
		}
		invalidConfig = ScalingConfigOrT[Percentage]{
			Value: &invalidPerc,
		}
		mockCooldown = Cooldown{
			ScaleInCooldown: &timeMinute,
		}
		mockAdvancedInvConfig = ScalingConfigOrT[Percentage]{
			ScalingConfig: AdvancedScalingConfig[Percentage]{
				Value:    &invalidPerc,
				Cooldown: mockCooldown,
			},
		}
	)
	testCases := map[string]struct {
		AdvancedCount AdvancedCount

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"error if invalid autoscaling fields set": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				CPU: mockConfig,
				QueueScaling: QueueScaling{
					AcceptableLatency: durationp(10 * time.Second),
					AvgProcessingTime: durationp(1 * time.Second),
				},
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedError: fmt.Errorf(`autoscaling field "queue_delay" is invalid with workload type Load Balanced Web Service`),
		},
		"error if multiple invalid autoscaling fields set": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				CPU: mockConfig,
				QueueScaling: QueueScaling{
					AcceptableLatency: durationp(10 * time.Second),
					AvgProcessingTime: durationp(1 * time.Second),
				},
				Requests: ScalingConfigOrT[int]{
					Value: aws.Int(10),
				},
				ResponseTime: ScalingConfigOrT[time.Duration]{
					Value: &timeMinute,
				},
				workloadType: manifestinfo.WorkerServiceType,
			},
			wantedError: fmt.Errorf(`autoscaling fields "requests" and "response_time" are invalid with workload type Worker Service`),
		},
		"cannot have autoscaling for scheduled jobs": {
			AdvancedCount: AdvancedCount{
				Spot:         aws.Int(42),
				workloadType: manifestinfo.ScheduledJobType,
			},
			wantedError: errors.New("cannot have autoscaling options for workloads of type 'Scheduled Job'"),
		},
		"valid if only spot is specified": {
			AdvancedCount: AdvancedCount{
				Spot:         aws.Int(42),
				workloadType: manifestinfo.BackendServiceType,
			},
		},
		"valid when range and and at least one autoscaling fields are specified": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				CPU: mockConfig,
				QueueScaling: QueueScaling{
					AcceptableLatency: durationp(10 * time.Second),
					AvgProcessingTime: durationp(1 * time.Second),
				},
				workloadType: manifestinfo.WorkerServiceType,
			},
		},
		"error if both spot and autoscaling fields are specified": {
			AdvancedCount: AdvancedCount{
				Spot:         aws.Int(123),
				CPU:          mockConfig,
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedError: fmt.Errorf(`must specify one, not both, of "spot" and "range/cpu_percentage/memory_percentage/requests/response_time"`),
		},
		"error if fail to validate range": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("")),
				},
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedErrorMsgPrefix: `validate "range": `,
		},
		"error if range is missing when autoscaling fields are set for Load Balanced Web Service": {
			AdvancedCount: AdvancedCount{
				Requests: ScalingConfigOrT[int]{
					Value: aws.Int(123),
				},
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedError: fmt.Errorf(`"range" must be specified if "cpu_percentage", "memory_percentage", "requests" or "response_time" are specified`),
		},
		"error if range is specified but no autoscaling fields are specified for a Load Balanced Web Service": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage", "memory_percentage", "requests" or "response_time" if "range" is specified`),
		},
		"error if range is specified but no autoscaling fields are specified for a Backend Service": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				workloadType: manifestinfo.BackendServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage", "memory_percentage", "requests" or "response_time" if "range" is specified`),
		},
		"error if range is specified but no autoscaling fields are specified for a Worker Service": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				workloadType: manifestinfo.WorkerServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage", "memory_percentage" or "queue_delay" if "range" is specified`),
		},
		"error if cooldown is specified but no autoscaling fields are specified for a Load Balanced Web Service": {
			AdvancedCount: AdvancedCount{
				Cooldown:     mockCooldown,
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage", "memory_percentage", "requests" or "response_time" if "cooldown" is specified`),
		},
		"error if cooldown is specified but no autoscaling fields are specified for a Backend Service": {
			AdvancedCount: AdvancedCount{
				Cooldown:     mockCooldown,
				workloadType: manifestinfo.BackendServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage", "memory_percentage", "requests" or "response_time" if "cooldown" is specified`),
		},
		"error if cooldown is specified but no autoscaling fields are specified for a Worker Service": {
			AdvancedCount: AdvancedCount{
				Cooldown:     mockCooldown,
				workloadType: manifestinfo.WorkerServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage", "memory_percentage" or "queue_delay" if "cooldown" is specified`),
		},
		"error if range is missing when autoscaling fields are set for Backend Service": {
			AdvancedCount: AdvancedCount{
				CPU:          mockConfig,
				workloadType: manifestinfo.BackendServiceType,
			},
			wantedError: fmt.Errorf(`"range" must be specified if "cpu_percentage", "memory_percentage", "requests" or "response_time" are specified`),
		},
		"error if range is missing when autoscaling fields are set for Worker Service": {
			AdvancedCount: AdvancedCount{
				CPU:          mockConfig,
				workloadType: manifestinfo.WorkerServiceType,
			},
			wantedError: fmt.Errorf(`"range" must be specified if "cpu_percentage", "memory_percentage" or "queue_delay" are specified`),
		},
		"wrap error from queue_delay on failure": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					RangeConfig: RangeConfig{
						Min:      aws.Int(1),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(6),
					},
				},
				QueueScaling: QueueScaling{
					AcceptableLatency: nil,
					AvgProcessingTime: durationp(1 * time.Second),
				},
				workloadType: manifestinfo.WorkerServiceType,
			},
			wantedErrorMsgPrefix: `validate "queue_delay": `,
		},
		"error if CPU config is not valid": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(stringP("1-2")),
				},
				CPU:          invalidConfig,
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedErrorMsgPrefix: `validate "cpu_percentage": `,
		},
		"error if CPU advanced config is not valid": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(stringP("1-2")),
				},
				CPU:          mockAdvancedInvConfig,
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedErrorMsgPrefix: `validate "cpu_percentage": `,
		},
		"error if memory config is not valid": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(stringP("1-2")),
				},
				Memory:       invalidConfig,
				workloadType: manifestinfo.LoadBalancedWebServiceType,
			},
			wantedErrorMsgPrefix: `validate "memory_percentage": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.AdvancedCount.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestPercentage_validate(t *testing.T) {
	testCases := map[string]struct {
		in     Percentage
		wanted error
	}{
		"should return an error if percentage is not valid": {
			in:     Percentage(120),
			wanted: errors.New("percentage value 120 must be an integer from 0 to 100"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestQueueScaling_validate(t *testing.T) {
	testCases := map[string]struct {
		in     QueueScaling
		wanted error
	}{
		"should return an error if only msg_processing_time is specified": {
			in: QueueScaling{
				AvgProcessingTime: durationp(1 * time.Second),
			},
			wanted: errors.New(`"acceptable_latency" must be specified if "msg_processing_time" is specified`),
		},
		"should return an error if only acceptable_latency is specified": {
			in: QueueScaling{
				AcceptableLatency: durationp(1 * time.Second),
			},
			wanted: errors.New(`"msg_processing_time" must be specified if "acceptable_latency" is specified`),
		},
		"should return an error if the msg_processing_time is 0": {
			in: QueueScaling{
				AcceptableLatency: durationp(1 * time.Second),
				AvgProcessingTime: durationp(0 * time.Second),
			},
			wanted: errors.New(`"msg_processing_time" cannot be 0`),
		},
		"should return an error if the msg_processing_time is longer than acceptable_latency": {
			in: QueueScaling{
				AcceptableLatency: durationp(500 * time.Millisecond),
				AvgProcessingTime: durationp(1 * time.Second),
			},
			wanted: errors.New(`"msg_processing_time" cannot be longer than "acceptable_latency"`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIntRangeBand_validate(t *testing.T) {
	testCases := map[string]struct {
		IntRangeBand IntRangeBand

		wantedError error
	}{
		"error if range value is in invalid format": {
			IntRangeBand: IntRangeBand(*aws.String("")),
			wantedError:  fmt.Errorf("invalid range value : valid format is ${min}-${max}"),
		},
		"error if range min is greater than max": {
			IntRangeBand: IntRangeBand(*aws.String("6-4")),
			wantedError:  fmt.Errorf("min value 6 cannot be greater than max value 4"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.IntRangeBand.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestRangeConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		RangeConfig RangeConfig

		wantedError error
	}{
		"error if max is not set": {
			RangeConfig: RangeConfig{
				Min: aws.Int(2),
			},
			wantedError: fmt.Errorf(`"min/max" must be specified`),
		},
		"error if range min is greater than max": {
			RangeConfig: RangeConfig{
				Min: aws.Int(2),
				Max: aws.Int(1),
			},
			wantedError: fmt.Errorf("min value 2 cannot be greater than max value 1"),
		},
		"error if spot_from value is negative": {
			RangeConfig: RangeConfig{
				Min:      aws.Int(2),
				Max:      aws.Int(10),
				SpotFrom: aws.Int(-3),
			},
			wantedError: fmt.Errorf("min value 2, max value 10, and spot_from value -3 must all be positive"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.RangeConfig.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestStorage_validate(t *testing.T) {
	testCases := map[string]struct {
		Storage Storage

		wantedErrorMsgPrefix string
		wantedError          error
	}{
		"error if ephemeral is invalid": {
			Storage: Storage{
				Ephemeral: aws.Int(19),
			},
			wantedError: fmt.Errorf(`validate "ephemeral": ephemeral storage must be between 20 GiB and 200 GiB`),
		},
		"error if fail to validate volumes": {
			Storage: Storage{
				Volumes: map[string]*Volume{
					"foo": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
					},
				},
			},
			wantedErrorMsgPrefix: `validate "volumes[foo]": `,
		},
		"error if storage has more than one managed volume": {
			Storage: Storage{
				Volumes: map[string]*Volume{
					"foo": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
					"bar": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
				},
			},
			wantedError: fmt.Errorf("cannot specify more than one managed volume per service"),
		},
		"valid": {
			Storage: Storage{
				Volumes: map[string]*Volume{
					"foo": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
					"bar": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(false),
						},
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
					"foobar": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								FileSystemID: StringOrFromCFN{Plain: aws.String("fs-1234567")},
							},
						},
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.Storage.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			if tc.wantedErrorMsgPrefix != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestVolume_validate(t *testing.T) {
	testCases := map[string]struct {
		Volume Volume

		wantedErrorPrefix string
	}{
		"error if fail to validate efs": {
			Volume: Volume{
				EFS: EFSConfigOrBool{
					Advanced: EFSVolumeConfiguration{
						UID:           aws.Uint32(123),
						RootDirectory: aws.String("mockDir"),
					},
				},
			},
			wantedErrorPrefix: `validate "efs": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.Volume.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEFSVolumeConfiguration_validate(t *testing.T) {
	testCases := map[string]struct {
		EFSVolumeConfiguration EFSVolumeConfiguration

		wantedError error
	}{
		"error if uid/gid are specified with id/root_dir/auth": {
			EFSVolumeConfiguration: EFSVolumeConfiguration{
				UID:        aws.Uint32(123),
				AuthConfig: AuthorizationConfig{IAM: aws.Bool(true)},
			},
			wantedError: fmt.Errorf(`must specify one, not both, of "uid/gid" and "id/root_dir/auth"`),
		},
		"error if uid is set but gid is not": {
			EFSVolumeConfiguration: EFSVolumeConfiguration{
				UID: aws.Uint32(123),
			},
			wantedError: fmt.Errorf(`"gid" must be specified if "uid" is specified`),
		},
		"error if gid is set but uid is not": {
			EFSVolumeConfiguration: EFSVolumeConfiguration{
				GID: aws.Uint32(123),
			},
			wantedError: fmt.Errorf(`"uid" must be specified if "gid" is specified`),
		},
		"error if uid is 0": {
			EFSVolumeConfiguration: EFSVolumeConfiguration{
				UID: aws.Uint32(0),
				GID: aws.Uint32(0),
			},
			wantedError: fmt.Errorf(`"uid" must not be 0`),
		},
		"error if AuthorizationConfig is not configured correctly": {
			EFSVolumeConfiguration: EFSVolumeConfiguration{
				AuthConfig: AuthorizationConfig{
					AccessPointID: aws.String("mockID"),
				},
				RootDirectory: aws.String("mockDir"),
			},
			wantedError: fmt.Errorf(`"root_dir" must be either empty or "/" and "auth.iam" must be true when "access_point_id" is used`),
		},
		"error if root_dir is invalid": {
			EFSVolumeConfiguration: EFSVolumeConfiguration{
				RootDirectory: aws.String("!!!!"),
			},
			wantedError: fmt.Errorf(`validate "root_dir": path can only contain the characters a-zA-Z0-9.-_/`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.EFSVolumeConfiguration.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestSidecarConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		config SidecarConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate image": {
			config:            SidecarConfig{},
			wantedErrorPrefix: `must specify one of "image", "image.build, or "image.location"`,
		},
		"error if fail to validate image build": {
			config: SidecarConfig{
				Image: AdvancedToUnion[*string](ImageLocationOrBuild{
					Build: BuildArgsOrString{
						BuildString: aws.String("mockDockerfile"),
					},
					Location: aws.String("mockimage:tag"),
				}),
			},
			wantedErrorPrefix: `validate "image": `,
		},
		"error if fail to validate mount_points": {
			config: SidecarConfig{
				Image: BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
				MountPoints: []SidecarMountPoint{
					{},
				},
			},
			wantedErrorPrefix: `validate "mount_points[0]": `,
		},
		"error if fail to validate depends_on": {
			config: SidecarConfig{
				Image: BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
				DependsOn: DependsOn{
					"foo": "bar",
				},
			},
			wantedErrorPrefix: `validate "depends_on": `,
		},
		"error if invalid env file": {
			config: SidecarConfig{
				Image:   BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
				EnvFile: aws.String("foo"),
			},
			wantedErrorPrefix: `environment file foo must`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestSidecarMountPoint_validate(t *testing.T) {
	testCases := map[string]struct {
		in     SidecarMountPoint
		wanted error
	}{
		"should return an error if source_volume is not set": {
			in:     SidecarMountPoint{},
			wanted: errors.New(`"source_volume" must be specified`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMountPointOpts_validate(t *testing.T) {
	testCases := map[string]struct {
		in     MountPointOpts
		wanted error
	}{
		"should return an error if path is not set": {
			in:     MountPointOpts{},
			wanted: errors.New(`"path" must be specified`),
		},
		"should return an error if path is invalid": {
			in: MountPointOpts{
				ContainerPath: aws.String("!!!!!!"),
			},
			wanted: errors.New(`validate "path": path can only contain the characters a-zA-Z0-9.-_/`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNetworkConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		config NetworkConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate vpc": {
			config: NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{
						PlacementString: (*PlacementString)(aws.String("")),
					},
				},
			},
			wantedErrorPrefix: `validate "vpc": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestRequestDrivenWebServiceNetworkConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		config RequestDrivenWebServiceNetworkConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate vpc": {
			config: RequestDrivenWebServiceNetworkConfig{
				VPC: rdwsVpcConfig{
					Placement: PlacementArgOrString{
						PlacementString: (*PlacementString)(aws.String("")),
					},
				},
			},
			wantedErrorPrefix: `validate "vpc": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestRdwsVpcConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		config rdwsVpcConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate placement": {
			config: rdwsVpcConfig{
				Placement: PlacementArgOrString{
					PlacementString: (*PlacementString)(aws.String("")),
				},
			},
			wantedErrorPrefix: `validate "placement": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestVpcConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		config vpcConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate placement": {
			config: vpcConfig{
				Placement: PlacementArgOrString{
					PlacementString: (*PlacementString)(aws.String("")),
				},
			},
			wantedErrorPrefix: `validate "placement": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestPlacementString_validate(t *testing.T) {
	mockEmptyPlacement := PlacementString("")
	mockInvalidPlacement := PlacementString("external")
	testCases := map[string]struct {
		in     *PlacementString
		wanted error
	}{
		"should return an error if placement is empty": {
			in:     &mockEmptyPlacement,
			wanted: errors.New(`"placement" cannot be empty`),
		},
		"should return an error if placement is invalid": {
			in:     &mockInvalidPlacement,
			wanted: errors.New(`"placement" external must be one of public, private`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAppRunnerInstanceConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		config            AppRunnerInstanceConfig
		wantedErrorPrefix string
		wantedError       error
	}{
		"error if fail to validate platforms": {
			config: AppRunnerInstanceConfig{
				Platform: PlatformArgsOrString{
					PlatformString: (*PlatformString)(aws.String("")),
				},
			},
			wantedErrorPrefix: `validate "platform": `,
		},
		"error if windows os in PlatformString": {
			config: AppRunnerInstanceConfig{
				Platform: PlatformArgsOrString{
					PlatformString: (*PlatformString)(aws.String("windows/amd64")),
				},
			},
			wantedError: fmt.Errorf("Windows is not supported for App Runner services"),
		},
		"error if windows os in PlatformArgs": {
			config: AppRunnerInstanceConfig{
				CPU:    nil,
				Memory: nil,
				Platform: PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("windows"),
						Arch:     aws.String("amd64"),
					},
				},
			},
			wantedError: fmt.Errorf("Windows is not supported for App Runner services"),
		},
		"error if invalid arch in PlatformString": {
			config: AppRunnerInstanceConfig{
				Platform: PlatformArgsOrString{
					PlatformArgs: PlatformArgs{
						OSFamily: aws.String("linux"),
						Arch:     aws.String("leg64"),
					},
				},
			},
			wantedError: fmt.Errorf("validate \"platform\": platform pair ('linux', 'leg64') is invalid: fields ('osfamily', 'architecture') must be one of ('linux', 'x86_64'), ('linux', 'amd64'), ('linux', 'arm'), ('linux', 'arm64'), ('windows', 'x86_64'), ('windows', 'amd64'), ('windows_server_2019_core', 'x86_64'), ('windows_server_2019_core', 'amd64'), ('windows_server_2019_full', 'x86_64'), ('windows_server_2019_full', 'amd64'), ('windows_server_2022_core', 'x86_64'), ('windows_server_2022_core', 'amd64'), ('windows_server_2022_full', 'x86_64'), ('windows_server_2022_full', 'amd64')"),
		},
		"error if App Runner + ARM": {
			config: AppRunnerInstanceConfig{
				Platform: PlatformArgsOrString{
					PlatformString: (*PlatformString)(aws.String("linux/arm64")),
				},
			},
			wantedError: fmt.Errorf("App Runner services can only build on amd64 and x86_64 architectures"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestObservability_validate(t *testing.T) {
	testCases := map[string]struct {
		config            Observability
		wantedErrorPrefix string
	}{
		"error if tracing has invalid vendor": {
			config: Observability{
				Tracing: aws.String("unknown-vendor"),
			},
			wantedErrorPrefix: `invalid tracing vendor unknown-vendor: `,
		},
		"ok if tracing is aws-xray": {
			config: Observability{
				Tracing: aws.String("awsxray"),
			},
		},
		"ok if observability is empty": {
			config: Observability{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.NotNil(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestJobTriggerConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		in     *JobTriggerConfig
		wanted error
	}{
		"should return an error if schedule is empty": {
			in:     &JobTriggerConfig{},
			wanted: errors.New(`"schedule" must be specified`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPublishConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		config PublishConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate topics": {
			config: PublishConfig{
				Topics: []Topic{
					{},
				},
			},
			wantedErrorPrefix: `validate "topics[0]": `,
		},
		"error if empty topic name": {
			config: PublishConfig{
				Topics: []Topic{
					{
						Name: aws.String(""),
					},
				},
			},
			wantedErrorPrefix: `validate "topics[0]": `,
		},
		"error if invalid topic name": {
			config: PublishConfig{
				Topics: []Topic{
					{
						Name: aws.String("mytopic.lifo"),
					},
				},
			},
			wantedErrorPrefix: `validate "topics[0]": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestTopic_validate(t *testing.T) {
	testCases := map[string]struct {
		in     Topic
		wanted error
	}{
		"should return an error if name is empty": {
			in:     Topic{},
			wanted: errors.New(`"name" must be specified`),
		},
		"should return an error if name is not valid": {
			in: Topic{
				Name: aws.String("!@#"),
			},
			wanted: errors.New(`"name" can only contain letters, numbers, underscores, and hyphens`),
		},
		"should not return an error if name is valid": {
			in: Topic{
				Name: aws.String("validtopic"),
			},
			wanted: nil,
		},
		"should not return an error if name is valid with fifo enabled": {
			in: Topic{
				Name: aws.String("validtopic"),
				FIFO: FIFOTopicAdvanceConfigOrBool{
					Enable: aws.Bool(true),
				},
			},
			wanted: nil,
		},
		"should not return an error if name is valid with advanced fifo config": {
			in: Topic{
				Name: aws.String("validtopic"),
				FIFO: FIFOTopicAdvanceConfigOrBool{
					Advanced: FIFOTopicAdvanceConfig{
						ContentBasedDeduplication: aws.Bool(true),
					},
				},
			},
			wanted: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSubscribeConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		config SubscribeConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate topics": {
			config: SubscribeConfig{
				Topics: []TopicSubscription{
					{
						Name: aws.String("mockTopic"),
					},
				},
			},
			wantedErrorPrefix: `validate "topics[0]": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestTopicSubscription_validate(t *testing.T) {
	duration111Seconds := 111 * time.Second
	testCases := map[string]struct {
		in     TopicSubscription
		wanted error
	}{
		"should return an error if topic name is empty": {
			in:     TopicSubscription{},
			wanted: errors.New(`"name" must be specified`),
		},
		"should return an error if service is empty": {
			in: TopicSubscription{
				Name: aws.String("mockTopic"),
			},
			wanted: errors.New(`"service" must be specified`),
		},
		"should return an error if service is in invalid format": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("!!!!!"),
			},
			wanted: errors.New("service name must start with a letter, contain only lower-case letters, numbers, and hyphens, and have no consecutive or trailing hyphen"),
		},
		"should not return an error if service is in valid format": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
			},
			wanted: nil,
		},
		"should not return error if standard queue is enabled": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Enabled: aws.Bool(true),
				},
			},
			wanted: nil,
		},
		"should not return error if fifo queue is enabled": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						FIFO: FIFOAdvanceConfigOrBool{
							Enable: aws.Bool(true),
						},
					},
				},
			},
			wanted: nil,
		},
		"should return error if invalid fifo throughput limit": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						FIFO: FIFOAdvanceConfigOrBool{
							Advanced: FIFOAdvanceConfig{
								FIFOThroughputLimit: aws.String("incorrectFIFOThoughoutLimit"),
							},
						},
					},
				},
			},
			wanted: errors.New(`validate "queue": validate "throughput_limit": fifo throughput limit value must be one of perMessageGroupId or perQueue`),
		},
		"should not return error if valid fifo throughput limit": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						FIFO: FIFOAdvanceConfigOrBool{
							Advanced: FIFOAdvanceConfig{
								FIFOThroughputLimit: aws.String(sqsFIFOThroughputLimitPerMessageGroupID),
							},
						},
					},
				},
			},
			wanted: nil,
		},
		"should return error if invalid deduplicate scope": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						FIFO: FIFOAdvanceConfigOrBool{
							Advanced: FIFOAdvanceConfig{
								DeduplicationScope: aws.String("incorrectDeduplicateScope"),
							},
						},
					},
				},
			},
			wanted: errors.New(`validate "queue": validate "deduplication_scope": deduplication scope value must be one of messageGroup or queue`),
		},
		"should not return error if valid deduplicate scope": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						FIFO: FIFOAdvanceConfigOrBool{
							Advanced: FIFOAdvanceConfig{
								DeduplicationScope: aws.String(sqsDeduplicationScopeMessageGroup),
							},
						},
					},
				},
			},
			wanted: nil,
		},
		"should return error if high_throughput is defined along with deduplication_scope": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						FIFO: FIFOAdvanceConfigOrBool{
							Advanced: FIFOAdvanceConfig{
								HighThroughputFifo: aws.Bool(true),
								DeduplicationScope: aws.String(sqsDeduplicationScopeMessageGroup),
							},
						},
					},
				},
			},
			wanted: errors.New(`validate "queue": must specify one, not both, of "high_throughput" and "deduplication_scope"`),
		},
		"should return error if high_throughput is defined along with  throughput_limit": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						FIFO: FIFOAdvanceConfigOrBool{
							Advanced: FIFOAdvanceConfig{
								HighThroughputFifo:  aws.Bool(true),
								FIFOThroughputLimit: aws.String(sqsFIFOThroughputLimitPerMessageGroupID),
							},
						},
					},
				},
			},
			wanted: errors.New(`validate "queue": must specify one, not both, of "high_throughput" and "throughput_limit"`),
		},
		"should return error if invalid combination of deduplication_scope and throughput_limit is defined": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						FIFO: FIFOAdvanceConfigOrBool{
							Advanced: FIFOAdvanceConfig{
								FIFOThroughputLimit: aws.String(sqsFIFOThroughputLimitPerMessageGroupID),
								DeduplicationScope:  aws.String(sqsDeduplicationScopeQueue),
							},
						},
					},
				},
			},
			wanted: errors.New(`validate "queue": "throughput_limit" must be set to "perQueue" when "deduplication_scope" is set to "queue"`),
		},
		"should not return error if valid standard queue config defined": {
			in: TopicSubscription{
				Name:    aws.String("mockTopic"),
				Service: aws.String("mockservice"),
				Queue: SQSQueueOrBool{
					Advanced: SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
					},
				},
			},
			wanted: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOverrideRule_validate(t *testing.T) {
	testCases := map[string]struct {
		in     OverrideRule
		wanted error
	}{
		"should return an error if override rule is invalid": {
			in: OverrideRule{
				Path: "ContainerDefinitions[1].Name",
			},
			wanted: errors.New(`"ContainerDefinitions\[\d+\].Name" cannot be overridden with a custom value`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateLoadBalancerTarget(t *testing.T) {
	testCases := map[string]struct {
		in     validateTargetContainerOpts
		wanted error
	}{
		"should return an error if target container doesn't exist": {
			in: validateTargetContainerOpts{
				mainContainerName: "mockMainContainer",
				targetContainer:   aws.String("foo"),
			},
			wanted: fmt.Errorf(`target container "foo" doesn't exist`),
		},
		"should return an error if target container doesn't expose a port": {
			in: validateTargetContainerOpts{
				mainContainerName: "mockMainContainer",
				targetContainer:   aws.String("foo"),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {},
				},
			},
			wanted: fmt.Errorf(`target container "foo" doesn't expose a port`),
		},
		"success with no target container set": {
			in: validateTargetContainerOpts{
				mainContainerName: "mockMainContainer",
				targetContainer:   nil,
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {},
				},
			},
		},
		"success": {
			in: validateTargetContainerOpts{
				mainContainerName: "mockMainContainer",
				targetContainer:   aws.String("foo"),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateTargetContainer(tc.in)

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateLogging(t *testing.T) {
	testCases := map[string]struct {
		in          Logging
		wantedError error
	}{
		"should return an error if env file has wrong extension": {
			in: Logging{
				EnvFile: aws.String("path/to/envFile.sh"),
			},
			wantedError: fmt.Errorf("environment file path/to/envFile.sh must have a .env file extension"),
		},
		"success": {
			in: Logging{
				EnvFile: aws.String("test.env"),
			},
			wantedError: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
				return
			}
			require.NoError(t, gotErr)
		})
	}
}

func TestValidateContainerDeps(t *testing.T) {
	testCases := map[string]struct {
		in     validateDependenciesOpts
		wanted error
	}{
		"should return an error if main container dependencies status is invalid": {
			in: validateDependenciesOpts{
				mainContainerName: "mockMainContainer",
				imageConfig: Image{
					DependsOn: DependsOn{
						"mockMainContainer": "complete",
					},
				},
			},
			wanted: fmt.Errorf("validate mockMainContainer container dependencies status: essential container mockMainContainer can only have status START or HEALTHY"),
		},
		"should return an error if sidecar container dependencies status is invalid": {
			in: validateDependenciesOpts{
				mainContainerName: "mockMainContainer",
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						DependsOn: DependsOn{
							"mockMainContainer": "success",
						},
					},
				},
			},
			wanted: fmt.Errorf("validate foo container dependencies status: essential container mockMainContainer can only have status START or HEALTHY"),
		},
		"should return an error if a main container dependency does not exist": {
			in: validateDependenciesOpts{
				mainContainerName: "mockMainContainer",
				imageConfig: Image{
					DependsOn: DependsOn{
						"foo": "healthy",
					},
				},
			},
			wanted: fmt.Errorf("container foo does not exist"),
		},
		"should return an error if a firelens container does not exist": {
			in: validateDependenciesOpts{
				mainContainerName: "mockMainContainer",
				imageConfig: Image{
					DependsOn: DependsOn{
						"firelens_log_router": "start",
					},
				},
			},
			wanted: fmt.Errorf("container firelens_log_router does not exist"),
		},
		"should return an error if a sidecar container dependency does not exist": {
			in: validateDependenciesOpts{
				mainContainerName: "mockMainContainer",
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						DependsOn: DependsOn{
							"bar": "healthy",
						},
					},
				},
			},
			wanted: fmt.Errorf("container bar does not exist"),
		},
		"should return an error if container depends on itself": {
			in: validateDependenciesOpts{
				mainContainerName: "mockMainContainer",
				imageConfig: Image{
					DependsOn: DependsOn{
						"mockMainContainer": "healthy",
					},
				},
			},
			wanted: fmt.Errorf("container mockMainContainer cannot depend on itself"),
		},
		"should return an error if container dependencies graph is cyclic": {
			in: validateDependenciesOpts{
				mainContainerName: "alpha",
				imageConfig: Image{
					DependsOn: DependsOn{
						"beta": "healthy",
					},
				},
				sidecarConfig: map[string]*SidecarConfig{
					"beta": {
						DependsOn: DependsOn{
							"gamma": "healthy",
						},
					},
					"gamma": {
						DependsOn: DependsOn{
							"alpha": "healthy",
						},
					},
					"zeta": {
						DependsOn: DependsOn{
							"alpha": "healthy",
						},
					},
				},
			},
			wanted: fmt.Errorf("circular container dependency chain includes the following containers: [alpha beta gamma]"),
		},
		"success": {
			in: validateDependenciesOpts{
				mainContainerName: "alpha",
				imageConfig: Image{
					DependsOn: DependsOn{
						"firelens_log_router": "start",
						"beta":                "complete",
					},
				},
				logging: Logging{
					Image: aws.String("foobar"),
				},
				sidecarConfig: map[string]*SidecarConfig{
					"beta": {
						Essential: aws.Bool(false),
						DependsOn: DependsOn{
							"firelens_log_router": "start",
						},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateContainerDeps(tc.in)

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateWindows(t *testing.T) {
	testCases := map[string]struct {
		in          validateWindowsOpts
		wantedError error
	}{
		"error if efs specified": {
			in: validateWindowsOpts{
				efsVolumes: map[string]*Volume{
					"someVolume": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
				},
			},
			wantedError: errors.New(`'EFS' is not supported when deploying a Windows container`),
		},
		"should return nil when no fields are specified": {
			in:          validateWindowsOpts{},
			wantedError: nil,
		},
		"error if readonlyfs is true": {
			in: validateWindowsOpts{
				readOnlyFS: aws.Bool(true),
			},
			wantedError: fmt.Errorf(`%q can not be set to 'true' when deploying a Windows container`, "readonly_fs"),
		},
		"should return nil if readonly_fs is false": {
			in: validateWindowsOpts{
				readOnlyFS: aws.Bool(false),
			},
			wantedError: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateWindows(tc.in)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateARM(t *testing.T) {
	testCases := map[string]struct {
		in          validateARMOpts
		wantedError error
	}{
		"should return an error if Spot specified inline": {
			in: validateARMOpts{
				Spot: aws.Int(2),
			},
			wantedError: fmt.Errorf(`'Fargate Spot' is not supported when deploying on ARM architecture`),
		},
		"should return an error if Spot specified with spot_from": {
			in: validateARMOpts{
				SpotFrom: aws.Int(2),
			},
			wantedError: fmt.Errorf(`'Fargate Spot' is not supported when deploying on ARM architecture`),
		},
		"should return nil if Spot not specified": {
			in: validateARMOpts{
				Spot: nil,
			},
			wantedError: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateARM(tc.in)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeploymentConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		deployConfig DeploymentConfig
		wanted       string
	}{
		"error if deploy config has invalid rolling strategy": {
			deployConfig: DeploymentConfig{
				DeploymentControllerConfig: DeploymentControllerConfig{
					Rolling: aws.String("unknown"),
				}},
			wanted: `invalid rolling deployment strategy "unknown", must be one of default or recreate`,
		},
		"ok if deployment strategy is recreate": {
			deployConfig: DeploymentConfig{
				DeploymentControllerConfig: DeploymentControllerConfig{
					Rolling: aws.String("recreate"),
				}},
		},
		"ok if deployment strategy is default": {
			deployConfig: DeploymentConfig{
				DeploymentControllerConfig: DeploymentControllerConfig{
					Rolling: aws.String("default"),
				}},
		},
		"ok if deployment is empty": {
			deployConfig: DeploymentConfig{},
		},
		"ok if deployment strategy is empty but alarm indicated": {
			deployConfig: DeploymentConfig{
				RollbackAlarms: BasicToUnion[[]string, AlarmArgs]([]string{"alarmName"})},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.deployConfig.validate()

			if tc.wanted != "" {
				require.NotNil(t, gotErr)
				require.Contains(t, gotErr.Error(), tc.wanted)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestFromEnvironment_validate(t *testing.T) {
	testCases := map[string]struct {
		in          fromCFN
		wantedError error
	}{
		"error if name is an empty string": {
			in: fromCFN{
				Name: aws.String(""),
			},
			wantedError: errors.New("name cannot be an empty string"),
		},
		"ok": {
			in: fromCFN{
				Name: aws.String("db"),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.in.validate()

			if tc.wantedError != nil {
				require.NotNil(t, gotErr)
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestValidateHealthCheckPorts(t *testing.T) {
	lbws := LoadBalancedWebService{
		Workload: Workload{
			Name: aws.String("mockWorkload"),
		},
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
					Port: aws.Uint16(80),
				},
			},
			HTTPOrBool: HTTPOrBool{
				HTTP: HTTP{
					Main: RoutingRule{
						Path: aws.String("/"),
					},
				},
			},
			NLBConfig: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("8080/udp"),
					HealthCheck: NLBHealthCheckArgs{
						Port: aws.Int(80),
					},
				},
				AdditionalListeners: []NetworkLoadBalancerListener{
					{
						Port: aws.String("8081/udp"),
						HealthCheck: NLBHealthCheckArgs{
							Port: aws.Int(80),
						},
					},
					{
						Port: aws.String("8082"),
					},
				},
			},
		},
	}
	lbwsWithInvalidHealthChecks := LoadBalancedWebService{
		LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
			HTTPOrBool: HTTPOrBool{
				HTTP: HTTP{
					Main: RoutingRule{
						Path: aws.String("/"),
						HealthCheck: HealthCheckArgsOrString{
							Union[string, HTTPHealthCheckArgs]{
								Advanced: HTTPHealthCheckArgs{
									Port: aws.Int(8080),
								},
							},
						},
						TargetPort: aws.Uint16(80),
					},
				},
			},
			NLBConfig: NetworkLoadBalancerConfiguration{
				Listener: NetworkLoadBalancerListener{
					Port: aws.String("8080/udp"),
				},
			},
		},
	}
	exposedPortIndex, _ := lbws.ExposedPorts()
	testCases := map[string]struct {
		in     validateHealthCheckPortsOpts
		wanted error
	}{
		"error with healthcheck on nlb udp": {
			in: validateHealthCheckPortsOpts{
				exposedPorts:      exposedPortIndex,
				mainContainerPort: lbws.ImageConfig.Port,
				nlb:               lbwsWithInvalidHealthChecks.NLBConfig,
			},
			wanted: fmt.Errorf(`container "mockWorkload" exposes port 8080 using protocol udp invalid for health checks. Valid protocol is "TCP".`),
		},
		"error with healthcheck on nlb udp from alb routing rule": {
			in: validateHealthCheckPortsOpts{
				exposedPorts:      exposedPortIndex,
				mainContainerPort: lbws.ImageConfig.Port,
				alb:               lbwsWithInvalidHealthChecks.HTTPOrBool.HTTP,
			},
			wanted: fmt.Errorf(`container "mockWorkload" exposes port 8080 using protocol udp invalid for health checks. Valid protocol is "TCP".`),
		},
		"error with healthcheck from image port": {
			in: validateHealthCheckPortsOpts{
				exposedPorts:      exposedPortIndex,
				mainContainerPort: aws.Uint16(8080),
				alb:               lbws.HTTPOrBool.HTTP,
				nlb:               lbws.NLBConfig,
			},
			wanted: fmt.Errorf(`container "mockWorkload" exposes port 8080 using protocol udp invalid for health checks. Valid protocol is "TCP".`),
		},
		"no error with valid healthchecks": {
			in: validateHealthCheckPortsOpts{
				exposedPorts:      exposedPortIndex,
				mainContainerPort: lbws.ImageConfig.Port,
				alb:               lbws.HTTPOrBool.HTTP,
				nlb:               lbws.NLBConfig,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateHealthCheckPorts(tc.in)

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateExposedPorts(t *testing.T) {
	testCases := map[string]struct {
		in     validateExposedPortsOpts
		wanted error
	}{
		"should return an error if main container and sidecar container is exposing the same port": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(80),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
			},
			wanted: fmt.Errorf(`containers "mockMainContainer" and "foo" are exposing the same port 80`),
		},
		"should not error out when main container uses non-default protocol": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(80),
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:       aws.String("8080/udp"),
						TargetPort: aws.Int(80),
					},
				},
			},
			wanted: nil,
		},
		"should not error out when alb target_port is same as that of sidecar container port but target_container is empty": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort: aws.Uint16(80),
					},
				},
			},
			wanted: nil,
		},
		"should not error out when nlb target_port is same as that of sidecar container port but target_container is empty": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:       aws.String("8080/tcp"),
						TargetPort: aws.Int(80),
					},
				},
			},
			wanted: nil,
		},
		"should not error out when nlb target_port is same as that of sidecar container port but target_container and protocol is empty": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:       aws.String("8080"),
						TargetPort: aws.Int(80),
					},
				},
			},
			wanted: nil,
		},
		"should not error out when tls is terminated exposing a tcp port": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80/tcp"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:       aws.String("8080/tls"),
						TargetPort: aws.Int(80),
					},
				},
			},
			wanted: nil,
		},
		"should return an error when nlb target_port is same as that of sidecar container port but sidecar uses non default protocol": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80/udp"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:       aws.String("8080"),
						TargetPort: aws.Int(80),
					},
				},
			},
			wanted: fmt.Errorf(`validate "nlb": container "foo" is exposing the same port 80 with protocol TCP and UDP`),
		},
		"should return an error if alb target_port points to one sidecar container port and target_container points to another sidecar container": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(5000),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("8080"),
					},
					"nginx": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetContainer: aws.String("nginx"),
						TargetPort:      aws.Uint16(8080),
					},
				},
			},
			wanted: fmt.Errorf(`containers "nginx" and "foo" are exposing the same port 8080`),
		},
		"should not return an error if main container and sidecar container is exposing different ports": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
			},
			wanted: nil,
		},
		"doesn't error out when similar config is present in target_port and target_container as that of primary container": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort:      aws.Uint16(8080),
						TargetContainer: aws.String("mockMainContainer"),
					},
				},
			},
			wanted: nil,
		},
		"doesn't error out when similar config is present in target_port and target_container as that of sidecar container": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort:      aws.Uint16(80),
						TargetContainer: aws.String("foo"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:            aws.String("8080/tcp"),
						TargetPort:      aws.Int(80),
						TargetContainer: aws.String("foo"),
					},
				},
			},
			wanted: nil,
		},
		"doesn't error out when target_port exposing different port of the primary container than its main port": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort: aws.Uint16(8081),
					},
				},
			},
			wanted: nil,
		},
		"doesn't error out when multiple ports are open through additional_rules": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(8080),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort: aws.Uint16(8081),
					},
					AdditionalRoutingRules: []RoutingRule{
						{
							TargetPort: aws.Uint16(8082),
						},
						{
							TargetPort: aws.Uint16(8083),
						},
					},
				},
			},
			wanted: nil,
		},
		"should not return an error if alb and nlb target_port trying to expose same container port of the primary container": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(5000),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("8080"),
					},
					"nginx": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort: aws.Uint16(5001),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:       aws.String("5001/tcp"),
						TargetPort: aws.Int(5001),
					},
				},
			},
		},
		"should not return an error if alb and nlb target_port trying to expose same container port sidecar container": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(5000),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("8080"),
					},
					"nginx": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort:      aws.Uint16(5001),
						TargetContainer: aws.String("foo"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:            aws.String("5001/tcp"),
						TargetPort:      aws.Int(5001),
						TargetContainer: aws.String("foo"),
					},
				},
			},
		},
		"should return an error if alb and nlb target_port trying to expose same container port of different containers": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(5000),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("8080"),
					},
					"nginx": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort:      aws.Uint16(5001),
						TargetContainer: aws.String("foo"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:            aws.String("5001/tcp"),
						TargetPort:      aws.Int(5001),
						TargetContainer: aws.String("nginx"),
					},
				},
			},
			wanted: fmt.Errorf(`validate "nlb": containers "nginx" and "foo" are exposing the same port 5001`),
		},
		"should return an error if alb and nlb target_port trying to expose same container port with different protocol": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(5000),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("8080"),
					},
					"nginx": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort:      aws.Uint16(5001),
						TargetContainer: aws.String("foo"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:            aws.String("5001/udp"),
						TargetPort:      aws.Int(5001),
						TargetContainer: aws.String("foo"),
					},
				},
			},
			wanted: fmt.Errorf(`validate "nlb": container "foo" is exposing the same port 5001 with protocol UDP and TCP`),
		},
		"should not return an error if nlb is trying to expose multiple ports": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(5000),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("8080"),
					},
					"nginx": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort:      aws.Uint16(5001),
						TargetContainer: aws.String("foo"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:            aws.String("5001/tcp"),
						TargetPort:      aws.Int(5001),
						TargetContainer: aws.String("foo"),
					},
					AdditionalListeners: []NetworkLoadBalancerListener{
						{
							Port:            aws.String("5002/tcp"),
							TargetPort:      aws.Int(5002),
							TargetContainer: aws.String("foo"),
						},
					},
				},
			},
		},
		"should return an error if nlb is trying to expose same port as from different containers using additional listeners": {
			in: validateExposedPortsOpts{
				mainContainerName: "mockMainContainer",
				mainContainerPort: aws.Uint16(5000),
				sidecarConfig: map[string]*SidecarConfig{
					"foo": {
						Port: aws.String("8080"),
					},
					"nginx": {
						Port: aws.String("80"),
					},
				},
				alb: &HTTP{
					Main: RoutingRule{
						TargetPort:      aws.Uint16(5001),
						TargetContainer: aws.String("foo"),
					},
				},
				nlb: &NetworkLoadBalancerConfiguration{
					Listener: NetworkLoadBalancerListener{
						Port:            aws.String("5001/tcp"),
						TargetPort:      aws.Int(5001),
						TargetContainer: aws.String("foo"),
					},
					AdditionalListeners: []NetworkLoadBalancerListener{
						{
							Port:            aws.String("5002/tcp"),
							TargetPort:      aws.Int(5001),
							TargetContainer: aws.String("nginx"),
						},
					},
				},
			},
			wanted: fmt.Errorf(`validate "nlb.additional_listeners[0]": containers "nginx" and "foo" are exposing the same port 5001`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateExposedPorts(tc.in)

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestImageLocationOrBuild_validate(t *testing.T) {
	testCases := map[string]struct {
		in          ImageLocationOrBuild
		wantedError error
	}{
		"should return error if both build and location are specified": {
			in: ImageLocationOrBuild{
				Build:    BuildArgsOrString{BuildString: aws.String("web/Dockerfile")},
				Location: aws.String("mockLocation"),
			},
			wantedError: fmt.Errorf(`must specify one of "build" and "location"`),
		},
		"return nil if only build is specified": {
			in: ImageLocationOrBuild{
				Build: BuildArgsOrString{BuildString: aws.String("web/Dockerfile")},
			},
		},
		"return nil if only location is specified": {
			in: ImageLocationOrBuild{
				Location: aws.String("mockLocation"),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStaticSiteConfig_validate(t *testing.T) {
	testCases := map[string]struct {
		in          StaticSiteConfig
		wantedError error
	}{
		"should return error if alias is not specified when certificate is set": {
			in: StaticSiteConfig{
				HTTP: StaticSiteHTTP{
					Certificate: "arn:aws:acm:us-east-1:1234567890:certificate/1115a386-a3db-4fb8-9b39-dfed63968129",
				},
			},
			wantedError: fmt.Errorf(`validate "http": "alias" must be specified if "certificate" is specified`),
		},
		"should return error if certificate is not in us-east-1": {
			in: StaticSiteConfig{
				HTTP: StaticSiteHTTP{
					Alias:       "foobar.com",
					Certificate: "arn:aws:acm:us-east-2:1234567890:certificate/1115a386-a3db-4fb8-9b39-dfed63968129",
				},
			},
			wantedError: fmt.Errorf(`validate "http": cdn certificate must be in region us-east-1`),
		},
		"should return error if source is missing": {
			in: StaticSiteConfig{
				FileUploads: []FileUpload{{}},
			},
			wantedError: fmt.Errorf(`validate "files[0]": "source" must be specified`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.validate()

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
