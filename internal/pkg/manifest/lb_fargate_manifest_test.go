// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestLBFargateManifest_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, manifest *LBFargateManifest)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *LBFargateManifest) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(lbFargateManifestPath, *manifest).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *LBFargateManifest) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(lbFargateManifestPath, *manifest).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			manifest := &LBFargateManifest{}
			tc.mockDependencies(ctrl, manifest)

			// WHEN
			b, err := manifest.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestLBFargateManifest_EnvConf(t *testing.T) {
	testCases := map[string]struct {
		inDefaultConfig  LBFargateConfig
		inEnvNameToQuery string
		inEnvOverride    map[string]LBFargateConfig

		wantedConfig LBFargateConfig
	}{
		"with no existing environments": {
			inDefaultConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    1024,
					Memory: 1024,
					Count:  1,
				},
			},
			inEnvNameToQuery: "prod-iad",

			wantedConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    1024,
					Memory: 1024,
					Count:  1,
				},
			},
		},
		"with partial overrides": {
			inDefaultConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    1024,
					Memory: 1024,
					Count:  1,
					Variables: map[string]string{
						"LOG_LEVEL":      "DEBUG",
						"DDB_TABLE_NAME": "awards",
					},
					Secrets: map[string]string{
						"GITHUB_TOKEN": "1111",
						"TWILIO_TOKEN": "1111",
					},
				},
				Scaling: &AutoScalingConfig{
					MinCount:     1,
					MaxCount:     2,
					TargetCPU:    75.0,
					TargetMemory: 0,
				},
			},
			inEnvNameToQuery: "prod-iad",
			inEnvOverride: map[string]LBFargateConfig{
				"prod-iad": {
					ContainersConfig: ContainersConfig{
						CPU: 2046,
						Variables: map[string]string{
							"DDB_TABLE_NAME": "awards-prod",
						},
					},
					Scaling: &AutoScalingConfig{
						MaxCount: 5,
					},
				},
			},

			wantedConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    2046,
					Memory: 1024,
					Count:  1,
					Variables: map[string]string{
						"LOG_LEVEL":      "DEBUG",
						"DDB_TABLE_NAME": "awards-prod",
					},
					Secrets: map[string]string{
						"GITHUB_TOKEN": "1111",
						"TWILIO_TOKEN": "1111",
					},
				},
				Scaling: &AutoScalingConfig{
					MinCount:  1,
					MaxCount:  5,
					TargetCPU: 75.0,
				},
			},
		},
		"with complete override": {
			inDefaultConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    1024,
					Memory: 1024,
					Count:  1,
				},
				Scaling: &AutoScalingConfig{
					MinCount:     1,
					MaxCount:     2,
					TargetCPU:    75.0,
					TargetMemory: 0,
				},
			},
			inEnvNameToQuery: "prod-iad",
			inEnvOverride: map[string]LBFargateConfig{
				"prod-iad": {
					RoutingRule: RoutingRule{Path: "/frontend*"},
					ContainersConfig: ContainersConfig{
						CPU:    2046,
						Memory: 2046,
						Count:  3,
						Variables: map[string]string{
							"LOG_LEVEL":      "WARN",
							"DDB_TABLE_NAME": "awards-prod",
						},
						Secrets: map[string]string{
							"GITHUB_TOKEN": "2222",
							"TWILIO_TOKEN": "2222",
						},
					},
					Scaling: &AutoScalingConfig{
						MinCount:     2,
						MaxCount:     5,
						TargetCPU:    75.0,
						TargetMemory: 75.0,
					},
				},
			},

			wantedConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/frontend*"},
				ContainersConfig: ContainersConfig{
					CPU:    2046,
					Memory: 2046,
					Count:  3,
					Variables: map[string]string{
						"LOG_LEVEL":      "WARN",
						"DDB_TABLE_NAME": "awards-prod",
					},
					Secrets: map[string]string{
						"GITHUB_TOKEN": "2222",
						"TWILIO_TOKEN": "2222",
					},
				},
				Scaling: &AutoScalingConfig{
					MinCount:     2,
					MaxCount:     5,
					TargetCPU:    75.0,
					TargetMemory: 75.0,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			m := &LBFargateManifest{
				LBFargateConfig: tc.inDefaultConfig,
				Environments:    tc.inEnvOverride,
			}

			// WHEN
			conf := m.EnvConf(tc.inEnvNameToQuery)

			// THEN
			require.Equal(t, tc.wantedConfig, conf, "returned configuration should have overrides from the environment")
			require.Equal(t, m.LBFargateConfig, tc.inDefaultConfig, "values in the default configuration should not be overwritten")
		})
	}
}
