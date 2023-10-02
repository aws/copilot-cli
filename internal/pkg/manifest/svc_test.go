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
	"gopkg.in/yaml.v3"
)

func TestUnmarshalSvc(t *testing.T) {
	perc := Percentage(70)
	mockConfig := ScalingConfigOrT[Percentage]{
		Value: &perc,
	}
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
taskdef_overrides:
  - path: "ContainerDefinitions[0].Ulimits[-].HardLimit"
    value: !Ref ParamName
image:
  location: foo/bar
  credentials: some arn
  port: 80
cpu: 512
memory: 1024
count: 1
exec: true
network:
  connect: true
http:
  path: "svc"
  target_container: "frontend"
  alias:
    - foobar.com
    - v1.foobar.com
  allowed_source_ips:
    - 10.1.0.0/24
    - 10.1.1.0/24
variables:
  LOG_LEVEL: "WARN"
secrets:
  DB_PASSWORD: MYSQL_DB_PASSWORD
sidecars:
  xray:
    port: 2000/udp
    image: 123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon
    credentialsParameter: some arn
  nginx:
    image:
      build:
        dockerfile: "web/Dockerfile"
        context: "pathto/Dockerfile"
        target: "build-stage"
        cache_from:
          - foo/bar:latest
        args:
          arg1: value1
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
					Workload: Workload{Name: aws.String("frontend"), Type: aws.String(manifestinfo.LoadBalancedWebServiceType)},
					LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
						ImageConfig: ImageWithPortAndHealthcheck{
							ImageWithPort: ImageWithPort{Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build:    BuildArgsOrString{},
									Location: aws.String("foo/bar"),
								},
								Credentials: aws.String("some arn"),
							}, Port: aws.Uint16(80)},
						},
						HTTPOrBool: HTTPOrBool{
							HTTP: HTTP{
								Main: RoutingRule{
									Alias: Alias{
										AdvancedAliases: []AdvancedAlias{},
										StringSliceOrString: StringSliceOrString{
											StringSlice: []string{
												"foobar.com",
												"v1.foobar.com",
											},
										},
									},
									Path:             aws.String("svc"),
									TargetContainer:  aws.String("frontend"),
									HealthCheck:      HealthCheckArgsOrString{},
									AllowedSourceIps: []IPNet{IPNet("10.1.0.0/24"), IPNet("10.1.1.0/24")},
								},
							},
						},
						TaskConfig: TaskConfig{
							CPU:    aws.Int(512),
							Memory: aws.Int(1024),
							Count: Count{
								Value: aws.Int(1),
								AdvancedCount: AdvancedCount{
									workloadType: manifestinfo.LoadBalancedWebServiceType,
								},
							},
							ExecuteCommand: ExecuteCommand{
								Enable: aws.Bool(true),
							},
							Variables: map[string]Variable{
								"LOG_LEVEL": {
									StringOrFromCFN{
										Plain: stringP("WARN"),
									},
								},
							},
							Secrets: map[string]Secret{
								"DB_PASSWORD": {
									from: StringOrFromCFN{
										Plain: aws.String("MYSQL_DB_PASSWORD"),
									},
								},
							},
						},
						Sidecars: map[string]*SidecarConfig{
							"xray": {
								Port:       aws.String("2000/udp"),
								Image:      BasicToUnion[*string, ImageLocationOrBuild](aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon")),
								CredsParam: aws.String("some arn"),
							},
							"nginx": {
								Image: AdvancedToUnion[*string](
									ImageLocationOrBuild{
										Build: BuildArgsOrString{
											BuildArgs: DockerBuildArgs{
												Dockerfile: aws.String("web/Dockerfile"),
												Context:    aws.String("pathto/Dockerfile"),
												Target:     aws.String("build-stage"),
												CacheFrom:  []string{"foo/bar:latest"},
												Args: map[string]string{
													"arg1": "value1",
												},
											},
										},
									},
								),
							},
						},
						Logging: Logging{
							Destination: map[string]string{
								"exclude-pattern": "^.*[aeiou]$",
								"include-pattern": "^[a-z][aeiou].*$",
								"Name":            "cloudwatch",
							},
							EnableMetadata: aws.Bool(false),
							ConfigFile:     aws.String("/extra.conf"),
							SecretOptions: map[string]Secret{
								"LOG_TOKEN": {
									from: StringOrFromCFN{
										Plain: aws.String("LOG_TOKEN"),
									},
								},
							},
						},
						Network: NetworkConfig{
							VPC: vpcConfig{
								Placement: PlacementArgOrString{
									PlacementString: placementStringP(PublicSubnetPlacement),
								},
							},
							Connect: ServiceConnectBoolOrArgs{
								EnableServiceConnect: aws.Bool(true),
							},
						},
						TaskDefOverrides: []OverrideRule{
							{
								Path: "ContainerDefinitions[0].Ulimits[-].HardLimit",
								Value: yaml.Node{
									Kind:   8,
									Style:  1,
									Tag:    "!Ref",
									Value:  "ParamName",
									Line:   7,
									Column: 12,
								},
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
										Range: Range{
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
										Range: Range{
											Value: &mockRange,
										},
										CPU: mockConfig,
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
						Type: aws.String(manifestinfo.BackendServiceType),
					},
					BackendServiceConfig: BackendServiceConfig{
						ImageConfig: ImageWithHealthcheckAndOptionalPort{
							ImageWithOptionalPort: ImageWithOptionalPort{
								Image: Image{
									ImageLocationOrBuild: ImageLocationOrBuild{
										Build: BuildArgsOrString{
											BuildString: aws.String("./subscribers/Dockerfile"),
										},
									},
								},
								Port: aws.Uint16(8080),
							},
							HealthCheck: ContainerHealthCheck{
								Command: []string{"CMD-SHELL", "curl http://localhost:5000/ || exit 1"},
							},
						},
						TaskConfig: TaskConfig{
							CPU:    aws.Int(1024),
							Memory: aws.Int(1024),
							Count: Count{
								Value: aws.Int(1),
								AdvancedCount: AdvancedCount{
									workloadType: manifestinfo.BackendServiceType,
								},
							},
							ExecuteCommand: ExecuteCommand{
								Enable: aws.Bool(false),
							},
							Secrets: map[string]Secret{
								"API_TOKEN": {
									from: StringOrFromCFN{
										Plain: aws.String("SUBS_API_TOKEN"),
									},
								},
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
					Environments: map[string]*BackendServiceConfig{},
				}
				require.Equal(t, wantedManifest, actualManifest)
			},
		},
		"Worker Service": {
			inContent: `
name: dogcategorizer
type: Worker Service
image:
  build: ./dogcategorizer/Dockerfile
cpu: 1024
memory: 1024
exec: true     # Enable running commands in your container.
count: 1

subscribe:
  queue:
    delay: 15s
    dead_letter:
          tries: 5
  topics:
    - name: publisher1
      service: testpubsvc
    - name: publisher2
      service: testpubjob
      queue:
        timeout: 15s`,
			requireCorrectValues: func(t *testing.T, i interface{}) {
				actualManifest, ok := i.(*WorkerService)
				duration15Seconds := 15 * time.Second
				require.True(t, ok)
				wantedManifest := &WorkerService{
					Workload: Workload{
						Name: aws.String("dogcategorizer"),
						Type: aws.String(manifestinfo.WorkerServiceType),
					},
					WorkerServiceConfig: WorkerServiceConfig{
						ImageConfig: ImageWithHealthcheck{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build: BuildArgsOrString{
										BuildString: aws.String("./dogcategorizer/Dockerfile"),
									},
								},
							},
						},
						TaskConfig: TaskConfig{
							CPU:    aws.Int(1024),
							Memory: aws.Int(1024),
							Count: Count{
								Value: aws.Int(1),
								AdvancedCount: AdvancedCount{
									workloadType: manifestinfo.WorkerServiceType,
								},
							},
							ExecuteCommand: ExecuteCommand{
								Enable: aws.Bool(true),
							},
						},
						Network: NetworkConfig{
							VPC: vpcConfig{
								Placement: PlacementArgOrString{
									PlacementString: placementStringP(PublicSubnetPlacement),
								},
							},
						},
						Subscribe: SubscribeConfig{
							Topics: []TopicSubscription{
								{
									Name:    aws.String("publisher1"),
									Service: aws.String("testpubsvc"),
								},
								{
									Name:    aws.String("publisher2"),
									Service: aws.String("testpubjob"),
									Queue: SQSQueueOrBool{
										Advanced: SQSQueue{
											Timeout: &duration15Seconds,
										},
									},
								},
							},
							Queue: SQSQueue{
								Delay: &duration15Seconds,
								DeadLetter: DeadLetterQueue{
									Tries: aws.Uint16(5),
								},
							},
						},
					},
					Environments: map[string]*WorkerServiceConfig{},
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
				tc.requireCorrectValues(t, m.Manifest())
			}
		})
	}
}

