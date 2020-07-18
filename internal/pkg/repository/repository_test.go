// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/repository/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestECRRepository_BuildAndPushToRepo(t *testing.T) {
	inRepoName := "my-repo"
	inDockerfilePath := "path/to/dockerfile"

	mockTag1, mockTag2, mockTag3 := "tag1", "tag2", "tag3"
	mockRepoURI := "mockURI"

	testCases := map[string]struct {
		inRepoName string
		inDockerfilePath string

		mockRepoGetter func(m *mocks.MockECRRepositoryGetter)
		mockDocker func(m *mocks.MockContainerManager)

		wantedError error
		wantedURI string
	}{
		"failed to get ECR auth": {
			mockRepoGetter: func(m *mocks.MockECRRepositoryGetter) {
				m.EXPECT().GetECRAuth().Return(ecr.Auth{}, errors.New("error getting auth"))
			},
			mockDocker: func(m *mocks.MockContainerManager) {
				m.EXPECT().Build(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().Login(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Push(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedError: errors.New("get ECR auth: error getting auth"),
		},
		"failed to build image": {
			mockRepoGetter: func(m *mocks.MockECRRepositoryGetter) {
				m.EXPECT().GetECRAuth().Return(ecr.Auth{}, nil).AnyTimes()
			},
			mockDocker: func(m *mocks.MockContainerManager) {
				m.EXPECT().Build(mockRepoURI, inDockerfilePath, mockTag1, mockTag2, mockTag3).Return(errors.New("error building image"))
				m.EXPECT().Login(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().Push(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedError: fmt.Errorf("build Dockerfile at %s: error building image", inDockerfilePath),
		},
		"failed to login": {
			mockRepoGetter: func(m *mocks.MockECRRepositoryGetter) {
				m.EXPECT().GetECRAuth().Return(ecr.Auth{
					Username: "my-name",
					Password: "my-pwd",
				}, nil)
			},
			mockDocker: func(m *mocks.MockContainerManager) {
				m.EXPECT().Build(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.EXPECT().Login(mockRepoURI, "my-name", "my-pwd").Return(errors.New("error logging in"))
				m.EXPECT().Push(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedError: fmt.Errorf("login to repo %s: error logging in", inRepoName),
		},
		"failed to push": {
			mockRepoGetter: func(m *mocks.MockECRRepositoryGetter) {
				m.EXPECT().GetECRAuth().Times(1)
			},
			mockDocker: func(m *mocks.MockContainerManager) {
				m.EXPECT().Build(mockRepoURI, inDockerfilePath, mockTag1, mockTag2, mockTag3).Times(1)
				m.EXPECT().Login(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
				m.EXPECT().Push(mockRepoURI, mockTag1, mockTag2, mockTag3).Return(errors.New("error pushing image"))
			},
			wantedError: errors.New("push to repo my-repo: error pushing image"),
		},
		"success": {
			mockRepoGetter: func(m *mocks.MockECRRepositoryGetter) {
				m.EXPECT().GetECRAuth().Return(ecr.Auth{
					Username: "my-name",
					Password: "my-pwd",
				}, nil).Times(1)
			},
			mockDocker: func(m *mocks.MockContainerManager) {
				m.EXPECT().Build(mockRepoURI, inDockerfilePath, mockTag1, mockTag2, mockTag3).Return(nil).Times(1)
				m.EXPECT().Login(mockRepoURI, "my-name", "my-pwd").Return(nil).Times(1)
				m.EXPECT().Push(mockRepoURI, mockTag1, mockTag2, mockTag3).Return(nil)
			},
			wantedURI: mockRepoURI,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepoGetter := mocks.NewMockECRRepositoryGetter(ctrl)
			mockDocker := mocks.NewMockContainerManager(ctrl)

			if tc.mockRepoGetter != nil {
				tc.mockRepoGetter(mockRepoGetter)
			}
			if tc.mockDocker!= nil {
				tc.mockDocker(mockDocker)
			}

			repo := &ECRRepository{
				repositoryName:   inRepoName,
				repositoryGetter: mockRepoGetter,
				docker:           mockDocker,

				uri: mockRepoURI,
			}

			err := repo.BuildAndPush(inDockerfilePath, mockTag1, []string{mockTag2, mockTag3}...)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}