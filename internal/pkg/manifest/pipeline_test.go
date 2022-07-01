// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/fatih/structs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	defaultGHBranch = "main"
	defaultCCBranch = "main"
)

func TestNewProvider(t *testing.T) {
	testCases := map[string]struct {
		providerConfig interface{}
		expectedErr    error
	}{
		"successfully create GitHub provider": {
			providerConfig: &GitHubProperties{
				RepositoryURL: "aws/amazon-ecs-cli-v2",
				Branch:        defaultGHBranch,
			},
		},
		"successfully create CodeCommit provider": {
			providerConfig: &CodeCommitProperties{
				RepositoryURL: "https://us-west-2.console.aws.amazon.com/codesuite/codecommit/repositories/wings/browse",
				Branch:        defaultCCBranch,
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

func TestNewPipelineManifest(t *testing.T) {
	const pipelineName = "pipepiper"

	testCases := map[string]struct {
		beforeEach  func() error
		provider    Provider
		inputStages []PipelineStage

		expectedManifest *Pipeline
		expectedErr      error
	}{
		"errors out when no stage provided": {
			provider: func() Provider {
				p, err := NewProvider(&GitHubProperties{
					RepositoryURL: "aws/amazon-ecs-cli-v2",
					Branch:        defaultGHBranch,
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			expectedErr: fmt.Errorf("a pipeline %s can not be created without a deployment stage",
				pipelineName),
		},
		"happy case with non-default stages": {
			provider: func() Provider {
				p, err := NewProvider(&GitHubProperties{
					RepositoryURL: "aws/amazon-ecs-cli-v2",
					Branch:        defaultGHBranch,
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			inputStages: []PipelineStage{
				{
					Name:             "chicken",
					RequiresApproval: false,
				},
				{
					Name:             "wings",
					RequiresApproval: true,
				},
			},
			expectedManifest: &Pipeline{
				Name:    "pipepiper",
				Version: Ver1,
				Source: &Source{
					ProviderName: "GitHub",
					Properties: structs.Map(GitHubProperties{
						RepositoryURL: "aws/amazon-ecs-cli-v2",
						Branch:        defaultGHBranch,
					}),
				},
				Stages: []PipelineStage{
					{
						Name:             "chicken",
						RequiresApproval: false,
					},
					{
						Name:             "wings",
						RequiresApproval: true,
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			expectedBytes, err := yaml.Marshal(tc.expectedManifest)
			require.NoError(t, err)

			// WHEN
			m, err := NewPipeline(pipelineName, tc.provider, tc.inputStages)

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				actualBytes, err := yaml.Marshal(m)
				require.NoError(t, err)
				require.Equal(t, expectedBytes, actualBytes, "the manifest is different from the expected")
			}
		})
	}
}

func TestPipelineManifest_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, manifest *Pipeline)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *Pipeline) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(pipelineManifestPath, *manifest).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *Pipeline) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(pipelineManifestPath, *manifest).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			manifest := &Pipeline{}
			tc.mockDependencies(ctrl, manifest)

			// WHEN
			b, err := manifest.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestUnmarshalPipeline(t *testing.T) {
	testCases := map[string]struct {
		inContent        string
		expectedManifest *Pipeline
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
    branch: main

stages:
    -
      name: test
    -
      name: prod
`,
			expectedErr: &ErrInvalidPipelineManifestVersion{
				invalidVersion: PipelineSchemaMajorVersion(-1),
			},
		},
		"invalid pipeline.yml": {
			inContent:   `corrupted yaml`,
			expectedErr: errors.New("yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `corrupt...` into manifest.Pipeline"),
		},
		"valid pipeline.yml without build": {
			inContent: `
name: pipepiper
version: 1

source:
  provider: GitHub
  properties:
    repository: aws/somethingCool
    access_token_secret: "github-token-badgoose-backend"
    branch: main

stages:
    -
      name: chicken
      test_commands: []
    -
      name: wings
      test_commands: []
`,
			expectedManifest: &Pipeline{
				Name:    "pipepiper",
				Version: Ver1,
				Source: &Source{
					ProviderName: "GitHub",
					Properties: map[string]interface{}{
						"access_token_secret": "github-token-badgoose-backend",
						"repository":          "aws/somethingCool",
						"branch":              defaultGHBranch,
					},
				},
				Stages: []PipelineStage{
					{
						Name:         "chicken",
						TestCommands: []string{},
					},
					{
						Name:         "wings",
						TestCommands: []string{},
					},
				},
			},
		},
		"valid pipeline.yml with build": {
			inContent: `
name: pipepiper
version: 1

source:
  provider: GitHub
  properties:
    repository: aws/somethingCool
    access_token_secret: "github-token-badgoose-backend"
    branch: main

build:
  image: aws/codebuild/standard:3.0

stages:
    -
      name: chicken
      test_commands: []
`,
			expectedManifest: &Pipeline{
				Name:    "pipepiper",
				Version: Ver1,
				Source: &Source{
					ProviderName: "GitHub",
					Properties: map[string]interface{}{
						"access_token_secret": "github-token-badgoose-backend",
						"repository":          "aws/somethingCool",
						"branch":              defaultGHBranch,
					},
				},
				Build: &Build{
					Image: "aws/codebuild/standard:3.0",
				},
				Stages: []PipelineStage{
					{
						Name:         "chicken",
						TestCommands: []string{},
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

func TestActionOverrides_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inAction []byte

		wantedStruct ActionOverride
		wantedError  error
	}{
		"action specified in string": {
			inAction: []byte(`action: '*'`),
			wantedStruct: ActionOverride{
				String:      aws.String("*"),
				StringSlice: nil,
			},
		},
		"action specified in slice of strings": {
			inAction: []byte(`action: 
- get:s3
- get:asd`),
			wantedStruct: ActionOverride{
				String:      nil,
				StringSlice: []string{"get:s3", "get:asd"},
			},
		},
		"Error if action is unmarshalable": {
			inAction: []byte(`action: {"*", "get:s3*"}`),
			wantedStruct: ActionOverride{
				String:      nil,
				StringSlice: nil,
			},
			wantedError: errUnmarshalAction,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			e := Statement{
				Action: ActionOverride{
					String: aws.String("wrong"),
				},
			}

			err := yaml.Unmarshal(tc.inAction, &e)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct.StringSlice, e.Action.StringSlice)
				require.Equal(t, tc.wantedStruct.String, e.Action.String)
			}
		})
	}
}

func TestSourceOverrides_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inReSource []byte

		wantedStruct ResourceOverride
		wantedError  error
	}{
		"resource specified in string": {
			inReSource: []byte(`resource: '*'`),
			wantedStruct: ResourceOverride{
				String:      aws.String("*"),
				StringSlice: nil,
			},
		},
		"resource specified in slice of strings": {
			inReSource: []byte(`resource: 
- get:s3
- get:asd`),
			wantedStruct: ResourceOverride{
				String:      nil,
				StringSlice: []string{"get:s3", "get:asd"},
			},
		},
		"Error if resource is unmarshalable": {
			inReSource: []byte(`resource: {"*", "get:s3*"}`),
			wantedStruct: ResourceOverride{
				String:      nil,
				StringSlice: nil,
			},
			wantedError: errUnmarshalResource,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			e := Statement{
				Resource: ResourceOverride{
					String: aws.String("wrong"),
				},
			}

			err := yaml.Unmarshal(tc.inReSource, &e)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct.StringSlice, e.Resource.StringSlice)
				require.Equal(t, tc.wantedStruct.String, e.Resource.String)
			}
		})
	}
}