func TestStringOrFromCFN_UnmarshalYAML(t *testing.T) {
	type mockField struct {
		StringOrFromCFN
	}
	type mockParentField struct {
		MockField mockField `yaml:"mock_field"`
	}
	testCases := map[string]struct {
		in          []byte
		wanted      mockParentField
		wantedError error
	}{
		"unmarshal plain string": {
			in: []byte(`mock_field: hey`),
			wanted: mockParentField{
				MockField: mockField{
					StringOrFromCFN{
						Plain: aws.String("hey"),
					},
				},
			},
		},
		"unmarshal import name": {
			in: []byte(`mock_field:
  from_cfn: yo`),
			wanted: mockParentField{
				MockField: mockField{
					StringOrFromCFN{
						FromCFN: fromCFN{
							Name: aws.String("yo"),
						},
					},
				},
			},
		},
		"nothing to unmarshal": {
			in: []byte(`other_field: yo`),
		},
		"fail to unmarshal": {
			in: []byte(`mock_field:
  heyyo: !`),
			wantedError: errors.New(`cannot unmarshal field to a string or into a map`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var s mockParentField
			err := yaml.Unmarshal(tc.in, &s)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, s)
			}
		})
	}
}

func TestCount_UnmarshalYAML(t *testing.T) {
	var (
		perc               = Percentage(70)
		timeMinute         = 60 * time.Second
		reqNum             = 1000
		responseTime       = 500 * time.Millisecond
		mockRange          = IntRangeBand("1-10")
		mockAdvancedConfig = ScalingConfigOrT[Percentage]{
			ScalingConfig: AdvancedScalingConfig[Percentage]{
				Value: &perc,
				Cooldown: Cooldown{
					ScaleInCooldown:  &timeMinute,
					ScaleOutCooldown: &timeMinute,
				},
			},
		}
		mockConfig = ScalingConfigOrT[Percentage]{
			Value: &perc,
		}
		mockCooldown = Cooldown{
			ScaleInCooldown:  &timeMinute,
			ScaleOutCooldown: &timeMinute,
		}
		mockRequests = ScalingConfigOrT[int]{
			Value: &reqNum,
		}
		mockResponseTime = ScalingConfigOrT[time.Duration]{
			Value: &responseTime,
		}
	)

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
  cpu_percentage:
    value: 70
    cooldown:
      in: 1m
      out: 1m
  memory_percentage: 70
  requests: 1000
  response_time: 500ms
`),
			wantedStruct: Count{
				AdvancedCount: AdvancedCount{
					Range:        Range{Value: &mockRange},
					CPU:          mockAdvancedConfig,
					Memory:       mockConfig,
					Requests:     mockRequests,
					ResponseTime: mockResponseTime,
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
					Range: Range{
						RangeConfig: RangeConfig{
							Min: aws.Int(5),
							Max: aws.Int(15),
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
  cooldown:
    in: 1m
    out: 1m
  cpu_percentage: 70
`),
			wantedStruct: Count{
				AdvancedCount: AdvancedCount{
					Range: Range{
						RangeConfig: RangeConfig{
							Min:      aws.Int(2),
							Max:      aws.Int(8),
							SpotFrom: aws.Int(3),
						},
					},
					Cooldown: mockCooldown,
					CPU:      mockConfig,
				},
			},
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
				require.Equal(t, tc.wantedStruct.AdvancedCount.Cooldown, b.Count.AdvancedCount.Cooldown)
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
	}{
		"success with range value": {
			input: Range{
				Value: &mockRange,
			},

			wantedMin: 1,
			wantedMax: 10,
		},
		"success with range config": {
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

			require.NoError(t, err)
			require.Equal(t, tc.wantedMin, gotMin)
			require.Equal(t, tc.wantedMax, gotMax)
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
					Range: Range{
						Value: &mockRange,
					},
				},
			},
			expected: aws.Int(1),
		},
		"with autoscaling range with spot capacity": {
			input: &Count{
				AdvancedCount: AdvancedCount{
					Range: Range{
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
				Union: BasicToUnion[string, HTTPHealthCheckArgs]("/"),
			},
			wanted: false,
		},
		"should return false if a value is set via the advanced configuration": {
			hc: HealthCheckArgsOrString{
				Union: BasicToUnion[string, HTTPHealthCheckArgs]("/"),
			},
			wanted: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.hc.IsZero())
		})
	}
}

