// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnmarshalSvc(t *testing.T) {
	testCases := map[string]struct {
		inContent string

		requireCorrectValues func(t *testing.T, i interface{})
		wantedErr            error
	}{
		"load balanced web service": {
			inContent: `
version: 1.0
name: frontend
type: "Load Balanced Web Service"
image:
  location: foo/bar
  port: 80
cpu: 512
memory: 1024
count: 1
exec: true
http:
  path: "svc"
  target_container: "frontend"
variables:
  LOG_LEVEL: "WARN"
secrets:
  DB_PASSWORD: MYSQL_DB_PASSWORD
sidecars:
  xray:
    port: 2000/udp
    image: 123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon
    credentialsParameter: some arn
logging:
  destination:
    Name: cloudwatch
    include-pattern: ^[a-z][aeiou].*$
    exclude-pattern: ^.*[aeiou]$
  enableMetadata: false
  secretOptions:
    LOG_TOKEN: LOG_TOKEN
  configFilePath: /extra.conf
environments:
  test:
    count: 3
  staging1:
    count:
      spot: 5
  staging2:
    count:
      range:
        min: 2
        max: 8
        spot_from: 4
  prod:
    count:
      range: 1-10
      cpu_percentage: 70
`,
			requireCorrectValues: func(t *testing.T, i interface{}) {
				mockRange := IntRangeBand("1-10")
				actualManifest, ok := i.(*LoadBalancedWebService)
				require.True(t, ok)
				wantedManifest := &LoadBalancedWebService{
					Workload: Workload{Name: aws.String("frontend"), Type: aws.String(LoadBalancedWebServiceType)},
					LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
						ImageConfig: ImageWithPortAndHealthcheck{
							ImageWithPort: ImageWithPort{Image: Image{Build: BuildArgsOrString{},
								Location: aws.String("foo/bar"),
							}, Port: aws.Uint16(80)},
						},
						RoutingRule: RoutingRule{
							Path:            aws.String("svc"),
							TargetContainer: aws.String("frontend"),
							HealthCheck: HealthCheckArgsOrString{
								HealthCheckPath: aws.String("/"),
							},
						},
						TaskConfig: TaskConfig{
							CPU:    aws.Int(512),
							Memory: aws.Int(1024),
							Count: Count{
								Value: aws.Int(1),
							},
							ExecuteCommand: ExecuteCommand{
								Enable: aws.Bool(true),
							},
							Variables: map[string]string{
								"LOG_LEVEL": "WARN",
							},
							Secrets: map[string]string{
								"DB_PASSWORD": "MYSQL_DB_PASSWORD",
							},
						},
						Sidecars: map[string]*SidecarConfig{
							"xray": {
								Port:       aws.String("2000/udp"),
								Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
								CredsParam: aws.String("some arn"),
							},
						},
						Logging: &Logging{
							Destination: map[string]string{
								"exclude-pattern": "^.*[aeiou]$",
								"include-pattern": "^[a-z][aeiou].*$",
								"Name":            "cloudwatch",
							},
							EnableMetadata: aws.Bool(false),
							ConfigFile:     aws.String("/extra.conf"),
							SecretOptions: map[string]string{
								"LOG_TOKEN": "LOG_TOKEN",
							},
						},
						Network: &NetworkConfig{
							VPC: &vpcConfig{
								Placement: stringP("public"),
							},
						},
					},
					Environments: map[string]*LoadBalancedWebServiceConfig{
						"test": {
							TaskConfig: TaskConfig{
								Count: Count{
									Value: aws.Int(3),
								},
							},
						},
						"staging1": {
							TaskConfig: TaskConfig{
								Count: Count{
									AdvancedCount: AdvancedCount{
										Spot: aws.Int(5),
									},
								},
							},
						},
						"staging2": {
							TaskConfig: TaskConfig{
								Count: Count{
									AdvancedCount: AdvancedCount{
										Range: &Range{
											RangeConfig: RangeConfig{
												Min:      aws.Int(2),
												Max:      aws.Int(8),
												SpotFrom: aws.Int(4),
											},
										},
									},
								},
							},
						},
						"prod": {
							TaskConfig: TaskConfig{
								Count: Count{
									AdvancedCount: AdvancedCount{
										Range: &Range{
											Value: &mockRange,
										},
										CPU: aws.Int(70),
									},
								},
							},
						},
					},
				}
				require.Equal(t, wantedManifest, actualManifest)
			},
		},
		"Backend Service": {
			inContent: `
name: subscribers
type: Backend Service
image:
  build: ./subscribers/Dockerfile
  port: 8080
  healthcheck:
    command: ['CMD-SHELL', 'curl http://localhost:5000/ || exit 1']
cpu: 1024
memory: 1024
secrets:
  API_TOKEN: SUBS_API_TOKEN`,
			requireCorrectValues: func(t *testing.T, i interface{}) {
				actualManifest, ok := i.(*BackendService)
				require.True(t, ok)
				wantedManifest := &BackendService{
					Workload: Workload{
						Name: aws.String("subscribers"),
						Type: aws.String(BackendServiceType),
					},
					BackendServiceConfig: BackendServiceConfig{
						ImageConfig: ImageWithPortAndHealthcheck{
							ImageWithPort: ImageWithPort{
								Image: Image{
									Build: BuildArgsOrString{
										BuildString: aws.String("./subscribers/Dockerfile"),
									},
								},
								Port: aws.Uint16(8080),
							},
							HealthCheck: &ContainerHealthCheck{
								Command: []string{"CMD-SHELL", "curl http://localhost:5000/ || exit 1"},
							},
						},
						TaskConfig: TaskConfig{
							CPU:    aws.Int(1024),
							Memory: aws.Int(1024),
							Count: Count{
								Value: aws.Int(1),
							},
							ExecuteCommand: ExecuteCommand{
								Enable: aws.Bool(false),
							},
							Secrets: map[string]string{
								"API_TOKEN": "SUBS_API_TOKEN",
							},
						},
						Network: &NetworkConfig{
							VPC: &vpcConfig{
								Placement: stringP("public"),
							},
						},
					},
				}
				require.Equal(t, wantedManifest, actualManifest)
			},
		},
		"invalid svc type": {
			inContent: `
name: CowSvc
type: 'OH NO'
`,
			wantedErr: &ErrInvalidWorkloadType{Type: "OH NO"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := UnmarshalWorkload([]byte(tc.inContent))

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				tc.requireCorrectValues(t, m)
			}
		})
	}
}

