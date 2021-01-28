// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEFSIDOrConfig_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct EFSIDOrConfig
		wantedError  error
	}{
		"simple fs id": {
			inContent:    []byte(`efs: fs-12345`),
			wantedStruct: EFSIDOrConfig{EFSID: aws.String("fs-12345")},
		},
		"full specification": {
			inContent: []byte(`
efs: 
  filesystem_id: fs-12345
  root_directory: "/"
  transit_encryption: false
  authorization_config:
    access_point_id: ap-567
    iam: true
`),
			wantedStruct: EFSIDOrConfig{
				EFSConfig: EFSVolumeConfiguration{
					FileSystemID:      aws.String("fs-12345"),
					RootDirectory:     aws.String("/"),
					TransitEncryption: false,
					AuthConfig: AuthorizationConfig{
						IAM:           true,
						AccessPointID: aws.String("ap-567"),
					},
				},
			},
		},
		"error if unmarshalable": {
			inContent: []byte(`
efs:
  wat: false
`),
			wantedError: errUnmarshalEFSOpts,
		},
	}

	type testStorageStruct struct {
		EFS EFSIDOrConfig `yaml:"efs"`
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			s := testStorageStruct{}

			err := yaml.Unmarshal(tc.inContent, &s)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				// check memberwise dereferenced pointer equality
				require.Equal(t, tc.wantedStruct.EFSID, s.EFS.EFSID)
				require.Equal(t, tc.wantedStruct.EFSConfig.FileSystemID, s.EFS.EFSConfig.FileSystemID)
				require.Equal(t, tc.wantedStruct.EFSConfig.RootDirectory, s.EFS.EFSConfig.RootDirectory)
				require.Equal(t, tc.wantedStruct.EFSConfig.TransitEncryption, s.EFS.EFSConfig.TransitEncryption)
				require.Equal(t, tc.wantedStruct.EFSConfig.AuthConfig.AccessPointID, s.EFS.EFSConfig.AuthConfig.AccessPointID)
				require.Equal(t, tc.wantedStruct.EFSConfig.AuthConfig.IAM, s.EFS.EFSConfig.AuthConfig.IAM)
				require.Equal(t, tc.wantedStruct.EFSConfig.isEmpty(), s.EFS.EFSConfig.isEmpty())
			}
		})
	}
}
