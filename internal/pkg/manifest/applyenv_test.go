// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestEnsureTransformersOrder(t *testing.T) {
	t.Run("ensure we call basic transformer first", func(t *testing.T) {
		_, ok := defaultTransformers[0].(basicTransformer)
		require.True(t, ok, "basicTransformer needs to used before the rest of the custom transformers, because the other transformers do not merge anything - they just unset the fields that do not get specified in source manifest.")
	})
}

func TestApplyEnv_Bool(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"bool value overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.Stickiness = aws.Bool(false)
				svc.Environments["test"].RoutingRule.Stickiness = aws.Bool(true)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.Stickiness = aws.Bool(true)
			},
		},
		"bool value overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.Stickiness = aws.Bool(true)
				svc.Environments["test"].RoutingRule.Stickiness = aws.Bool(false)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.Stickiness = aws.Bool(false)
			},
		},
		"bool value not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.Stickiness = aws.Bool(true)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.Stickiness = aws.Bool(true)
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

func TestApplyEnv_Int(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"int overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.CPU = aws.Int(24)
				svc.Environments["test"].TaskConfig.CPU = aws.Int(42)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.CPU = aws.Int(42)
			},
		},
		"int overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.CPU = aws.Int(24)
				svc.Environments["test"].TaskConfig.CPU = aws.Int(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.CPU = aws.Int(0)
			},
		},
		"int not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.CPU = aws.Int(24)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.CPU = aws.Int(24)
			},
		},
		"int64 overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(24)
				svc.Environments["test"].RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(42)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(42)
			},
		},
		"int64 overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(24)
				svc.Environments["test"].RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(0)
			},
		},
		"int64 not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(24)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(24)
			},
		},
		"uint16 overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
				svc.Environments["test"].ImageConfig.Port = aws.Uint16(42)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(42)
			},
		},
		"uint16 overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
				svc.Environments["test"].ImageConfig.Port = aws.Uint16(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(0)
			},
		},
		"uint16 not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
			},
		},
		"uint32 overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Environments["test"].ImageConfig.Port = aws.Uint16(42)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(42)
			},
		},
		"uint32 overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
				svc.Environments["test"].ImageConfig.Port = aws.Uint16(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(0)
			},
		},
		"uint32 not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
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

func TestApplyEnv_UInt16(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"uint16 overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
				svc.Environments["test"].ImageConfig.Port = aws.Uint16(42)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(42)
			},
		},
		"uint16 overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
				svc.Environments["test"].ImageConfig.Port = aws.Uint16(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(0)
			},
		},
		"uint16 not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Port = aws.Uint16(24)
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

func TestApplyEnv_Int64(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"int64 overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(24)
				svc.Environments["test"].RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(42)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(42)
			},
		},
		"int64 overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(24)
				svc.Environments["test"].RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(0)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(0)
			},
		},
		"int64 not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(24)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.RoutingRule.HealthCheck.HealthCheckArgs.HealthyThreshold = aws.Int64(24)
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

func TestApplyEnv_Uint32(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"uint32 overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID: aws.Uint32(24),
							},
						},
					},
				}
				svc.Environments["test"].Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID: aws.Uint32(42),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID: aws.Uint32(42),
							},
						},
					},
				}
			},
		},
		"uint32 overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID: aws.Uint32(24),
							},
						},
					},
				}
				svc.Environments["test"].Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID: aws.Uint32(0),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID: aws.Uint32(0),
							},
						},
					},
				}
			},
		},
		"uint32 not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID: aws.Uint32(24),
							},
						},
					},
				}
				svc.Environments["test"].Storage.Volumes = map[string]*Volume{
					"volume1": {},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Advanced: EFSVolumeConfiguration{
								UID: aws.Uint32(24),
							},
						},
					},
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

func TestApplyEnv_Duration(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"duration overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockDuration, mockDurationTest := 24*time.Second, 42*time.Second
				svc.RoutingRule.DeregistrationDelay = &mockDuration
				svc.Environments["test"].RoutingRule.DeregistrationDelay = &mockDurationTest
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDurationTest := 42 * time.Second
				svc.RoutingRule.DeregistrationDelay = &mockDurationTest
			},
		},
		"duration overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockDuration, mockDurationTest := 24*time.Second, 0*time.Second
				svc.RoutingRule.DeregistrationDelay = &mockDuration
				svc.Environments["test"].RoutingRule.DeregistrationDelay = &mockDurationTest
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDurationTest := 0 * time.Second
				svc.RoutingRule.DeregistrationDelay = &mockDurationTest
			},
		},
		"duration not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockDuration := 24 * time.Second
				svc.RoutingRule.DeregistrationDelay = &mockDuration
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDurationTest := 24 * time.Second
				svc.RoutingRule.DeregistrationDelay = &mockDurationTest
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

