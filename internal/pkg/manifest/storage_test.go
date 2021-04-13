// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type testVolume struct {
	EFS *EFSConfigOrBool `yaml:"efs"`
}

func Test_UnmarshalEFS(t *testing.T) {
	testCases := map[string]struct {
		manifest []byte
		want     testVolume
		wantErr  string
	}{
		"simple case": {
			manifest: []byte(`
efs: 
  id: fs-12345`),
			want: testVolume{
				EFS: &EFSConfigOrBool{
					Config: EFSVolumeConfiguration{
						FileSystemID: aws.String("fs-12345"),
					},
				},
			},
		},
		"with managed FS and custom UID": {
			manifest: []byte(`
efs: 
  uid: 1000
  gid: 10000`),
			want: testVolume{
				EFS: &EFSConfigOrBool{
					Config: EFSVolumeConfiguration{
						UID: aws.Uint32(1000),
						GID: aws.Uint32(10000),
					},
				},
			},
		},
		"with just managed ": {
			manifest: []byte(`
efs: true`),
			want: testVolume{
				EFS: &EFSConfigOrBool{
					Enabled: aws.Bool(true),
				},
			},
		},
		"with auth": {
			manifest: []byte(`
efs:
  id: fs-12345
  root_directory: "/"
  auth:
    iam: true
    access_point_id: fsap-1234`),
			want: testVolume{
				EFS: &EFSConfigOrBool{
					Config: EFSVolumeConfiguration{
						FileSystemID:  aws.String("fs-12345"),
						RootDirectory: aws.String("/"),
						AuthConfig: &AuthorizationConfig{
							IAM:           aws.Bool(true),
							AccessPointID: aws.String("fsap-1234"),
						},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			v := testVolume{
				EFS: &EFSConfigOrBool{},
			}

			// WHEN
			err := yaml.Unmarshal(tc.manifest, &v)
			// THEN
			if tc.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tc.want.EFS.Enabled, v.EFS.Enabled)
				require.Equal(t, tc.want.EFS.Config.FileSystemID, v.EFS.Config.FileSystemID)
				require.Equal(t, tc.want.EFS.Config.AuthConfig, v.EFS.Config.AuthConfig)
				require.Equal(t, tc.want.EFS.Config.UID, v.EFS.Config.UID)
				require.Equal(t, tc.want.EFS.Config.GID, v.EFS.Config.GID)
			} else {
				require.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

func Test_EmptyVolume(t *testing.T) {
	testCases := map[string]struct {
		in   EFSConfigOrBool
		want bool
	}{
		"with bool set": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(true),
			},
			want: false,
		},
		"with bool set to false": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(false),
			},
			want: true,
		},
		"with uid/gid set": {
			in: EFSConfigOrBool{
				Config: EFSVolumeConfiguration{
					UID: aws.Uint32(1000),
					GID: aws.Uint32(10000),
				},
			},
			want: false,
		},
		"empty": {
			in:   EFSConfigOrBool{},
			want: true,
		},
		"misconfigured with boolean enabled": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(true),
				Config: EFSVolumeConfiguration{
					FileSystemID: aws.String("fs-1234"),
				},
			},
			want: false,
		},
		"misconfigured with FSID and UID": {
			in: EFSConfigOrBool{
				Config: EFSVolumeConfiguration{
					FileSystemID: aws.String("fs-12345"),
					UID:          aws.Uint32(6777),
					GID:          aws.Uint32(6777),
				},
			},
			want: false,
		},
		"misconfigured with bool set to false and extra config (should respect bool)": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(false),
				Config: EFSVolumeConfiguration{
					UID: aws.Uint32(6777),
					GID: aws.Uint32(6777),
				},
			},
			want: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.in.EmptyVolume())
		})
	}

}
func Test_UseManagedFS(t *testing.T) {
	testCases := map[string]struct {
		in   EFSConfigOrBool
		want bool
	}{
		"with bool set": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(true),
			},
			want: true,
		},
		"with bool set to false": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(false),
			},
			want: false,
		},
		"with uid/gid set": {
			in: EFSConfigOrBool{
				Config: EFSVolumeConfiguration{
					UID: aws.Uint32(1000),
					GID: aws.Uint32(10000),
				},
			},
			want: true,
		},
		"empty": {
			in:   EFSConfigOrBool{},
			want: false,
		},
		"misconfigured with boolean enabled": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(true),
				Config: EFSVolumeConfiguration{
					FileSystemID: aws.String("fs-1234"),
				},
			},
			want: true,
		},
		"misconfigured with FSID and UID": {
			in: EFSConfigOrBool{
				Config: EFSVolumeConfiguration{
					FileSystemID: aws.String("fs-12345"),
					UID:          aws.Uint32(6777),
					GID:          aws.Uint32(6777),
				},
			},
			want: true,
		},
		"misconfigured with bool set to false and extra config (should respect bool)": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(false),
				Config: EFSVolumeConfiguration{
					UID: aws.Uint32(6777),
					GID: aws.Uint32(6777),
				},
			},
			want: false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.in.UseManagedFS())
		})
	}
}