func TestCount_UnmarshalYAML(t *testing.T) {
	mockResponseTime := 500 * time.Millisecond
	mockRange := IntRangeBand("1-10")
	testCases := map[string]struct {
		inContent []byte

		wantedStruct Count
		wantedError  error
	}{
		"legacy case: simple task count": {
			inContent: []byte(`count: 1`),

			wantedStruct: Count{
				Value: aws.Int(1),
			},
		},
		"With auto scaling enabled": {
			inContent: []byte(`count:
  range: 1-10
  cpu_percentage: 70
  memory_percentage: 80
  requests: 1000
  response_time: 500ms
`),
			wantedStruct: Count{
				AdvancedCount: AdvancedCount{
					Range:        &Range{Value: &mockRange},
					CPU:          aws.Int(70),
					Memory:       aws.Int(80),
					Requests:     aws.Int(1000),
					ResponseTime: &mockResponseTime,
				},
			},
		},
		"With spot specified as count": {
			inContent: []byte(`count:
  spot: 42
`),
			wantedStruct: Count{
				AdvancedCount: AdvancedCount{
					Spot: aws.Int(42),
				},
			},
		},
		"With range specified as min-max": {
			inContent: []byte(`count:
  range:
    min: 5
    max: 15
`),
			wantedStruct: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{
						RangeConfig: RangeConfig{
							Min: aws.Int(5),
							Max: aws.Int(15),
						},
					},
				},
			},
		},
		"With all RangeConfig fields specified": {
			inContent: []byte(`count:
  range:
    min: 2
    max: 8
    spot_from: 3
`),
			wantedStruct: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{
						RangeConfig: RangeConfig{
							Min:      aws.Int(2),
							Max:      aws.Int(8),
							SpotFrom: aws.Int(3),
						},
					},
				},
			},
		},
		"With all RangeConfig fields specified and autoscaling field": {
			inContent: []byte(`count:
  range:
    min: 2
    max: 8
    spot_from: 3
  cpu_percentage: 50
`),
			wantedStruct: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{
						RangeConfig: RangeConfig{
							Min:      aws.Int(2),
							Max:      aws.Int(8),
							SpotFrom: aws.Int(3),
						},
					},
					CPU: aws.Int(50),
				},
			},
		},
		"Error if spot specified as int with range": {
			inContent: []byte(`count:
  range: 1-10
  spot: 3
`),
			wantedError: errInvalidAdvancedCount,
		},
		"Error if autoscaling specified without range": {
			inContent: []byte(`count:
  cpu_percentage: 30
`),
			wantedError: errInvalidAutoscaling,
		},
		"Error if unmarshalable": {
			inContent: []byte(`count: badNumber
`),
			wantedError: errUnmarshalCountOpts,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var b TaskConfig
			err := yaml.Unmarshal(tc.inContent, &b)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				// check memberwise dereferenced pointer equality
				require.Equal(t, tc.wantedStruct.Value, b.Count.Value)
				require.Equal(t, tc.wantedStruct.AdvancedCount.Range, b.Count.AdvancedCount.Range)
				require.Equal(t, tc.wantedStruct.AdvancedCount.CPU, b.Count.AdvancedCount.CPU)
				require.Equal(t, tc.wantedStruct.AdvancedCount.Memory, b.Count.AdvancedCount.Memory)
				require.Equal(t, tc.wantedStruct.AdvancedCount.Requests, b.Count.AdvancedCount.Requests)
				require.Equal(t, tc.wantedStruct.AdvancedCount.ResponseTime, b.Count.AdvancedCount.ResponseTime)
				require.Equal(t, tc.wantedStruct.AdvancedCount.Spot, b.Count.AdvancedCount.Spot)
			}
		})
	}
}

