// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConvertBackendService(t *testing.T) {
	fiveSeconds := types.Duration(5 * time.Second)
	threeSeconds := types.Duration(3 * time.Second)
	oneSecond := types.Duration(time.Second)

	testCases := map[string]struct {
		inSvc  types.ServiceConfig
		inPort uint16

		wantBackendSvc manifest.BackendServiceConfig
		wantIgnored    IgnoredKeys
		wantError      error
	}{
		"happy path trivial image": {
			inSvc: types.ServiceConfig{
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
			inSvc: types.ServiceConfig{
				Name: "web",

				Command: types.ShellCommand{
					"CMD-SHELL",
					"/bin/nginx",
				},
				Entrypoint: types.ShellCommand{
					"CMD",
					"/bin/sh",
				},
				EnvFile: []string{"/test.env"},
				Environment: map[string]*string{
					"HOST_PATH":    aws.String("/home/nginx"),
					"ENABLE_HTTPS": aws.String("true"),
				},
				Platform: "linux/arm64",
				HealthCheck: &types.HealthCheckConfig{
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
				Build: &types.BuildConfig{
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
			inSvc: types.ServiceConfig{
				Name:  "web",
				Image: "nginx",
				EnvFile: []string{
					"/envfile1.env",
					"/envfile2.env",
				},
			},
			inPort: 8080,

			wantError: errors.New("convert task config: at most one env file is supported, but 2 env files were attached to this service"),
		},
		"env variables with missing values": {
			inSvc: types.ServiceConfig{
				Name:  "web",
				Image: "nginx",
				Environment: map[string]*string{
					"test":  nil,
					"test2": aws.String("value"),
				},
			},
			inPort: 8080,

			wantError: errors.New("convert task config: convert environment variables: entry '[test]' is missing " +
				"a value and requires user input, this is unsupported in Copilot"),
		},
		"platform windows": {
			inSvc: types.ServiceConfig{
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
			inSvc: types.ServiceConfig{
				Name: "web",
				HealthCheck: &types.HealthCheckConfig{
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
			inSvc: types.ServiceConfig{
				Name: "web",
				HealthCheck: &types.HealthCheckConfig{
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
			inSvc: types.ServiceConfig{
				Name: "web",
				HealthCheck: &types.HealthCheckConfig{
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
		"extension fields": {
			wantIgnored: []string{
				"build.ext",
				"healthcheck.extfield",
				"unrecognized",
			},

			inSvc: types.ServiceConfig{
				Name: "web",
				HealthCheck: &types.HealthCheckConfig{
					Extensions: map[string]interface{}{
						"extfield": 1,
					},
				},
				Build: &types.BuildConfig{
					Context: "here/",
					Extensions: map[string]interface{}{
						"ext": "field",
					},
				},
				Extensions: map[string]interface{}{
					"unrecognized": "ignored",
				},
			},
			inPort: 443,

			wantBackendSvc: manifest.BackendServiceConfig{ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
				ImageWithOptionalPort: manifest.ImageWithOptionalPort{
					Image: manifest.Image{
						Build: manifest.BuildArgsOrString{BuildArgs: manifest.DockerBuildArgs{
							Context: aws.String("here/"),
						}},
					},
					Port: aws.Uint16(443),
				},
				HealthCheck: manifest.ContainerHealthCheck{
					Retries: aws.Int(3),
				},
			}},
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
