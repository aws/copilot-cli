// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestGetECRAuth(t *testing.T) {
	mockError := errors.New("error")

	mockUsername := "mockUsername"
	mockPassword := "mockPassword"

	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", mockUsername, mockPassword)))

	testCases := map[string]struct {
		mockECRClient func(m *mocks.Mockapi)

		wantAuth Auth
		wantErr  error
	}{
		"should return wrapped error given error returned from GetAuthorizationToken": {
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().GetAuthorizationToken(gomock.Any()).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("get ECR auth: %w", mockError),
		},
		"should return Auth data": {
			mockECRClient: func(m *mocks.Mockapi) {
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

			mockECRAPI := mocks.NewMockapi(ctrl)
			tc.mockECRClient(mockECRAPI)

			client := ECR{
				mockECRAPI,
			}

			gotAuth, gotErr := client.GetECRAuth()

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
		mockECRClient func(m *mocks.Mockapi)

		wantURI string
		wantErr error
	}{
		"should return wrapped error given error returned from DescribeRepositories": {
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRepositories(gomock.Any()).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("ecr describe repository %s: %w", mockRepoName, mockError),
		},
		"should return error given no repositories returned in list": {
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRepositories(&ecr.DescribeRepositoriesInput{
					RepositoryNames: aws.StringSlice([]string{mockRepoName}),
				}).Return(&ecr.DescribeRepositoriesOutput{
					Repositories: []*ecr.Repository{},
				}, nil)
			},
			wantErr: errors.New("no repositories found"),
		},
		"should return repository URI": {
			mockECRClient: func(m *mocks.Mockapi) {
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

			mockECRAPI := mocks.NewMockapi(ctrl)
			tc.mockECRClient(mockECRAPI)

			client := ECR{
				mockECRAPI,
			}

			gotURI, gotErr := client.GetRepository(mockRepoName)

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
		mockECRClient func(m *mocks.Mockapi)

		wantImages []Image
		wantError  error
	}{
		"should wrap error returned by ECR DescribeImages": {
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeImages(gomock.Any()).Return(nil, mockError)
			},
			wantImages: nil,
			wantError:  fmt.Errorf("ecr repo %s describe images: %w", mockRepoName, mockError),
		},
		"should return Image list": {
			mockECRClient: func(m *mocks.Mockapi) {
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
			mockECRClient: func(m *mocks.Mockapi) {
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

			mockECRAPI := mocks.NewMockapi(ctrl)
			tc.mockECRClient(mockECRAPI)

			client := ECR{
				mockECRAPI,
			}

			gotImages, gotError := client.ListImages(mockRepoName)

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
		images        []Image
		mockECRClient func(m *mocks.Mockapi)

		wantError error
	}{
		"should not return error if input Image list is empty": {
			images:        nil,
			mockECRClient: func(m *mocks.Mockapi) {},
			wantError:     nil,
		},
		"should wrap error return from BatchDeleteImage": {
			images: mockImages,
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().BatchDeleteImage(gomock.Any()).Return(nil, mockError)
			},
			wantError: fmt.Errorf("ecr repo %s batch delete image: %w", mockRepoName, mockError),
		},
		"should return nil if call to BatchDeleteImage successful": {
			images: mockImages,
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().BatchDeleteImage(&ecr.BatchDeleteImageInput{
					RepositoryName: aws.String(mockRepoName),
					ImageIds:       imageIdentifiers,
				}).Return(&ecr.BatchDeleteImageOutput{}, nil)
			},
			wantError: nil,
		},
		fmt.Sprintf("should be able to batch delete more than %d images", batchDeleteLimit): {
			images: mockBatchImages,
			mockECRClient: func(m *mocks.Mockapi) {
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
			mockECRClient: func(m *mocks.Mockapi) {
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

			mockECRAPI := mocks.NewMockapi(ctrl)
			tc.mockECRClient(mockECRAPI)

			client := ECR{
				mockECRAPI,
			}

			got := client.DeleteImages(tc.images, mockRepoName)

			require.Equal(t, tc.wantError, got)
		})
	}
}

func TestClearRepository(t *testing.T) {
	mockRepoName := "mockRepoName"
	mockAwsError := awserr.New("someErrorCode", "some error", nil)
	mockError := errors.New("some error")
	mockRepoNotFoundError := awserr.New("RepositoryNotFoundException", "some error", nil)
	mockDigest := "mockDigest"
	mockImageID := ecr.ImageIdentifier{
		ImageDigest: aws.String(mockDigest),
	}

	tests := map[string]struct {
		mockECRClient func(m *mocks.Mockapi)

		wantError error
	}{
		"should clear repo if exists": {
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeImages(&ecr.DescribeImagesInput{
					RepositoryName: aws.String(mockRepoName),
				}).Return(&ecr.DescribeImagesOutput{
					ImageDetails: []*ecr.ImageDetail{
						&ecr.ImageDetail{
							ImageDigest: aws.String(mockDigest),
						},
					},
				}, nil)
				m.EXPECT().BatchDeleteImage(&ecr.BatchDeleteImageInput{
					RepositoryName: aws.String(mockRepoName),
					ImageIds:       []*ecr.ImageIdentifier{&mockImageID},
				}).Return(&ecr.BatchDeleteImageOutput{}, nil)
			},
			wantError: nil,
		},
		"returns nil if repo not exists": {
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeImages(&ecr.DescribeImagesInput{
					RepositoryName: aws.String(mockRepoName),
				}).Return(nil, mockRepoNotFoundError)
			},
			wantError: nil,
		},
		"returns error if fail to check repo existance": {
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeImages(&ecr.DescribeImagesInput{
					RepositoryName: aws.String(mockRepoName),
				}).Return(nil, mockAwsError)
			},
			wantError: fmt.Errorf("ecr repo mockRepoName describe images: %w", mockAwsError),
		},
		"returns error if fail to check repo existance because of non-awserr error type": {
			mockECRClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeImages(&ecr.DescribeImagesInput{
					RepositoryName: aws.String(mockRepoName),
				}).Return(nil, mockError)
			},
			wantError: fmt.Errorf("ecr repo mockRepoName describe images: %w", mockError),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECRAPI := mocks.NewMockapi(ctrl)
			tc.mockECRClient(mockECRAPI)

			client := ECR{
				mockECRAPI,
			}

			gotError := client.ClearRepository(mockRepoName)

			require.Equal(t, tc.wantError, gotError)
		})
	}
}
