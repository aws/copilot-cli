// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/stretchr/testify/require"
)

func TestApplyEnv_HTTP(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"path overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Path = aws.String("mockPath")
				svc.Environments["test"].Path = aws.String("mockPathTest")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Path = aws.String("mockPathTest")
			},
		},
		"path explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Path = aws.String("mockPath")
				svc.Environments["test"].Path = aws.String("")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Path = aws.String("")
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: path not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Path = aws.String("mockPath")
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Path = aws.String("mockPath")
			},
		},
		"healthcheck overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPath"),
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPathTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPathTest"),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: healthcheck not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPath"),
				}
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("mockPath"),
				}
			},
		},
		"deregistration_delay overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockDeregistrationDelay := 10 * time.Second
				mockDeregistrationDelayTest := 42 * time.Second
				svc.DeregistrationDelay = &mockDeregistrationDelay
				svc.Environments["test"].DeregistrationDelay = &mockDeregistrationDelayTest
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDeregistrationDelayTest := 42 * time.Second
				svc.DeregistrationDelay = &mockDeregistrationDelayTest
			},
		},
		"deregistration_delay explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockDeregistrationDelay := 10 * time.Second
				mockDeregistrationDelayTest := 0 * time.Second
				svc.DeregistrationDelay = &mockDeregistrationDelay
				svc.Environments["test"].DeregistrationDelay = &mockDeregistrationDelayTest
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDeregistrationDelayTest := 0 * time.Second
				svc.DeregistrationDelay = &mockDeregistrationDelayTest
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: deregistration_delay not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockDeregistrationDelay := 10 * time.Second
				svc.DeregistrationDelay = &mockDeregistrationDelay
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDeregistrationDelay := 10 * time.Second
				svc.DeregistrationDelay = &mockDeregistrationDelay
			},
		},
		"target_container overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TargetContainer = aws.String("mockTargetContainer")
				svc.Environments["test"].TargetContainer = aws.String("mockTargetContainerTest")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TargetContainer = aws.String("mockTargetContainerTest")
			},
		},
		"target_container explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TargetContainer = aws.String("mockTargetContainer")
				svc.Environments["test"].TargetContainer = aws.String("")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TargetContainer = aws.String("")
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: target_container not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TargetContainer = aws.String("mockTargetContainer")
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TargetContainer = aws.String("mockTargetContainer")
			},
		},
		"targetContainer overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TargetContainerCamelCase = aws.String("mockTargetContainer")
				svc.Environments["test"].TargetContainerCamelCase = aws.String("mockTargetContainerTest")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TargetContainerCamelCase = aws.String("mockTargetContainerTest")
			},
		},
		"targetContainer explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TargetContainerCamelCase = aws.String("mockTargetContainer")
				svc.Environments["test"].TargetContainerCamelCase = aws.String("")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TargetContainerCamelCase = aws.String("")
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: targetContainer not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TargetContainerCamelCase = aws.String("mockTargetContainer")
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TargetContainerCamelCase = aws.String("mockTargetContainer")
			},
		},
		"stickness overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Stickiness = aws.Bool(false)
				svc.Environments["test"].Stickiness = aws.Bool(true)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Stickiness = aws.Bool(true)
			},
		},
		"stickness explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Stickiness = aws.Bool(true)
				svc.Environments["test"].Stickiness = aws.Bool(false)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Stickiness = aws.Bool(false)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: stickness not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Stickiness = aws.Bool(true)
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Stickiness = aws.Bool(true)
			},
		},
		"allowed_source_ips overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.AllowedSourceIps = []string{"1", "2"}
				svc.Environments["test"].AllowedSourceIps = []string{"3", "4"}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.AllowedSourceIps = []string{"3", "4"}
			},
		},
		"allowed_source_ips explicitly overridden by zero slice": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.AllowedSourceIps = []string{"1", "2"}
				svc.Environments["test"].AllowedSourceIps = []string{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.AllowedSourceIps = []string{}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: allowed_source_ips not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.AllowedSourceIps = []string{"1", "2"}
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.AllowedSourceIps = []string{"1", "2"}
			},
		},
		"alias overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mockAlias"),
				}
				svc.Environments["test"].Alias = Alias{
					String: aws.String("mockAliasTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mockAliasTest"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: alias not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mockAlias"),
				}
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mockAlias"),
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
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

func TestApplyEnv_HTTP_HealthCheck(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"FIXED_BUG: composite fields: string is overridden if args is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/"),
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/test"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/test"),
					},
				}
			},
		},
		"FIXED_BUG: composite fields: args is overridden if string is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/test"),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/"),
				}
			},
		},
		"string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/"),
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/test"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/test"),
				}
			},
		},
		"string explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/"),
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String(""),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String(""),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/"),
				}
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckPath: aws.String("/"),
				}
			},
		},
		"args overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/"),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path:         aws.String("/test"),
						SuccessCodes: aws.String("200"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path:         aws.String("/test"),
						SuccessCodes: aws.String("200"),
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: args not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/"),
					},
				}
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/"),
					},
				}
			},
		},
		"path overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/"),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/test"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/test"),
					},
				}
			},
		},
		"path explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/"),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String(""),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String(""),
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: path not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/"),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Path: aws.String("/"),
					},
				}
			},
		},
		"success_codes overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						SuccessCodes: aws.String("200"),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						SuccessCodes: aws.String("200,201"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						SuccessCodes: aws.String("200,201"),
					},
				}
			},
		},
		"success_codes explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						SuccessCodes: aws.String("200"),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						SuccessCodes: aws.String(""),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						SuccessCodes: aws.String(""),
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: success_codes not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						SuccessCodes: aws.String("200"),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						SuccessCodes: aws.String("200"),
					},
				}
			},
		},
		"healthy_threshold overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						HealthyThreshold: aws.Int64(13),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						HealthyThreshold: aws.Int64(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						HealthyThreshold: aws.Int64(42),
					},
				}
			},
		},
		"healthy_threshold explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						HealthyThreshold: aws.Int64(13),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						HealthyThreshold: aws.Int64(0),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						HealthyThreshold: aws.Int64(0),
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: healthy_threshold not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						HealthyThreshold: aws.Int64(13),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						HealthyThreshold: aws.Int64(13),
					},
				}
			},
		},
		"unhealthy_threshold overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						UnhealthyThreshold: aws.Int64(13),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						UnhealthyThreshold: aws.Int64(42),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						UnhealthyThreshold: aws.Int64(42),
					},
				}
			},
		},
		"unhealthy_threshold explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						UnhealthyThreshold: aws.Int64(13),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						UnhealthyThreshold: aws.Int64(0),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						UnhealthyThreshold: aws.Int64(0),
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: unhealthy_threshold not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						UnhealthyThreshold: aws.Int64(13),
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						UnhealthyThreshold: aws.Int64(13),
					},
				}
			},
		},
		"timeout overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockTimeout := 10 * time.Second
				mockTimeoutTest := 42 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Timeout: &mockTimeout,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Timeout: &mockTimeoutTest,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockTimeoutWanted := 42 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Timeout: &mockTimeoutWanted,
					},
				}
			},
		},
		"timeout explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockTimeout := 10 * time.Second
				mockTimeoutTest := 0 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Timeout: &mockTimeout,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Timeout: &mockTimeoutTest,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockTimeoutWanted := 0 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Timeout: &mockTimeoutWanted,
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: timeout not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockTimeout := 10 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Timeout: &mockTimeout,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockTimeoutWanted := 10 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Timeout: &mockTimeoutWanted,
					},
				}
			},
		},
		"interval overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockInterval := 10 * time.Second
				mockIntervalTest := 42 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Interval: &mockInterval,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Interval: &mockIntervalTest,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockIntervalWanted := 42 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Interval: &mockIntervalWanted,
					},
				}
			},
		},
		"interval explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockInterval := 10 * time.Second
				mockIntervalTest := 0 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Interval: &mockInterval,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Interval: &mockIntervalTest,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockIntervalWanted := 0 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Interval: &mockIntervalWanted,
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: interval not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockInterval := 10 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Interval: &mockInterval,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockIntervalWanted := 10 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						Interval: &mockIntervalWanted,
					},
				}
			},
		},
		"grace_period overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockGracePeriod := 10 * time.Second
				mockGracePeriodTest := 42 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						GracePeriod: &mockGracePeriod,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						GracePeriod: &mockGracePeriodTest,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockGracePeriodWanted := 42 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						GracePeriod: &mockGracePeriodWanted,
					},
				}
			},
		},
		"grace_period explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockGracePeriod := 10 * time.Second
				mockGracePeriodTest := 0 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						GracePeriod: &mockGracePeriod,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						GracePeriod: &mockGracePeriodTest,
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockGracePeriodWanted := 0 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						GracePeriod: &mockGracePeriodWanted,
					},
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: grace_period not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockGracePeriod := 10 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						GracePeriod: &mockGracePeriod,
					},
				}
				svc.Environments["test"].HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockGracePeriodWanted := 10 * time.Second
				svc.HealthCheck = HealthCheckArgsOrString{
					HealthCheckArgs: HTTPHealthCheckArgs{
						GracePeriod: &mockGracePeriodWanted,
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Run(name, func(t *testing.T) {
				var inSvc, wantedSvc LoadBalancedWebService
				inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
					"test": {},
				}

				tc.inSvc(&inSvc)
				tc.wanted(&wantedSvc)

				got, err := inSvc.ApplyEnv("test")

				require.NoError(t, err)
				require.Equal(t, &wantedSvc, got)
			})
		})
	}
}