func TestQueueScaling_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     QueueScaling
		wanted bool
	}{
		"should return false if msg_processing_time is not nil": {
			in: QueueScaling{
				AvgProcessingTime: durationp(5 * time.Second),
			},
		},
		"should return false if acceptable_latency is not nil": {
			in: QueueScaling{
				AcceptableLatency: durationp(1 * time.Minute),
			},
		},
		"should return true if there are no fields set": {
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.IsEmpty())
		})
	}
}

func TestQueueScaling_AcceptableBacklogPerTask(t *testing.T) {
	testCases := map[string]struct {
		in            QueueScaling
		wantedBacklog int
		wantedErr     error
	}{
		"should return an error if queue scaling is empty": {
			in:        QueueScaling{},
			wantedErr: errors.New(`"queue_delay" must be specified in order to calculate the acceptable backlog`),
		},
		"should round up to an integer if backlog number has a decimal": {
			in: QueueScaling{
				AcceptableLatency: durationp(10 * time.Second),
				AvgProcessingTime: durationp(300 * time.Millisecond),
			},
			wantedBacklog: 34,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual, err := tc.in.AcceptableBacklogPerTask()
			if tc.wantedErr != nil {
				require.NotNil(t, err)
			} else {
				require.Equal(t, tc.wantedBacklog, actual)
			}
		})
	}
}

