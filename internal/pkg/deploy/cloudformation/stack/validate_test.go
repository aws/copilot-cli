// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/stretchr/testify/require"
)

func Test_validateEFSConfig(t *testing.T) {
	testCases := map[string]struct {
		inConfig *manifest.EFSConfigOrBool

		wantErr error
	}{
		"no EFS config": {
			inConfig: nil,
			wantErr:  nil,
		},
		"managed EFS config": {
			inConfig: &manifest.EFSConfigOrBool{
				Enabled: aws.Bool(true),
			},
		},
		"EFS explicitly disabled": {
			inConfig: &manifest.EFSConfigOrBool{
				Enabled: aws.Bool(false),
			},
		},
		"advanced managed EFS config": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					UID: aws.Uint32(12345),
					GID: aws.Uint32(12345),
				},
			},
		},
		"BYO EFS": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					FileSystemID:  aws.String("fs-1234"),
					RootDirectory: aws.String("/files"),
					AuthConfig: &manifest.AuthorizationConfig{
						IAM: aws.Bool(true),
					},
				},
			},
		},
		"error when access point specified with root dir": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					FileSystemID:  aws.String("fs-1234"),
					RootDirectory: aws.String("/files"),
					AuthConfig: &manifest.AuthorizationConfig{
						IAM:           aws.Bool(true),
						AccessPointID: aws.String("fsap-12345"),
					},
				},
			},
			wantErr: errAccessPointWithRootDirectory,
		},
		"error when access point specified without IAM": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					FileSystemID: aws.String("fs-1234"),
					AuthConfig: &manifest.AuthorizationConfig{
						IAM:           aws.Bool(false),
						AccessPointID: aws.String("fsap-12345"),
					},
				},
			},
			wantErr: errAccessPointWithoutIAM,
		},
		"Enabled with advanced config": {
			inConfig: &manifest.EFSConfigOrBool{
				Enabled: aws.Bool(true),
				Advanced: manifest.EFSVolumeConfiguration{
					UID: aws.Uint32(12345),
					GID: aws.Uint32(12345),
				},
			},
			wantErr: errInvalidEFSConfig,
		},
		"UID with BYO": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					FileSystemID: aws.String("fs-1234"),
					UID:          aws.Uint32(12345),
					GID:          aws.Uint32(12345),
				},
			},
			wantErr: errUIDWithNonManagedFS,
		},
		"invalid UID config": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					UID: aws.Uint32(12345),
				},
			},
			wantErr: errInvalidUIDGIDConfig,
		},
		"invalid GID config": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					GID: aws.Uint32(12345),
				},
			},
			wantErr: errInvalidUIDGIDConfig,
		},
		"error when UID is 0": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					UID: aws.Uint32(0),
					GID: aws.Uint32(12345),
				},
			},
			wantErr: errReservedUID,
		},
		"empty EFS config should be invalid": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{},
			},
			wantErr: errEmptyEFSConfig,
		},
		"FSID not specified for BYO": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					RootDirectory: aws.String("/storage"),
					AuthConfig: &manifest.AuthorizationConfig{
						IAM: aws.Bool(true),
					},
				},
			},
			wantErr: errNoFSID,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			vol := manifest.Volume{
				EFS: tc.inConfig,
			}
			gotErr := validateEFSConfig(vol)
			if tc.wantErr == nil {
				require.NoError(t, gotErr)
			} else {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateSidecarDependsOn(t *testing.T) {
	mockSidecarName := "sidecar"
	mockManifestName := "frontend"
	testCases := map[string]struct {
		inSidecar   *manifest.SidecarConfig
		allSidecars map[string]*manifest.SidecarConfig

		wantErr error
	}{
		"No sidecar dependencies": {
			inSidecar: &manifest.SidecarConfig{},
			wantErr:   nil,
		},
		"Working set essential sidecar with container dependency": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar1": "start",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar1": {
					Essential: aws.Bool(true),
				},
			},
			wantErr: nil,
		},
		"Working implied essential container with container dependency": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"frontend": "start",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: nil,
		},
		"Working non-essential sidecar with container dependency": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar2": "complete",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
				"sidecar2": {
					Essential: aws.Bool(false),
				},
			},
			wantErr: nil,
		},
		"Error when sidecar container dependency status is invalid": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar2": "end",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
				"sidecar2": {
					Essential: aws.Bool(false),
				},
			},
			wantErr: errInvalidDependsOnStatus,
		},
		"Error when set essential sidecar has a status besides start": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar2": "complete",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
				"sidecar2": {
					Essential: aws.Bool(true),
				},
			},
			wantErr: errEssentialContainerStatus,
		},
		"Error when implied essential sidecar has a status besides start": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"frontend": "complete",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: errEssentialContainerStatus,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := validateSidecarDependsOn(*tc.inSidecar, mockSidecarName, tc.allSidecars, mockManifestName)
			if tc.wantErr == nil {
				require.NoError(t, gotErr)
			} else {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateNoCircularDependency(t *testing.T) {
	mockManifestName := "frontend"
	image := manifest.Image{}
	testCases := map[string]struct {
		allSidecars map[string]*manifest.SidecarConfig

		wantErr error
	}{
		"No sidecar dependencies": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: nil,
		},
		"Working sidecars with container dependency": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"frontend": "start",
					},
				},
			},
			wantErr: nil,
		},
		"Error when sidecar depends on itself": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"sidecar": "start",
					},
				},
			},
			wantErr: errCircularDependency,
		},
		"Error when sidecars circularly depend on each other": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"frontend": "start",
					},
				},
				"frontend": {
					DependsOn: map[string]string{
						"sidecar": "start",
					},
				},
			},
			wantErr: errCircularDependency,
		},
		"Error when sidecars inadvertently depend on each other": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"secondCar": "start",
					},
				},
				"secondCar": {
					DependsOn: map[string]string{
						"thirdCar": "start",
					},
				},
				"thirdCar": {
					DependsOn: map[string]string{
						"fourthCar": "start",
					},
				},
				"fourthCar": {
					DependsOn: map[string]string{
						"sidecar": "start",
					},
				},
			},
			wantErr: errCircularDependency,
		},
		"Error when container doesn't exist": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"something": "start",
					},
				},
			},
			wantErr: errInvalidContainer,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := validateNoCircularDependencies(tc.allSidecars, image, mockManifestName)
			if tc.wantErr == nil {
				require.NoError(t, gotErr)
			} else {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateImageDependsOn(t *testing.T) {
	mockManifestName := "frontend"
	testCases := map[string]struct {
		inImage    *manifest.Image
		inSidecars map[string]*manifest.SidecarConfig

		wantErr error
	}{
		"No image container dependencies": {
			inImage: &manifest.Image{},
			wantErr: nil,
		},
		"Working image with container dependency": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "start",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: nil,
		},
		"Error when image depends on itself": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"frontend": "start",
				},
			},
			wantErr: errCircularDependency,
		},
		"Error when image container dependency status is invalid": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "end",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: errInvalidDependsOnStatus,
		},
		"Error when set essential container has a status besides start": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "complete",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					Essential: aws.Bool(true),
				},
			},
			wantErr: errEssentialContainerStatus,
		},
		"Error when implied essential container has a status besides start": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "complete",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: errEssentialContainerStatus,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := validateImageDependsOn(*tc.inImage, tc.inSidecars, mockManifestName)
			if tc.wantErr == nil {
				require.NoError(t, gotErr)
			} else {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})
	}
}
