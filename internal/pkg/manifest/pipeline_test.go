// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"strings"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	testCases := map[string]struct {
		providerConfig interface{}
		expectedErr    error
	}{
		"successfully create Github provider": {
			providerConfig: &GithubProperties{
				Repository: "aws/amazon-ecs-cli-v2",
				Branch:     "master",
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

func TestCreatePipeline(t *testing.T) {
	testCases := map[string]struct {
		beforeEach     func() error
		provider       Provider
		expectedErr    error
		inputStages    []PipelineStage
		expectedStages []PipelineStage
	}{
		"happy case with default stages": {
			provider: func() Provider {
				p, err := NewProvider(&GithubProperties{
					Repository: "aws/amazon-ecs-cli-v2",
					Branch:     "master",
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			expectedStages: []PipelineStage{
				{
					Environment: &archer.Environment{
						Name: "test",
					},
				},
				{
					Environment: &archer.Environment{
						Name: "prod",
					},
				},
			},
		},
		"happy case with non-default stages": {
			provider: func() Provider {
				p, err := NewProvider(&GithubProperties{
					Repository: "aws/amazon-ecs-cli-v2",
					Branch:     "master",
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			inputStages: []PipelineStage{
				{
					Environment: &archer.Environment{
						Name: "chicken",
					},
				},
				{
					Environment: &archer.Environment{
						Name: "wings",
					},
				},
			},
			expectedStages: []PipelineStage{
				{
					Environment: &archer.Environment{
						Name: "chicken",
					},
				},
				{
					Environment: &archer.Environment{
						Name: "wings",
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := CreatePipeline(tc.provider, tc.inputStages...)

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				p, ok := m.(*PipelineManifest)
				require.True(t, ok)
				require.Equal(t, tc.expectedStages, p.Stages, "the stages are different from the expected")
			}
		})
	}
}

func TestPipelineManifestMarshal(t *testing.T) {
	wantedContent := `# This YAML file defines the relationship and deployment ordering of your environments.

# The version of the schema used in this template
version: 1

# This section defines the source artifacts.
source:
  # The name of the provider that is used to store the source artifacts.
  provider: github
  # Additional properties that further specifies the exact location
  # the artifacts should be sourced from.
  properties:
    branch: master
    repository: aws/amazon-ecs-cli-v2

# The deployment section defines the order the pipeline will deploy
# to your environments
stages:
    - 
      # The name of the environment to deploy to.
      name: test
    - 
      # The name of the environment to deploy to.
      name: prod
`
	// reset the global map before each test case is run
	provider, err := NewProvider(&GithubProperties{
		Repository: "aws/amazon-ecs-cli-v2",
		Branch:     "master",
	})
	require.NoError(t, err)

	m, err := CreatePipeline(provider)
	require.NoError(t, err)

	b, err := m.Marshal()
	require.NoError(t, err)
	require.Equal(t, wantedContent, strings.Replace(string(b), "\r\n", "\n", -1))
}

func TestUnmarshalPipeline(t *testing.T) {
	testCases := map[string]struct {
		inContent        string
		expectedManifest *PipelineManifest
		expectedErr      error
	}{
		"invalid pipeline schema version": {
			inContent: `
version: -1

source:
  provider: github
  properties:
    repository: aws/somethingCool
    branch: master

stages:
    - 
      name: test
    - 
      name: prod
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
version: 1

source:
  provider: github
  properties:
    repository: aws/somethingCool
    branch: master

stages:
    - 
      name: chicken
    - 
      name: wings
`,
			expectedManifest: &PipelineManifest{
				Version: Ver1,
				Source: &Source{
					ProviderName: "github",
					Properties: map[string]interface{}{
						"repository": "aws/somethingCool",
						"branch":     "master",
					},
				},
				Stages: []PipelineStage{
					{
						Environment: &archer.Environment{
							Name: "chicken",
						},
					},
					{
						Environment: &archer.Environment{
							Name: "wings",
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
				actualManifest, ok := m.(*PipelineManifest)
				require.True(t, ok)
				require.Equal(t, actualManifest, tc.expectedManifest)
			}
		})
	}
}