func TestParsePortMapping(t *testing.T) {
	testCases := map[string]struct {
		inPort *string

		wantedPort     *string
		wantedProtocol *string
		wantedErr      error
	}{
		"error parsing port": {
			inPort:    stringP("1/2/3"),
			wantedErr: errors.New("cannot parse port mapping from 1/2/3"),
		},
		"no error if input is empty": {},
		"port number only": {
			inPort:     stringP("443"),
			wantedPort: stringP("443"),
		},
		"port and protocol": {
			inPort:         stringP("443/tcp"),
			wantedPort:     stringP("443"),
			wantedProtocol: stringP("tcp"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotPort, gotProtocol, err := ParsePortMapping(tc.inPort)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, gotPort, tc.wantedPort)
				require.Equal(t, gotProtocol, tc.wantedProtocol)
			}
		})
	}
}

func TestLoadBalancedWebService_NetworkLoadBalancerTarget(t *testing.T) {
	testCases := map[string]struct {
		in                    LoadBalancedWebService
		wantedTargetContainer []string
		wantedTargetPort      []string
		wantedErr             error
	}{
		"should return primary container name/nlb port as targetContainer/targetPort in case targetContainer and targetPort is not given ": {
			in: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("foo"),
					Type: aws.String("Load Balanced WebService"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port: aws.String("80/tcp"),
						},
						AdditionalListeners: []NetworkLoadBalancerListener{
							{
								Port: aws.String("81/tcp"),
							},
						},
					},
				},
			},
			wantedTargetContainer: []string{"foo", "foo"},
			wantedTargetPort:      []string{"80", "81"},
		},
		"should return targetContainer and targetPort as is if they are given ": {
			in: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("foo"),
					Type: aws.String("Load Balanced WebService"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port:            aws.String("80/tcp"),
							TargetPort:      aws.Int(81),
							TargetContainer: aws.String("bar"),
						},
						AdditionalListeners: []NetworkLoadBalancerListener{
							{
								Port:            aws.String("82/tcp"),
								TargetPort:      aws.Int(83),
								TargetContainer: aws.String("nginx"),
							},
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"bar": {
							Port: aws.String("8080"),
						},
						"nginx": {
							Port: aws.String("8081"),
						},
					},
				},
			},
			wantedTargetContainer: []string{"bar", "nginx"},
			wantedTargetPort:      []string{"81", "83"},
		},
		"should return error if targetPort is of incorrect type": {
			in: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("foo"),
					Type: aws.String("Load Balanced WebService"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port: aws.String("80/80/80"),
						},
					},
				},
			},
			wantedErr: errors.New(`cannot parse port mapping from 80/80/80`),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			exposedPorts, _ := tc.in.ExposedPorts()
			for idx, listener := range tc.in.NLBConfig.NLBListeners() {
				targetContainer, targetPort, err := listener.Target(exposedPorts)
				if tc.wantedErr != nil {
					require.EqualError(t, err, tc.wantedErr.Error())
				} else {
					require.Equal(t, tc.wantedTargetContainer[idx], targetContainer)
					require.Equal(t, tc.wantedTargetPort[idx], targetPort)
				}
			}
		})
	}
}

