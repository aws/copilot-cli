// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	compose "github.com/compose-spec/compose-go/types"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type decomposeTest struct {
	filename string
	svcName  string
	workDir  string

	wantLbws          *manifest.LoadBalancedWebServiceConfig
	wantBs            *manifest.BackendServiceConfig
	wantIgnored       IgnoredKeys
	wantError         error
	wantErrorContains string
}

func runDecomposeTests(t *testing.T, testCases map[string]decomposeTest) {
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", tc.filename)
			cfg, err := os.ReadFile(path)
			require.NoError(t, err)

			svc, ign, err := DecomposeService(cfg, tc.svcName, filepath.Join("testdata", tc.workDir))

			if tc.wantErrorContains != "" {
				require.ErrorContains(t, err, tc.wantErrorContains)
			} else if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantIgnored, ign)

				if tc.wantLbws != nil {
					require.NotNil(t, svc.LbSvc)
					require.Nil(t, svc.BackendSvc)
					require.Equal(t, tc.wantLbws, &svc.LbSvc.LoadBalancedWebServiceConfig)
				} else {
					require.Nil(t, svc.LbSvc)
					require.NotNil(t, svc.BackendSvc)
					require.Equal(t, tc.wantBs, &svc.BackendSvc.BackendServiceConfig)
				}
			}
		})
	}
}

func TestDecomposeService_General(t *testing.T) {
	fiveSeconds := compose.Duration(5 * time.Second)
	threeSeconds := compose.Duration(3 * time.Second)
	oneSecond := compose.Duration(time.Second)

	testCases := map[string]decomposeTest{
		"no services": {
			filename: "empty-compose.yml",
			svcName:  "test",

			wantError: errors.New("compose file has no services"),
		},
		"bad services": {
			filename: "bad-services-compose.yml",
			svcName:  "test",

			wantError: errors.New("\"services\" top-level element was not a map, was: invalid"),
		},
		"wrong name": {
			filename: "unsupported-keys.yml",
			svcName:  "test",

			wantError: errors.New("no service named \"test\" in this Compose file, valid services are: fatal1, fatal2, fatal3"),
		},
		"invalid service not a map": {
			filename: "bad-service-compose.yml",
			svcName:  "bad",

			wantError: errors.New("\"services.bad\" element was not a map"),
		},
		"unsupported keys fatal1": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal1",

			wantError: errors.New("\"services.fatal1\" relies on fatally-unsupported Compose keys: external_links, privileged"),
		},
		"unsupported keys fatal2": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal2",

			wantError: errors.New("convert Compose service to Copilot manifest: `build.ssh` and `build.secrets` are not supported yet, see https://github.com/aws/copilot-cli/issues/2090 for details"),
		},
		"unsupported keys fatal3": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal3",

			wantError: errors.New("\"services.fatal3\" relies on fatally-unsupported Compose keys: domainname, init, networks"),
		},
		"invalid compose": {
			filename: "invalid-compose.yml",
			svcName:  "invalid",

			wantError: errors.New("load Compose project: services.invalid.build.ssh must be a mapping"),
		},
		"nginx-golang-postgres backend": {
			filename: "nginx-golang-postgres.yml",
			svcName:  "backend",

			wantError: errors.New("\"services.backend\" relies on fatally-unsupported Compose keys: secrets"),
		},
		"nginx-golang-postgres db": {
			filename: "nginx-golang-postgres.yml",
			svcName:  "db",

			wantError: errors.New("\"services.db\" relies on fatally-unsupported Compose keys: secrets"),
		},
		"nginx-golang-postgres proxy": {
			filename: "nginx-golang-postgres.yml",
			svcName:  "proxy",

			wantError: errors.New("\"services.proxy\" relies on fatally-unsupported Compose keys: volumes"),
		},
		"react-express-mongo frontend": {
			filename: "react-express-mongo.yml",
			svcName:  "frontend",

			wantError: errors.New("\"services.frontend\" relies on fatally-unsupported Compose keys: networks"),
		},
		"react-express-mongo backend": {
			filename: "react-express-mongo.yml",
			svcName:  "backend",

			wantError: errors.New("\"services.backend\" relies on fatally-unsupported Compose keys: networks"),
		},
		"react-express-mongo mongo": {
			filename: "react-express-mongo.yml",
			svcName:  "mongo",

			wantError: errors.New("\"services.mongo\" relies on fatally-unsupported Compose keys: networks"),
		},
		"unrecognized-field-name": {
			filename: "unrecognized-field-name.yml",
			svcName:  "complete",

			wantError: errors.New("load Compose project: services.complete.healthcheck Additional property exthealthcheck is not allowed"),
		},
		"extends": {
			filename: "extends/extending.yml",
			svcName:  "web",
			workDir:  "extends",

			wantBs: &manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
						},
						Port: aws.Uint16(8096),
					},
				},
				TaskConfig: manifest.TaskConfig{
					CPU:    aws.Int(256),
					Memory: aws.Int(512),
					Count: manifest.Count{
						Value: aws.Int(1),
					},
				},
			},
		},
		"multiple env files": {
			filename: "single-service.yml",
			svcName:  "multiple-env-files",

			wantError: errors.New("convert Compose service to Copilot manifest: at most one env file is supported, but 3 env files were attached to this service"),
		},
		"single-service complete": {
			filename: "single-service.yml",
			svcName:  "complete",

			wantIgnored: []string{
				"oom_score_adj",
				"runtime",
				"userns_mode",
			},
			wantLbws: &manifest.LoadBalancedWebServiceConfig{
				ImageConfig: manifest.ImageWithPortAndHealthcheck{
					ImageWithPort: manifest.ImageWithPort{
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
					EnvFile: aws.String("/file-that-does-not-exist.env"),
					Variables: map[string]string{
						"HOST_PATH":    "/home/nginx",
						"ENABLE_HTTPS": "true",
					},
					CPU:    aws.Int(256),
					Memory: aws.Int(512),
					Count: manifest.Count{
						Value: aws.Int(1),
					},
				},
			},
		},
	}

	runDecomposeTests(t, testCases)
}

