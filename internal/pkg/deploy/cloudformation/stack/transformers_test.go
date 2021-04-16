// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
)

func Test_convertSidecar(t *testing.T) {
	mockImage := aws.String("mockImage")
	mockMap := map[string]string{"foo": "bar"}
	mockCredsParam := aws.String("mockCredsParam")
	testCases := map[string]struct {
		inPort      string
		inEssential bool

		wanted    *template.SidecarOpts
		wantedErr error
	}{
		"invalid port": {
			inPort: "b/a/d/P/o/r/t",

			wantedErr: fmt.Errorf("cannot parse port mapping from b/a/d/P/o/r/t"),
		},
		"good port without protocol": {
			inPort:      "2000",
			inEssential: true,

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(true),
			},
		},
		"good port with protocol": {
			inPort:      "2000/udp",
			inEssential: true,

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				Protocol:   aws.String("udp"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(true),
			},
		},
		"specify essential as false": {
			inPort:      "2000",
			inEssential: false,

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sidecar := map[string]*manifest.SidecarConfig{
				"foo": {
					CredsParam: mockCredsParam,
					Image:      mockImage,
					Secrets:    mockMap,
					Variables:  mockMap,
					Essential:  aws.Bool(tc.inEssential),
					Port:       aws.String(tc.inPort),
				},
			}
			got, err := convertSidecar(sidecar)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, got[0], tc.wanted)
			}
		})
	}
}

func Test_convertAutoscaling(t *testing.T) {
	const (
		mockRange    = "1-100"
		mockRequests = 1000
	)
	mockResponseTime := 512 * time.Millisecond
	testCases := map[string]struct {
		inRange        manifest.Range
		inCPU          int
		inMemory       int
		inRequests     int
		inResponseTime time.Duration

		wanted    *template.AutoscalingOpts
		wantedErr error
	}{
		"invalid range": {
			inRange: "badRange",

			wantedErr: fmt.Errorf("invalid range value badRange. Should be in format of ${min}-${max}"),
		},
		"success": {
			inRange:        mockRange,
			inCPU:          70,
			inMemory:       80,
			inRequests:     mockRequests,
			inResponseTime: mockResponseTime,

			wanted: &template.AutoscalingOpts{
				MaxCapacity:  aws.Int(100),
				MinCapacity:  aws.Int(1),
				CPU:          aws.Float64(70),
				Memory:       aws.Float64(80),
				Requests:     aws.Float64(1000),
				ResponseTime: aws.Float64(0.512),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			a := manifest.Autoscaling{
				Range:        &tc.inRange,
				CPU:          aws.Int(tc.inCPU),
				Memory:       aws.Int(tc.inMemory),
				Requests:     aws.Int(tc.inRequests),
				ResponseTime: &tc.inResponseTime,
			}
			got, err := convertAutoscaling(&a)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, got, tc.wanted)
			}
		})
	}
}

