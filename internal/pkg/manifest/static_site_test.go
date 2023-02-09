// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/stretchr/testify/require"
)

func TestStaticSite_ApplyEnv(t *testing.T) {
	var ()
	testCases := map[string]struct {
		in         *StaticSite
		envToApply string

		wanted *StaticSite
	}{
		"without existing environments": {
			in: &StaticSite{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.StaticSiteType),
				},
				StaticSiteConfig: StaticSiteConfig{
					FileUploads: []FileUpload{
						{
							Source:      "test",
							Destination: "test",
							Reinclude: StringSliceOrString{
								StringSlice: []string{"test/manifest.yml"},
							},
							Exclude: StringSliceOrString{
								String: aws.String("test/*.yml"),
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &StaticSite{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.StaticSiteType),
				},
				StaticSiteConfig: StaticSiteConfig{
					FileUploads: []FileUpload{
						{
							Source:      "test",
							Destination: "test",
							Reinclude: StringSliceOrString{
								StringSlice: []string{"test/manifest.yml"},
							},
							Exclude: StringSliceOrString{
								String: aws.String("test/*.yml"),
							},
						},
					},
				},
			},
		},
		"with overrides": {
			in: &StaticSite{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.StaticSiteType),
				},
				StaticSiteConfig: StaticSiteConfig{
					FileUploads: []FileUpload{
						{
							Exclude: StringSliceOrString{
								String: aws.String("test/*.yml"),
							},
						},
					},
				},
				Environments: map[string]*StaticSiteConfig{
					"prod-iad": {
						FileUploads: []FileUpload{
							{
								Reinclude: StringSliceOrString{
									StringSlice: []string{"test/manifest.yml"},
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &StaticSite{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.StaticSiteType),
				},
				StaticSiteConfig: StaticSiteConfig{
					FileUploads: []FileUpload{
						{
							Reinclude: StringSliceOrString{
								StringSlice: []string{"test/manifest.yml"},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			conf, _ := tc.in.applyEnv(tc.envToApply)

			// THEN
			require.Equal(t, tc.wanted, conf, "returned configuration should have overrides from the environment")
		})
	}
}
