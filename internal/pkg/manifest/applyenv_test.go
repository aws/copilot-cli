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
				svc.HTTPOrBool.Main.Stickiness = aws.Bool(false)
				svc.Environments["test"].HTTPOrBool.Main.Stickiness = aws.Bool(true)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HTTPOrBool.Main.Stickiness = aws.Bool(true)
			},
		},
		"bool value overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HTTPOrBool.Main.Stickiness = aws.Bool(true)
				svc.Environments["test"].HTTPOrBool.Main.Stickiness = aws.Bool(false)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HTTPOrBool.Main.Stickiness = aws.Bool(false)
			},
		},
		"bool value not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.HTTPOrBool.Main.Stickiness = aws.Bool(true)
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.HTTPOrBool.Main.Stickiness = aws.Bool(true)
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

			got, err := inSvc.applyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func testApplyEnv(t *testing.T, initial, env, expected LoadBalancedWebServiceConfig) {
	mft := LoadBalancedWebService{
		LoadBalancedWebServiceConfig: initial,
		Environments: map[string]*LoadBalancedWebServiceConfig{
			"test": &env,
		},
	}
	expectedMft := LoadBalancedWebService{
		LoadBalancedWebServiceConfig: expected,
	}

	got, err := mft.applyEnv("test")
	require.NoError(t, err)
	require.Equal(t, &expectedMft, got)
}

func TestApplyEnv_Int(t *testing.T) {
	tests := map[string]struct {
		initial  *int
		override *int
		expected *int
	}{
		"overridden": {
			initial:  aws.Int(24),
			override: aws.Int(42),
			expected: aws.Int(42),
		},
		"overridden by zero": {
			initial:  aws.Int(24),
			override: aws.Int(0),
			expected: aws.Int(0),
		},
		"not overridden": {
			initial:  aws.Int(24),
			expected: aws.Int(24),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			initial := LoadBalancedWebServiceConfig{
				TaskConfig: TaskConfig{
					CPU: tc.initial,
				},
			}
			override := LoadBalancedWebServiceConfig{
				TaskConfig: TaskConfig{
					CPU: tc.override,
				},
			}
			expected := LoadBalancedWebServiceConfig{
				TaskConfig: TaskConfig{
					CPU: tc.expected,
				},
			}

			testApplyEnv(t, initial, override, expected)
		})
	}
}

func TestApplyEnv_Int64(t *testing.T) {
	tests := map[string]struct {
		initial  *int64
		override *int64
		expected *int64
	}{
		"overridden": {
			initial:  aws.Int64(24),
			override: aws.Int64(42),
			expected: aws.Int64(42),
		},
		"overridden by zero": {
			initial:  aws.Int64(24),
			override: aws.Int64(0),
			expected: aws.Int64(0),
		},
		"not overridden": {
			initial:  aws.Int64(24),
			expected: aws.Int64(24),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			initial := LoadBalancedWebServiceConfig{
				HTTPOrBool: HTTPOrBool{
					HTTP: HTTP{
						Main: RoutingRule{
							HealthCheck: HealthCheckArgsOrString{
								AdvancedToUnion[string](HTTPHealthCheckArgs{
									HealthyThreshold: tc.initial,
								}),
							},
						},
					},
				},
			}
			override := LoadBalancedWebServiceConfig{
				HTTPOrBool: HTTPOrBool{
					HTTP: HTTP{
						Main: RoutingRule{
							HealthCheck: HealthCheckArgsOrString{
								AdvancedToUnion[string](HTTPHealthCheckArgs{
									HealthyThreshold: tc.override,
								}),
							},
						},
					},
				},
			}
			expected := LoadBalancedWebServiceConfig{
				HTTPOrBool: HTTPOrBool{
					HTTP: HTTP{
						Main: RoutingRule{
							HealthCheck: HealthCheckArgsOrString{
								AdvancedToUnion[string](HTTPHealthCheckArgs{
									HealthyThreshold: tc.expected,
								}),
							},
						},
					},
				},
			}

			testApplyEnv(t, initial, override, expected)
		})
	}
}

func TestApplyEnv_Uint16(t *testing.T) {
	tests := map[string]struct {
		initial  *uint16
		override *uint16
		expected *uint16
	}{
		"overridden": {
			initial:  aws.Uint16(24),
			override: aws.Uint16(42),
			expected: aws.Uint16(42),
		},
		"overridden by zero": {
			initial:  aws.Uint16(24),
			override: aws.Uint16(0),
			expected: aws.Uint16(0),
		},
		"not overridden": {
			initial:  aws.Uint16(24),
			expected: aws.Uint16(24),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			initial := LoadBalancedWebServiceConfig{
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Port: tc.initial,
					},
				},
			}
			override := LoadBalancedWebServiceConfig{
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Port: tc.override,
					},
				},
			}
			expected := LoadBalancedWebServiceConfig{
				ImageConfig: ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Port: tc.expected,
					},
				},
			}

			testApplyEnv(t, initial, override, expected)
		})
	}
}

