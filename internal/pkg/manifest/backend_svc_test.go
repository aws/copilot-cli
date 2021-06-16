// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
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
				},
			},
			wantedManifest: &BackendService{
				Workload: Workload{
					Name: aws.String("subscribers"),
					Type: aws.String(BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./subscribers/Dockerfile"),
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
					Network: &NetworkConfig{
						VPC: &vpcConfig{
							Placement: stringP("public"),
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
				HealthCheck: &ContainerHealthCheck{
					Command: []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
				},
				Port: 8080,
			},
			wantedManifest: &BackendService{
				Workload: Workload{
					Name: aws.String("subscribers"),
					Type: aws.String(BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Location: aws.String("mockImage"),
							},
							Port: aws.Uint16(8080),
						},
						HealthCheck: &ContainerHealthCheck{
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
					Network: &NetworkConfig{
						VPC: &vpcConfig{
							Placement: stringP("public"),
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

func TestBackendSvc_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		inProps BackendServiceProps

		wantedTestdata string
	}{
		"without healthcheck and port": {
			inProps: BackendServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
			},
			wantedTestdata: "backend-svc-nohealthcheck.yml",
		},
		"with custom healthcheck command": {
			inProps: BackendServiceProps{
				WorkloadProps: WorkloadProps{
					Name:  "subscribers",
					Image: "flask-sample",
				},
				HealthCheck: &ContainerHealthCheck{
					Command:     []string{"CMD-SHELL", "curl -f http://localhost:8080 || exit 1"},
					Interval:    durationp(6 * time.Second),
					Retries:     aws.Int(0),
					Timeout:     durationp(20 * time.Second),
					StartPeriod: durationp(15 * time.Second),
				},
				Port: 8080,
			},
			wantedTestdata: "backend-svc-customhealthcheck.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestdata)
			wantedBytes, err := ioutil.ReadFile(path)
			require.NoError(t, err)
			manifest := NewBackendService(tc.inProps)

			// WHEN
			tpl, err := manifest.MarshalBinary()
			require.NoError(t, err)

			// THEN
			require.Equal(t, string(wantedBytes), string(tpl))
		})
	}
}

func TestBackendSvc_ApplyEnv(t *testing.T) {
	mockBackendServiceWithNoEnvironments := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
					Image: Image{
						Build: BuildArgsOrString{
							BuildArgs: DockerBuildArgs{
								Dockerfile: aws.String("./Dockerfile"),
							},
						},
					},
					Port: aws.Uint16(8080),
				},
				HealthCheck: &ContainerHealthCheck{
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
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
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
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
					Port: aws.Uint16(80),
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"test": {
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Port: aws.Uint16(5000),
					},
				},
			},
		},
	}
	mockBackendServiceWithAllOverride := BackendService{
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
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
					Port:  aws.String("2000/udp"),
					Image: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
				},
			},
			Logging: &Logging{
				Destination: map[string]string{
					"Name":            "datadog",
					"exclude-pattern": "*",
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"test": {
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
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
							CPU: aws.Int(70),
						},
					},
					CPU: aws.Int(512),
					Variables: map[string]string{
						"LOG_LEVEL": "",
					},
				},
				Sidecars: map[string]*SidecarConfig{
					"xray": {
						CredsParam: aws.String("some arn"),
					},
				},
				Logging: &Logging{
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
			Type: aws.String(BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
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
		Environments: map[string]*BackendServiceConfig{
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
	}
	mockBackendServiceWithImageOverrideLocationByLocation := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
					Image: Image{
						Location: aws.String("original location"),
					},
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
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
	}
	mockBackendServiceWithImageOverrideBuildByBuild := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
					Image: Image{
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
		Environments: map[string]*BackendServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("env overridden dockerfile"),
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
			Type: aws.String(BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: ImageWithPortAndHealthcheck{
				ImageWithPort: ImageWithPort{
					Image: Image{
						Location: aws.String("original location"),
					},
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("env overridden dockerfile"),
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
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
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
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
								CPU: aws.Int(70),
							},
						},
						Variables: map[string]string{
							"LOG_LEVEL": "",
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
					Type: aws.String(BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Location: aws.String("env-override location"),
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
					Type: aws.String(BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Location: aws.String("env-override location"),
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
					Type: aws.String(BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("env overridden dockerfile"),
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
					Type: aws.String(BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: ImageWithPortAndHealthcheck{
						ImageWithPort: ImageWithPort{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("env overridden dockerfile"),
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
			got, _ := tc.svc.ApplyEnv(tc.inEnvName)

			// Should override properly.
			require.Equal(t, tc.wanted, got)
			// Should not impact the original manifest struct.
			require.Equal(t, tc.svc, tc.original)
		})
	}
}

func TestBackendSvc_ApplyEnv_CountOverrides(t *testing.T) {
	mockRange := IntRangeBand("1-10")
	testCases := map[string]struct {
		svcCount Count
		envCount Count

		expected *BackendService
	}{
		"empty env advanced count override": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{Value: &mockRange},
					CPU:   aws.Int(80),
				},
			},
			envCount: Count{},
			expected: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: &Range{Value: &mockRange},
								CPU:   aws.Int(80),
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
					Range: &Range{Value: &mockRange},
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
					Range: &Range{Value: &mockRange},
				},
			},
			envCount: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{
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
								Range: &Range{
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
			actual, _ := svc.ApplyEnv("test")

			// THEN
			require.Equal(t, tc.expected, actual)
		})
	}
}
