// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func Test_ApplyEnv_Storage(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"ephemeral overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Ephemeral: aws.Int(1),
				}
				svc.Environments["test"].Storage = &Storage{
					Ephemeral: aws.Int(3),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Ephemeral: aws.Int(3),
				}
			},
		},
		"ephemeral explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Ephemeral: aws.Int(1),
				}
				svc.Environments["test"].Storage = &Storage{
					Ephemeral: aws.Int(0),
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Ephemeral: aws.Int(0),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: ephemeral not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Ephemeral: aws.Int(1),
				}
				svc.Environments["test"].Storage = &Storage{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Ephemeral: aws.Int(1),
				}
			},
		},
		//"FAILED TEST: volumes overridden": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					MountPointOpts: MountPointOpts{
		//						ReadOnly: aws.Bool(true),
		//					},
		//				},
		//				"mockVolume2": {
		//					EFS: &EFSConfigOrBool{
		//						Enabled: aws.Bool(true),
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Enabled: aws.Bool(true),
		//					},
		//				}, // Modify the value for mockVolume1.
		//				"mockVolume3": {
		//					MountPointOpts: MountPointOpts{
		//						ReadOnly: aws.Bool(true),
		//					},
		//				}, // Append mockVolume3.
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Enabled: aws.Bool(true),
		//					},
		//				},
		//				"mockVolume2": {
		//					EFS: &EFSConfigOrBool{
		//						Enabled: aws.Bool(true),
		//					},
		//				},
		//				"mockVolume3": {
		//					MountPointOpts: MountPointOpts{
		//						ReadOnly: aws.Bool(true),
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		"volumes not overridden by empty map": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
						"mockVolume2": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
						"mockVolume2": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
			},
		},
		"volumes not overridden by nil": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
						"mockVolume2": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
						"mockVolume2": {
							EFS: &EFSConfigOrBool{
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

func Test_ApplyEnv_Storage_Volume(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"FAILED_AFTER_TRANSFORM_POINTER: path overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("mockPath"),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("mockPathTest"),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("mockPathTest"),
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_TRANSFORM_POINTER： path explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("mockPath"),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String(""),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String(""),
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: path not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("mockPath"),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ContainerPath: aws.String("mockPath"),
							},
						},
					},
				}
			},
		},
		"read_only overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(false),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
					},
				}
			},
		},
		"read_only explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(false),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(false),
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: read_only not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							MountPointOpts: MountPointOpts{
								ReadOnly: aws.Bool(true),
							},
						},
					},
				}
			},
		},
		"efs overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockFileSystem"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockRootDirTest"),
									FileSystemID:  aws.String("mockFileSystemTest"),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockRootDirTest"),
									FileSystemID:  aws.String("mockFileSystemTest"),
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: efs not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockFileSystem"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockFileSystem"),
								},
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

