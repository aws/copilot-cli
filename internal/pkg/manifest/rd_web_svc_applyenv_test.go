// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0s

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/stretchr/testify/require"
)

func TestRequestDrivenWebService_ApplyEnv_HTTP(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *RequestDrivenWebService)
		wanted func(svc *RequestDrivenWebService)
	}{
		"healthcheck overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.HealthCheckConfiguration = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPath"),
				}
				svc.Environments["test"].HealthCheckConfiguration = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPathTest"),
				}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.HealthCheckConfiguration = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPathTest"),
				}
			},
		},
		"PENDING: healthcheck not overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.HealthCheckConfiguration = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPath"),
				}
				svc.Environments["test"].RequestDrivenWebServiceHttpConfig = RequestDrivenWebServiceHttpConfig{}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.HealthCheckConfiguration = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPath"),
				}
			},
		},
		"alias overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.Alias = aws.String("mockAlias")
				svc.Environments["test"].Alias = aws.String("mockAliasTest")
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.Alias = aws.String("mockAliasTest")
			},
		},
		"alias overridden by zero value": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.Alias = aws.String("mockAlias")
				svc.Environments["test"].Alias = aws.String("")
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.Alias = aws.String("")
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: alias not overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.Alias = aws.String("mockAlias")
				svc.Environments["test"].RequestDrivenWebServiceHttpConfig = RequestDrivenWebServiceHttpConfig{}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.Alias = aws.String("mockAlias")
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc RequestDrivenWebService
			inSvc.Environments = map[string]*RequestDrivenWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestRequestDrivenWebService_ApplyEnv_New(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *RequestDrivenWebService)
		wanted func(svc *RequestDrivenWebService)
	}{
		"PENDING: http overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.RequestDrivenWebServiceHttpConfig = RequestDrivenWebServiceHttpConfig{
					HealthCheckConfiguration: HealthCheckArgsOrString{
						HealthCheckPath: aws.String("mockPath"),
					},
				}
				svc.Environments["test"].RequestDrivenWebServiceHttpConfig = RequestDrivenWebServiceHttpConfig{
					Alias: aws.String("mockAlias"),
				}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.RequestDrivenWebServiceHttpConfig = RequestDrivenWebServiceHttpConfig{
					HealthCheckConfiguration: HealthCheckArgsOrString{
						HealthCheckPath: aws.String("mockPath"),
					},
					Alias: aws.String("mockAlias"),
				}
			},
		},
		"PENDING: http not overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.RequestDrivenWebServiceHttpConfig = RequestDrivenWebServiceHttpConfig{
					HealthCheckConfiguration: HealthCheckArgsOrString{
						HealthCheckPath: aws.String("mockPath"),
					},
				}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.RequestDrivenWebServiceHttpConfig = RequestDrivenWebServiceHttpConfig{
					HealthCheckConfiguration: HealthCheckArgsOrString{
						HealthCheckPath: aws.String("mockPath"),
					},
				}
			},
		},
		"image overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.ImageConfig = ImageWithPort{
					Image: Image{
						Location: aws.String("mockLocation"),
					},
				}
				svc.Environments["test"].ImageConfig = ImageWithPort{
					Image: Image{
						Location: aws.String("mockLocationTest"),
					},
					Port: aws.Uint16(8080),
				}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.ImageConfig = ImageWithPort{
					Image: Image{
						Location: aws.String("mockLocationTest"),
					},
					Port: aws.Uint16(8080),
				}
			},
		},
		"image not overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.ImageConfig = ImageWithPort{
					Image: Image{
						Location: aws.String("mockLocation"),
					},
				}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.ImageConfig = ImageWithPort{
					Image: Image{
						Location: aws.String("mockLocation"),
					},
				}
			},
		},
		"cpu overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.CPU = aws.Int(1024)
				svc.Environments["test"].InstanceConfig.CPU = aws.Int(2048)
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.CPU = aws.Int(2048)
			},
		},
		"cpu explicitly overridden by zero value": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.CPU = aws.Int(1024)
				svc.Environments["test"].InstanceConfig.CPU = aws.Int(0)
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.CPU = aws.Int(0)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: cpu not overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.CPU = aws.Int(1024)
				svc.Environments["test"].InstanceConfig = AppRunnerInstanceConfig{}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.CPU = aws.Int(1024)
			},
		},
		"memory overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Memory = aws.Int(1024)
				svc.Environments["test"].InstanceConfig.Memory = aws.Int(2048)
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Memory = aws.Int(2048)
			},
		},
		"memory explicitly overridden by zero value": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Memory = aws.Int(1024)
				svc.Environments["test"].InstanceConfig.Memory = aws.Int(0)
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Memory = aws.Int(0)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: memory not overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Memory = aws.Int(1024)
				svc.Environments["test"].InstanceConfig = AppRunnerInstanceConfig{}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Memory = aws.Int(1024)
			},
		},
		"platform overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
				svc.Environments["test"].InstanceConfig.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatformTest"),
				}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatformTest"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: platform not overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
				svc.Environments["test"].InstanceConfig = AppRunnerInstanceConfig{}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.InstanceConfig.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
			},
		},
		"variables overridden": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				svc.Environments["test"].Variables = map[string]string{
					"mockVar1": "3", // Override the value of mockVar1
					"mockVar3": "3", // Append a new variable mockVar3
				}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "3",
					"mockVar2": "2",
					"mockVar3": "3",
				}
			},
		},
		"variables not overridden by empty map": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				svc.Environments["test"].Variables = map[string]string{}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
		},
		"variables not overridden by nil": {
			inSvc: func(svc *RequestDrivenWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
			wanted: func(svc *RequestDrivenWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc RequestDrivenWebService
			inSvc.Environments = map[string]*RequestDrivenWebServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}
