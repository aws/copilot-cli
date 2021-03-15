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
					ImageConfig: imageWithPortAndHealthcheck{
						ServiceImageWithPort: ServiceImageWithPort{
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
					Network: NetworkConfig{
						VPC: vpcConfig{
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
					ImageConfig: imageWithPortAndHealthcheck{
						ServiceImageWithPort: ServiceImageWithPort{
							Image: Image{
								Location: aws.String("mockImage"),
							},
							Port: aws.Uint16(8080),
						},
						HealthCheck: &ContainerHealthCheck{
							Command:     []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
							Interval:    durationp(10 * time.Second),
							Retries:     aws.Int(2),
							Timeout:     durationp(5 * time.Second),
							StartPeriod: durationp(0 * time.Second),
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
	mockBackendServiceWithNoOverride := BackendService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: imageWithPortAndHealthcheck{
				ServiceImageWithPort: ServiceImageWithPort{
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
	mockBackendServiceWithMinimalOverride := BackendService{
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: imageWithPortAndHealthcheck{
				ServiceImageWithPort: ServiceImageWithPort{
					Port: aws.Uint16(80),
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"test": {
				ImageConfig: imageWithPortAndHealthcheck{
					ServiceImageWithPort: ServiceImageWithPort{
						Port: aws.Uint16(5000),
					},
				},
			},
		},
	}
	mockBackendServiceWithAllOverride := BackendService{
		BackendServiceConfig: BackendServiceConfig{
			ImageConfig: imageWithPortAndHealthcheck{
				ServiceImageWithPort: ServiceImageWithPort{
					Port: aws.Uint16(80),
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
				TaskConfig: TaskConfig{
					Count: Count{
						Autoscaling: Autoscaling{
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
	testCases := map[string]struct {
		svc       *BackendService
		inEnvName string

		wanted   *BackendService
		original *BackendService
	}{
		"no env override": {
			svc:       &mockBackendServiceWithNoOverride,
			inEnvName: "test",

			wanted:   &mockBackendServiceWithNoOverride,
			original: &mockBackendServiceWithNoOverride,
		},
		"uses env minimal overrides": {
			svc:       &mockBackendServiceWithMinimalOverride,
			inEnvName: "test",

			wanted: &BackendService{
				BackendServiceConfig: BackendServiceConfig{
					ImageConfig: imageWithPortAndHealthcheck{
						ServiceImageWithPort: ServiceImageWithPort{
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
					ImageConfig: imageWithPortAndHealthcheck{
						ServiceImageWithPort: ServiceImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(512),
						Memory: aws.Int(256),
						Count: Count{
							Value: aws.Int(1),
							Autoscaling: Autoscaling{
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
