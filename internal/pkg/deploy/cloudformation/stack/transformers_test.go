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
		inLabels    map[string]string

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
			inLabels: map[string]string{
				"com.amazonaws.ecs.copilot.sidecar.description": "wow",
			},

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				DockerLabels: map[string]string{
					"com.amazonaws.ecs.copilot.sidecar.description": "wow",
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sidecar := map[string]*manifest.SidecarConfig{
				"foo": {
					CredsParam:   mockCredsParam,
					Image:        mockImage,
					Secrets:      mockMap,
					Variables:    mockMap,
					Essential:    aws.Bool(tc.inEssential),
					Port:         aws.String(tc.inPort),
					DockerLabels: tc.inLabels,
				},
			}
			got, err := convertSidecar(sidecar)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, got[0])
			}
		})
	}
}

func Test_convertCapacityProviders(t *testing.T) {
	mockRange := manifest.Range("1-10")
	minCapacity := 1
	spotFrom := 3
	testCases := map[string]struct {
		input       *manifest.AdvancedCount
		expected    []*template.CapacityProviderStrategy
		expectedErr error
	}{
		"with spot as desiredCount": {
			input: &manifest.AdvancedCount{
				Spot: aws.Int(3),
			},

			expected: []*template.CapacityProviderStrategy{
				{
					Weight:           aws.Int(1),
					CapacityProvider: capacityProviderFargateSpot,
				},
			},
		},
		"with scaling only on spot": {
			input: &manifest.AdvancedCount{
				Range: &manifest.RangeOpts{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(minCapacity),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(minCapacity),
					},
				},
			},

			expected: []*template.CapacityProviderStrategy{
				{
					Weight:           aws.Int(1),
					CapacityProvider: capacityProviderFargateSpot,
				},
			},
		},
		"with scaling into spot": {
			input: &manifest.AdvancedCount{
				Range: &manifest.RangeOpts{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(minCapacity),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(spotFrom),
					},
				},
			},

			expected: []*template.CapacityProviderStrategy{
				{
					Weight:           aws.Int(1),
					CapacityProvider: capacityProviderFargateSpot,
				},
				{
					Base:             aws.Int(spotFrom - 1),
					Weight:           aws.Int(0),
					CapacityProvider: capacityProviderFargate,
				},
			},
		},
		"errors if spot specified with range": {
			input: &manifest.AdvancedCount{
				Range: &manifest.RangeOpts{
					Range: &mockRange,
				},
				Spot: aws.Int(3),
			},
			expectedErr: errInvalidSpotConfig,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual, err := convertCapacityProviders(tc.input)

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}

func Test_convertAutoscaling(t *testing.T) {
	mockRange := manifest.Range("1-100")
	badRange := manifest.Range("badRange")
	mockRequests := 1000
	mockResponseTime := 512 * time.Millisecond
	testCases := map[string]struct {
		input *manifest.AdvancedCount

		wanted    *template.AutoscalingOpts
		wantedErr error
	}{
		"invalid range": {
			input: &manifest.AdvancedCount{
				Range: &manifest.RangeOpts{
					Range: &badRange,
				},
			},

			wantedErr: fmt.Errorf("invalid range value badRange. Should be in format of ${min}-${max}"),
		},
		"success": {
			input: &manifest.AdvancedCount{
				Range: &manifest.RangeOpts{
					Range: &mockRange,
				},
				CPU:          aws.Int(70),
				Memory:       aws.Int(80),
				Requests:     aws.Int(mockRequests),
				ResponseTime: &mockResponseTime,
			},

			wanted: &template.AutoscalingOpts{
				MaxCapacity:  aws.Int(100),
				MinCapacity:  aws.Int(1),
				CPU:          aws.Float64(70),
				Memory:       aws.Float64(80),
				Requests:     aws.Float64(1000),
				ResponseTime: aws.Float64(0.512),
			},
		},
		"success with range subfields": {
			input: &manifest.AdvancedCount{
				Range: &manifest.RangeOpts{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(5),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(5),
					},
				},
				CPU:          aws.Int(70),
				Memory:       aws.Int(80),
				Requests:     aws.Int(mockRequests),
				ResponseTime: &mockResponseTime,
			},

			wanted: &template.AutoscalingOpts{
				MaxCapacity:  aws.Int(10),
				MinCapacity:  aws.Int(5),
				CPU:          aws.Float64(70),
				Memory:       aws.Float64(80),
				Requests:     aws.Float64(1000),
				ResponseTime: aws.Float64(0.512),
			},
		},
		"returns nil if spot specified": {
			input: &manifest.AdvancedCount{
				Spot: aws.Int(5),
			},
			wanted: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := convertAutoscaling(tc.input)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, got)
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

func Test_convertStorageOpts(t *testing.T) {
	testCases := map[string]struct {
		inVolumes map[string]manifest.Volume
		wantOpts  template.StorageOpts
		wantErr   string
	}{
		"minimal configuration": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrID{
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
					EFS: &manifest.EFSConfigOrID{
						Config: manifest.EFSVolumeConfiguration{
							RootDirectory: aws.String("/"),
						},
					},
				},
			},
			wantErr: errNoFSID.Error(),
		},
		"container path not specified": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrID{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
				},
			},
			wantErr: errNoContainerPath.Error(),
		},
		"full specification with access point renders correctly": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrID{
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
					EFS: &manifest.EFSConfigOrID{
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
					EFS: &manifest.EFSConfigOrID{
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
			wantErr: errAcessPointWithRootDirectory.Error(),
		},
		"error when AP is specified without IAM": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrID{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/wordpress"),
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
			wantErr: errAccessPointWithoutIAM.Error(),
		},
		"efs specified with just ID": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrID{
						ID: "fs-1234",
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
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
		"managed EFS": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrID{
						ID: "copilot",
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
					EFS: &manifest.EFSConfigOrID{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("copilot"),
							UID:          aws.Uint32(1000),
							GID:          aws.Uint32(10000),
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
					EFS: &manifest.EFSConfigOrID{
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
			wantErr: errUIDWithNonManagedFS.Error(),
		},
		"uid/gid out of bounds": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrID{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("managed"),
							UID:          aws.Uint32(0),
							GID:          aws.Uint32(100),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantErr: errReservedUID.Error(),
		},
		"uid specified without gid": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrID{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("managed"),
							UID:          aws.Uint32(10000),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantErr: errInvalidUIDGIDConfig.Error(),
		},
		"gid specified without uid": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrID{
						Config: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("managed"),
							GID:          aws.Uint32(10000),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantErr: errInvalidUIDGIDConfig.Error(),
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
				require.Equal(t, &tc.wantOpts, got)
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

			require.Equal(t, tc.wanted, got)
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
			got, err := convertSidecarMountPoints(tc.inMountPoints)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
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
