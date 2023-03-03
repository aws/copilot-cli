// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/asset"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestStaticSiteDeployer_UploadArtifacts(t *testing.T) {
	const mockS3Bucket = "mockBucket"

	tests := map[string]struct {
		mockUploadFn func(fs afero.Fs, source, destination string, opts *asset.UploadOpts) ([]string, error)

		wantErr error
	}{
		"error if failed to upload": {
			mockUploadFn: func(fs afero.Fs, source, destination string, opts *asset.UploadOpts) ([]string, error) {
				return nil, errors.New("some error")
			},
			wantErr: fmt.Errorf("some error"),
		},
		"success": {
			mockUploadFn: func(fs afero.Fs, source, destination string, opts *asset.UploadOpts) ([]string, error) {
				if source != "frontend/assets" {
					return nil, fmt.Errorf("unexpected full source path")
				}
				if opts.Reincludes != nil {
					return nil, fmt.Errorf("unexpected reinclude")
				}
				if len(opts.Excludes) != 1 || opts.Excludes[0] != "*.manifest" {
					return nil, fmt.Errorf("unexpected exclude")
				}
				return nil, nil
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
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
				bucketName: mockS3Bucket,
				uploadFn:   tc.mockUploadFn,
			}
			_, gotErr := deployer.UploadArtifacts()

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}