func Test_convertHTTPHealthCheck(t *testing.T) {
	// These are used by reference to represent the output of the manifest.durationp function.
	duration15Seconds := time.Duration(15 * time.Second)
	duration60Seconds := time.Duration(60 * time.Second)
	testCases := map[string]struct {
		inputPath               *string
		inputHealthyThreshold   *int64
		inputUnhealthyThreshold *int64
		inputInterval           *time.Duration
		inputTimeout            *time.Duration

		wantedOpts template.HTTPHealthCheckOpts
	}{
		"no fields indicated in manifest": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
			},
		},
		"just HealthyThreshold": {
			inputPath:               nil,
			inputHealthyThreshold:   aws.Int64(5),
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:  "/",
				HealthyThreshold: aws.Int64(5),
			},
		},
		"just UnhealthyThreshold": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: aws.Int64(5),
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    "/",
				UnhealthyThreshold: aws.Int64(5),
			},
		},
		"just Interval": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           &duration15Seconds,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Interval:        aws.Int64(15),
			},
		},
		"just Timeout": {
			inputPath:               nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            &duration15Seconds,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Timeout:         aws.Int64(15),
			},
		},
		"all values changed in manifest": {
			inputPath:               aws.String("/road/to/nowhere"),
			inputHealthyThreshold:   aws.Int64(3),
			inputUnhealthyThreshold: aws.Int64(3),
			inputInterval:           &duration60Seconds,
			inputTimeout:            &duration60Seconds,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    "/road/to/nowhere",
				HealthyThreshold:   aws.Int64(3),
				UnhealthyThreshold: aws.Int64(3),
				Interval:           aws.Int64(60),
				Timeout:            aws.Int64(60),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			hc := manifest.HealthCheckArgsOrString{
				HealthCheckPath: tc.inputPath,
				HealthCheckArgs: manifest.HTTPHealthCheckArgs{
					Path:               tc.inputPath,
					HealthyThreshold:   tc.inputHealthyThreshold,
					UnhealthyThreshold: tc.inputUnhealthyThreshold,
					Timeout:            tc.inputTimeout,
					Interval:           tc.inputInterval,
				},
			}
			// WHEN
			actualOpts := convertHTTPHealthCheck(&hc)

			// THEN
			require.Equal(t, tc.wantedOpts, actualOpts)
		})
	}
}

