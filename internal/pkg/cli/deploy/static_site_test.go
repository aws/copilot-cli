// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/asset"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestStaticSiteDeployer_UploadArtifacts(t *testing.T) {
	tests := map[string]struct {
		mock func(m *mocks.MockcacheUploader)

		expected *UploadArtifactsOutput
		wantErr  error
	}{
		"error if failed to upload": {
			mock: func(m *mocks.MockcacheUploader) {
				m.EXPECT().UploadToCache("frontend/assets", "static", &asset.UploadOpts{
					Recursive: true,
					Excludes:  []string{"*.manifest"},
				}).Return(nil, errors.New("some error"))
			},
			wantErr: fmt.Errorf("upload #1/1: some error"),
		},
		"success": {
			mock: func(m *mocks.MockcacheUploader) {
				m.EXPECT().UploadToCache("frontend/assets", "static", &asset.UploadOpts{
					Recursive: true,
					Excludes:  []string{"*.manifest"},
				}).Return([]asset.Cached{
					{
						LocalPath:       "frontend/assets/index.html",
						CachePath:       "/asdf",
						DestinationPath: "/index.html",
					},
				}, nil)
			},
			expected: &UploadArtifactsOutput{
				CustomResourceURLs: map[string]string{},
				CachedAssets: []asset.Cached{
					{
						LocalPath:       "frontend/assets/index.html",
						CachePath:       "/asdf",
						DestinationPath: "/index.html",
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := mocks.NewMockcacheUploader(ctrl)
			if tc.mock != nil {
				tc.mock(m)
			}

			deployer := &staticSiteDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						customResources: func(fs template.Reader) ([]*customresource.CustomResource, error) {
							return nil, nil
						},
						mft: &mockWorkloadMft{},
					},
				},
				staticSiteMft: &manifest.StaticSite{
					StaticSiteConfig: manifest.StaticSiteConfig{
						FileUploads: []manifest.FileUpload{
							{
								Source:      "assets",
								Context:     "frontend",
								Destination: "static",
								Recursive:   true,
								Exclude: manifest.StringSliceOrString{
									String: aws.String("*.manifest"),
								},
							},
						},
					},
				},
				uploader: m,
				// bucketName: mockS3Bucket,
			}

			actual, err := deployer.UploadArtifacts()
			if tc.wantErr != nil {
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}
