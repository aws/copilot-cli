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

func TestNewBackendSvc(t *testing.T) {
	testCases := map[string]struct {
		inProps BackendSvcProps

		wantedManifest *BackendSvc
	}{
		"without healthcheck": {
			inProps: BackendSvcProps{
				SvcProps: SvcProps{
					SvcName:    "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				Port: 8080,
			},
			wantedManifest: &BackendSvc{
				Svc: Svc{
					Name: "subscribers",
					Type: BackendService,
				},
				Image: imageWithPortAndHealthcheck{
					SvcImageWithPort: SvcImageWithPort{
						SvcImage: SvcImage{
							Build: "./subscribers/Dockerfile",
						},
						Port: 8080,
					},
				},
				TaskConfig: TaskConfig{
					CPU:    256,
					Memory: 512,
					Count:  intp(1),
				},
			},
		},
		"with custom healthcheck command": {
			inProps: BackendSvcProps{
				SvcProps: SvcProps{
					SvcName:    "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				HealthCheck: &ContainerHealthCheck{
					Command: []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
				},
				Port: 8080,
			},
			wantedManifest: &BackendSvc{
				Svc: Svc{
					Name: "subscribers",
					Type: BackendService,
				},
				Image: imageWithPortAndHealthcheck{
					SvcImageWithPort: SvcImageWithPort{
						SvcImage: SvcImage{
							Build: "./subscribers/Dockerfile",
						},
						Port: 8080,
					},
					HealthCheck: &ContainerHealthCheck{
						Command:     []string{"CMD", "curl -f http://localhost:8080 || exit 1"},
						Interval:    durationp(10 * time.Second),
						Retries:     intp(2),
						Timeout:     durationp(5 * time.Second),
						StartPeriod: durationp(0 * time.Second),
					},
				},
				TaskConfig: TaskConfig{
					CPU:    256,
					Memory: 512,
					Count:  intp(1),
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
			actualBytes, err := yaml.Marshal(NewBackendSvc(tc.inProps))
			require.NoError(t, err)

			require.Equal(t, string(wantedBytes), string(actualBytes))
		})
	}
}

func TestBackendSvc_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		inProps BackendSvcProps

		wantedTestdata string
	}{
		"without healthcheck": {
			inProps: BackendSvcProps{
				SvcProps: SvcProps{
					SvcName:    "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				Port: 8080,
			},
			wantedTestdata: "backend-svc-nohealthcheck.yml",
		},
		"with custom healthcheck command": {
			inProps: BackendSvcProps{
				SvcProps: SvcProps{
					SvcName:    "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
				},
				HealthCheck: &ContainerHealthCheck{
					Command:     []string{"CMD-SHELL", "curl -f http://localhost:8080 || exit 1"},
					Interval:    durationp(6 * time.Second),
					Retries:     intp(0),
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
			manifest := NewBackendSvc(tc.inProps)

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
	manifest := NewBackendSvc(BackendSvcProps{
		SvcProps: SvcProps{
			SvcName:    "subscribers",
			Dockerfile: "./subscribers/Dockerfile",
		},
		Port: 8080,
	})

	require.Equal(t, "./subscribers/Dockerfile", manifest.DockerfilePath())
}

func TestBackendSvc_ApplyEnv(t *testing.T) {
	testCases := map[string]struct {
		svc       *BackendSvc
		inEnvName string

		wanted *BackendSvc
	}{
		"environment doesn't exist": {
			svc: &BackendSvc{
				Svc: Svc{
					Name: "phonetool",
					Type: BackendService,
				},
				Image: imageWithPortAndHealthcheck{
					SvcImageWithPort: SvcImageWithPort{
						SvcImage: SvcImage{
							Build: "./Dockerfile",
						},
						Port: 8080,
					},
					HealthCheck: &ContainerHealthCheck{
						Command:     []string{"hello", "world"},
						Interval:    durationp(1 * time.Second),
						Retries:     intp(100),
						Timeout:     durationp(100 * time.Minute),
						StartPeriod: durationp(5 * time.Second),
					},
				},
				TaskConfig: TaskConfig{
					CPU:    256,
					Memory: 256,
					Count:  intp(1),
				},
			},
			inEnvName: "test",

			wanted: &BackendSvc{
				Svc: Svc{
					Name: "phonetool",
					Type: BackendService,
				},
				Image: imageWithPortAndHealthcheck{
					SvcImageWithPort: SvcImageWithPort{
						SvcImage: SvcImage{
							Build: "./Dockerfile",
						},
						Port: 8080,
					},
					HealthCheck: &ContainerHealthCheck{
						Command:     []string{"hello", "world"},
						Interval:    durationp(1 * time.Second),
						Retries:     intp(100),
						Timeout:     durationp(100 * time.Minute),
						StartPeriod: durationp(5 * time.Second),
					},
				},
				TaskConfig: TaskConfig{
					CPU:    256,
					Memory: 256,
					Count:  intp(1),
				},
			},
		},
		"uses env overrides": {
			svc: &BackendSvc{
				TaskConfig: TaskConfig{
					CPU:    256,
					Memory: 256,
					Count:  intp(1),
				},
				Environments: map[string]TaskConfig{
					"test": {
						Count: intp(0),
						Variables: map[string]string{
							"LOG_LEVEL": "DEBUG",
						},
					},
				},
			},
			inEnvName: "test",

			wanted: &BackendSvc{
				TaskConfig: TaskConfig{
					CPU:    256,
					Memory: 256,
					Count:  intp(0),
					Variables: map[string]string{
						"LOG_LEVEL": "DEBUG",
					},
					Secrets: map[string]string{},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.svc.ApplyEnv(tc.inEnvName)

			require.Equal(t, tc.wanted, got)
		})
	}
}
