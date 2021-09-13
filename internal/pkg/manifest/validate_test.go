// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
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

		wantedError error
	}{
		"error if both target_container and targetContainer are specified": {
			RoutingRule: RoutingRule{
				TargetContainer:          aws.String("mockContainer"),
				TargetContainerCamelCase: aws.String("mockContainer"),
			},
			wantedError: fmt.Errorf(`must specify one, not both, of "target_container" and "targetContainer"`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.RoutingRule.Validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestTaskConfig_Validate(t *testing.T) {
	testCases := map[string]struct {
		TaskConfig TaskConfig

		wantedErrorPrefix string
	}{
		"error if fail to validate count": {
			TaskConfig: TaskConfig{
				Count: Count{
					AdvancedCount: AdvancedCount{
						Spot: aws.Int(123),
						CPU:  aws.Int(123),
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

func TestAdvancedCount_Validate(t *testing.T) {
	testCases := map[string]struct {
		AdvancedCount AdvancedCount

		wantedError          error
		wantedErrorMsgPrefix string
	}{
		"valid if only spot is specified": {
			AdvancedCount: AdvancedCount{
				Spot: aws.Int(42),
			},
		},
		"valid when range and and at least one autoscaling fields are specified": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				},
				CPU: aws.Int(70),
				QueueScaling: queueScaling{
					AcceptableLatency: durationp(10 * time.Second),
					AvgProcessingTime: durationp(1 * time.Second),
				},
			},
		},
		"error if both spot and autoscaling fields are specified": {
			AdvancedCount: AdvancedCount{
				Spot: aws.Int(123),
				CPU:  aws.Int(70),
			},
			wantedError: fmt.Errorf(`must specify one, not both, of "spot" and "range/cpu_percentage/memory_percentage/requests/response_time/queue_delay"`),
		},
		"error if fail to validate range": {
			AdvancedCount: AdvancedCount{
				Range: Range{
					Value: (*IntRangeBand)(aws.String("")),
				},
			},
			wantedErrorMsgPrefix: `validate "range": `,
		},
		"error if range is missing when autoscaling fields are set": {
			AdvancedCount: AdvancedCount{
				Requests: aws.Int(123),
			},
			wantedError: fmt.Errorf(`"range" must be specified if "cpu_percentage, memory_percentage, requests, response_time or queue_delay" are specified`),
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
				require.Contains(t, gotErr.Error(), tc.wantedErrorMsgPrefix)
				return
			}
			require.NoError(t, gotErr)
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
