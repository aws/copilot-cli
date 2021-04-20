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
	EFS *EFSConfigOrID `yaml:"efs"`
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
				EFS: &EFSConfigOrID{
					Config: EFSVolumeConfiguration{
						FileSystemID: aws.String("fs-12345"),
					},
				},
			},
		},
		"with just ID": {
			manifest: []byte(`efs: fs-12345`),
			want: testVolume{
				EFS: &EFSConfigOrID{
					ID: "fs-12345",
				},
			},
		},
		"with magic ID and custom UID": {
			manifest: []byte(`
efs: 
  id: copilot
  uid: 1000
  gid: 10000`),
			want: testVolume{
				EFS: &EFSConfigOrID{
					Config: EFSVolumeConfiguration{
						FileSystemID: aws.String("copilot"),
						UID:          aws.Uint32(1000),
						GID:          aws.Uint32(10000),
					},
				},
			},
		},
		"with just magic ID": {
			manifest: []byte(`
efs: managed`),
			want: testVolume{
				EFS: &EFSConfigOrID{
					ID: "managed",
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
				EFS: &EFSConfigOrID{
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
				EFS: &EFSConfigOrID{},
			}

			// WHEN
			err := yaml.Unmarshal(tc.manifest, &v)
			// THEN
			if tc.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tc.want.EFS.ID, v.EFS.ID)
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
