// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template/templatetest"
	"github.com/golang/mock/gomock"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/stretchr/testify/require"
)

var testStaticSiteManifest = &manifest.StaticSite{
	Workload: manifest.Workload{
		Name: aws.String(testServiceName),
		Type: aws.String(manifestinfo.StaticSiteType),
	},
}

func TestStaticSite_NewStaticSite(t *testing.T) {
	t.Cleanup(func() {
		fs = realEmbedFS
	})
	fs = templatetest.Stub{}

	type testInput struct {
		mft  *manifest.StaticSite
		env  string
		rc   RuntimeConfig
		app  string
		urls map[string]string
	}

	testCases := map[string]struct {
		input            testInput
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService)

		wantedStack *StaticSite
		wantedError error
	}{
		"should return StaticSite": {
			input: testInput{
				mft: testStaticSiteManifest,
				env: testEnvName,
				rc:  RuntimeConfig{},
				app: testAppName,
				urls: map[string]string{
					"custom-domain-app-runner": "mockURL1",
					"aws-sdk-layer":            "mockURL2",
				},
			},

			wantedStack: &StaticSite{
				wkld: &wkld{
					name: aws.StringValue(testStaticSiteManifest.Name),
					env:  testEnvName,
					app:  testAppName,
				},
				manifest: testStaticSiteManifest,
				appInfo: deploy.AppInformation{
					Name: testAppName,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			addons := mocks.NewMockNestedStackConfigurer(ctrl)

			stack, err := NewStaticSite(StaticSiteConfig{
				EnvManifest: &manifest.Environment{
					Workload: manifest.Workload{
						Name: &tc.input.env,
					},
				},
				App: &config.Application{
					Name: tc.input.app,
				},
				Manifest:      tc.input.mft,
				RuntimeConfig: tc.input.rc,
				Addons:        addons,
			})

			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedStack.name, stack.name)
			require.Equal(t, tc.wantedStack.env, stack.env)
			require.Equal(t, tc.wantedStack.app, stack.app)
			require.Equal(t, tc.wantedStack.rc, stack.rc)
			require.Equal(t, tc.wantedStack.image, stack.image)
			require.Equal(t, tc.wantedStack.manifest, stack.manifest)
			require.NotNil(t, stack.addons)
		})
	}
}

func TestStaticSite_SerializedParameters(t *testing.T) {
	c := &StaticSite{
		wkld: &wkld{
			name: "frontend",
			env:  testEnvName,
			app:  testAppName,
			rc: RuntimeConfig{
				AdditionalTags: map[string]string{
					"owner": "copilot",
				},
			},
		},
		manifest: testStaticSiteManifest,
	}

	params, err := c.SerializedParameters()
	require.NoError(t, err)
	require.Equal(t, params, `{
  "Parameters": {},
  "Tags": {
    "copilot-application": "phonetool",
    "copilot-environment": "test",
    "copilot-service": "frontend",
    "owner": "copilot"
  }
}`)
}
