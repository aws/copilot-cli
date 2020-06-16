// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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

func TestCreatePipeline(t *testing.T) {
	const pipelineName = "pipepiper"

	testCases := map[string]struct {
		beforeEach     func() error
		provider       Provider
		expectedErr    error
		inputStages    []string
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
		"happy case with non-default stages": {
			provider: func() Provider {
				p, err := NewProvider(&GitHubProperties{
					OwnerAndRepository: "aws/amazon-ecs-cli-v2",
					Branch:             "master",
				})
				require.NoError(t, err, "failed to create provider")
				return p
			}(),
			inputStages: []string{"chicken", "wings"},
			expectedStages: []PipelineStage{
				{Name: "chicken",
					TestCommands: []string(nil)},
				{Name: "wings",
					TestCommands: []string(nil)},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m, err := CreatePipeline(pipelineName, tc.provider, tc.inputStages)

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Equal(t, tc.expectedStages, m.Stages, "the stages are different from the expected")
			}
		})
	}
}

func TestPipelineManifest_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, manifest *PipelineManifest)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *PipelineManifest) {
				m := mocks.NewMockParser(ctrl)
				manifest.parser = m
				m.EXPECT().Parse(pipelineManifestPath, *manifest).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, manifest *PipelineManifest) {
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
			manifest := &PipelineManifest{}
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
name: pipepiper
version: 1

source:
  provider: GitHub
  properties:
    repository: aws/somethingCool
    access_token_secret: "github-token-badgoose-backend"
    branch: master

stages:
    -
      name: chicken
      test_commands: []
    -
      name: wings
      test_commands: []
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