func TestIntRangeBand_Parse(t *testing.T) {
	testCases := map[string]struct {
		inRange string

		wantedMin int
		wantedMax int
		wantedErr error
	}{
		"invalid format": {
			inRange: "badRange",

			wantedErr: fmt.Errorf("invalid range value badRange. Should be in format of ${min}-${max}"),
		},
		"invalid minimum": {
			inRange: "a-100",

			wantedErr: fmt.Errorf("cannot convert minimum value a to integer"),
		},
		"invalid maximum": {
			inRange: "1-a",

			wantedErr: fmt.Errorf("cannot convert maximum value a to integer"),
		},
		"success": {
			inRange: "1-10",

			wantedMin: 1,
			wantedMax: 10,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			r := IntRangeBand(tc.inRange)
			gotMin, gotMax, err := r.Parse()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedMin, gotMin)
				require.Equal(t, tc.wantedMax, gotMax)
			}
		})
	}
}

func TestRange_Parse(t *testing.T) {
	mockRange := IntRangeBand("1-10")
	testCases := map[string]struct {
		input Range

		wantedMin int
		wantedMax int
		wantedErr error
	}{
		"error when both range and RangeConfig specified": {
			input: Range{
				Value: &mockRange,
				RangeConfig: RangeConfig{
					Min: aws.Int(1),
					Max: aws.Int(3),
				},
			},

			wantedErr: errInvalidRangeOpts,
		},
		"success": {
			input: Range{
				RangeConfig: RangeConfig{
					Min: aws.Int(2),
					Max: aws.Int(8),
				},
			},

			wantedMin: 2,
			wantedMax: 8,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotMin, gotMax, err := tc.input.Parse()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedMin, gotMin)
				require.Equal(t, tc.wantedMax, gotMax)
			}
		})
	}
}

func Test_ServiceDockerfileBuildRequired(t *testing.T) {
	testCases := map[string]struct {
		svc interface{}

		wanted    bool
		wantedErr error
	}{
		"invalid type": {
			svc: struct{}{},

			wantedErr: fmt.Errorf("service does not have required methods BuildRequired()"),
		},
		"fail to check": {
			svc: &LoadBalancedWebService{},

			wantedErr: fmt.Errorf("check if service requires building from local Dockerfile: either \"image.build\" or \"image.location\" needs to be specified in the manifest"),
		},
		"success with false": {
			svc: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Location: aws.String("mockLocation"),
							},
						},
					},
				},
			},
		},
		"success with true": {
			svc: &LoadBalancedWebService{
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("mockDockerfile"),
								},
							},
						},
					},
				},
			},
			wanted: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			got, err := ServiceDockerfileBuildRequired(tc.svc)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, got)
			}
		})
	}
}