func TestApplyEnv_String(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"string overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.Location = aws.String("cairo")
				svc.Environments["test"].ImageConfig.Image.Location = aws.String("nerac")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.Location = aws.String("nerac")
			},
		},
		"string overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.Location = aws.String("cairo")
				svc.Environments["test"].ImageConfig.Image.Location = aws.String("")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.Location = aws.String("")
			},
		},
		"string not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.Location = aws.String("cairo")
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.Location = aws.String("cairo")
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

func TestApplyEnv_StringSlice(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"string slice overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck.Command = []string{"walk", "like", "an", "egyptian"}
				svc.Environments["test"].ImageConfig.HealthCheck.Command = []string{"walk", "on", "the", "wild", "side"}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck.Command = []string{"walk", "on", "the", "wild", "side"}
			},
		},
		"string slice overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck.Command = []string{"walk", "like", "an", "egyptian"}
				svc.Environments["test"].ImageConfig.HealthCheck.Command = []string{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck.Command = []string{}
			},
		},
		"string slice not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck.Command = []string{"walk", "like", "an", "egyptian"}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.HealthCheck.Command = []string{"walk", "like", "an", "egyptian"}
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

func TestApplyEnv_StructSlice(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"struct slice overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.PublishConfig.Topics = []Topic{
					{
						Name: aws.String("walk like an egyptian"),
					},
				}
				svc.Environments["test"].PublishConfig.Topics = []Topic{
					{
						Name: aws.String("walk on the wild side"),
					},
				}

			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.PublishConfig.Topics = []Topic{
					{
						Name: aws.String("walk on the wild side"),
					},
				}
			},
		},
		"string slice overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.PublishConfig.Topics = []Topic{
					{
						Name: aws.String("walk like an egyptian"),
					},
				}
				svc.Environments["test"].PublishConfig.Topics = []Topic{}

			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.PublishConfig.Topics = []Topic{}
			},
		},
		"string slice not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.PublishConfig.Topics = []Topic{
					{
						Name: aws.String("walk like an egyptian"),
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.PublishConfig.Topics = []Topic{
					{
						Name: aws.String("walk like an egyptian"),
					},
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

func TestApplyEnv_MapToString(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"map upserted": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent is johnny rivers",
				}
				svc.Environments["test"].TaskConfig.Variables = map[string]string{
					"var1": "the secret sauce is blue cheese which has mold in it",
					"var3": "the secret route is through egypt",
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]string{
					"var1": "the secret sauce is blue cheese which has mold in it", // Overridden.
					"var2": "the secret agent is johnny rivers",                    // Kept.
					"var3": "the secret route is through egypt",                    // Appended
				}
			},
		},
		"map not overridden by zero map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent man is johnny rivers",
				}
				svc.Environments["test"].TaskConfig.Variables = map[string]string{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent man is johnny rivers",
				}
			},
		},
		"map not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent man is johnny rivers",
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent man is johnny rivers",
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

func TestApplyEnv_MapToPStruct(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"map upserted": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
					"volume2": {
						MountPointOpts: MountPointOpts{
							ReadOnly: aws.Bool(true),
						},
					},
				}
				svc.Environments["test"].Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPathTest"),
						},
					},
					"volume3": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPathTest"),
						},
					}, // Overridden.
					"volume2": {
						MountPointOpts: MountPointOpts{
							ReadOnly: aws.Bool(true),
						},
					}, // Kept.
					"volume3": {
						EFS: EFSConfigOrBool{
							Enabled: aws.Bool(true),
						},
					}, // Appended.
				}
			},
		},
		"map not overridden by zero map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
					"volume2": {
						MountPointOpts: MountPointOpts{
							ReadOnly: aws.Bool(true),
						},
					},
				}
				svc.Environments["test"].Storage.Volumes = map[string]*Volume{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
					"volume2": {
						MountPointOpts: MountPointOpts{
							ReadOnly: aws.Bool(true),
						},
					},
				}
			},
		},
		"map not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
					"volume2": {
						MountPointOpts: MountPointOpts{
							ReadOnly: aws.Bool(true),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage.Volumes = map[string]*Volume{
					"volume1": {
						MountPointOpts: MountPointOpts{
							ContainerPath: aws.String("mockPath"),
						},
					},
					"volume2": {
						MountPointOpts: MountPointOpts{
							ReadOnly: aws.Bool(true),
						},
					},
				}
			},
		},
		"override a nil value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Volumes: map[string]*Volume{
						"mockVolume1": nil,
					},
				}
				svc.Environments["test"].Storage = Storage{
					Volumes: map[string]*Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("mockPath"),
							},
							EFS: EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = Storage{
					Volumes: map[string]*Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("mockPath"),
							},
							EFS: EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
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
