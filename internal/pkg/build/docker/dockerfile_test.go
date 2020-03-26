// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"bufio"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestParseDockerfile(t *testing.T) {
	testCases := map[string]struct {
		mockDockerfile string
		err            error
		wantedConfig   DockerfileConfig
	}{
		"correctly parses directly exposed port": {
			mockDockerfile: `EXPOSE 5000`,
			err:            nil,
			wantedConfig: DockerfileConfig{
				ExposedPorts: []PortConfig{
					{
						Port:      5000,
						Protocol:  "",
						RawString: "5000",
					},
				},
			},
		},
		"correctly parses exposed port and protocol": {
			mockDockerfile: `EXPOSE 5000/tcp`,
			err:            nil,
			wantedConfig: DockerfileConfig{
				ExposedPorts: []PortConfig{
					{
						Port:      5000,
						Protocol:  "tcp",
						RawString: "5000/tcp",
					},
				},
			},
		},
		"multiple ports with one expose line": {
			mockDockerfile: `EXPOSE 5000/tcp 8080/tcp 6000`,
			err:            nil,
			wantedConfig: DockerfileConfig{
				ExposedPorts: []PortConfig{
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
			methods := getLineParseMethods()
			r := strings.NewReader(tc.mockDockerfile)
			scanner := bufio.NewScanner(r)
			got, err := parseDockerfileFromScanner(scanner, methods)

			if tc.err != nil {
				require.EqualError(t, err, tc.err.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedConfig, got)
			}
		})
	}

}

func TestDockerfileInterface(t *testing.T) {
	testCases := map[string]struct {
		mockDockerfile []byte
		wantedPorts    []PortConfig
	}{
		"one exposed port": {
			mockDockerfile: []byte("EXPOSE 8080"),
			wantedPorts: []PortConfig{
				PortConfig{
					Port:      8080,
					RawString: "8080",
				},
			},
		},
		"two exposed ports": {
			mockDockerfile: []byte(`
EXPOSE 8080
EXPOSE 80`),
			wantedPorts: []PortConfig{
				PortConfig{
					Port:      8080,
					RawString: "8080",
				},
				PortConfig{
					Port:      80,
					RawString: "80",
				},
			},
		},
		"two exposed ports one line": {
			mockDockerfile: []byte("EXPOSE 80/tcp 3000"),
			wantedPorts: []PortConfig{
				PortConfig{
					Port:      80,
					Protocol:  "tcp",
					RawString: "80/tcp",
				},
				PortConfig{
					Port:      3000,
					RawString: "3000",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fs := afero.Afero{Fs: afero.NewMemMapFs()}
			err := fs.WriteFile("./Dockerfile", tc.mockDockerfile, 0644)
			df := NewDockerfileConfig(fs, "./Dockerfile")

			err = df.parseDockerfile()

			require.NoError(t, err)

			require.Equal(t, tc.wantedPorts, df.ExposedPorts)

		})
	}
}