func TestApplyEnv_HTTP_Alias(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"composite fields: string slice is overridden if string is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					StringSlice: []string{"mock", "alias"},
				}
				svc.Environments["test"].Alias = Alias{
					String: aws.String("mock alias test"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mock alias test"),
				}
			},
		},
		"FIXED_BUG: composite fields: string is overridden if string slice is not nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mock alias"),
				}
				svc.Environments["test"].Alias = Alias{
					StringSlice: []string{"mock", "alias_test", "test"},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					StringSlice: []string{"mock", "alias_test", "test"},
				}
			},
		},
		"string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mock alias"),
				}
				svc.Environments["test"].Alias = Alias{
					String: aws.String("mock alias test"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mock alias test"),
				}
			},
		},
		"string explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mock alias"),
				}
				svc.Environments["test"].Alias = Alias{
					String: aws.String(""),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String(""),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mock alias"),
				}
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					String: aws.String("mock alias"),
				}
			},
		},
		"string slice overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					StringSlice: []string{"mock", "alias"},
				}
				svc.Environments["test"].Alias = Alias{
					StringSlice: []string{"mock", "alias_test", "test"},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					StringSlice: []string{"mock", "alias_test", "test"},
				}
			},
		},
		"string slice explicitly overridden by zero slice": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					StringSlice: []string{"mock", "entrypoint"},
				}
				svc.Environments["test"].Alias = Alias{
					StringSlice: []string{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					StringSlice: []string{},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: string slice not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					StringSlice: []string{"mock", "alias"},
				}
				svc.Environments["test"].RoutingRule = RoutingRule{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Alias = Alias{
					StringSlice: []string{"mock", "alias"},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
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

func TestLoadBalancedWebService_ApplyEnv_New(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"http overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule = RoutingRule{
					Path: aws.String("/"),
				}
				svc.Environments["test"].RoutingRule = RoutingRule{
					Path:       aws.String("/test"),
					Stickiness: aws.Bool(true),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule = RoutingRule{
					Path:       aws.String("/test"),
					Stickiness: aws.Bool(true),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: http not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule = RoutingRule{
					Path: aws.String("/"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule = RoutingRule{
					Path: aws.String("/"),
				}
			},
		},
		"empty image overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(3),
					},
				}
				svc.Environments["test"].ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							DependsOn: map[string]string{"foo": "bar"},
							Location:  aws.String("mockLocation"),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							DependsOn: map[string]string{"foo": "bar"},
							Location:  aws.String("mockLocation"),
						},
					},
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(3),
					},
				}
			},
		},
		"image overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Location: aws.String("mockLocation"),
						},
					},
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(3),
					},
				}
				svc.Environments["test"].ImageConfig = ImageWithPortAndHealthcheck{
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(1),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Location: aws.String("mockLocation"),
						},
					},
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(1),
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: image not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Location: aws.String("mockLocation"),
						},
					},
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(3),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Location: aws.String("mockLocation"),
						},
					},
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(3),
					},
				}
			},
		},
		"entrypoint overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = EntryPointOverride{
					String: aws.String("mockEntrypoint"),
				}
				svc.Environments["test"].EntryPoint = EntryPointOverride{
					String: aws.String("mockEntrypointTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = EntryPointOverride{
					String: aws.String("mockEntrypointTest"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: entrypoint not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = EntryPointOverride{
					String: aws.String("mockEntrypoint"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.EntryPoint = EntryPointOverride{
					String: aws.String("mockEntrypoint"),
				}
			},
		},
		"command overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = CommandOverride{
					String: aws.String("mockCommand"),
				}
				svc.Environments["test"].Command = CommandOverride{
					String: aws.String("mockCommandTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = CommandOverride{
					String: aws.String("mockCommandTest"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: command not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Command = CommandOverride{
					String: aws.String("mockCommand"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Command = CommandOverride{
					String: aws.String("mockCommand"),
				}
			},
		},
		"cpu overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.CPU = aws.Int(1024)
				svc.Environments["test"].CPU = aws.Int(2048)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.CPU = aws.Int(2048)
			},
		},
		"cpu explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.CPU = aws.Int(1024)
				svc.Environments["test"].CPU = aws.Int(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.CPU = aws.Int(0)
			},
		},
		"FAILED_AFTER_UPGRADE: cpu not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.CPU = aws.Int(1024)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.CPU = aws.Int(1024)
			},
		},
		"memory overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Memory = aws.Int(1024)
				svc.Environments["test"].Memory = aws.Int(2048)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Memory = aws.Int(2048)
			},
		},
		"memory explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Memory = aws.Int(1024)
				svc.Environments["test"].Memory = aws.Int(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Memory = aws.Int(0)
			},
		},
		"FAILED_AFTER_UPGRADE: memory not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.CPU = aws.Int(1024)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.CPU = aws.Int(1024)
			},
		},
		"platform overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatformTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatformTest"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: platform not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mockPlatform"),
				}
			},
		},
		"count overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(3),
				}
				svc.Environments["test"].Count = Count{
					Value: aws.Int(42),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(42),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: count not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(3),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Count = Count{
					Value: aws.Int(3),
				}
			},
		},
		"exec overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
				svc.Environments["test"].ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
		},
		"exec explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
				svc.Environments["test"].ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: exec not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
		},
		"network overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
				svc.Environments["test"].Network = NetworkConfig{
					VPC: vpcConfig{
						Placement:      aws.String("mockPlacementTest"),
						SecurityGroups: []string{"mock", "security_group", "test"},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement:      aws.String("mockPlacementTest"),
						SecurityGroups: []string{"mock", "security_group", "test"},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: network not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
			},
		},
		"variables overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				svc.Environments["test"].Variables = map[string]string{
					"mockVar1": "3", // Override the value of mockVar1
					"mockVar3": "3", // Append a new variable mockVar3
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "3",
					"mockVar2": "2",
					"mockVar3": "3",
				}
			},
		},
		"variables not overridden by empty map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				svc.Environments["test"].Variables = map[string]string{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
		},
		"variables not overridden by nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
		},
		"secrets overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				svc.Environments["test"].Secrets = map[string]string{
					"mockSecret1": "3", // Override the value of mockSecret1
					"mockSecret3": "3", // Append a new variable mockSecret3
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "3",
					"mockSecret2": "2",
					"mockSecret3": "3",
				}
			},
		},
		"secrets not overridden by empty map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				svc.Environments["test"].Secrets = map[string]string{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
			},
		},
		"secrets not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
			},
		},
		"storage overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Ephemeral: aws.Int(3),
				}
				svc.Environments["test"].Storage = Storage{
					Ephemeral: aws.Int(5),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Ephemeral: aws.Int(5),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: storage not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Ephemeral: aws.Int(3),
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Ephemeral: aws.Int(3),
				}
			},
		},
		"logging overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = Logging{
					Image: aws.String("mockImage"),
				}
				svc.Environments["test"].Logging = Logging{
					Image: aws.String("mockImageTest"),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = Logging{
					Image: aws.String("mockImageTest"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: logging not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Logging = Logging{
					Image: aws.String("mockImage"),
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Logging = Logging{
					Image: aws.String("mockImage"),
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc LoadBalancedWebService
			inSvc.Environments = map[string]*LoadBalancedWebServiceConfig{
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
