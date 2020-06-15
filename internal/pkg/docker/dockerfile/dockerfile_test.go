// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package dockerfile

import (
	"fmt"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestParseExposeDockerfile(t *testing.T) {
	testCases := map[string]struct {
		dockerfile   string
		err          error
		wantedConfig Dockerfile
	}{
		"correctly parses directly exposed port": {
			dockerfile: `EXPOSE 5000`,
			err:        nil,
			wantedConfig: Dockerfile{
				ExposedPorts: []portConfig{
					{
						Port:      5000,
						Protocol:  "",
						RawString: "5000",
					},
				},
				HealthCheck: nil,
			},
		},
		"correctly parses exposed port and protocol": {
			dockerfile: `EXPOSE 5000/tcp`,
			err:        nil,
			wantedConfig: Dockerfile{
				ExposedPorts: []portConfig{
					{
						Port:      5000,
						Protocol:  "tcp",
						RawString: "5000/tcp",
					},
				},
				HealthCheck: nil,
			},
		},
		"multiple ports with one expose line": {
			dockerfile: `EXPOSE 5000/tcp 8080/tcp 6000`,
			err:        nil,
			wantedConfig: Dockerfile{
				ExposedPorts: []portConfig{
					{
						Port:      5000,
						Protocol:  "tcp",
						RawString: "5000/tcp",
					},
					{
						Port:      8080,
						Protocol:  "tcp",
						RawString: "8080/tcp",
					},
					{
						Port:      6000,
						Protocol:  "",
						RawString: "6000",
					},
				},
				HealthCheck: nil,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, _ := parse(tc.dockerfile)
			require.Equal(t, tc.wantedConfig, *got)
		})
	}
}

func getUintPorts(inPorts []portConfig) []uint16 {
	if len(inPorts) == 0 {
		return []uint16(nil)
	}
	var ports []uint16
	for _, p := range inPorts {
		if p.Port != 0 {
			ports = append(ports, p.Port)
		}
	}
	return ports
}

func TestDockerfile_GetExposedPort(t *testing.T) {
	wantedPath := "./Dockerfile"
	testCases := map[string]struct {
		dockerfilePath string
		dockerfile     []byte
		wantedPorts    []portConfig
		wantedErr      error
	}{
		"no exposed ports": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
	FROM nginx
	ARG arg=80`),
			wantedPorts: []portConfig{},
			wantedErr:   ErrNoExpose{Dockerfile: wantedPath},
		},
		"one exposed port": {
			dockerfilePath: wantedPath,
			dockerfile:     []byte("EXPOSE 8080"),
			wantedPorts: []portConfig{
				{
					Port:      8080,
					RawString: "8080",
				},
			},
		},
		"two exposed ports": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
EXPOSE 8080
EXPOSE 80`),
			wantedPorts: []portConfig{
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
			dockerfile:     []byte("EXPOSE 80/tcp 3000"),
			wantedPorts: []portConfig{
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
			dockerfile:     []byte(`EXPOSE $arg`),
			wantedPorts: []portConfig{
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
EXPOSE 80
EXPOSE $arg
EXPOSE 8080/tcp 5000`),
			wantedPorts: nil,
			wantedErr:   ErrInvalidPort{Match: "EXPOSE $arg"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			wantedUintPorts := getUintPorts(tc.wantedPorts)
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			err := fs.WriteFile("./Dockerfile", tc.dockerfile, 0644)
			if err != nil {
				t.FailNow()
			}

			df := New(fs, "./Dockerfile")

			ports, err := df.GetExposedPorts()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, wantedUintPorts, ports)

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
			dockerfile: []byte(`HEALTHCHECK CMD curl -f http://localhost/ || exit 1`),
			wantedErr:  nil,
			wantedConfig: &HealthCheck{
				Interval:    10 * time.Second,
				Timeout:     5 * time.Second,
				StartPeriod: 0,
				Retries:     2,
				Cmd:         []string{"CMD curl -f http://localhost/ || exit 1"},
			},
		},
		"correctly parses healthcheck with user's values": {
			dockerfile: []byte(`HEALTHCHECK --interval=5m --timeout=3s --start-period=2s --retries=3 \
			CMD curl -f http://localhost/ || exit 1`),
			wantedErr: nil,
			wantedConfig: &HealthCheck{
				Interval:    300 * time.Second,
				Timeout:     3 * time.Second,
				StartPeriod: 2 * time.Second,
				Retries:     3,
				Cmd:         []string{"CMD curl -f http://localhost/ || exit 1"},
			},
		},
		"correctly parses healthcheck with NONE": {
			dockerfile:   []byte(`HEALTHCHECK NONE`),
			wantedErr:    nil,
			wantedConfig: nil,
		},
		"healthcheck contains an invalid flag": {
			dockerfile: []byte(`HEALTHCHECK --interval=5m --randomFlag=4s CMD curl -f http://localhost/ || exit 1`),
			wantedErr:  fmt.Errorf("parse instructions: Unknown flag: randomFlag"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			err := fs.WriteFile("./Dockerfile", tc.dockerfile, 0644)
			if err != nil {
				t.FailNow()
			}

			df := New(fs, "./Dockerfile")
			hc, err := df.GetHealthCheck()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantedConfig, hc)
		})
	}
}
