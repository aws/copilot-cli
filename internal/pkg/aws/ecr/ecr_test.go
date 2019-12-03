// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	awsmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestGetECRAuth(t *testing.T) {
	mockError := errors.New("error")

	mockUsername := "mockUsername"
	mockPassword := "mockPassword"

	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", mockUsername, mockPassword)))

	testCases := map[string]struct {
		mockECRAPI func(m *awsmocks.MockECRAPI)

		wantAuth Auth
		wantErr  error
	}{
		"should return wrapped error given error returned from GetAuthorizationToken": {
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().GetAuthorizationToken(gomock.Any()).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("get ECR auth: %w", mockError),
		},
		"should return Auth data": {
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().GetAuthorizationToken(gomock.Any()).Return(&ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []*ecr.AuthorizationData{
						&ecr.AuthorizationData{
							AuthorizationToken: aws.String(encoded),
						},
					},
				}, nil)
			},
			wantAuth: Auth{
				Username: mockUsername,
				Password: mockPassword,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECRAPI := awsmocks.NewMockECRAPI(ctrl)
			tc.mockECRAPI(mockECRAPI)

			service := Service{
				ecr: mockECRAPI,
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
		mockECRAPI func(m *awsmocks.MockECRAPI)

		wantURI string
		wantErr error
	}{
		"should return wrapped error given error returned from DescribeRepositories": {
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().DescribeRepositories(gomock.Any()).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("ecr describe repository %s: %w", mockRepoName, mockError),
		},
		"should return error given no repositories returned in list": {
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().DescribeRepositories(&ecr.DescribeRepositoriesInput{
					RepositoryNames: aws.StringSlice([]string{mockRepoName}),
				}).Return(&ecr.DescribeRepositoriesOutput{
					Repositories: []*ecr.Repository{},
				}, nil)
			},
			wantErr: errors.New("no repositories found"),
		},
		"should return repository URI": {
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().DescribeRepositories(&ecr.DescribeRepositoriesInput{
					RepositoryNames: aws.StringSlice([]string{mockRepoName}),
				}).Return(&ecr.DescribeRepositoriesOutput{
					Repositories: []*ecr.Repository{
						&ecr.Repository{
							RepositoryUri: aws.String(mockRepoURI),
						},
					},
				}, nil)
			},
			wantURI: mockRepoURI,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECRAPI := awsmocks.NewMockECRAPI(ctrl)
			tc.mockECRAPI(mockECRAPI)

			service := Service{
				mockECRAPI,
			}

			gotURI, gotErr := service.GetRepository(mockRepoName)

			require.Equal(t, tc.wantURI, gotURI)
			require.Equal(t, tc.wantErr, gotErr)
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

func TestListImages(t *testing.T) {
	mockRepoName := "mockRepoName"
	mockError := errors.New("mockError")
	mockDigest := "mockDigest"
	mockNextToken := "next"

	tests := map[string]struct {
		mockECRAPI func(m *awsmocks.MockECRAPI)

		wantImages []Image
		wantError  error
	}{
		"should wrap error returned by ECR DescribeImages": {
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().DescribeImages(gomock.Any()).Return(nil, mockError)
			},
			wantImages: nil,
			wantError:  fmt.Errorf("ecr repo %s describe images: %w", mockRepoName, mockError),
		},
		"should return Image list": {
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().DescribeImages(gomock.Any()).Return(&ecr.DescribeImagesOutput{
					ImageDetails: []*ecr.ImageDetail{
						&ecr.ImageDetail{
							ImageDigest: aws.String(mockDigest),
						},
					},
				}, nil)
			},
			wantImages: []Image{Image{Digest: mockDigest}},
			wantError:  nil,
		},
		"should return all images when paginated": {
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().DescribeImages(&ecr.DescribeImagesInput{
					RepositoryName: aws.String(mockRepoName),
				}).Return(&ecr.DescribeImagesOutput{
					ImageDetails: []*ecr.ImageDetail{
						&ecr.ImageDetail{
							ImageDigest: aws.String(mockDigest),
						},
					},
					NextToken: &mockNextToken,
				}, nil)
				m.EXPECT().DescribeImages(&ecr.DescribeImagesInput{
					RepositoryName: aws.String(mockRepoName),
					NextToken:      &mockNextToken,
				}).Return(&ecr.DescribeImagesOutput{
					ImageDetails: []*ecr.ImageDetail{
						&ecr.ImageDetail{
							ImageDigest: aws.String(mockDigest),
						},
					},
				}, nil)
			},
			wantImages: []Image{Image{Digest: mockDigest}, Image{Digest: mockDigest}},
			wantError:  nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECRAPI := awsmocks.NewMockECRAPI(ctrl)
			tc.mockECRAPI(mockECRAPI)

			service := Service{
				mockECRAPI,
			}

			gotImages, gotError := service.ListImages(mockRepoName)

			require.ElementsMatch(t, tc.wantImages, gotImages)
			require.Equal(t, tc.wantError, gotError)
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
	mockFailCode := "400"
	mockFailReason := "some reason"
	// with only one image identifier
	var imageIdentifiers []*ecr.ImageIdentifier
	for _, image := range mockImages {
		imageIdentifiers = append(imageIdentifiers, image.imageIdentifier())
	}
	var mockBatchImages []Image
	for ii := 0; ii < batchDeleteLimit+1; ii++ {
		mockBatchImages = append(mockBatchImages, mockImages[0])
	}
	// with a batch limit number of image identifiers
	var batchImageIdentifiers []*ecr.ImageIdentifier
	for ii := 0; ii < batchDeleteLimit; ii++ {
		batchImageIdentifiers = append(batchImageIdentifiers, mockImages[0].imageIdentifier())
	}

	tests := map[string]struct {
		images     []Image
		mockECRAPI func(m *awsmocks.MockECRAPI)

		wantError error
	}{
		"should not return error if input Image list is empty": {
			images:     nil,
			mockECRAPI: func(m *awsmocks.MockECRAPI) {},
			wantError:  nil,
		},
		"should wrap error return from BatchDeleteImage": {
			images: mockImages,
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().BatchDeleteImage(gomock.Any()).Return(nil, mockError)
			},
			wantError: fmt.Errorf("ecr repo %s batch delete image: %w", mockRepoName, mockError),
		},
		"should return nil if call to BatchDeleteImage successful": {
			images: mockImages,
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().BatchDeleteImage(&ecr.BatchDeleteImageInput{
					RepositoryName: aws.String(mockRepoName),
					ImageIds:       imageIdentifiers,
				}).Return(&ecr.BatchDeleteImageOutput{}, nil)
			},
			wantError: nil,
		},
		fmt.Sprintf("should be able to batch delete more than %d images", batchDeleteLimit): {
			images: mockBatchImages,
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().BatchDeleteImage(&ecr.BatchDeleteImageInput{
					RepositoryName: aws.String(mockRepoName),
					ImageIds:       batchImageIdentifiers,
				}).Return(&ecr.BatchDeleteImageOutput{}, nil).Times(1)
				m.EXPECT().BatchDeleteImage(&ecr.BatchDeleteImageInput{
					RepositoryName: aws.String(mockRepoName),
					ImageIds:       imageIdentifiers,
				}).Return(&ecr.BatchDeleteImageOutput{}, nil).Times(1)
			},
			wantError: nil,
		},
		"warns if fail to delete some images": {
			images: mockImages,
			mockECRAPI: func(m *awsmocks.MockECRAPI) {
				m.EXPECT().BatchDeleteImage(&ecr.BatchDeleteImageInput{
					RepositoryName: aws.String(mockRepoName),
					ImageIds:       imageIdentifiers,
				}).Return(&ecr.BatchDeleteImageOutput{
					Failures: []*ecr.ImageFailure{
						&ecr.ImageFailure{
							FailureCode:   &mockFailCode,
							FailureReason: &mockFailReason,
							ImageId:       imageIdentifiers[0],
						},
					},
				}, nil)
			},
			wantError: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECRAPI := awsmocks.NewMockECRAPI(ctrl)
			tc.mockECRAPI(mockECRAPI)

			service := Service{
				mockECRAPI,
			}

			got := service.DeleteImages(tc.images, mockRepoName)

			require.Equal(t, tc.wantError, got)
		})
	}
}
