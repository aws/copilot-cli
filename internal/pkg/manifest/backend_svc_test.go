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
		"without healthcheck": {
			inProps: BackendServiceProps{
				ServiceProps: ServiceProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				Port: 8080,
			},
			wantedManifest: &BackendService{
				Service: Service{
					Name: aws.String("subscribers"),
					Type: aws.String(BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					Image: imageWithPortAndHealthcheck{
						ServiceImageWithPort: ServiceImageWithPort{
							ServiceImage: ServiceImage{
								Build: aws.String("./subscribers/Dockerfile"),
							},
							Port: aws.Uint16(8080),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count:  aws.Int(1),
					},
				},
			},
		},
		"with custom healthcheck command": {
			inProps: BackendServiceProps{
				ServiceProps: ServiceProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				HealthCheck: &ContainerHealthCheck{
					Command: []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
				},
				Port: 8080,
			},
			wantedManifest: &BackendService{
				Service: Service{
					Name: aws.String("subscribers"),
					Type: aws.String(BackendServiceType),
				},
				BackendServiceConfig: BackendServiceConfig{
					Image: imageWithPortAndHealthcheck{
						ServiceImageWithPort: ServiceImageWithPort{
							ServiceImage: ServiceImage{
								Build: aws.String("./subscribers/Dockerfile"),
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
						Count:  aws.Int(1),
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
		"without healthcheck": {
			inProps: BackendServiceProps{
				ServiceProps: ServiceProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				Port: 8080,
			},
			wantedTestdata: "backend-svc-nohealthcheck.yml",
		},
		"with custom healthcheck command": {
			inProps: BackendServiceProps{
				ServiceProps: ServiceProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
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

func TestBackendSvc_DockerfilePath(t *testing.T) {
	// GIVEN
	manifest := NewBackendService(BackendServiceProps{
		ServiceProps: ServiceProps{
			Name:       "subscribers",
			Dockerfile: "./subscribers/Dockerfile",
		},
		Port: 8080,
	})

	require.Equal(t, "./subscribers/Dockerfile", manifest.DockerfilePath())
}

func TestBackendSvc_ApplyEnv(t *testing.T) {
	mockBackendServiceWithNoOverride := BackendService{
		Service: Service{
			Name: aws.String("phonetool"),
			Type: aws.String(BackendServiceType),
		},
		BackendServiceConfig: BackendServiceConfig{
			Image: imageWithPortAndHealthcheck{
				ServiceImageWithPort: ServiceImageWithPort{
					ServiceImage: ServiceImage{
						Build: aws.String("./Dockerfile"),
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
				Count:  aws.Int(1),
			},
		},
	}
	mockBackendServiceWithMinimalOverride := BackendService{
		BackendServiceConfig: BackendServiceConfig{
			Image: imageWithPortAndHealthcheck{
				ServiceImageWithPort: ServiceImageWithPort{
					Port: aws.Uint16(80),
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"test": {
				Image: imageWithPortAndHealthcheck{
					ServiceImageWithPort: ServiceImageWithPort{
						Port: aws.Uint16(5000),
					},
				},
			},
		},
	}
	mockBackendServiceWithAllOverride := BackendService{
		BackendServiceConfig: BackendServiceConfig{
			Image: imageWithPortAndHealthcheck{
				ServiceImageWithPort: ServiceImageWithPort{
					Port: aws.Uint16(80),
				},
			},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(256),
				Count:  aws.Int(1),
			},
			Sidecar: Sidecar{
				Sidecars: map[string]*SidecarConfig{
					"xray": {
						Port:  aws.String("2000/udp"),
						Image: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
					},
				},
			},
			LogConfig: &LogConfig{
				Destination: map[string]string{
					"Name":            "datadog",
					"exclude-pattern": "*",
				},
			},
		},
		Environments: map[string]*BackendServiceConfig{
			"test": {
				TaskConfig: TaskConfig{
					Count: aws.Int(0),
					CPU:   aws.Int(512),
					Variables: map[string]string{
						"LOG_LEVEL": "",
					},
				},
				Sidecar: Sidecar{
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							CredsParam: aws.String("some arn"),
						},
					},
				},
				LogConfig: &LogConfig{
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
					Image: imageWithPortAndHealthcheck{
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
					Image: imageWithPortAndHealthcheck{
						ServiceImageWithPort: ServiceImageWithPort{
							Port: aws.Uint16(80),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(512),
						Memory: aws.Int(256),
						Count:  aws.Int(0),
						Variables: map[string]string{
							"LOG_LEVEL": "",
						},
					},
					Sidecar: Sidecar{
						Sidecars: map[string]*SidecarConfig{
							"xray": {
								Port:       aws.String("2000/udp"),
								Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
								CredsParam: aws.String("some arn"),
							},
						},
					},
					LogConfig: &LogConfig{
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
