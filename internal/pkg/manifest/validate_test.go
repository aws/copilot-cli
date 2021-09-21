// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancedWebServiceConfig_Validate(t *testing.T) {
	testImageConfig := ImageWithPortAndHealthcheck{
		ImageWithPort: ImageWithPort{
			Image: Image{
				Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
			},
			Port: uint16P(80),
		},
	}
	testCases := map[string]struct {
		lbConfig LoadBalancedWebServiceConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate image": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
							Location: aws.String("mockLocation"),
						},
					},
				},
			},
			wantedErrorPrefix: `validate "image": `,
		},
		"error if fail to validate http": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
				RoutingRule: RoutingRule{
					TargetContainer:          aws.String("mockTargetContainer"),
					TargetContainerCamelCase: aws.String("mockTargetContainer"),
				},
			},
			wantedErrorPrefix: `validate "http": `,
		},
		"error if fail to validate network": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
				RoutingRule: RoutingRule{
					TargetContainer: aws.String("mockTargetContainer"),
				},
				Network: NetworkConfig{
					vpcConfig{
						SecurityGroups: []string{},
					},
				},
			},
			wantedErrorPrefix: `validate "network": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.lbConfig.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestBackendServiceConfig_Validate(t *testing.T) {
	testImageConfig := ImageWithPortAndHealthcheck{
		ImageWithPort: ImageWithPort{
			Image: Image{
				Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
			},
			Port: uint16P(80),
		},
	}
	testCases := map[string]struct {
		config BackendServiceConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate image": {
			config: BackendServiceConfig{
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
							Location: aws.String("mockLocation"),
						},
					},
				},
			},
			wantedErrorPrefix: `validate "image": `,
		},
		"error if fail to validate network": {
			config: BackendServiceConfig{
				ImageConfig: testImageConfig,
				Network: NetworkConfig{
					vpcConfig{
						SecurityGroups: []string{},
					},
				},
			},
			wantedErrorPrefix: `validate "network": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestRequestDrivenWebServiceConfig_Validate(t *testing.T) {
	testCases := map[string]struct {
		config RequestDrivenWebServiceConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate image": {
			config: RequestDrivenWebServiceConfig{
				ImageConfig: ImageWithPort{
					Image: Image{
						Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
						Location: aws.String("mockLocation"),
					},
				},
			},
			wantedErrorPrefix: `validate "image": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestWorkerServiceConfig_Validate(t *testing.T) {
	testImageConfig := ImageWithHealthcheck{
		Image: Image{
			Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
		},
	}
	testCases := map[string]struct {
		config WorkerServiceConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate image": {
			config: WorkerServiceConfig{
				ImageConfig: ImageWithHealthcheck{
					Image: Image{
						Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
						Location: aws.String("mockLocation"),
					},
				},
			},
			wantedErrorPrefix: `validate "image": `,
		},
		"error if fail to validate network": {
			config: WorkerServiceConfig{
				ImageConfig: testImageConfig,
				Network: NetworkConfig{
					vpcConfig{
						SecurityGroups: []string{},
					},
				},
			},
			wantedErrorPrefix: `validate "network": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestScheduledJobConfig_Validate(t *testing.T) {
	testImageConfig := ImageWithHealthcheck{
		Image: Image{
			Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
		},
	}
	testCases := map[string]struct {
		config ScheduledJobConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate image": {
			config: ScheduledJobConfig{
				ImageConfig: ImageWithHealthcheck{
					Image: Image{
						Build:    BuildArgsOrString{BuildString: aws.String("mockBuild")},
						Location: aws.String("mockLocation"),
					},
				},
			},
			wantedErrorPrefix: `validate "image": `,
		},
		"error if fail to validate network": {
			config: ScheduledJobConfig{
				ImageConfig: testImageConfig,
				Network: NetworkConfig{
					vpcConfig{
						SecurityGroups: []string{},
					},
				},
			},
			wantedErrorPrefix: `validate "network": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestImageWithPort_Validate(t *testing.T) {
	testCases := map[string]struct {
		ImageWithPort ImageWithPort

		wantedError error
	}{
		"error if port is not specified": {
			ImageWithPort: ImageWithPort{
				Image: Image{
					Location: aws.String("mockLocation"),
				},
			},
			wantedError: fmt.Errorf(`"port" must be specified`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.ImageWithPort.Validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestImage_Validate(t *testing.T) {
	testCases := map[string]struct {
		Image Image

		wantedError error
	}{
		"error if build and location both specified": {
			Image: Image{
				Build: BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				},
				Location: aws.String("mockLocation"),
			},
			wantedError: fmt.Errorf(`must specify one of "build" and "location"`),
		},
		"error if neither build nor location is specified": {
			Image:       Image{},
			wantedError: fmt.Errorf(`must specify one of "build" and "location"`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.Image.Validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestRoutingRule_Validate(t *testing.T) {
	testCases := map[string]struct {
		RoutingRule RoutingRule

		wantedErrorMsgPrefix string
		wantedError          error
	}{
		"error if both target_container and targetContainer are specified": {
			RoutingRule: RoutingRule{
				TargetContainer:          aws.String("mockContainer"),
				TargetContainerCamelCase: aws.String("mockContainer"),
			},
			wantedError: fmt.Errorf(`must specify one, not both, of "target_container" and "targetContainer"`),
		},
		"error if one of allowed_source_ips is not valid": {
			RoutingRule: RoutingRule{
				AllowedSourceIps: []IPNet{
					IPNet("10.1.0.0/24"),
					IPNet("badIP"),
					IPNet("10.1.1.0/24"),
				},
			},
			wantedErrorMsgPrefix: `validate "allowed_source_ips[1]": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.RoutingRule.Validate()

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

func TestIPNet_Validate(t *testing.T) {
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
			err := tc.in.Validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTaskConfig_Validate(t *testing.T) {
	mockPerc := Percentage(70)
	testCases := map[string]struct {
		TaskConfig TaskConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate count": {
			TaskConfig: TaskConfig{
				Count: Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(123),
						CPU:  &mockPerc,
					},
				},
			},
			wantedErrorPrefix: `validate "count": `,
		},
		"error if fail to validate storage": {
			TaskConfig: TaskConfig{
				Storage: Storage{
					Volumes: map[string]*Volume{
						"foo": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID:          aws.Uint32(123),
									FileSystemID: aws.String("mockID"),
								},
							},
						},
					},
				},
			},
			wantedErrorPrefix: `validate "storage": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.TaskConfig.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestPlatformString_Validate(t *testing.T) {
	testCases := map[string]struct {
		in     PlatformString
		wanted error
	}{}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.Validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAdvancedCount_Validate(t *testing.T) {
	var (
		mockPerc    = Percentage(70)
		invalidPerc = Percentage(-1)
	)
	testCases := map[string]struct {
		AdvancedCount AdvancedCount

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"cannot have autoscaling for scheduled jobs": {
			AdvancedCount: AdvancedCount{
				Spot:         aws.Int(42),
				workloadType: ScheduledJobType,
			},
			wantedError: errors.New("cannot have autoscaling options for workloads of type 'Scheduled Job'"),
		},
		"valid if only spot is specified": {
			AdvancedCount: AdvancedCount{
				Spot:         aws.Int(42),
				workloadType: BackendServiceType,
			},
		},
		"valid when range and and at least one autoscaling fields are specified": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				CPU: &mockPerc,
				QueueScaling: QueueScaling{
					AcceptableLatency: durationp(10 * time.Second),
					AvgProcessingTime: durationp(1 * time.Second),
				},
				workloadType: WorkerServiceType,
			},
		},
		"error if both spot and autoscaling fields are specified": {
			AdvancedCount: AdvancedCount{
				Spot:         aws.Int(123),
				CPU:          &mockPerc,
				workloadType: LoadBalancedWebServiceType,
			},
			wantedError: fmt.Errorf(`must specify one, not both, of "spot" and "range/cpu_percentage/memory_percentage/requests/response_time"`),
		},
		"error if fail to validate range": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("")),
				},
				workloadType: LoadBalancedWebServiceType,
			},
			wantedErrorMsgPrefix: `validate "range": `,
		},
		"error if range is missing when autoscaling fields are set for Load Balanced Web Service": {
			AdvancedCount: AdvancedCount{
				Requests:     aws.Int(123),
				workloadType: LoadBalancedWebServiceType,
			},
			wantedError: fmt.Errorf(`"range" must be specified if "cpu_percentage, memory_percentage, requests or response_time" are specified`),
		},
		"error if range is specified but no autoscaling fields are specified for a Load Balanced Web Service": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				workloadType: LoadBalancedWebServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage", "memory_percentage", "requests" or "response_time" if "range" is specified`),
		},
		"error if range is specified but no autoscaling fields are specified for a Backend Service": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				workloadType: BackendServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage" or "memory_percentage" if "range" is specified`),
		},
		"error if range is specified but no autoscaling fields are specified for a Worker Service": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				workloadType: WorkerServiceType,
			},
			wantedError: fmt.Errorf(`must specify at least one of "cpu_percentage", "memory_percentage" or "queue_delay" if "range" is specified`),
		},
		"error if range is missing when autoscaling fields are set for Backend Service": {
			AdvancedCount: AdvancedCount{
				CPU:          &mockPerc,
				workloadType: BackendServiceType,
			},
			wantedError: fmt.Errorf(`"range" must be specified if "cpu_percentage or memory_percentage" are specified`),
		},
		"error if range is missing when autoscaling fields are set for Worker Service": {
			AdvancedCount: AdvancedCount{
				CPU:          &mockPerc,
				workloadType: WorkerServiceType,
			},
			wantedError: fmt.Errorf(`"range" must be specified if "cpu_percentage, memory_percentage or queue_delay" are specified`),
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
				workloadType: WorkerServiceType,
			},
			wantedErrorMsgPrefix: `validate "queue_delay": `,
		},
		"error if CPU perc is not valid": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(stringP("1-2")),
				},
				CPU:          &invalidPerc,
				workloadType: LoadBalancedWebServiceType,
			},
			wantedErrorMsgPrefix: `validate "cpu_percentage": `,
		},
		"error if memory perc is not valid": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(stringP("1-2")),
				},
				Memory:       &invalidPerc,
				workloadType: LoadBalancedWebServiceType,
			},
			wantedErrorMsgPrefix: `validate "memory_percentage": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.AdvancedCount.Validate()

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

func TestPercentage_Validate(t *testing.T) {
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
			err := tc.in.Validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestQueueScaling_Validate(t *testing.T) {
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
			err := tc.in.Validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIntRangeBand_Validate(t *testing.T) {
	testCases := map[string]struct {
		IntRangeBand IntRangeBand

		wantedError error
	}{
		"error if range value is in invalid format": {
			IntRangeBand: IntRangeBand(*aws.String("")),
			wantedError:  fmt.Errorf("invalid range value . Should be in format of ${min}-${max}"),
		},
		"error if range min is greater than max": {
			IntRangeBand: IntRangeBand(*aws.String("6-4")),
			wantedError:  fmt.Errorf("min value 6 cannot be greater than max value 4"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.IntRangeBand.Validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestRangeConfig_Validate(t *testing.T) {
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
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.RangeConfig.Validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestStorage_Validate(t *testing.T) {
	testCases := map[string]struct {
		Storage Storage

		wantedErrorPrefix string
	}{
		"error if fail to validate volumes": {
			Storage: Storage{
				Volumes: map[string]*Volume{
					"foo": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
					},
					"bar": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID:           aws.Uint32(123),
								RootDirectory: aws.String("mockDir"),
							},
						},
					},
				},
			},
			wantedErrorPrefix: `validate "volumes[bar]": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.Storage.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestVolume_Validate(t *testing.T) {
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
			gotErr := tc.Volume.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestEFSVolumeConfiguration_Validate(t *testing.T) {
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
		"error if AuthorizationConfig is not configured correctly": {
			EFSVolumeConfiguration: EFSVolumeConfiguration{
				AuthConfig: AuthorizationConfig{
					AccessPointID: aws.String("mockID"),
				},
				RootDirectory: aws.String("mockDir"),
			},
			wantedError: fmt.Errorf(`"root_dir" must be either empty or "/" and "auth.iam" must be true when "access_point_id" is used`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.EFSVolumeConfiguration.Validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestNetworkConfig_Validate(t *testing.T) {
	testCases := map[string]struct {
		config NetworkConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate vpc": {
			config: NetworkConfig{
				VPC: vpcConfig{
					SecurityGroups: []string{},
				},
			},
			wantedErrorPrefix: `validate "vpc": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestVpcConfig_Validate(t *testing.T) {
	testCases := map[string]struct {
		config vpcConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate placement": {
			config: vpcConfig{
				SecurityGroups: []string{},
			},
			wantedErrorPrefix: `validate "placement": `,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.config.Validate()

			if tc.wantedErrorPrefix != "" {
				require.Contains(t, gotErr.Error(), tc.wantedErrorPrefix)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestPlacement_Validate(t *testing.T) {
	mockInvalidPlacement := Placement("external")
	testCases := map[string]struct {
		in     *Placement
		wanted error
	}{
		"should return an error if placement is empty": {
			wanted: errors.New(`"placement" cannot be empty`),
		},
		"should return an error if placement is invalid": {
			in:     &mockInvalidPlacement,
			wanted: errors.New(`"placement" external must be one of public, private`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.in.Validate()

			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
