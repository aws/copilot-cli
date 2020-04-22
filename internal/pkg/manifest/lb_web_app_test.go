// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

func TestLoadBalancedWebApp_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, manifest *LoadBalancedWebApp)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *LoadBalancedWebApp) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(lbWebAppManifestPath, *manifest).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *LoadBalancedWebApp) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(lbWebAppManifestPath, *manifest).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			manifest := &LoadBalancedWebApp{}
			tc.mockDependencies(ctrl, manifest)

			// WHEN
			b, err := manifest.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestLoadBalancedWebApp_ApplyEnv(t *testing.T) {
	testCases := map[string]struct {
		in         *LoadBalancedWebApp
		envToApply string

		wanted *LoadBalancedWebApp
	}{
		"with no existing environments": {
			in: &LoadBalancedWebApp{
				App: App{
					Name: "phonetool",
					Type: LoadBalancedWebApplication,
				},
				Image: AppImageWithPort{
					AppImage: AppImage{
						Build: "./Dockerfile",
					},
					Port: 80,
				},
				LoadBalancedWebAppConfig: LoadBalancedWebAppConfig{
					RoutingRule: RoutingRule{
						Path:            "/awards/*",
						HealthCheckPath: "/",
					},
					TaskConfig: TaskConfig{
						CPU:    1024,
						Memory: 1024,
						Count:  intp(1),
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebApp{
				App: App{
					Name: "phonetool",
					Type: LoadBalancedWebApplication,
				},
				Image: AppImageWithPort{
					AppImage: AppImage{
						Build: "./Dockerfile",
					},
					Port: 80,
				},
				LoadBalancedWebAppConfig: LoadBalancedWebAppConfig{
					RoutingRule: RoutingRule{
						Path:            "/awards/*",
						HealthCheckPath: "/",
					},
					TaskConfig: TaskConfig{
						CPU:    1024,
						Memory: 1024,
						Count:  intp(1),
					},
				},
			},
		},
		"with overrides": {
			in: &LoadBalancedWebApp{
				App: App{
					Name: "phonetool",
					Type: LoadBalancedWebApplication,
				},
				Image: AppImageWithPort{
					AppImage: AppImage{
						Build: "./Dockerfile",
					},
					Port: 80,
				},
				LoadBalancedWebAppConfig: LoadBalancedWebAppConfig{
					RoutingRule: RoutingRule{
						Path:            "/awards/*",
						HealthCheckPath: "/",
					},
					TaskConfig: TaskConfig{
						CPU:    1024,
						Memory: 1024,
						Count:  intp(1),
						Variables: map[string]string{
							"LOG_LEVEL":      "DEBUG",
							"DDB_TABLE_NAME": "awards",
						},
						Secrets: map[string]string{
							"GITHUB_TOKEN": "1111",
							"TWILIO_TOKEN": "1111",
						},
					},
				},
				Environments: map[string]LoadBalancedWebAppConfig{
					"prod-iad": {
						TaskConfig: TaskConfig{
							CPU:   2046,
							Count: intp(0),
							Variables: map[string]string{
								"DDB_TABLE_NAME": "awards-prod",
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &LoadBalancedWebApp{
				App: App{
					Name: "phonetool",
					Type: LoadBalancedWebApplication,
				},
				Image: AppImageWithPort{
					AppImage: AppImage{
						Build: "./Dockerfile",
					},
					Port: 80,
				},
				LoadBalancedWebAppConfig: LoadBalancedWebAppConfig{
					RoutingRule: RoutingRule{
						Path:            "/awards/*",
						HealthCheckPath: "/",
					},
					TaskConfig: TaskConfig{
						CPU:    2046,
						Memory: 1024,
						Count:  intp(0),
						Variables: map[string]string{
							"LOG_LEVEL":      "DEBUG",
							"DDB_TABLE_NAME": "awards-prod",
						},
						Secrets: map[string]string{
							"GITHUB_TOKEN": "1111",
							"TWILIO_TOKEN": "1111",
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN

			// WHEN
			conf := tc.in.ApplyEnv(tc.envToApply)

			// THEN
			require.Equal(t, tc.wanted, conf, "returned configuration should have overrides from the environment")
		})
	}
}
