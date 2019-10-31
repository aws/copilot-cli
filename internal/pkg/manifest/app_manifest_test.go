// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
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
				_, ok := i.(*LBFargateManifest)
				require.True(t, ok)
			},
		},
		"invalid app type": {
			inAppName:    "CowApp",
			inAppType:    "Cow App",
			inDockerfile: "CowApp/Dockerfile",

			wantedErr: &ErrInvalidAppManifestType{Type: "Cow App"},
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

func TestUnmarshalApp(t *testing.T) {
	testCases := map[string]struct {
		inContent string

		requireCorrectValues func(t *testing.T, i interface{})
		wantedErr            error
	}{
		"load balanced web application": {
			inContent: `
version: 1.0
name: frontend
type: "Load Balanced Web App"
image:
  build: frontend/Dockerfile
  port: 80
cpu: 512
memory: 1024
count: 1
http:
  path: "*"
public: false
variables:
  LOG_LEVEL: "WARN"
secrets:
  DB_PASSWORD: MYSQL_DB_PASSWORD
scaling:
  minCount: 1
  maxCount: 50
  targetMemory: 60
environments:
  test:
    count: 3
    public: true
`,
			requireCorrectValues: func(t *testing.T, i interface{}) {
				actualManifest, ok := i.(*LBFargateManifest)
				require.True(t, ok)
				wantedManifest := &LBFargateManifest{
					AppManifest: AppManifest{Name: "frontend", Type: LoadBalancedWebApplication},
					Image:       ImageWithPort{AppImage: AppImage{Build: "frontend/Dockerfile"}, Port: 80},
					LBFargateConfig: LBFargateConfig{
						RoutingRule: RoutingRule{
							Path: "*",
						},
						ContainersConfig: ContainersConfig{
							CPU:    512,
							Memory: 1024,
							Count:  1,
							Variables: map[string]string{
								"LOG_LEVEL": "WARN",
							},
							Secrets: map[string]string{
								"DB_PASSWORD": "MYSQL_DB_PASSWORD",
							},
						},
						Public: aws.Bool(false),
						Scaling: &AutoScalingConfig{
							MinCount:     1,
							MaxCount:     50,
							TargetMemory: 60.0,
						},
					},
					Environments: map[string]LBFargateConfig{
						"test": {
							ContainersConfig: ContainersConfig{
								Count: 3,
							},
							Public: aws.Bool(true),
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
			wantedErr: &ErrInvalidAppManifestType{Type: "OH NO"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := UnmarshalApp([]byte(tc.inContent))

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				tc.requireCorrectValues(t, m)
			}
		})
	}
}