func TestDecomposeService_ExposedPorts(t *testing.T) {
	taskCfg := manifest.TaskConfig{
		CPU:    aws.Int(256),
		Memory: aws.Int(512),
		Count: manifest.Count{
			Value: aws.Int(1),
		},
	}
	nginxNoPorts := manifest.BackendServiceConfig{
		ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
			ImageWithOptionalPort: manifest.ImageWithOptionalPort{
				Image: manifest.Image{
					Location: aws.String("nginx"),
				},
			},
		},
		TaskConfig: taskCfg,
	}
	nginxLbws8096 := manifest.LoadBalancedWebServiceConfig{
		ImageConfig: manifest.ImageWithPortAndHealthcheck{
			ImageWithPort: manifest.ImageWithPort{
				Image: manifest.Image{
					Location: aws.String("nginx"),
				},
				Port: aws.Uint16(8096),
			},
		},
		TaskConfig: taskCfg,
	}

	testCases := []decomposeTest{
		{
			svcName:           "two-public-ports",
			wantErrorContains: "convert Compose service to Copilot manifest: cannot expose more than one public port in Copilot, but 2 ports are exposed publicly",
		},
		{
			svcName:   "two-exposed-ports",
			wantError: errors.New("convert Compose service to Copilot manifest: cannot expose more than one port in Copilot, but 2 ports are exposed: 80, 443"),
		},
		{
			svcName: "no-exposed-ports",
			wantBs:  &nginxNoPorts,
		},
		{
			svcName: "different-public-and-exposed",
			wantLbws: &manifest.LoadBalancedWebServiceConfig{
				ImageConfig: manifest.ImageWithPortAndHealthcheck{
					ImageWithPort: manifest.ImageWithPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
						},
						Port: aws.Uint16(432),
					},
				},
				TaskConfig: taskCfg,
			},
		},
		{
			svcName:  "expose-and-ports",
			wantLbws: &nginxLbws8096,
		},
		{
			svcName:   "remap-ports",
			wantError: errors.New("convert Compose service to Copilot manifest: cannot publish the container port 80 under a different public port 8080 in Copilot"),
		},
		{
			svcName:   "remap-ports-long-form",
			wantError: errors.New("convert Compose service to Copilot manifest: cannot publish the container port 80 under a different public port 8080 in Copilot"),
		},
		{
			svcName:   "invalid-expose",
			wantError: errors.New("convert Compose service to Copilot manifest: could not parse exposed port: strconv.Atoi: parsing \"pony\": invalid syntax"),
		},
		{
			filename:          "invalid-ports.yml",
			svcName:           "invalid-ports",
			wantErrorContains: "load Compose project: ",
		},
		{
			svcName: "no-ports",
			wantBs:  &nginxNoPorts,
		},
		{
			svcName: "expose-only",
			wantBs: &manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
						},
						Port: aws.Uint16(80),
					},
				},
				TaskConfig: taskCfg,
			},
		},
		{
			svcName: "parsed-from-dockerfile",
			wantBs: &manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Build: manifest.BuildArgsOrString{
								BuildArgs: manifest.DockerBuildArgs{
									Context:    aws.String("buildtest"),
									Dockerfile: aws.String("Dockerfile-expose"),
								},
							},
						},
						Port: aws.Uint16(8096),
					},
				},
				TaskConfig: taskCfg,
			},
		},
		{
			svcName: "dockerfile-no-expose",
			wantBs: &manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Build: manifest.BuildArgsOrString{
								BuildArgs: manifest.DockerBuildArgs{
									Context:    aws.String("buildtest"),
									Dockerfile: aws.String("Dockerfile-no-expose"),
								},
							},
						},
					},
				},
				TaskConfig: taskCfg,
			},
		},
		{
			svcName:           "dockerfile-cant-parse",
			wantErrorContains: "convert Compose service to Copilot manifest: parse dockerfile for exposed ports: ",
		},
		{
			svcName:           "dockerfile-doesnt-exist",
			wantErrorContains: "convert Compose service to Copilot manifest: parse dockerfile for exposed ports: open Dockerfile:",
		},
		{
			svcName:  "public-port-container-only",
			wantLbws: &nginxLbws8096,
		},
		{
			svcName:  "public-host-ip",
			wantLbws: &nginxLbws8096,

			wantIgnored: []string{"ports.<port>.host_ip"},
		},
		{
			svcName:   "public-port-range-to-one",
			wantError: errors.New("convert Compose service to Copilot manifest: cannot map a published port range (8000-9000) to a single container port (80) yet"),
		},
		{
			svcName:           "public-port-range-to-range",
			wantErrorContains: "convert Compose service to Copilot manifest: cannot expose more than one public port in Copilot, but 1001 ports are exposed publicly: ",
		},
		{
			svcName:  "public-port-complete",
			wantLbws: &nginxLbws8096,

			wantIgnored: []string{"ports.<port>.mode"},
		},
	}

	actualTestCases := map[string]decomposeTest{}

	for _, tc := range testCases {
		if tc.filename == "" {
			tc.filename = "exposed-port-tests.yml"
		}
		actualTestCases[tc.svcName] = tc
	}

	runDecomposeTests(t, actualTestCases)
}