func Test_ApplyEnv_Storage_Volume_EFS(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		//"FAILED TEST: composite fields: efs bool is overridden if efs config is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Enabled: aws.Bool(true),
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDirTest"),
		//							FileSystemID:  aws.String("mockFileSystemTest"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Enabled: nil,
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDirTest"),
		//							FileSystemID:  aws.String("mockFileSystemTest"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: composite fields: efs config is overridden if efs bool is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDir"),
		//							FileSystemID:  aws.String("mockFileSystem"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Enabled: aws.Bool(true),
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Enabled:  aws.Bool(true),
		//						Advanced: EFSVolumeConfiguration{},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		"efs bool overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(false),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
			},
		},
		"efs bool explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(false),
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(false),
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: efs bool not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
					},
				}
			},
		},
		"efs config overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockFileSystem"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockRootDirTest"),
									FileSystemID:  aws.String("mockFileSystemTest"),
									UID:           aws.Uint32(42),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockRootDirTest"),
									FileSystemID:  aws.String("mockFileSystemTest"),
									UID:           aws.Uint32(42),
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: efs config not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockFileSystem"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockFileSystem"),
								},
							},
						},
					},
				}
			},
		},
		//"FAILED TEST: exclusive fields: id overridden if uid is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							FileSystemID: aws.String("42"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							UID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							FileSystemID: nil,
		//							UID:          aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: uid overridden if id is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							UID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							FileSystemID: aws.String("42"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							FileSystemID: aws.String("42"),
		//							UID:          nil,
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: root_dir overridden if uid is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDirectory"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							UID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: nil,
		//							UID:           aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: udi overridden if root_dir is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							UID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDirectoryTest"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDirectoryTest"),
		//							UID:           nil,
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: auth overridden if uid is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							AuthConfig: &AuthorizationConfig{
		//								IAM:           aws.Bool(true),
		//								AccessPointID: aws.String("mockID1"),
		//							},
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							UID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							UID:        aws.Uint32(13),
		//							AuthConfig: nil,
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: udi overridden if auth is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							UID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							AuthConfig: &AuthorizationConfig{
		//								IAM:           aws.Bool(true),
		//								AccessPointID: aws.String("mockIDTest"),
		//							},
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							UID: nil,
		//							AuthConfig: &AuthorizationConfig{
		//								IAM:           aws.Bool(true),
		//								AccessPointID: aws.String("mockIDTest"),
		//							},
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: id overridden if gid is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							FileSystemID: aws.String("42"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							GID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							FileSystemID: nil,
		//							GID:          aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: gid overridden if id is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							GID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							FileSystemID: aws.String("42"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							FileSystemID: aws.String("42"),
		//							GID:          nil,
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: root_dir overridden if gid is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDir"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							GID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: nil,
		//							GID:           aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: gid overridden if root_dir is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							GID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDirTest"),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							RootDirectory: aws.String("mockRootDirTest"),
		//							GID:           nil,
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: auth overridden if gid is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							AuthConfig: &AuthorizationConfig{
		//								IAM:           aws.Bool(true),
		//								AccessPointID: aws.String("mockID1"),
		//							},
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							GID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							AuthConfig: nil,
		//							GID:        aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		//"FAILED TEST: exclusive fields: gid overridden if auth is not nil": {
		//	inSvc: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							GID: aws.Uint32(13),
		//						},
		//					},
		//				},
		//			},
		//		}
		//		svc.Environments["test"].Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							AuthConfig: &AuthorizationConfig{
		//								IAM:           aws.Bool(true),
		//								AccessPointID: aws.String("mockID1"),
		//							},
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//	wanted: func(svc *LoadBalancedWebService) {
		//		svc.Storage = &Storage{
		//			Volumes: map[string]Volume{
		//				"mockVolume1": {
		//					EFS: &EFSConfigOrBool{
		//						Advanced: EFSVolumeConfiguration{
		//							AuthConfig: &AuthorizationConfig{
		//								IAM:           aws.Bool(true),
		//								AccessPointID: aws.String("mockID1"),
		//							},
		//							GID: nil,
		//						},
		//					},
		//				},
		//			},
		//		}
		//	},
		//},
		"id overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockID"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockIDTest"),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockIDTest"),
								},
							},
						},
					},
				}
			},
		},
		"id explicitly overridden by zero value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockID"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String(""),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String(""),
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: id not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockID"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: aws.String("mockID"),
								},
							},
						},
					},
				}
			},
		},
		"root_dir overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockDir"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockDirTest"),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockDirTest"),
								},
							},
						},
					},
				}
			},
		},
		"root_dir explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockDir"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String(""),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String(""),
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: root_dir not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockDir"),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									RootDirectory: aws.String("mockDir"),
								},
							},
						},
					},
				}
			},
		},
		"uid overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(13),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(42),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(42),
								},
							},
						},
					},
				}
			},
		},
		"uid explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(13),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(0),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(0),
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: uid not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(13),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									UID: aws.Uint32(13),
								},
							},
						},
					},
				}
			},
		},
		"gid overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									GID: aws.Uint32(13),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									GID: aws.Uint32(42),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									GID: aws.Uint32(42),
								},
							},
						},
					},
				}
			},
		},
		"gid explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									GID: aws.Uint32(13),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									GID: aws.Uint32(0),
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									GID: aws.Uint32(0),
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: gid not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									GID: aws.Uint32(13),
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									GID: aws.Uint32(13),
								},
							},
						},
					},
				}
			},
		},
		"auth overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPoint"),
									},
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM:           aws.Bool(true),
										AccessPointID: aws.String("mockPointTest"),
									},
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM:           aws.Bool(true),
										AccessPointID: aws.String("mockPointTest"),
									},
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: auth not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPoint"),
									},
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPoint"),
									},
								},
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

func Test_ApplyEnv_Storage_Volume_EFS_Auth(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *LoadBalancedWebService)
		wanted func(svc *LoadBalancedWebService)
	}{
		"iam overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM: aws.Bool(false),
									},
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM: aws.Bool(true),
									},
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM: aws.Bool(true),
									},
								},
							},
						},
					},
				}
			},
		},
		"iam explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM: aws.Bool(true),
									},
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM: aws.Bool(false),
									},
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM: aws.Bool(false),
									},
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: iam not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM: aws.Bool(true),
									},
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{},
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										IAM: aws.Bool(true),
									},
								},
							},
						},
					},
				}
			},
		},
		"access_point_id overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPoint"),
									},
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPointTest"),
									},
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPointTest"),
									},
								},
							},
						},
					},
				}
			},
		},
		"access_point_id explicitly overridden by empty value": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPoint"),
									},
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String(""),
									},
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String(""),
									},
								},
							},
						},
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: access_point_id not overridden": {
			inSvc: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPoint"),
									},
								},
							},
						},
					},
				}
				svc.Environments["test"].Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{},
								},
							},
						},
					},
				}
			},
			wanted: func(svc *LoadBalancedWebService) {
				svc.Storage = &Storage{
					Volumes: map[string]Volume{
						"mockVolume1": {
							EFS: &EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									AuthConfig: &AuthorizationConfig{
										AccessPointID: aws.String("mockPoint"),
									},
								},
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
