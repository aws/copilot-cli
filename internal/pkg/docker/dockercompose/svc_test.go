// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	compose "github.com/compose-spec/compose-go/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConvertBackendService(t *testing.T) {
	fiveSeconds := compose.Duration(5 * time.Second)
	threeSeconds := compose.Duration(3 * time.Second)
	oneSecond := compose.Duration(time.Second)

	testCases := map[string]struct {
		inSvc  compose.ServiceConfig
		inPort uint16

		wantBackendSvc manifest.BackendServiceConfig
		wantIgnored    IgnoredKeys
		wantError      error
	}{
		"happy path trivial image": {
			inSvc: compose.ServiceConfig{
				Name:  "web",
				Image: "nginx",
			},
			inPort: 8080,

			wantBackendSvc: manifest.BackendServiceConfig{ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: manifest.ImageWithOptionalPort{
					Image: manifest.Image{
						Location: aws.String("nginx"),
					},
					Port: aws.Uint16(8080),
				},
			}},
		},
		"happy path complete": {
			inSvc: compose.ServiceConfig{
				Name: "web",

				Command: compose.ShellCommand{
					"CMD-SHELL",
					"/bin/nginx",
				},
				Entrypoint: compose.ShellCommand{
					"CMD",
					"/bin/sh",
				},
				EnvFile: []string{"/test.env"},
				Environment: map[string]*string{
					"HOST_PATH":    aws.String("/home/nginx"),
					"ENABLE_HTTPS": aws.String("true"),
				},
				Platform: "linux/arm64",
				HealthCheck: &compose.HealthCheckConfig{
					Test: []string{
						"CMD",
						"/bin/echo",
					},
					Timeout:     &fiveSeconds,
					Interval:    &oneSecond,
					Retries:     aws.Uint64(100),
					StartPeriod: &threeSeconds,
				},
				Labels: map[string]string{
					"docker.test":  "val",
					"docker.test2": "val2",
				},
				Image: "nginx",
				Build: &compose.BuildConfig{
					Context:    "dir",
					Dockerfile: "dir/Dockerfile",
					Args: map[string]*string{
						"GIT_COMMIT": aws.String("323189ab"),
						"ARG2":       aws.String("VAL"),
					},
					CacheFrom: []string{"example.com"},
					Target:    "myapp",
				},
			},
			inPort: 443,

			wantBackendSvc: manifest.BackendServiceConfig{ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: manifest.ImageWithOptionalPort{
					Image: manifest.Image{
						Location: aws.String("nginx"),
						DockerLabels: map[string]string{
							"docker.test":  "val",
							"docker.test2": "val2",
						},
					},
					Port: aws.Uint16(443),
				},
				HealthCheck: manifest.ContainerHealthCheck{
					Command: []string{
						"CMD",
						"/bin/echo",
					},
					Timeout:     (*time.Duration)(&fiveSeconds),
					Interval:    (*time.Duration)(&oneSecond),
					Retries:     aws.Int(100),
					StartPeriod: (*time.Duration)(&threeSeconds),
				},
			},
				ImageOverride: manifest.ImageOverride{
					Command: manifest.CommandOverride{
						StringSlice: []string{
							"CMD-SHELL",
							"/bin/nginx",
						},
					},
					EntryPoint: manifest.EntryPointOverride{
						StringSlice: []string{
							"CMD",
							"/bin/sh",
						},
					},
				},
				TaskConfig: manifest.TaskConfig{
					Platform: manifest.PlatformArgsOrString{
						PlatformString: (*manifest.PlatformString)(aws.String("linux/arm64")),
					},
					Variables: map[string]string{
						"HOST_PATH":    "/home/nginx",
						"ENABLE_HTTPS": "true",
					},
					EnvFile: aws.String("/test.env"),
				}},
		},
		"multiple env files": {
			inSvc: compose.ServiceConfig{
				Name:  "web",
				Image: "nginx",
				EnvFile: []string{
					"/envfile1.env",
					"/envfile2.env",
				},
			},
			inPort: 8080,

			wantError: errors.New("at most one env file is supported, but 2 env files were attached to this service"),
		},
		"env variables with missing values": {
			inSvc: compose.ServiceConfig{
				Name:  "web",
				Image: "nginx",
				Environment: map[string]*string{
					"test":  nil,
					"test2": aws.String("value"),
				},
			},
			inPort: 8080,

			wantError: errors.New("convert environment variables: entry '[test]' is missing " +
				"a value; this is unsupported in Copilot"),
		},
		"platform windows": {
			inSvc: compose.ServiceConfig{
				Name:     "web",
				Image:    "nginx",
				Platform: "windows",
			},
			inPort: 8080,

			wantBackendSvc: manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
						},
						Port: aws.Uint16(8080),
					},
				},
				TaskConfig: manifest.TaskConfig{
					Platform: manifest.PlatformArgsOrString{
						PlatformString: (*manifest.PlatformString)(aws.String("windows")),
					},
				},
			},
		},
		"partial healthcheck": {
			inSvc: compose.ServiceConfig{
				Name: "web",
				HealthCheck: &compose.HealthCheckConfig{
					Timeout:     &fiveSeconds,
					StartPeriod: &threeSeconds,
				},
				Image: "nginx",
			},
			inPort: 443,

			wantBackendSvc: manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
						},
						Port: aws.Uint16(443),
					},
					HealthCheck: manifest.ContainerHealthCheck{
						Retries:     aws.Int(3),
						Timeout:     (*time.Duration)(&fiveSeconds),
						StartPeriod: (*time.Duration)(&threeSeconds),
					},
				},
			},
		},
		"disabled healthcheck": {
			inSvc: compose.ServiceConfig{
				Name: "web",
				HealthCheck: &compose.HealthCheckConfig{
					Timeout:     &fiveSeconds,
					StartPeriod: &threeSeconds,
					Disable:     true,
				},
				Image: "nginx",
			},
			inPort: 443,

			wantBackendSvc: manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
						},
						Port: aws.Uint16(443),
					},
					HealthCheck: manifest.ContainerHealthCheck{
						Command:     []string{"NONE"},
						Retries:     aws.Int(3),
						Timeout:     (*time.Duration)(&fiveSeconds),
						StartPeriod: (*time.Duration)(&threeSeconds),
					},
				},
			},
		},
		"disabled healthcheck with cmd": {
			inSvc: compose.ServiceConfig{
				Name: "web",
				HealthCheck: &compose.HealthCheckConfig{
					Test: []string{
						"CMD",
						"/bin/echo",
					},
					Timeout:     &fiveSeconds,
					StartPeriod: &threeSeconds,
					Disable:     true,
				},
				Image: "nginx",
			},
			inPort: 443,

			wantBackendSvc: manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
						},
						Port: aws.Uint16(443),
					},
					HealthCheck: manifest.ContainerHealthCheck{
						Command:     []string{"NONE"},
						Retries:     aws.Int(3),
						Timeout:     (*time.Duration)(&fiveSeconds),
						StartPeriod: (*time.Duration)(&threeSeconds),
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			svc, ignored, err := convertBackendService(&tc.inSvc, tc.inPort)

			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantIgnored, ignored)
				require.Equal(t, tc.wantBackendSvc, *svc)
			}
		})
	}
}
