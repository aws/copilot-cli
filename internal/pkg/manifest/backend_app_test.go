// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewBackendApp(t *testing.T) {
	testCases := map[string]struct {
		inProps BackendAppProps

		wantedManifest *BackendApp
	}{
		"without healthcheck": {
			inProps: BackendAppProps{
				AppProps: AppProps{
					AppName:    "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				Port: 8080,
			},
			wantedManifest: &BackendApp{
				App: App{
					Name: "subscribers",
					Type: BackendApplication,
				},
				Image: imageWithPortAndHealthcheck{
					AppImageWithPort: AppImageWithPort{
						AppImage: AppImage{
							Build: "./subscribers/Dockerfile",
						},
						Port: 8080,
					},
				},
				TaskConfig: TaskConfig{
					CPU:    256,
					Memory: 512,
					Count:  1,
				},
			},
		},
		"with custom healthcheck command": {
			inProps: BackendAppProps{
				AppProps: AppProps{
					AppName:    "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				HealthCheck: &ContainerHealthCheck{
					Command: []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
				},
				Port: 8080,
			},
			wantedManifest: &BackendApp{
				App: App{
					Name: "subscribers",
					Type: BackendApplication,
				},
				Image: imageWithPortAndHealthcheck{
					AppImageWithPort: AppImageWithPort{
						AppImage: AppImage{
							Build: "./subscribers/Dockerfile",
						},
						Port: 8080,
					},
					HealthCheck: &ContainerHealthCheck{
						Command:     []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
						Interval:    durationPointer(10 * time.Second),
						Retries:     intPointer(2),
						Timeout:     durationPointer(5 * time.Second),
						StartPeriod: durationPointer(0 * time.Second),
					},
				},
				TaskConfig: TaskConfig{
					CPU:    256,
					Memory: 512,
					Count:  1,
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
			actualBytes, err := yaml.Marshal(NewBackendApp(tc.inProps))
			require.NoError(t, err)

			require.Equal(t, string(wantedBytes), string(actualBytes))
		})
	}
}

func TestBackendApp_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		inProps BackendAppProps

		wantedTestdata string
	}{
		"without healthcheck": {
			inProps: BackendAppProps{
				AppProps: AppProps{
					AppName:    "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				Port: 8080,
			},
			wantedTestdata: "backend-app-nohealthcheck.yml",
		},
		"with custom healthcheck command": {
			inProps: BackendAppProps{
				AppProps: AppProps{
					AppName:    "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				HealthCheck: &ContainerHealthCheck{
					Command:     []string{"CMD-SHELL", "curl -f http://localhost:8080 || exit 1"},
					Interval:    durationPointer(6 * time.Second),
					Retries:     intPointer(0),
					Timeout:     durationPointer(20 * time.Second),
					StartPeriod: durationPointer(15 * time.Second),
				},
				Port: 8080,
			},
			wantedTestdata: "backend-app-customhealthcheck.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestdata)
			wantedBytes, err := ioutil.ReadFile(path)
			require.NoError(t, err)
			manifest := NewBackendApp(tc.inProps)

			// WHEN
			tpl, err := manifest.MarshalBinary()
			require.NoError(t, err)

			// THEN
			require.Equal(t, string(wantedBytes), string(tpl))
		})
	}
}

func TestBackendApp_DockerfilePath(t *testing.T) {
	// GIVEN
	manifest := NewBackendApp(BackendAppProps{
		AppProps: AppProps{
			AppName:    "subscribers",
			Dockerfile: "./subscribers/Dockerfile",
		},
		Port: 8080,
	})

	require.Equal(t, "./subscribers/Dockerfile", manifest.DockerfilePath())
}

func durationPointer(dur time.Duration) *time.Duration {
	return &dur
}
func intPointer(i int) *int {
	return &i
}
