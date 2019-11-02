// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const githubRepo = "https://github.com/badGoose/chaOS"
const githubToken = "hunter2"

func TestInitPipelineOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inEnvironments      []string
		inGitHubRepo        string
		inGitHubAccessToken string

		mockEnvStore func(m *mocks.MockEnvironmentStore)
		mockPrompt   func(m *climocks.Mockprompter)

		expectedGitHubRepo        string
		expectedGitHubAccessToken string
		expectedEnvironments      []string
		expectedError             string
	}{
		"prompts for all input": {
			inEnvironments: []string{},
			inGitHubRepo: "",
			inGitHubAccessToken: "",

			mockEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*archer.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod",
					},
				}, nil).Times(3)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(3)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubRepoPrompt), gomock.Any(), gomock.Any()).Return(githubRepo, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: https://github.com/badGoose/chaOS"), gomock.Any()).Return(githubToken, nil).Times(1)
			},

			expectedGitHubRepo: githubRepo,
			expectedGitHubAccessToken: githubToken,
			expectedEnvironments: []string{"test", "prod"},
			expectedError: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
			mockPrompt := climocks.NewMockprompter(ctrl)

			opts := &InitPipelineOpts{
				Environments:      tc.inEnvironments,
				GitHubRepo:        tc.inGitHubRepo,
				GitHubAccessToken: tc.inGitHubAccessToken,

				envStore: mockEnvStore,
				prompt:   mockPrompt,
			}

			tc.mockEnvStore(mockEnvStore)
			tc.mockPrompt(mockPrompt)


			// WHEN
			err := opts.Ask()

			// THEN
			require.Equal(t, tc.expectedGitHubRepo, opts.GitHubRepo)
			require.Equal(t, tc.expectedGitHubAccessToken, opts.GitHubAccessToken)
			require.ElementsMatch(t, tc.expectedEnvironments, opts.Environments)

			if tc.expectedError != "" {
				require.EqualError(t, err, tc.expectedError)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
