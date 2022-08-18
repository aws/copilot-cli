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

func TestDecomposeService(t *testing.T) {
	fiveSeconds := compose.Duration(5 * time.Second)
	threeSeconds := compose.Duration(3 * time.Second)
	oneSecond := compose.Duration(time.Second)

	testCases := map[string]struct {
		filename string
		svcName  string
		workDir  string

		wantSvc     manifest.BackendServiceConfig
		wantIgnored IgnoredKeys
		wantError   error
	}{
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

			wantError: errors.New("no service named \"test\" in this Compose file, valid services are: [fatal1 fatal2 fatal3]"),
		},
		"invalid service not a map": {
			filename: "bad-service-compose.yml",
			svcName:  "bad",

			wantError: errors.New("\"services.bad\" element was not a map"),
		},
		"unsupported keys fatal1": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal1",

			wantError: errors.New("get unsupported service keys: \"services.fatal1\" relies on fatally-unsupported Compose keys: [external_links privileged]"),
		},
		"unsupported keys fatal2": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal2",

			wantError: errors.New("convert Compose service to Copilot manifest: convert image config: `build.ssh` and `build.secrets` are not supported yet, see https://github.com/aws/copilot-cli/issues/2090 for details"),
		},
		"unsupported keys fatal3": {
			filename: "unsupported-keys.yml",
			svcName:  "fatal3",

			wantError: errors.New("get unsupported service keys: \"services.fatal3\" relies on fatally-unsupported Compose keys: [domainname init networks]"),
		},
		"invalid compose": {
			filename: "invalid-compose.yml",
			svcName:  "invalid",

			wantError: errors.New("load Compose project: services.invalid.build.ssh must be a mapping"),
		},
		"nginx-golang-postgres backend": {
			filename: "nginx-golang-postgres.yml",
			svcName:  "backend",

			wantError: errors.New("get unsupported service keys: \"services.backend\" relies on fatally-unsupported Compose keys: [secrets]"),
		},
		"nginx-golang-postgres db": {
			filename: "nginx-golang-postgres.yml",
			svcName:  "db",

			wantError: errors.New("get unsupported service keys: \"services.db\" relies on fatally-unsupported Compose keys: [expose secrets volumes]"),
		},
		"nginx-golang-postgres proxy": {
			filename: "nginx-golang-postgres.yml",
			svcName:  "proxy",

			wantError: errors.New("get unsupported service keys: \"services.proxy\" relies on fatally-unsupported Compose keys: [ports volumes]"),
		},
		"react-express-mongo frontend": {
			filename: "react-express-mongo.yml",
			svcName:  "frontend",

			wantError: errors.New("get unsupported service keys: \"services.frontend\" relies on fatally-unsupported Compose keys: [networks ports volumes]"),
		},
		"react-express-mongo backend": {
			filename: "react-express-mongo.yml",
			svcName:  "backend",

			wantError: errors.New("get unsupported service keys: \"services.backend\" relies on fatally-unsupported Compose keys: [expose networks volumes]"),
		},
		"react-express-mongo mongo": {
			filename: "react-express-mongo.yml",
			svcName:  "mongo",

			wantError: errors.New("get unsupported service keys: \"services.mongo\" relies on fatally-unsupported Compose keys: [expose networks volumes]"),
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

			wantSvc: manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
						},
						Port: aws.Uint16(80),
					},
				},
			},
		},
		"single-service complete": {
			filename: "single-service.yml",
			svcName:  "complete",

			wantIgnored: []string{
				"oom_score_adj",
				"runtime",
				"userns_mode",
			},
			wantSvc: manifest.BackendServiceConfig{
				ImageConfig: manifest.ImageWithHealthcheckAndOptionalPort{
					ImageWithOptionalPort: manifest.ImageWithOptionalPort{
						Image: manifest.Image{
							Location: aws.String("nginx"),
							DockerLabels: map[string]string{
								"docker.test":  "val",
								"docker.test2": "val2",
							},
						},
						Port: aws.Uint16(80),
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
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", tc.filename)
			cfg, err := os.ReadFile(path)
			require.NoError(t, err)

			svc, ign, err := DecomposeService(cfg, tc.svcName, filepath.Join("testdata", tc.workDir))

			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, &tc.wantSvc, svc)
				require.Equal(t, tc.wantIgnored, ign)
			}
		})
	}
}
