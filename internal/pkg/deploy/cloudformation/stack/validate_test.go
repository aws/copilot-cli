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
