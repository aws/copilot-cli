// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const projectName = "project"

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
	const pipelineName = "pipepiper"

	testCases := map[string]struct {
		beforeEach     func() error
		provider       Provider
		expectedErr    error
		inputStages    []PipelineStage
		expectedStages []PipelineStage
	}{
		"errors out when no stage provided": {
			provider: func() Provider {
				p, err := NewProvider(&GithubProperties{
					Repository: "aws/amazon-ecs-cli-v2",
					Branch:     "master",
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			expectedErr: fmt.Errorf("a pipeline %s can not be created without a deployment stage",
				pipelineName),
		},
		"certain stages use different project names": {
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
					associatedEnvironment: &associatedEnvironment{
						ProjectName: projectName + "somethingElse",
						Name:        "chicken",
					},
				},
				{
					associatedEnvironment: &associatedEnvironment{
						ProjectName: projectName,
						Name:        "wings",
					},
				},
			},
			expectedErr: fmt.Errorf("failed to create a pipieline that is associated with multiple projects, found at least: [%s, %s]",
				projectName+"somethingElse", projectName),
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
					associatedEnvironment: &associatedEnvironment{
						ProjectName: projectName,
						Name:        "chicken",
					},
				},
				{
					associatedEnvironment: &associatedEnvironment{
						ProjectName: projectName,
						Name:        "wings",
					},
				},
			},
			expectedStages: []PipelineStage{
				{
					associatedEnvironment: &associatedEnvironment{
						ProjectName: projectName,
						Name:        "chicken",
					},
				},
				{
					associatedEnvironment: &associatedEnvironment{
						ProjectName: projectName,
						Name:        "wings",
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := CreatePipeline(pipelineName, tc.provider, tc.inputStages...)

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				p, ok := m.(*pipelineManifest)
				require.True(t, ok)
				require.Equal(t, tc.expectedStages, p.Stages, "the stages are different from the expected")
			}
		})
	}
}

func TestPipelineManifestMarshal(t *testing.T) {
	const pipelineName = "pipepiper"
	wantedContent := `# This YAML file defines the relationship and deployment ordering of your environments.

# The name of the pipeline
name: pipepiper

# The version of the schema used in this template
version: 1

# This section defines the source artifacts.
source:
  # The name of the provider that is used to store the source artifacts.
  provider: Github
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
      name: chicken
`
	// reset the global map before each test case is run
	provider, err := NewProvider(&GithubProperties{
		Repository: "aws/amazon-ecs-cli-v2",
		Branch:     "master",
	})
	require.NoError(t, err)

	m, err := CreatePipeline(pipelineName, provider, PipelineStage{
		associatedEnvironment: &associatedEnvironment{
			ProjectName: projectName,
			Name:        "chicken",
		},
	})
	require.NoError(t, err)

	b, err := m.Marshal()
	require.NoError(t, err)
	require.Equal(t, wantedContent, strings.Replace(string(b), "\r\n", "\n", -1))
}

func TestUnmarshalPipeline(t *testing.T) {
	testCases := map[string]struct {
		overrideFetchFunc func(stageName string) (*associatedEnvironment, []string, error)
		inContent         string
		expectedManifest  *pipelineManifest
		expectedErr       error
	}{
		"invalid pipeline schema version": {
			overrideFetchFunc: func(_ string) (*associatedEnvironment, []string, error) {
				return &associatedEnvironment{
					ProjectName: projectName,
					Name:        "test",
					Region:      "testRegion",
					AccountID:   "testAccountId",
					Prod:        false,
				}, []string{}, nil
			},
			inContent: `
name: pipepiper
version: -1

source:
  provider: Github
  properties:
    repository: aws/somethingCool
    branch: master

stages:
    - 
      name: test
`,
			expectedErr: &ErrInvalidPipelineManifestVersion{
				PipelineSchemaMajorVersion(-1),
			},
		},
		"invalid pipeline.yml": {
			inContent:   `corrupted yaml`,
			expectedErr: errors.New("yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `corrupt...` into manifest.pipelineManifest"),
		},
		"valid pipeline.yml": {
			overrideFetchFunc: func(stageName string) (*associatedEnvironment, []string, error) {
				require.Equal(t, "chicken", stageName)
				return &associatedEnvironment{
					ProjectName: projectName,
					Name:        "chicken",
					Region:      "testRegion",
					AccountID:   "testAccountId",
					Prod:        false,
				}, []string{"app1", "app2"}, nil
			},
			inContent: `
name: pipepiper
version: 1

source:
  provider: Github
  properties:
    repository: aws/somethingCool
    branch: master

stages:
    - 
      name: chicken
`,
			expectedManifest: &pipelineManifest{
				Name:    "pipepiper",
				Version: Ver1,
				Source: &Source{
					ProviderName: "Github",
					Properties: map[string]interface{}{
						"repository": "aws/somethingCool",
						"branch":     "master",
					},
				},
				Stages: []PipelineStage{
					{
						associatedEnvironment: &associatedEnvironment{
							ProjectName: projectName,
							Name:        "chicken",
							Region:      "testRegion",
							AccountID:   "testAccountId",
							Prod:        false,
						},
						LocalApplications: []string{"app1", "app2"},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			existingFunc := fetchAssociatedEnvAndApps
			fetchAssociatedEnvAndApps = tc.overrideFetchFunc
			m, err := UnmarshalPipeline([]byte(tc.inContent))

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				actualManifest, ok := m.(*pipelineManifest)
				require.True(t, ok)
				require.Equal(t, tc.expectedManifest, actualManifest)
			}
			fetchAssociatedEnvAndApps = existingFunc
		})
	}
}
