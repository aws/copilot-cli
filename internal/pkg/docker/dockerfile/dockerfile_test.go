// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerfile

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestDockerfile_GetExposedPort(t *testing.T) {
	wantedPath := "./Dockerfile"
	testCases := map[string]struct {
		dockerfilePath string
		dockerfile     []byte
		wantedPorts    []Port
		wantedErr      error
	}{
		"no exposed ports": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
	FROM nginx
	ARG arg=80`),
			wantedPorts: []Port{},
			wantedErr:   ErrNoExpose{Dockerfile: wantedPath},
		},
		"one exposed port": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
FROM nginx
EXPOSE 8080
`),
			wantedPorts: []Port{
				{
					Port:      8080,
					RawString: "8080",
				},
			},
		},
		"two exposed ports": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
FROM nginx
EXPOSE 8080
EXPOSE 80`),
			wantedPorts: []Port{
				{
					Port:      8080,
					RawString: "8080",
				},
				{
					Port:      80,
					RawString: "80",
				},
			},
		},
		"two exposed ports one line": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
FROM nginx
EXPOSE 80/tcp 3000`),
			wantedPorts: []Port{
				{
					Port:      80,
					Protocol:  "tcp",
					RawString: "80/tcp",
				},
				{
					Port:      3000,
					RawString: "3000",
				},
			},
		},
		"bad expose token single port": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
FROM nginx
EXPOSE $arg
`),
			wantedPorts: []Port{
				{
					RawString: "EXPOSE $arg",
					err: ErrInvalidPort{
						Match: "EXPOSE $arg",
					},
				},
			},
			wantedErr: ErrInvalidPort{Match: "EXPOSE $arg"},
		},
		"bad expose token multiple ports": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
FROM nginx
EXPOSE 80
EXPOSE $arg
EXPOSE 8080/tcp 5000`),
			wantedPorts: nil,
			wantedErr:   ErrInvalidPort{Match: "EXPOSE $arg"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			err := fs.WriteFile("./Dockerfile", tc.dockerfile, 0644)
			if err != nil {
				t.FailNow()
			}

			// Ensure the dockerfile is parse-able by Docker.
			dat, err := fs.ReadFile("./Dockerfile")
			require.NoError(t, err)
			ast, err := parser.Parse(bytes.NewReader(dat))
			require.NoError(t, err)
			stages, _, err := instructions.Parse(ast.AST)
			require.NoError(t, err)

			ports, err := New(fs, "./Dockerfile").GetExposedPorts()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedPorts, ports, "expected ports do not match")

				// Compare our parsing against Docker's.
				var portsFromDocker []string
				for _, cmd := range stages[0].Commands {
					switch v := cmd.(type) {
					case *instructions.ExposeCommand:
						portsFromDocker = append(portsFromDocker, v.Ports...)
					}
				}
				require.ElementsMatch(t, portsFromDocker, stringifyPorts(ports), "ports from Docker do not match")
			}
		})
	}
}

func TestDockerfile_GetHealthCheck(t *testing.T) {
	testCases := map[string]struct {
		dockerfilePath string
		dockerfile     []byte
		wantedConfig   *HealthCheck
		wantedErr      error
	}{
		"correctly parses healthcheck with default values": {
			dockerfile: []byte(`
FROM nginx
HEALTHCHECK CMD curl -f http://localhost/ || exit 1
`),
			wantedErr: nil,
			wantedConfig: &HealthCheck{
				Interval:    10 * time.Second,
				Timeout:     5 * time.Second,
				StartPeriod: 0,
				Retries:     2,
				Cmd:         []string{cmdShell, "curl -f http://localhost/ || exit 1"},
			},
		},
		"correctly parses healthcheck with user's values": {
			dockerfile: []byte(`
FROM nginx
HEALTHCHECK --interval=5m --timeout=3s --start-period=2s --retries=3 \
	CMD curl -f http://localhost/ || exit 1`),
			wantedErr: nil,
			wantedConfig: &HealthCheck{
				Interval:    300 * time.Second,
				Timeout:     3 * time.Second,
				StartPeriod: 2 * time.Second,
				Retries:     3,
				Cmd:         []string{cmdShell, "curl -f http://localhost/ || exit 1"},
			},
		},
		"correctly parses healthcheck with NONE": {
			dockerfile: []byte(`
FROM nginx
HEALTHCHECK NONE
`),
			wantedErr:    nil,
			wantedConfig: nil,
		},
		"correctly parses no healthchecks": {
			dockerfile:   []byte(`FROM nginx`),
			wantedErr:    nil,
			wantedConfig: nil,
		},
		"correctly parses HEALTHCHECK instruction with awkward spacing": {
			dockerfile: []byte(`
FROM nginx
HEALTHCHECK   CMD   a b
`),
			wantedErr: nil,
			wantedConfig: &HealthCheck{
				Interval: 10 * time.Second,
				Timeout:  5 * time.Second,
				Retries:  2,
				Cmd:      []string{cmdShell, "a b"},
			},
		},
		"correctly parses HEALTHCHECK instruction with exec array format": {
			dockerfile: []byte(`
FROM nginx
EXPOSE 80
HEALTHCHECK   CMD     ["a",    "b"]
`),
			wantedErr: nil,
			wantedConfig: &HealthCheck{
				Interval: 10 * time.Second,
				Timeout:  5 * time.Second,
				Retries:  2,
				Cmd:      []string{cmdShell, "a", "b"},
			},
		},
		"healthcheck contains an invalid flag": {
			dockerfile: []byte(`HEALTHCHECK --interval=5m --randomFlag=4s CMD curl -f http://localhost/ || exit 1`),
			wantedErr:  fmt.Errorf("parse instructions: Unknown flag: randomFlag"),
		},
		"healthcheck does not contain CMD": {
			dockerfile: []byte(`HEALTHCHECK --interval=5m curl -f http://localhost/ || exit 1`),
			wantedErr:  fmt.Errorf("parse instructions: Unknown type \"CURL\" in HEALTHCHECK (try CMD)"),
		},
		"healthcheck does not contain command": {
			dockerfile: []byte(`HEALTHCHECK --interval=5m CMD`),
			wantedErr:  fmt.Errorf("parse instructions: Missing command after HEALTHCHECK CMD"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			err := fs.WriteFile("./Dockerfile", tc.dockerfile, 0644)
			if err != nil {
				t.FailNow()
			}

			hc, err := New(fs, "./Dockerfile").GetHealthCheck()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantedConfig, hc)
		})
	}
}

func stringifyPorts(ports []Port) []string {
	var arr []string
	for _, p := range ports {
		if p.err != nil {
			continue
		}
		arr = append(arr, p.String())
	}
	return arr
}
