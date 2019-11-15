// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/stretchr/testify/require"
)

type mockECR struct {
	ecriface.ECRAPI

	mockGetAuthorizationToken func(*ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error)
	mockCreateRepository      func(*ecr.CreateRepositoryInput) (*ecr.CreateRepositoryOutput, error)
	mockDescribeRepositories  func(*ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error)
}

func (m mockECR) GetAuthorizationToken(input *ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
	return m.mockGetAuthorizationToken(input)
}

func (m mockECR) CreateRepository(input *ecr.CreateRepositoryInput) (*ecr.CreateRepositoryOutput, error) {
	return m.mockCreateRepository(input)
}

func (m mockECR) DescribeRepositories(input *ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
	return m.mockDescribeRepositories(input)
}

func TestGetECRAuth(t *testing.T) {
	mockError := errors.New("error")

	mockUsername := "mockUsername"
	mockPassword := "mockPassword"

	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", mockUsername, mockPassword)))

	testCases := map[string]struct {
		mockGetAuthorizationToken func(*ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error)

		wantAuth Auth
		wantErr  error
	}{
		"should return wrapped error given error returned from GetAuthorizationToken": {
			mockGetAuthorizationToken: func(*ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
				return nil, mockError
			},
			wantErr: fmt.Errorf("get ECR auth: %w", mockError),
		},
		"should return Auth data": {
			mockGetAuthorizationToken: func(*ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
				return &ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []*ecr.AuthorizationData{
						&ecr.AuthorizationData{
							AuthorizationToken: aws.String(encoded),
						},
					},
				}, nil
			},
			wantAuth: Auth{
				Username: mockUsername,
				Password: mockPassword,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			service := Service{
				mockECR{
					mockGetAuthorizationToken: tc.mockGetAuthorizationToken,
				},
			}

			gotAuth, gotErr := service.GetECRAuth()

			require.Equal(t, tc.wantAuth, gotAuth)
			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}

func TestGetRepository(t *testing.T) {
	mockError := errors.New("error")

	mockRepoName := "mockRepoName"
	mockRepoURI := "mockRepoURI"

	testCases := map[string]struct {
		mockDescribeRepositories func(*ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error)

		wantURI string
		wantErr error
	}{
		"should return wrapped error given error returned from DescribeRepositories": {
			mockDescribeRepositories: func(*ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
				return nil, mockError
			},
			wantErr: fmt.Errorf("repository %s not found: %w", mockRepoName, mockError),
		},
		"should return error given no repositories returned in list": {
			mockDescribeRepositories: func(input *ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
				return &ecr.DescribeRepositoriesOutput{
					Repositories: []*ecr.Repository{},
				}, nil
			},
			wantErr: errors.New("no repositories found"),
		},
		"should return repository URI": {
			mockDescribeRepositories: func(input *ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
				require.Equal(t, mockRepoName, *input.RepositoryNames[0])

				return &ecr.DescribeRepositoriesOutput{
					Repositories: []*ecr.Repository{
						&ecr.Repository{
							RepositoryUri: aws.String(mockRepoURI),
						},
					},
				}, nil
			},
			wantURI: mockRepoURI,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			service := Service{
				mockECR{
					mockDescribeRepositories: tc.mockDescribeRepositories,
				},
			}

			gotURI, gotErr := service.GetRepository(mockRepoName)

			require.Equal(t, tc.wantURI, gotURI)
			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}
