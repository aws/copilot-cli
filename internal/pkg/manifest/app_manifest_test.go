// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	testCases := map[string]struct {
		inAppName    string
		inAppType    string
		inDockerfile string

		requireCorrectType func(t *testing.T, i interface{})
		wantedErr          error
	}{
		"load balanced web application": {
			inAppName:    "ChickenApp",
			inAppType:    LoadBalancedWebApplication,
			inDockerfile: "ChickenApp/Dockerfile",

			requireCorrectType: func(t *testing.T, i interface{}) {
				_, ok := i.(*LoadBalancedFargateManifest)
				require.True(t, ok)
			},
		},
		"invalid app type": {
			inAppName:    "CowApp",
			inAppType:    "Cow App",
			inDockerfile: "CowApp/Dockerfile",

			wantedErr: &ErrInvalidManifestType{Type: "Cow App"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := CreateApp(tc.inAppName, tc.inAppType, tc.inDockerfile)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				tc.requireCorrectType(t, m)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	testCases := map[string]struct {
		inContent string

		requireCorrectValues func(t *testing.T, i interface{})
		wantedErr            error
	}{
		"load balanced web application": {
			inContent: `
name: ChickenApp
image:
  build: ChickenApp/Dockerfile
  port: 5000
type: 'Load Balanced Web App'
containerPort: 8080
cpu: 2048
memory: 1024
logging: true
public: false
stages:
  -
    env: test
    desiredCount: 2
`,
			requireCorrectValues: func(t *testing.T, i interface{}) {
				actualManifest, ok := i.(*LoadBalancedFargateManifest)
				require.True(t, ok)
				wantedManifest := &LoadBalancedFargateManifest{
					AppManifest: AppManifest{Name: "ChickenApp", Type: LoadBalancedWebApplication},
					Image:       LBFargateImage{AppImage: AppImage{Build: "ChickenApp/Dockerfile"}, Port: 5000},
					CPU:         2048,
					Memory:      1024,
					Logging:     true,
					Public:      false,
					Stages: []AppStage{
						{
							EnvName:      "test",
							DesiredCount: 2,
						},
					},
				}
				require.Equal(t, wantedManifest, actualManifest)
			},
		},
		"invalid app type": {
			inContent: `
name: CowApp
type: 'OH NO'
`,
			wantedErr: &ErrInvalidManifestType{Type: "OH NO"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := Unmarshal([]byte(tc.inContent))

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				tc.requireCorrectValues(t, m)
			}
		})
	}
}
