// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template/templatetest"
	"github.com/golang/mock/gomock"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
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
				rc: RuntimeConfig{
					Region: "us-west-2",
				},
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
					rc: RuntimeConfig{
						CustomResourcesURL: map[string]string{
							"CertificateValidationFunction": "https://mockBucket.s3.us-west-2.amazonaws.com/manual/scripts/custom-resources/certificatevalidationfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
							"CustomDomainFunction":          "https://mockBucket.s3.us-west-2.amazonaws.com/manual/scripts/custom-resources/customdomainfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
							"TriggerStateMachineFunction":   "https://mockBucket.s3.us-west-2.amazonaws.com/manual/scripts/custom-resources/triggerstatemachinefunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
						},
						Region: "us-west-2",
					},
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

			stack, err := NewStaticSite(&StaticSiteConfig{
				EnvManifest: &manifest.Environment{
					Workload: manifest.Workload{
						Name: &tc.input.env,
					},
				},
				App: &config.Application{
					Name: tc.input.app,
				},
				Manifest:           tc.input.mft,
				RuntimeConfig:      tc.input.rc,
				ArtifactBucketName: "mockBucket",
				Addons:             addons,
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

func TestStaticSite_Template(t *testing.T) {
	t.Cleanup(func() {
		fs = realEmbedFS
	})
	fs = templatetest.Stub{}

	t.Run("returns a wrapped error when addons template parsing fails", func(t *testing.T) {
		// GIVEN
		static, err := NewStaticSite(&StaticSiteConfig{
			App:         &config.Application{},
			EnvManifest: &manifest.Environment{},
			Manifest: &manifest.StaticSite{
				Workload: manifest.Workload{
					Name: aws.String("frontend"),
				},
			},
			ArtifactBucketName: "mockBucket",
			RuntimeConfig: RuntimeConfig{
				Region: "us-west-2",
			},
			Addons: mockAddons{tplErr: errors.New("some error")},
		})
		require.NoError(t, err)

		// WHEN
		_, err = static.Template()

		// THEN
		require.EqualError(t, err, "generate addons template for frontend: some error")
	})

	t.Run("returns a wrapped error when addons parameter parsing fails", func(t *testing.T) {
		// GIVEN
		static, err := NewStaticSite(&StaticSiteConfig{
			App:         &config.Application{},
			EnvManifest: &manifest.Environment{},
			Manifest: &manifest.StaticSite{
				Workload: manifest.Workload{
					Name: aws.String("frontend"),
				},
			},
			ArtifactBucketName: "mockBucket",
			RuntimeConfig: RuntimeConfig{
				Region: "us-west-2",
			},
			Addons: mockAddons{paramsErr: errors.New("some error")},
		})
		require.NoError(t, err)

		// WHEN
		_, err = static.Template()

		// THEN
		require.EqualError(t, err, "parse addons parameters for frontend: some error")
	})

	t.Run("returns a wrapped error when asset mapping url is invalid", func(t *testing.T) {
		// GIVEN
		addons := mockAddons{
			tpl: `
  Resources:
    AdditionalResourcesPolicy:
      Type: AWS::IAM::ManagedPolicy
  Outputs:
    AdditionalResourcesPolicyArn:
      Value: hello`,
		}
		static, err := NewStaticSite(&StaticSiteConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			Manifest:           &manifest.StaticSite{},
			Addons:             addons,
			ArtifactBucketName: "mockBucket",
			RuntimeConfig: RuntimeConfig{
				Region: "us-west-2",
			},
			AssetMappingURL: "notAnS3URL",
		})
		require.NoError(t, err)

		// WHEN
		_, err = static.Template()

		// THEN
		require.EqualError(t, err, "cannot parse S3 URL notAnS3URL into bucket name and key")
	})

	t.Run("returns the error when parsing the service template fails", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		parser := mocks.NewMockstaticSiteReadParser(ctrl)
		parser.EXPECT().ParseStaticSite(gomock.Any()).Return(nil, errors.New("some error"))
		addons := mockAddons{
			tpl: `
  Resources:
    AdditionalResourcesPolicy:
      Type: AWS::IAM::ManagedPolicy
  Outputs:
    AdditionalResourcesPolicyArn:
      Value: hello`,
		}
		static, err := NewStaticSite(&StaticSiteConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			Manifest:           &manifest.StaticSite{},
			Addons:             addons,
			ArtifactBucketName: "mockBucket",
			RuntimeConfig: RuntimeConfig{
				Region: "us-west-2",
			},
			AssetMappingURL: "s3://bucket/path/to/file",
		})
		static.parser = parser
		require.NoError(t, err)

		// WHEN
		_, err = static.Template()

		// THEN
		require.EqualError(t, err, "some error")
	})
}

func TestStaticSite_Parameters(t *testing.T) {
	t.Cleanup(func() {
		fs = realEmbedFS
	})
	fs = templatetest.Stub{}
	testCases := map[string]struct {
		expectedParams []*cloudformation.Parameter
		expectedErr    error
	}{
		"HTTPS Enabled": {
			expectedParams: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(WorkloadAppNameParamKey),
					ParameterValue: aws.String("phonetool"),
				},
				{
					ParameterKey:   aws.String(WorkloadEnvNameParamKey),
					ParameterValue: aws.String("test"),
				},
				{
					ParameterKey:   aws.String(WorkloadNameParamKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(WorkloadAddonsTemplateURLParamKey),
					ParameterValue: aws.String("mockURL"),
				},
				{
					ParameterKey:   aws.String(WorkloadArtifactKeyARNParamKey),
					ParameterValue: aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			testManifest := manifest.NewStaticSite(manifest.StaticSiteProps{
				Name: "frontend",
			})

			// GIVEN
			conf, err := NewStaticSite(&StaticSiteConfig{
				App: &config.Application{
					Name: testAppName,
				},
				RuntimeConfig: RuntimeConfig{
					AddonsTemplateURL: "mockURL",
				},
				EnvManifest: &manifest.Environment{
					Workload: manifest.Workload{
						Name: aws.String(testEnvName),
					},
				},
				Manifest:    testManifest,
				ArtifactKey: "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab",
			})
			require.NoError(t, err)

			// WHEN
			params, err := conf.Parameters()

			// THEN
			if err == nil {
				require.ElementsMatch(t, tc.expectedParams, params)
			} else {
				require.EqualError(t, tc.expectedErr, err.Error())
			}
		})
	}
}

func TestStaticSite_SerializedParameters(t *testing.T) {
	t.Cleanup(func() {
		fs = realEmbedFS
	})
	fs = templatetest.Stub{}
	c, _ := NewStaticSite(&StaticSiteConfig{
		EnvManifest: &manifest.Environment{
			Workload: manifest.Workload{
				Name: aws.String(testEnvName),
			},
		},
		App: &config.Application{
			Name: testAppName,
		},
		Manifest: testStaticSiteManifest,
		RuntimeConfig: RuntimeConfig{
			AdditionalTags: map[string]string{
				"owner": "copilot",
			},
		},
		ArtifactKey: "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab",
	})
	params, err := c.SerializedParameters()
	require.NoError(t, err)
	require.Equal(t, params, `{
  "Parameters": {
    "AddonsTemplateURL": "",
    "AppName": "phonetool",
    "ArtifactKeyARN": "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab",
    "EnvName": "test",
    "WorkloadName": "frontend"
  },
  "Tags": {
    "copilot-application": "phonetool",
    "copilot-environment": "test",
    "copilot-service": "frontend",
    "owner": "copilot"
  }
}`)
}