func TestLoadBalancedWebService_ServiceConnectTarget(t *testing.T) {
	testCases := map[string]struct {
		in                         LoadBalancedWebService
		wantedServiceConnectTarget *ServiceConnectTargetContainer
	}{
		"should return primary container name/image port as targetContainer/targetPort in case targetContainer and targetPort is not given": {
			in: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("foo"),
					Type: aws.String("Load Balanced WebService"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
				},
			},
			wantedServiceConnectTarget: &ServiceConnectTargetContainer{
				Container: "foo",
				Port:      "80",
			},
		},
		"should return alb targetContainer/targetPort if main routing rule is defined": {
			in: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("foo"),
					Type: aws.String("Load Balanced WebService"),
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
								TargetContainer: aws.String("foo"),
								TargetPort:      aws.Uint16(8080),
							},
						},
					},
				},
			},
			wantedServiceConnectTarget: &ServiceConnectTargetContainer{
				Container: "foo",
				Port:      "8080",
			},
		},
		"should return nil if container protocol is udp": {
			in: LoadBalancedWebService{
				Workload: Workload{
					Name: aws.String("foo"),
					Type: aws.String("Load Balanced WebService"),
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					NLBConfig: NetworkLoadBalancerConfiguration{
						Listener: NetworkLoadBalancerListener{
							Port: aws.String("80/udp"),
						},
					},
				},
			},
			wantedServiceConnectTarget: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			exposedPorts, _ := tc.in.ExposedPorts()
			targetSCContainer := tc.in.ServiceConnectTarget(exposedPorts)
			require.Equal(t, tc.wantedServiceConnectTarget, targetSCContainer)
		})
	}
}
