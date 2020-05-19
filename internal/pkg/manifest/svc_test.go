// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalSvc(t *testing.T) {
	testCases := map[string]struct {
		inContent string

		requireCorrectValues func(t *testing.T, i interface{})
		wantedErr            error
	}{
		"load balanced web service": {
			inContent: `
version: 1.0
name: frontend
type: "Load Balanced Web Service"
image:
  build: frontend/Dockerfile
  port: 80
cpu: 512
memory: 1024
count: 1
http:
  path: "svc"
variables:
  LOG_LEVEL: "WARN"
secrets:
  DB_PASSWORD: MYSQL_DB_PASSWORD
sidecars:
  xray:
    port: 2000/udp
    image: 123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon
    credentialsParameter: some arn
environments:
  test:
    count: 3
`,
			requireCorrectValues: func(t *testing.T, i interface{}) {
				actualManifest, ok := i.(*LoadBalancedWebService)
				require.True(t, ok)
				wantedManifest := &LoadBalancedWebService{
					Service: Service{Name: "frontend", Type: LoadBalancedWebServiceType},
					Image:   ServiceImageWithPort{ServiceImage: ServiceImage{Build: "frontend/Dockerfile"}, Port: 80},
					RoutingRule: RoutingRule{
						Path:            "svc",
						HealthCheckPath: "/",
					},
					LogsConfig: LogsConfig{
						LogRetention: 30,
					},
					TaskConfig: TaskConfig{
						CPU:    512,
						Memory: 1024,
						Count:  intp(1),
						Variables: map[string]string{
							"LOG_LEVEL": "WARN",
						},
						Secrets: map[string]string{
							"DB_PASSWORD": "MYSQL_DB_PASSWORD",
						},
					},
					Sidecar: Sidecar{
						Sidecars: map[string]SidecarConfig{
							"xray": {
								Port:      "2000/udp",
								Image:     "123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon",
								CredParam: "some arn",
							},
						},
					},
					Environments: map[string]loadBalancedWebServiceOverrideConfig{
						"test": {
							TaskConfig: TaskConfig{
								Count: intp(3),
							},
						},
					},
				}
				require.Equal(t, wantedManifest, actualManifest)
			},
		},
		"Backend Service": {
			inContent: `
name: subscribers
type: Backend Service
image:
  build: ./subscribers/Dockerfile
  port: 8080
  healthcheck:
    command: ['CMD-SHELL', 'curl http://localhost:5000/ || exit 1']
cpu: 1024
memory: 1024
secrets:
  API_TOKEN: SUBS_API_TOKEN`,
			requireCorrectValues: func(t *testing.T, i interface{}) {
				actualManifest, ok := i.(*BackendService)
				require.True(t, ok)
				wantedManifest := &BackendService{
					Service: Service{
						Name: "subscribers",
						Type: BackendServiceType,
					},
					Image: imageWithPortAndHealthcheck{
						ServiceImageWithPort: ServiceImageWithPort{
							ServiceImage: ServiceImage{
								Build: "./subscribers/Dockerfile",
							},
							Port: 8080,
						},
						HealthCheck: &ContainerHealthCheck{
							Command:     []string{"CMD-SHELL", "curl http://localhost:5000/ || exit 1"},
							Interval:    durationp(10 * time.Second),
							Retries:     intp(2),
							Timeout:     durationp(5 * time.Second),
							StartPeriod: durationp(0 * time.Second),
						},
					},
					TaskConfig: TaskConfig{
						CPU:    1024,
						Memory: 1024,
						Count:  intp(1),
						Secrets: map[string]string{
							"API_TOKEN": "SUBS_API_TOKEN",
						},
					},
				}
				require.Equal(t, wantedManifest, actualManifest)
			},
		},
		"invalid svc type": {
			inContent: `
name: CowSvc
type: 'OH NO'
`,
			wantedErr: &ErrInvalidSvcManifestType{Type: "OH NO"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := UnmarshalService([]byte(tc.inContent))

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				tc.requireCorrectValues(t, m)
			}
		})
	}
}
