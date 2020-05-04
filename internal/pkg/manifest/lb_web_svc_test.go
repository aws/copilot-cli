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

func TestLoadBalancedWebSvc_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, manifest *LoadBalancedWebService)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *LoadBalancedWebService) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(lbWebSvcManifestPath, *manifest).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *LoadBalancedWebService) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(lbWebSvcManifestPath, *manifest).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			manifest := &LoadBalancedWebService{}
			tc.mockDependencies(ctrl, manifest)

			// WHEN
			b, err := manifest.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestLoadBalancedWebSvc_ApplyEnv(t *testing.T) {
	testCases := map[string]struct {
		in         *LoadBalancedWebService
		envToApply string

		wanted *LoadBalancedWebService
	}{
		"with no existing environments": {
			in: &LoadBalancedWebService{
				Service: Service{
					Name: "phonetool",
					Type: LoadBalancedWebServiceType,
				},
				Image: ServiceImageWithPort{
					ServiceImage: ServiceImage{
						Build: "./Dockerfile",
					},
					Port: 80,
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
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

			wanted: &LoadBalancedWebService{
				Service: Service{
					Name: "phonetool",
					Type: LoadBalancedWebServiceType,
				},
				Image: ServiceImageWithPort{
					ServiceImage: ServiceImage{
						Build: "./Dockerfile",
					},
					Port: 80,
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
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
			in: &LoadBalancedWebService{
				Service: Service{
					Name: "phonetool",
					Type: LoadBalancedWebServiceType,
				},
				Image: ServiceImageWithPort{
					ServiceImage: ServiceImage{
						Build: "./Dockerfile",
					},
					Port: 80,
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
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
				Environments: map[string]LoadBalancedWebServiceConfig{
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

			wanted: &LoadBalancedWebService{
				Service: Service{
					Name: "phonetool",
					Type: LoadBalancedWebServiceType,
				},
				Image: ServiceImageWithPort{
					ServiceImage: ServiceImage{
						Build: "./Dockerfile",
					},
					Port: 80,
				},
				LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
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
