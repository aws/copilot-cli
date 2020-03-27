// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package dockerfile

import (
	"bufio"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestParseDockerfile(t *testing.T) {
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
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			r := strings.NewReader(tc.dockerfile)
			scanner := bufio.NewScanner(r)
			got := parseFromScanner(scanner)
			require.Equal(t, tc.wantedConfig, got)
		})
	}

}

func getUintPorts(inPorts []portConfig) []uint16 {
	if len(inPorts) == 0 {
		return []uint16(nil)
	}
	var ports []uint16
	for _, p := range inPorts {
		ports = append(ports, p.Port)
	}
	return ports
}

func TestDockerfileInterface(t *testing.T) {
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
		"bad expose token": {
			dockerfilePath: wantedPath,
			dockerfile: []byte(`
EXPOSE 80
EXPOSE $arg
EXPOSE 8080/tcp 5000`),
			wantedPorts: []portConfig{
				{
					Port:      80,
					RawString: "80",
				},
				{
					Port:      0,
					RawString: "EXPOSE $arg",
				},
				{
					Port:      8080,
					Protocol:  "tcp",
					RawString: "8080/tcp",
				},
				{
					Port:      5000,
					RawString: "5000",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			wantedUintPorts := getUintPorts(tc.wantedPorts)
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			err := fs.WriteFile("./Dockerfile", tc.dockerfile, 0644)
			if tc.wantedErr != nil {
				require.EqualError(t, tc.wantedErr, err.Error())
			} else {
				require.NoError(t, err)
			}
			df := New(fs, "./Dockerfile")

			ports := df.GetExposedPorts()

			require.Equal(t, wantedUintPorts, ports)

			require.Equal(t, tc.wantedPorts, df.ExposedPorts)

		})
	}
}
