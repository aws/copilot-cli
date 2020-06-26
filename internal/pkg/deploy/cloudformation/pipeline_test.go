// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCloudFormation_PipelineExists(t *testing.T) {
	in := &deploy.CreatePipelineInput{
		AppName: "kudos",
		Name:    "cicd",
	}
	testCases := map[string]struct {
		createMock   func(ctrl *gomock.Controller) cfnClient
		wantedExists bool
		wantedErr    error
	}{
		"return false and error on unexpected failure": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wantedErr: errors.New("some error"),
		},
		"return false if stack does not exist": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, fmt.Errorf("describe stack: %w", &cloudformation.ErrStackNotFound{}))
				return m
			},
			wantedExists: false,
		},
		"returns true if stack exists": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().Describe(gomock.Any()).Return(nil, nil)
				return m
			},
			wantedExists: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				cfnClient: tc.createMock(ctrl),
			}

			// WHEN
			exists, err := c.PipelineExists(in)

			// THEN
			require.Equal(t, tc.wantedExists, exists)
			require.Equal(t, tc.wantedErr, err)
		})
	}
}

func TestCloudFormation_UpdatePipeline(t *testing.T) {
	in := &deploy.CreatePipelineInput{
		AppName: "kudos",
		Name:    "cicd",
		Source: &deploy.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"repository":          "aws/somethingCool",
				"access_token_secret": "github-token-badgoose-backend",
				"branch":              "master",
			},
		},
		Stages:          nil,
		ArtifactBuckets: nil,
	}
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) cfnClient
		wantedErr  error
	}{
		"exits successfully if there are no updates": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().UpdateAndWait(gomock.Any()).Return(&cloudformation.ErrChangeSetEmpty{})
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := CloudFormation{
				cfnClient: tc.createMock(ctrl),
			}

			// WHEN
			err := c.UpdatePipeline(in)

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}