func TestCount_Desired(t *testing.T) {
	mockRange := IntRangeBand("1-10")
	testCases := map[string]struct {
		input *Count

		expected    *int
		expectedErr error
	}{
		"with value": {
			input: &Count{
				Value: aws.Int(42),
			},

			expected: aws.Int(42),
		},
		"with spot count": {
			input: &Count{
				AdvancedCount: AdvancedCount{
					Spot: aws.Int(31),
				},
			},
			expected: aws.Int(31),
		},
		"with autoscaling range on dedicated capacity": {
			input: &Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{
						Value: &mockRange,
					},
				},
			},
			expected: aws.Int(1),
		},
		"with autoscaling range with spot capacity": {
			input: &Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{
						RangeConfig: RangeConfig{
							Min: aws.Int(5),
							Max: aws.Int(10),
						},
					},
				},
			},
			expected: aws.Int(5),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			actual, err := tc.input.Desired()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}

func TestAdvancedCount_IsValid(t *testing.T) {
	mockRange := IntRangeBand("1-10")
	testCases := map[string]struct {
		input *AdvancedCount

		expectedErr error
	}{
		"with spot count": {
			input: &AdvancedCount{
				Spot: aws.Int(42),
			},

			expectedErr: nil,
		},
		"with range value": {
			input: &AdvancedCount{
				Range: &Range{
					Value: &mockRange,
				},
			},

			expectedErr: nil,
		},
		"with range config": {
			input: &AdvancedCount{
				Range: &Range{
					RangeConfig: RangeConfig{
						Min:      aws.Int(1),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(2),
					},
				},
			},

			expectedErr: nil,
		},
		"with range and autoscaling config": {
			input: &AdvancedCount{
				Range: &Range{
					Value: &mockRange,
				},
				CPU:      aws.Int(512),
				Memory:   aws.Int(1024),
				Requests: aws.Int(1000),
			},

			expectedErr: nil,
		},
		"with range config and autoscaling config": {
			input: &AdvancedCount{
				Range: &Range{
					RangeConfig: RangeConfig{
						Min: aws.Int(1),
						Max: aws.Int(10),
					},
				},
				CPU:      aws.Int(512),
				Memory:   aws.Int(1024),
				Requests: aws.Int(1000),
			},

			expectedErr: nil,
		},
		"with range config with spot and autoscaling config": {
			input: &AdvancedCount{
				Range: &Range{
					RangeConfig: RangeConfig{
						Min:      aws.Int(1),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(3),
					},
				},
				CPU:      aws.Int(512),
				Memory:   aws.Int(1024),
				Requests: aws.Int(1000),
			},

			expectedErr: nil,
		},
		"invalid with spot count and autoscaling config": {
			input: &AdvancedCount{
				Spot:     aws.Int(42),
				CPU:      aws.Int(512),
				Memory:   aws.Int(1024),
				Requests: aws.Int(1000),
			},

			expectedErr: errInvalidAdvancedCount,
		},
		"invalid with spot count and range": {
			input: &AdvancedCount{
				Spot: aws.Int(42),
				Range: &Range{
					Value: &mockRange,
				},
			},

			expectedErr: errInvalidAdvancedCount,
		},
		"invalid with spot count and range config": {
			input: &AdvancedCount{
				Spot: aws.Int(42),
				Range: &Range{
					RangeConfig: RangeConfig{
						Min:      aws.Int(1),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(3),
					},
				},
			},

			expectedErr: errInvalidAdvancedCount,
		},
		"invalid with autoscaling fields and no range": {
			input: &AdvancedCount{
				CPU:      aws.Int(512),
				Memory:   aws.Int(1024),
				Requests: aws.Int(1000),
			},

			expectedErr: errInvalidAutoscaling,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			err := tc.input.IsValid()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHealthCheckArgsOrString_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		hc     HealthCheckArgsOrString
		wanted bool
	}{
		"should return true if there are no settings": {
			wanted: true,
		},
		"should return false if a path is set via the basic configuration": {
			hc: HealthCheckArgsOrString{
				HealthCheckPath: aws.String("/"),
			},
			wanted: false,
		},
		"should return false if a value is set via the advanced configuration": {
			hc: HealthCheckArgsOrString{
				HealthCheckArgs: HTTPHealthCheckArgs{
					Path: aws.String("/"),
				},
			},
			wanted: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.hc.IsEmpty())
		})
	}
}
