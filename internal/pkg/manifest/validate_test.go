// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"testing"

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

		wantedError error
	}{
		"error if both build and location are specified": {
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
			wantedError: fmt.Errorf(`validate "image": must specify one, not both, not none, of "build" and "location"`),
		},
		"error if neither build nor location is specified": {
			lbConfig:    LoadBalancedWebServiceConfig{},
			wantedError: fmt.Errorf(`validate "image": must specify one, not both, not none, of "build" and "location"`),
		},
		"error if no image port": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{BuildString: aws.String("mockBuild")},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "image": "port" must be specified`),
		},
		"error if both formats of target container are specified": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
				RoutingRule: RoutingRule{
					TargetContainer:          aws.String("mockTargetContainer"),
					TargetContainerCamelCase: aws.String("mockTargetContainer"),
				},
			},
			wantedError: fmt.Errorf(`validate "http": must specify one, not both, of "target_container" and "targetContainer"`),
		},
		"error if both spot and autoscaling fields are specified": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
				TaskConfig: TaskConfig{
					Count: Count{
						AdvancedCount: AdvancedCount{
							Spot:  aws.Int(0),
							Range: &Range{},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "count": must specify one, not both, of "spot" and "range/cpu_percentage/memory_percentage/requests/response_time"`),
		},
		"error if range band is in invalid format": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
				TaskConfig: TaskConfig{
					Count: Count{
						AdvancedCount: AdvancedCount{
							Range: &Range{
								Value: (*IntRangeBand)(aws.String("badValue")),
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "count": validate "range": invalid range value badValue. Should be in format of ${min}-${max}`),
		},
		"error if range band min is greater than max": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
				TaskConfig: TaskConfig{
					Count: Count{
						AdvancedCount: AdvancedCount{
							Range: &Range{
								Value: (*IntRangeBand)(aws.String("4-2")),
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "count": validate "range": min value 4 cannot be greater than max value 2`),
		},
		"error if range config min is greater than max": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
				TaskConfig: TaskConfig{
					Count: Count{
						AdvancedCount: AdvancedCount{
							Range: &Range{
								Value: (*IntRangeBand)(aws.String("4-2")),
								RangeConfig: RangeConfig{
									Min: aws.Int(4),
									Max: aws.Int(3),
								},
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "count": validate "range": min value 4 cannot be greater than max value 2`),
		},
		"error if specify both BYO config and UID config": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
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
			},
			wantedError: fmt.Errorf(`validate "storage": validate "volumes[foo]": validate "efs": must specify one, not both, of "uid/gid" and "id/root_dir/auth"`),
		},
		"error if EFS access point is invalid": {
			lbConfig: LoadBalancedWebServiceConfig{
				ImageConfig: testImageConfig,
				TaskConfig: TaskConfig{
					Storage: Storage{
						Volumes: map[string]*Volume{
							"foo": {
								EFS: EFSConfigOrBool{
									Advanced: EFSVolumeConfiguration{
										AuthConfig: AuthorizationConfig{
											AccessPointID: aws.String("mockID"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantedError: fmt.Errorf(`validate "storage": validate "volumes[foo]": validate "efs": root_dir must be either empty or / and auth.iam must be true when access_point_id is in used`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.lbConfig.Validate()

			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}
