// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
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
  targetContainer: "frontend"
variables:
  LOG_LEVEL: "WARN"
secrets:
  DB_PASSWORD: MYSQL_DB_PASSWORD
sidecars:
  xray:
    port: 2000/udp
    image: 123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon
    credentialsParameter: some arn
logging:
  destination:
    Name: cloudwatch
    include-pattern: ^[a-z][aeiou].*$
    exclude-pattern: ^.*[aeiou]$
  enableMetadata: false
  secretOptions:
    LOG_TOKEN: LOG_TOKEN
  configFilePath: /extra.conf
environments:
  test:
    count: 3
`,
			requireCorrectValues: func(t *testing.T, i interface{}) {
				actualManifest, ok := i.(*LoadBalancedWebService)
				require.True(t, ok)
				wantedManifest := &LoadBalancedWebService{
					Service: Service{Name: aws.String("frontend"), Type: aws.String(LoadBalancedWebServiceType)},
					LoadBalancedWebServiceConfig: LoadBalancedWebServiceConfig{
						Image: ServiceImageWithPort{ServiceImage: ServiceImage{Build: aws.String("frontend/Dockerfile")}, Port: aws.Uint16(80)},
						RoutingRule: RoutingRule{
							Path:            aws.String("svc"),
							HealthCheckPath: aws.String("/"),
							TargetContainer: aws.String("frontend"),
						},
						TaskConfig: TaskConfig{
							CPU:    aws.Int(512),
							Memory: aws.Int(1024),
							Count:  aws.Int(1),
							Variables: map[string]string{
								"LOG_LEVEL": "WARN",
							},
							Secrets: map[string]string{
								"DB_PASSWORD": "MYSQL_DB_PASSWORD",
							},
						},
						Sidecar: Sidecar{
							Sidecars: map[string]*SidecarConfig{
								"xray": {
									Port:       aws.String("2000/udp"),
									Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
									CredsParam: aws.String("some arn"),
								},
							},
						},
						LogConfig: &LogConfig{
							Destination: map[string]string{
								"exclude-pattern": "^.*[aeiou]$",
								"include-pattern": "^[a-z][aeiou].*$",
								"Name":            "cloudwatch",
							},
							EnableMetadata: aws.Bool(false),
							ConfigFile:     aws.String("/extra.conf"),
							SecretOptions: map[string]string{
								"LOG_TOKEN": "LOG_TOKEN",
							},
						},
					},
					Environments: map[string]*LoadBalancedWebServiceConfig{
						"test": {
							TaskConfig: TaskConfig{
								Count: aws.Int(3),
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
						Name: aws.String("subscribers"),
						Type: aws.String(BackendServiceType),
					},
					BackendServiceConfig: BackendServiceConfig{
						Image: imageWithPortAndHealthcheck{
							ServiceImageWithPort: ServiceImageWithPort{
								ServiceImage: ServiceImage{
									Build: aws.String("./subscribers/Dockerfile"),
								},
								Port: aws.Uint16(8080),
							},
							HealthCheck: &ContainerHealthCheck{
								Command:     []string{"CMD-SHELL", "curl http://localhost:5000/ || exit 1"},
								Interval:    durationp(10 * time.Second),
								Retries:     aws.Int(2),
								Timeout:     durationp(5 * time.Second),
								StartPeriod: durationp(0 * time.Second),
							},
						},
						TaskConfig: TaskConfig{
							CPU:    aws.Int(1024),
							Memory: aws.Int(1024),
							Count:  aws.Int(1),
							Secrets: map[string]string{
								"API_TOKEN": "SUBS_API_TOKEN",
							},
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