func Test_convertManagedFSInfo(t *testing.T) {
	testCases := map[string]struct {
		inVolumes         map[string]manifest.Volume
		wantManagedConfig *template.ManagedVolumeCreationInfo
		wantVolumes       map[string]manifest.Volume
		wantErr           string
	}{
		"no managed config": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
			wantManagedConfig: nil,
			wantVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
		},
		"with managed config": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
			wantManagedConfig: &template.ManagedVolumeCreationInfo{
				Name:    aws.String("wordpress"),
				DirName: aws.String("fe"),
				UID:     aws.Uint32(1336298249),
				GID:     aws.Uint32(1336298249),
			},
			wantVolumes: map[string]manifest.Volume{},
		},
		"with custom UID": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							UID: aws.Uint32(10000),
							GID: aws.Uint32(100000),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
			wantManagedConfig: &template.ManagedVolumeCreationInfo{
				Name:    aws.String("wordpress"),
				DirName: aws.String("fe"),
				UID:     aws.Uint32(10000),
				GID:     aws.Uint32(100000),
			},
			wantVolumes: map[string]manifest.Volume{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			gotManaged, err := convertManagedFSInfo(aws.String("fe"), tc.inVolumes)

			// THEN
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantManagedConfig, gotManaged)
			}
		})
	}
}
func Test_convertStorageOpts(t *testing.T) {
	testCases := map[string]struct {
		inVolumes map[string]manifest.Volume
		wantOpts  template.StorageOpts
		wantErr   string
	}{
		"minimal configuration": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("wordpress"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							IAM:           aws.String("DISABLED"),
						},
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("wordpress"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID: aws.String("fs-1234"),
						Write:        false,
					},
				},
			},
		},
		"empty volume for shareable storage between sidecar and main container": {
			inVolumes: map[string]manifest.Volume{
				"scratch": {
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/scratch"),
					},
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(false),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("scratch"),
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/scratch"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("scratch"),
					},
				},
			},
		},
		"fsid not specified": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							RootDirectory: aws.String("/"),
						},
					},
				},
			},
			wantErr: fmt.Sprintf("validate EFS configuration for volume wordpress: %s", errNoFSID.Error()),
		},
		"container path not specified": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
				},
			},
			wantErr: fmt.Sprintf("validate container configuration for volume wordpress: %s", errNoContainerPath.Error()),
		},
		"full specification with access point renders correctly": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							AuthConfig: &manifest.AuthorizationConfig{
								IAM:           aws.Bool(true),
								AccessPointID: aws.String("ap-1234"),
							},
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("wordpress"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							IAM:           aws.String("ENABLED"),
							AccessPointID: aws.String("ap-1234"),
						},
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
						SourceVolume:  aws.String("wordpress"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID:  aws.String("fs-1234"),
						AccessPointID: aws.String("ap-1234"),
						Write:         true,
					},
				},
			},
		},
		"full specification without access point renders correctly": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/wordpress"),
							AuthConfig: &manifest.AuthorizationConfig{
								IAM: aws.Bool(true),
							},
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("wordpress"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/wordpress"),
							IAM:           aws.String("ENABLED"),
						},
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
						SourceVolume:  aws.String("wordpress"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID: aws.String("fs-1234"),
						Write:        true,
					},
				},
			},
		},
		"error when AP is specified with root dir": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/wordpress"),
							AuthConfig: &manifest.AuthorizationConfig{
								IAM:           aws.Bool(true),
								AccessPointID: aws.String("ap-1234"),
							},
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantErr: fmt.Sprintf("validate EFS configuration for volume wordpress: %s", errAccessPointWithRootDirectory.Error()),
		},
		"error when AP is specified without IAM": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							AuthConfig: &manifest.AuthorizationConfig{
								IAM:           aws.Bool(false),
								AccessPointID: aws.String("ap-1234"),
							},
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantErr: fmt.Sprintf("validate EFS configuration for volume wordpress: %s", errAccessPointWithoutIAM.Error()),
		},
		"managed EFS": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantOpts: template.StorageOpts{
				ManagedVolumeInfo: &template.ManagedVolumeCreationInfo{
					Name:    aws.String("efs"),
					DirName: aws.String("fe"),
					UID:     aws.Uint32(1336298249),
					GID:     aws.Uint32(1336298249),
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("efs"),
					},
				},
			},
		},
		"managed EFS with config": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							UID: aws.Uint32(1000),
							GID: aws.Uint32(10000),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantOpts: template.StorageOpts{
				ManagedVolumeInfo: &template.ManagedVolumeCreationInfo{
					Name:    aws.String("efs"),
					DirName: aws.String("fe"),
					UID:     aws.Uint32(1000),
					GID:     aws.Uint32(10000),
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("efs"),
					},
				},
			},
		},
		"error when gid/uid are specified for non-managed efs": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
							UID:          aws.Uint32(1234),
							GID:          aws.Uint32(5678),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantErr: fmt.Sprintf("validate EFS configuration for volume efs: %s", errUIDWithNonManagedFS.Error()),
		},
		"uid/gid out of bounds": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							UID: aws.Uint32(0),
							GID: aws.Uint32(100),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantErr: fmt.Sprintf("validate EFS configuration for volume efs: %s", errReservedUID.Error()),
		},
		"uid specified without gid": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							UID: aws.Uint32(10000),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantErr: fmt.Sprintf("validate EFS configuration for volume efs: %s", errInvalidUIDGIDConfig.Error()),
		},
		"gid specified without uid": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							GID: aws.Uint32(10000),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantErr: fmt.Sprintf("validate EFS configuration for volume efs: %s", errInvalidUIDGIDConfig.Error()),
		},
		"error when multiple managed volumes specified": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/abc"),
					},
				},
			},
			wantErr: "validate managed EFS: cannot specify more than one managed volume per service",
		},
		"managed EFS and BYO": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
				"otherefs": {
					EFS: &manifest.EFSConfigOrBool{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/stuff"),
						ReadOnly:      aws.Bool(false),
					},
				},
				"ephemeral": {
					EFS: nil,
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/ephemeral"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantOpts: template.StorageOpts{
				ManagedVolumeInfo: &template.ManagedVolumeCreationInfo{
					Name:    aws.String("efs"),
					DirName: aws.String("fe"),
					UID:     aws.Uint32(1336298249),
					GID:     aws.Uint32(1336298249),
				},
				Volumes: []*template.Volume{
					{
						Name: aws.String("otherefs"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							IAM:           aws.String("DISABLED"),
						},
					},
					{
						Name: aws.String("ephemeral"),
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("efs"),
					},
					{
						ContainerPath: aws.String("/var/stuff"),
						ReadOnly:      aws.Bool(false),
						SourceVolume:  aws.String("otherefs"),
					},
					{
						ContainerPath: aws.String("/var/ephemeral"),
						ReadOnly:      aws.Bool(false),
						SourceVolume:  aws.String("ephemeral"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID: aws.String("fs-1234"),
						Write:        true,
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			s := manifest.Storage{
				Volumes: tc.inVolumes,
			}

			// WHEN
			got, err := convertStorageOpts(aws.String("fe"), &s)

			// THEN
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantOpts.EFSPerms, got.EFSPerms)
				require.ElementsMatch(t, tc.wantOpts.MountPoints, got.MountPoints)
				require.ElementsMatch(t, tc.wantOpts.Volumes, got.Volumes)
				require.Equal(t, tc.wantOpts.ManagedVolumeInfo, got.ManagedVolumeInfo)
			}
		})
	}
}

