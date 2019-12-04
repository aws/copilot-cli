// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
)

func TestNewProvider(t *testing.T) {
	testCases := map[string]struct {
		providerConfig interface{}
		expectedErr    error
	}{
		"successfully create GitHub provider": {
			providerConfig: &GitHubProperties{
				OwnerAndRepository: "aws/amazon-ecs-cli-v2",
				Branch:             "master",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := NewProvider(tc.providerConfig)

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err, "unexpected error while calling NewProvider()")
			}
		})
	}
}

func genApps(names ...string) []archer.Manifest {
	result := make([]archer.Manifest, 0, len(names))
	for _, name := range names {
		result = append(result, &LBFargateManifest{
			AppManifest: AppManifest{
				Name: name,
				Type: LoadBalancedWebApplication,
			},
			Image: ImageWithPort{
				AppImage: AppImage{
					Build: name,
				},
			},
		})
	}
	return result
}

func TestCreatePipeline(t *testing.T) {
	const pipelineName = "pipepiper"
	const (
		app01 = "app01"
		app02 = "app02"
	)

	testCases := map[string]struct {
		beforeEach     func() error
		provider       Provider
		expectedErr    error
		inputStages    []string
		inputApps      []archer.Manifest
		expectedStages []PipelineStage
	}{
		"errors out when no stage provided": {
			provider: func() Provider {
				p, err := NewProvider(&GitHubProperties{
					OwnerAndRepository: "aws/amazon-ecs-cli-v2",
					Branch:             "master",
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			expectedErr: fmt.Errorf("a pipeline %s can not be created without a deployment stage",
				pipelineName),
		},
		"errors out when no app provided": {
			provider: func() Provider {
				p, err := NewProvider(&GitHubProperties{
					OwnerAndRepository: "aws/amazon-ecs-cli-v2",
					Branch:             "master",
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			inputStages: []string{"chicken", "wings"},
			expectedErr: fmt.Errorf("a pipeline %s can not be created without any app to deploy",
				pipelineName),
		},
		"happy case": {
			provider: func() Provider {
				p, err := NewProvider(&GitHubProperties{
					OwnerAndRepository: "aws/amazon-ecs-cli-v2",
					Branch:             "master",
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			inputStages: []string{"chicken", "wings"},
			inputApps:   genApps(app01, app02),
			expectedStages: []PipelineStage{
				{
					Name: "chicken",
					Apps: map[string]App{
						app01: App{
							IntegTestBuildspecPath: filepath.Join(app01, IntegTestBuildspecFileName),
						},
						app02: App{
							IntegTestBuildspecPath: filepath.Join(app02, IntegTestBuildspecFileName),
						},
					},
				},
				{
					Name: "wings",
					Apps: map[string]App{
						app01: App{
							IntegTestBuildspecPath: filepath.Join(app01, IntegTestBuildspecFileName),
						},
						app02: App{
							IntegTestBuildspecPath: filepath.Join(app02, IntegTestBuildspecFileName),
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := CreatePipeline(pipelineName, tc.provider, tc.inputStages, tc.inputApps)

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Equal(t, tc.expectedStages, m.Stages, "the stages are different from the expected")
			}
		})
	}
}

func TestPipelineManifestMarshal(t *testing.T) {
	const pipelineName = "pipepiper"
	const (
		app01 = "app01"
		app02 = "app02"
	)
	wantedContent := `# This YAML file defines the relationship and deployment ordering of your environments.

# The name of the pipeline
name: pipepiper

# The version of the schema used in this template
version: 1

# This section defines the source artifacts.
source:
  # The name of the provider that is used to store the source artifacts.
  provider: GitHub
  # Additional properties that further specifies the exact location
  # the artifacts should be sourced from. For example, the GitHub provider
  # has the following properties: repository, branch.
  properties:
    access_token_secret: github-token-badgoose-backend
    branch: master
    repository: aws/amazon-ecs-cli-v2

# The deployment section defines the order the pipeline will deploy
# to your environments.
stages:
  - # The name of the environment to deploy to.
    name: chicken
    apps:
      app01:
        integrationTestBuildspec: app01/buildspec_integ.yml
      app02:
        integrationTestBuildspec: app02/buildspec_integ.yml
  - # The name of the environment to deploy to.
    name: wings
    apps:
      app01:
        integrationTestBuildspec: app01/buildspec_integ.yml
      app02:
        integrationTestBuildspec: app02/buildspec_integ.yml`
	// reset the global map before each test case is run
	provider, err := NewProvider(&GitHubProperties{
		OwnerAndRepository:    "aws/amazon-ecs-cli-v2",
		GithubSecretIdKeyName: "github-token-badgoose-backend",
		Branch:                "master",
	})
	require.NoError(t, err)

	m, err := CreatePipeline(pipelineName, provider,
		[]string{"chicken", "wings"}, genApps(app01, app02))
	require.NoError(t, err)

	b, err := m.Marshal()
	require.NoError(t, err)
	require.Equal(t, wantedContent, strings.Replace(string(b), "\r\n", "\n", -1))
}

func TestUnmarshalPipeline(t *testing.T) {
	const (
		app01 = "app01"
		app02 = "app02"
	)
	testCases := map[string]struct {
		inContent        string
		expectedManifest *PipelineManifest
		expectedErr      error
	}{
		"invalid pipeline schema version": {
			inContent: `
name: pipepiper
version: -1

source:
  provider: GitHub
  properties:
    repository: aws/somethingCool
    branch: master

stages:
  - name: chicken
    apps:
      app01:
        integrationTestBuildspec: app01/buildspec_integ.yml
      app02:
        integrationTestBuildspec: app02/buildspec_integ.yml
  - name: wings
    apps:
      app01:
        integrationTestBuildspec: app01/buildspec_integ.yml
      app02:
        integrationTestBuildspec: app02/buildspec_integ.yml
`,
			expectedErr: &ErrInvalidPipelineManifestVersion{
				PipelineSchemaMajorVersion(-1),
			},
		},
		"invalid pipeline.yml": {
			inContent:   `corrupted yaml`,
			expectedErr: errors.New("yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `corrupt...` into manifest.PipelineManifest"),
		},
		"valid pipeline.yml": {
			inContent: `
name: pipepiper
version: 1

source:
  provider: GitHub
  properties:
    repository: aws/somethingCool
    access_token_secret: "github-token-badgoose-backend"
    branch: master

stages:
  - name: chicken
    apps:
      app01:
        integrationTestBuildspec: app01/buildspec_integ.yml
  - name: wings
    apps:
      app02:
        integrationTestBuildspec: app02/buildspec_integ.yml
`,
			expectedManifest: &PipelineManifest{
				Name:    "pipepiper",
				Version: Ver1,
				Source: &Source{
					ProviderName: "GitHub",
					Properties: map[string]interface{}{
						"access_token_secret": "github-token-badgoose-backend",
						"repository":          "aws/somethingCool",
						"branch":              "master",
					},
				},
				Stages: []PipelineStage{
					{
						Name: "chicken",
						Apps: map[string]App{
							app01: App{
								IntegTestBuildspecPath: filepath.Join(app01, IntegTestBuildspecFileName),
							},
						},
					},
					{
						Name: "wings",
						Apps: map[string]App{
							app02: App{
								IntegTestBuildspecPath: filepath.Join(app02, IntegTestBuildspecFileName),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := UnmarshalPipeline([]byte(tc.inContent))

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Equal(t, tc.expectedManifest, m)
			}
		})
	}
}
