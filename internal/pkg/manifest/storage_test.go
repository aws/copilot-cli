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
	EFS EFSConfigOrBool `yaml:"efs"`
}

func TestEFSConfigOrBool_UnmarshalYAML(t *testing.T) {
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
				EFS: EFSConfigOrBool{
					Advanced: EFSVolumeConfiguration{
						FileSystemID: StringOrFromCFN{Plain: aws.String("fs-12345")},
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
				EFS: EFSConfigOrBool{
					Advanced: EFSVolumeConfiguration{
						UID: aws.Uint32(1000),
						GID: aws.Uint32(10000),
					},
				},
			},
		},
		"with just managed": {
			manifest: []byte(`
efs: true`),
			want: testVolume{
				EFS: EFSConfigOrBool{
					Enabled: aws.Bool(true),
				},
			},
		},
		"with from_cfn": {
			manifest: []byte(`
efs:
  id:
   from_cfn: expoted-fs-id`),
			want: testVolume{
				EFS: EFSConfigOrBool{
					Advanced: EFSVolumeConfiguration{
						FileSystemID: StringOrFromCFN{FromCFN: fromCFN{Name: aws.String("expoted-fs-id")}},
					},
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
				EFS: EFSConfigOrBool{
					Advanced: EFSVolumeConfiguration{
						FileSystemID:  StringOrFromCFN{Plain: aws.String("fs-12345")},
						RootDirectory: aws.String("/"),
						AuthConfig: AuthorizationConfig{
							IAM:           aws.Bool(true),
							AccessPointID: aws.String("fsap-1234"),
						},
					},
				},
			},
		},
		"invalid": {
			manifest: []byte(`
efs: 
  uid: 1000
  gid: 10000
  id: 1`),
			wantErr: `must specify one, not both, of "uid/gid" and "id/root_dir/auth"`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			v := testVolume{
				EFS: EFSConfigOrBool{},
			}

			// WHEN
			err := yaml.Unmarshal(tc.manifest, &v)
			// THEN
			if tc.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tc.want.EFS.Enabled, v.EFS.Enabled)
				require.Equal(t, tc.want.EFS.Advanced.FileSystemID, v.EFS.Advanced.FileSystemID)
				require.Equal(t, tc.want.EFS.Advanced.AuthConfig, v.EFS.Advanced.AuthConfig)
				require.Equal(t, tc.want.EFS.Advanced.UID, v.EFS.Advanced.UID)
				require.Equal(t, tc.want.EFS.Advanced.GID, v.EFS.Advanced.GID)
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
				Advanced: EFSVolumeConfiguration{
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
				Advanced: EFSVolumeConfiguration{
					FileSystemID: StringOrFromCFN{Plain: aws.String("fs-12345")},
				},
			},
			want: false,
		},
		"misconfigured with FSID and UID": {
			in: EFSConfigOrBool{
				Advanced: EFSVolumeConfiguration{
					FileSystemID: StringOrFromCFN{Plain: aws.String("fs-12345")},
					UID:          aws.Uint32(6777),
					GID:          aws.Uint32(6777),
				},
			},
			want: false,
		},
		"misconfigured with bool set to false and extra config (should respect bool)": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(false),
				Advanced: EFSVolumeConfiguration{
					UID: aws.Uint32(6777),
					GID: aws.Uint32(6777),
				},
			},
			want: true,
		},
	}
	for name, tc := range testCases {
		v := Volume{
			EFS: tc.in,
		}
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, v.EmptyVolume())
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
				Advanced: EFSVolumeConfiguration{
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
				Advanced: EFSVolumeConfiguration{
					FileSystemID: StringOrFromCFN{Plain: aws.String("fs-12345")},
				},
			},
			want: true,
		},
		"misconfigured with FSID and UID": {
			in: EFSConfigOrBool{
				Advanced: EFSVolumeConfiguration{
					FileSystemID: StringOrFromCFN{Plain: aws.String("fs-12345")},
					UID:          aws.Uint32(6777),
					GID:          aws.Uint32(6777),
				},
			},
			want: true,
		},
		"misconfigured with bool set to false and extra config (should respect bool)": {
			in: EFSConfigOrBool{
				Enabled: aws.Bool(false),
				Advanced: EFSVolumeConfiguration{
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

func TestStorage_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     Storage
		wanted bool
	}{
		"empty storage": {
			in:     Storage{},
			wanted: true,
		},
		"non empty storage with ReadOnlyFS": {
			in: Storage{
				ReadonlyRootFS: aws.Bool(true),
			},
		},
		"non empty storage": {
			in: Storage{
				Volumes: map[string]*Volume{
					"volume1": nil,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.in.IsEmpty()

			// THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestAuthorizationConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     AuthorizationConfig
		wanted bool
	}{
		"empty auth": {
			in:     AuthorizationConfig{},
			wanted: true,
		},
		"non empty auth": {
			in: AuthorizationConfig{
				IAM: aws.Bool(false),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.in.IsEmpty()

			// THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}