func TestApplyEnv_Uint32(t *testing.T) {
	tests := map[string]struct {
		initial  *uint32
		override *uint32
		expected *uint32
	}{
		"overridden": {
			initial:  aws.Uint32(24),
			override: aws.Uint32(42),
			expected: aws.Uint32(42),
		},
		"overridden by zero": {
			initial:  aws.Uint32(24),
			override: aws.Uint32(0),
			expected: aws.Uint32(0),
		},
		"not overridden": {
			initial:  aws.Uint32(24),
			expected: aws.Uint32(24),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			initial := LoadBalancedWebServiceConfig{
				TaskConfig: TaskConfig{
					Storage: Storage{
						Volumes: map[string]*Volume{
							"volume1": {
								EFS: EFSConfigOrBool{
									Advanced: EFSVolumeConfiguration{
										UID: tc.initial,
									},
								},
							},
						},
					},
				},
			}
			override := LoadBalancedWebServiceConfig{
				TaskConfig: TaskConfig{
					Storage: Storage{
						Volumes: map[string]*Volume{
							"volume1": {
								EFS: EFSConfigOrBool{
									Advanced: EFSVolumeConfiguration{
										UID: tc.override,
									},
								},
							},
						},
					},
				},
			}
			expected := LoadBalancedWebServiceConfig{
				TaskConfig: TaskConfig{
					Storage: Storage{
						Volumes: map[string]*Volume{
							"volume1": {
								EFS: EFSConfigOrBool{
									Advanced: EFSVolumeConfiguration{
										UID: tc.expected,
									},
								},
							},
						},
					},
				},
			}

			testApplyEnv(t, initial, override, expected)
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
				svc.HTTPOrBool.Main.DeregistrationDelay = &mockDuration
				svc.Environments["test"].HTTPOrBool.Main.DeregistrationDelay = &mockDurationTest
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDurationTest := 42 * time.Second
				svc.HTTPOrBool.Main.DeregistrationDelay = &mockDurationTest
			},
		},
		"duration overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockDuration, mockDurationTest := 24*time.Second, 0*time.Second
				svc.HTTPOrBool.Main.DeregistrationDelay = &mockDuration
				svc.Environments["test"].HTTPOrBool.Main.DeregistrationDelay = &mockDurationTest
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDurationTest := 0 * time.Second
				svc.HTTPOrBool.Main.DeregistrationDelay = &mockDurationTest
			},
		},
		"duration not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				mockDuration := 24 * time.Second
				svc.HTTPOrBool.Main.DeregistrationDelay = &mockDuration
			},
			wanted: func(svc *LoadBalancedWebService) {
				mockDurationTest := 24 * time.Second
				svc.HTTPOrBool.Main.DeregistrationDelay = &mockDurationTest
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

			got, err := inSvc.applyEnv("test")

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

			got, err := inSvc.applyEnv("test")

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

			got, err := inSvc.applyEnv("test")

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

			got, err := inSvc.applyEnv("test")

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
				svc.ImageConfig.Image.DockerLabels = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent is johnny rivers",
				}
				svc.Environments["test"].ImageConfig.Image.DockerLabels = map[string]string{
					"var1": "the secret sauce is blue cheese which has mold in it",
					"var3": "the secret route is through egypt",
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.DockerLabels = map[string]string{
					"var1": "the secret sauce is blue cheese which has mold in it", // Overridden.
					"var2": "the secret agent is johnny rivers",                    // Kept.
					"var3": "the secret route is through egypt",                    // Appended
				}
			},
		},
		"map not overridden by zero map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.DockerLabels = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent man is johnny rivers",
				}
				svc.Environments["test"].ImageConfig.Image.DockerLabels = map[string]string{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.DockerLabels = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent man is johnny rivers",
				}
			},
		},
		"map not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.DockerLabels = map[string]string{
					"var1": "the secret sauce is mole",
					"var2": "the secret agent man is johnny rivers",
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.ImageConfig.Image.DockerLabels = map[string]string{
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

			got, err := inSvc.applyEnv("test")

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

			got, err := inSvc.applyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}

func TestApplyEnv_MapToStruct(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"map upserted": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							Plain: stringP("var1"),
						},
					},
					"VAR2": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("import-var2"),
							},
						},
					},
				}
				svc.Environments["test"].TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("import-var1-test"),
							},
						},
					},
					"VAR3": {
						StringOrFromCFN{
							Plain: stringP("var3-test"),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("import-var1-test"),
							},
						},
					},
					"VAR2": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("import-var2"),
							},
						},
					},
					"VAR3": {
						StringOrFromCFN{
							Plain: stringP("var3-test"),
						},
					},
				}
			},
		},
		"map not overridden by zero map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							Plain: stringP("var1"),
						},
					},
					"VAR2": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("import-var2"),
							},
						},
					},
				}
				svc.Environments["test"].TaskConfig.Variables = map[string]Variable{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							Plain: stringP("var1"),
						},
					},
					"VAR2": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("import-var2"),
							},
						},
					},
				}
			},
		},
		"map not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							Plain: stringP("var1"),
						},
					},
					"VAR2": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("import-var2"),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							Plain: stringP("var1"),
						},
					},
					"VAR2": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("import-var2"),
							},
						},
					},
				}
			},
		},
		"override a zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]Variable{
					"VAR1": {},
				}
				svc.Environments["test"].TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							Plain: stringP("var1-test"),
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.TaskConfig.Variables = map[string]Variable{
					"VAR1": {
						StringOrFromCFN{
							Plain: stringP("var1-test"),
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

			got, err := inSvc.applyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}
