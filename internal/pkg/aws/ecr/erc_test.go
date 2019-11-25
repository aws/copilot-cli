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

	t *testing.T

	mockGetAuthorizationToken func(*ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error)
	mockCreateRepository      func(*ecr.CreateRepositoryInput) (*ecr.CreateRepositoryOutput, error)
	mockDescribeRepositories  func(*ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error)
	mockDescribeImages        func(*testing.T, *ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error)
	mockBatchDeleteImage      func(*testing.T, *ecr.BatchDeleteImageInput) (*ecr.BatchDeleteImageOutput, error)
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

func (m mockECR) DescribeImages(input *ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error) {
	return m.mockDescribeImages(m.t, input)
}

func (m mockECR) BatchDeleteImage(input *ecr.BatchDeleteImageInput) (*ecr.BatchDeleteImageOutput, error) {
	return m.mockBatchDeleteImage(m.t, input)
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
			wantErr: fmt.Errorf("ecr describe repository %s: %w", mockRepoName, mockError),
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

func TestListImages(t *testing.T) {
	mockRepoName := "mockRepoName"
	mockError := errors.New("mockError")
	mockDigest := "mockDigest"

	tests := map[string]struct {
		mockDescribeImages func(*testing.T, *ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error)

		wantImages []Image
		wantError  error
	}{
		"should wrap error returned by ECR DescribeImages": {
			mockDescribeImages: func(t *testing.T, in *ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error) {
				return nil, mockError
			},
			wantImages: nil,
			wantError:  fmt.Errorf("ecr repo %s describe images: %w", mockRepoName, mockError),
		},
		"should return Image list": {
			mockDescribeImages: func(t *testing.T, in *ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error) {
				t.Helper()

				require.Equal(t, mockRepoName, *in.RepositoryName)

				return &ecr.DescribeImagesOutput{
					ImageDetails: []*ecr.ImageDetail{
						&ecr.ImageDetail{
							ImageDigest: aws.String(mockDigest),
						},
					},
				}, nil
			},
			wantImages: []Image{Image{Digest: mockDigest}},
			wantError:  nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := Service{
				ecr: mockECR{
					t: t,

					mockDescribeImages: test.mockDescribeImages,
				},
			}

			gotImages, gotError := s.ListImages(mockRepoName)

			require.ElementsMatch(t, test.wantImages, gotImages)
			require.Equal(t, test.wantError, gotError)
		})
	}
}

func TestDeleteImages(t *testing.T) {
	mockRepoName := "mockRepoName"
	mockError := errors.New("mockError")
	mockDigest := "mockDigest"
	mockImages := []Image{
		Image{
			Digest: mockDigest,
		},
	}

	tests := map[string]struct {
		images []Image

		mockBatchDeleteImage func(*testing.T, *ecr.BatchDeleteImageInput) (*ecr.BatchDeleteImageOutput, error)

		want error
	}{
		"should return an error if input Image list is empty": {
			images: nil,
			want:   nil,
		},
		"should wrap error return from BatchDeleteImage": {
			images: mockImages,
			mockBatchDeleteImage: func(t *testing.T, in *ecr.BatchDeleteImageInput) (*ecr.BatchDeleteImageOutput, error) {
				return nil, mockError
			},
			want: fmt.Errorf("ecr repo %s batch delete image: %w", mockRepoName, mockError),
		},
		"should return nil if call to BatchDeleteImage successful": {
			images: mockImages,
			mockBatchDeleteImage: func(t *testing.T, in *ecr.BatchDeleteImageInput) (*ecr.BatchDeleteImageOutput, error) {
				t.Helper()

				require.Equal(t, mockRepoName, *in.RepositoryName)

				var imageIdentifiers []*ecr.ImageIdentifier
				for _, image := range mockImages {
					imageIdentifiers = append(imageIdentifiers, image.imageIdentifier())
				}
				require.ElementsMatch(t, imageIdentifiers, in.ImageIds)

				return &ecr.BatchDeleteImageOutput{}, nil
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s := Service{
				ecr: mockECR{
					t: t,

					mockBatchDeleteImage: test.mockBatchDeleteImage,
				},
			}

			got := s.DeleteImages(test.images, mockRepoName)

			require.Equal(t, test.want, got)
		})
	}
}

func TestURIFromARN(t *testing.T) {

	testCases := map[string]struct {
		givenARN  string
		wantedURI string
		wantErr   error
	}{
		"valid arn": {
			givenARN:  "arn:aws:ecr:us-west-2:0123456789:repository/myrepo",
			wantedURI: "0123456789.dkr.ecr.us-west-2.amazonaws.com/myrepo",
		},
		"valid arn with namespace": {
			givenARN:  "arn:aws:ecr:us-west-2:0123456789:repository/myproject/myapp",
			wantedURI: "0123456789.dkr.ecr.us-west-2.amazonaws.com/myproject/myapp",
		},
		"separate region": {
			givenARN:  "arn:aws:ecr:us-east-1:0123456789:repository/myproject/myapp",
			wantedURI: "0123456789.dkr.ecr.us-east-1.amazonaws.com/myproject/myapp",
		},
		"invalid ARN": {
			givenARN: "myproject/myapp",
			wantErr:  fmt.Errorf("parsing repository ARN myproject/myapp: arn: invalid prefix"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uri, err := URIFromARN(tc.givenARN)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.wantedURI, uri)
			}
		})
	}
}