func Test_convertExecuteCommand(t *testing.T) {
	testCases := map[string]struct {
		inConfig manifest.ExecuteCommand

		wanted *template.ExecuteCommandOpts
	}{
		"without exec enabled": {
			inConfig: manifest.ExecuteCommand{},
			wanted:   nil,
		},
		"exec enabled": {
			inConfig: manifest.ExecuteCommand{
				Enable: aws.Bool(true),
			},
			wanted: &template.ExecuteCommandOpts{},
		},
		"exec enabled with config": {
			inConfig: manifest.ExecuteCommand{
				Config: manifest.ExecuteCommandConfig{
					Enable: aws.Bool(true),
				},
			},
			wanted: &template.ExecuteCommandOpts{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			exec := tc.inConfig
			got := convertExecuteCommand(&exec)

			require.Equal(t, got, tc.wanted)
		})
	}
}

func Test_convertSidecarMountPoints(t *testing.T) {
	testCases := map[string]struct {
		inMountPoints  []manifest.SidecarMountPoint
		wantErr        string
		wantMountPoint []*template.MountPoint
	}{
		"fully specified": {
			inMountPoints: []manifest.SidecarMountPoint{
				{
					SourceVolume: aws.String("wordpress"),
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www/wp-content"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantMountPoint: []*template.MountPoint{
				{
					ContainerPath: aws.String("/var/www/wp-content"),
					ReadOnly:      aws.Bool(false),
					SourceVolume:  aws.String("wordpress"),
				},
			},
		},
		"readonly defaults to true": {
			inMountPoints: []manifest.SidecarMountPoint{
				{
					SourceVolume: aws.String("wordpress"),
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www/wp-content"),
					},
				},
			},
			wantMountPoint: []*template.MountPoint{
				{
					ContainerPath: aws.String("/var/www/wp-content"),
					ReadOnly:      aws.Bool(true),
					SourceVolume:  aws.String("wordpress"),
				},
			},
		},
		"error when source not specified": {
			inMountPoints: []manifest.SidecarMountPoint{
				{
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www/wp-content"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantErr: errNoSourceVolume.Error(),
		},
		"error when path not specified": {
			inMountPoints: []manifest.SidecarMountPoint{
				{
					SourceVolume: aws.String("wordpress"),
					MountPointOpts: manifest.MountPointOpts{
						ReadOnly: aws.Bool(false),
					},
				},
			},
			wantErr: errNoContainerPath.Error(),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateSidecarMountPoints(tc.inMountPoints)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				got := convertSidecarMountPoints(tc.inMountPoints)
				require.Equal(t, tc.wantMountPoint, got)
			}
		})
	}
}

func Test_validatePaths(t *testing.T) {
	t.Run("containerPath should be properly validated", func(t *testing.T) {
		require.NoError(t, validateContainerPath("/abc/90_"), "contains underscore")
		require.EqualError(t, validateContainerPath("/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "path must be less than 242 bytes in length", "too long")
		require.EqualError(t, validateContainerPath("/etc /bin/sh cat `i'm evil` > /dev/null"), "paths can only contain the characters a-zA-Z0-9.-_/", "invalid characters disallowed")
	})
}
