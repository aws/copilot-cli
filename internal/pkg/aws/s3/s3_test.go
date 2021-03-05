// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestS3_PutArtifact(t *testing.T) {
	buf := &bytes.Buffer{}
	fmt.Fprint(buf, "some data")
	timeNow := strconv.FormatInt(time.Now().Unix(), 10)
	testCases := map[string]struct {
		inBucket            string
		inFileName          string
		inData              *bytes.Buffer
		mockS3ManagerClient func(m *mocks.Mocks3ManagerAPI)

		wantErr  error
		wantPath string
	}{
		"should put artifact to s3 bucket and return the path": {
			inBucket:   "mockBucket",
			inData:     buf,
			inFileName: "my-app.addons.stack.yml",
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(&s3manager.UploadInput{
					Body:   buf,
					Bucket: aws.String("mockBucket"),
					Key:    aws.String(fmt.Sprintf("manual/%s/my-app.addons.stack.yml", timeNow)),
				}).Return(&s3manager.UploadOutput{
					Location: fmt.Sprintf("https://mockBucket/manual/%s/my-app.addons.stack.yml", timeNow),
				}, nil)
			},

			wantPath: fmt.Sprintf("https://mockBucket/manual/%s/my-app.addons.stack.yml", timeNow),
		},
		"should return error if fail to upload": {
			inBucket:   "mockBucket",
			inData:     buf,
			inFileName: "my-app.addons.stack.yml",
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(&s3manager.UploadInput{
					Body:   buf,
					Bucket: aws.String("mockBucket"),
					Key:    aws.String(fmt.Sprintf("manual/%s/my-app.addons.stack.yml", timeNow)),
				}).Return(nil, errors.New("some error"))
			},

			wantErr: fmt.Errorf(fmt.Sprintf("put manual/%s/my-app.addons.stack.yml to bucket mockBucket: some error", timeNow)),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockS3ManagerClient := mocks.NewMocks3ManagerAPI(ctrl)
			tc.mockS3ManagerClient(mockS3ManagerClient)

			service := S3{
				s3Manager: mockS3ManagerClient,
			}

			gotPath, gotErr := service.PutArtifact(tc.inBucket, tc.inFileName, tc.inData)

			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.wantPath, gotPath)
			}
		})

	}
}

func TestS3_ZipAndUpload(t *testing.T) {
	timeNow := strconv.FormatInt(time.Now().Unix(), 10)
	testCases := map[string]struct {
		mockS3ManagerClient func(m *mocks.Mocks3ManagerAPI)

		wantedURL string
		wantError error
	}{
		"return error if upload fails": {
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(gomock.Any()).Do(func(in *s3manager.UploadInput) {
					require.Equal(t, aws.StringValue(in.Bucket), "mockBucket")
					require.Equal(t, aws.StringValue(in.Key), fmt.Sprintf("scripts/%s/mockFileName", timeNow))
				}).Return(nil, errors.New("some error"))
			},
			wantError: fmt.Errorf("upload mockFileName to bucket mockBucket: some error"),
		},
		"should upload to the s3 bucket": {
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(gomock.Any()).Do(func(in *s3manager.UploadInput) {
					require.Equal(t, aws.StringValue(in.Bucket), "mockBucket")
					require.Equal(t, aws.StringValue(in.Key), fmt.Sprintf("scripts/%s/mockFileName", timeNow))
				}).Return(&s3manager.UploadOutput{
					Location: "mockURL",
				}, nil)
			},
			wantedURL: "mockURL",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockS3ManagerClient := mocks.NewMocks3ManagerAPI(ctrl)
			tc.mockS3ManagerClient(mockS3ManagerClient)

			service := S3{
				s3Manager: mockS3ManagerClient,
			}

			gotURL, gotErr := service.ZipAndUpload("mockBucket", "mockFileName", namedBinary{})

			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantError.Error())
			} else {
				require.Equal(t, gotErr, nil)
				require.Equal(t, gotURL, tc.wantedURL)
			}
		})

	}
}

type namedBinary struct{}

func (n namedBinary) Name() string { return "foo" }

func (n namedBinary) Content() []byte { return []byte("bar") }

func TestS3_EmptyBucket(t *testing.T) {
	batchObject1 := make([]*s3.ObjectVersion, 1000)
	batchObject2 := make([]*s3.ObjectVersion, 10)
	deleteMarkers := make([]*s3.DeleteMarkerEntry, 10)
	batchObjectID1 := make([]*s3.ObjectIdentifier, 1000)
	batchObjectID2 := make([]*s3.ObjectIdentifier, 10)
	batchObjectID3 := make([]*s3.ObjectIdentifier, 20)
	for i := 0; i < 1000; i++ {
		batchObject1[i] = &s3.ObjectVersion{
			Key:       aws.String("mockKey"),
			VersionId: aws.String("mockVersion"),
		}
		batchObjectID1[i] = &s3.ObjectIdentifier{
			Key:       aws.String("mockKey"),
			VersionId: aws.String("mockVersion"),
		}
	}
	for i := 0; i < 10; i++ {
		batchObject2[i] = &s3.ObjectVersion{
			Key:       aws.String("mockKey"),
			VersionId: aws.String("mockVersion"),
		}
		deleteMarkers[i] = &s3.DeleteMarkerEntry{
			Key:       aws.String("mockKey"),
			VersionId: aws.String("mockVersion"),
		}
		batchObjectID2[i] = &s3.ObjectIdentifier{
			Key:       aws.String("mockKey"),
			VersionId: aws.String("mockVersion"),
		}
		batchObjectID3[i] = &s3.ObjectIdentifier{
			Key:       aws.String("mockKey"),
			VersionId: aws.String("mockVersion"),
		}
		batchObjectID3[i+10] = &s3.ObjectIdentifier{
			Key:       aws.String("mockKey"),
			VersionId: aws.String("mockVersion"),
		}
	}
	testCases := map[string]struct {
		inBucket     string
		mockS3Client func(m *mocks.Mocks3API)

		wantErr error
	}{
		"should delete all objects within the bucket": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().ListObjectVersions(&s3.ListObjectVersionsInput{
					Bucket: aws.String("mockBucket"),
				}).Return(&s3.ListObjectVersionsOutput{
					IsTruncated: aws.Bool(false),
					Versions:    batchObject2,
				}, nil)
				m.EXPECT().DeleteObjects(&s3.DeleteObjectsInput{
					Bucket: aws.String("mockBucket"),
					Delete: &s3.Delete{
						Objects: batchObjectID2,
					},
				}).Return(&s3.DeleteObjectsOutput{}, nil)
			},

			wantErr: nil,
		},
		"should batch delete all objects within the bucket": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().ListObjectVersions(&s3.ListObjectVersionsInput{
					Bucket: aws.String("mockBucket"),
				}).Return(&s3.ListObjectVersionsOutput{
					IsTruncated: aws.Bool(true),
					Versions:    batchObject1,
				}, nil)
				m.EXPECT().DeleteObjects(&s3.DeleteObjectsInput{
					Bucket: aws.String("mockBucket"),
					Delete: &s3.Delete{
						Objects: batchObjectID1,
					},
				}).Return(&s3.DeleteObjectsOutput{}, nil)
				m.EXPECT().ListObjectVersions(&s3.ListObjectVersionsInput{
					Bucket: aws.String("mockBucket"),
				}).Return(&s3.ListObjectVersionsOutput{
					IsTruncated: aws.Bool(false),
					Versions:    batchObject2,
				}, nil)
				m.EXPECT().DeleteObjects(&s3.DeleteObjectsInput{
					Bucket: aws.String("mockBucket"),
					Delete: &s3.Delete{
						Objects: batchObjectID2,
					},
				}).Return(&s3.DeleteObjectsOutput{}, nil)
			},

			wantErr: nil,
		},
		"should delete all objects within the bucket including delete markers": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().ListObjectVersions(&s3.ListObjectVersionsInput{
					Bucket: aws.String("mockBucket"),
				}).Return(&s3.ListObjectVersionsOutput{
					IsTruncated:   aws.Bool(false),
					Versions:      batchObject2,
					DeleteMarkers: deleteMarkers,
				}, nil)
				m.EXPECT().DeleteObjects(&s3.DeleteObjectsInput{
					Bucket: aws.String("mockBucket"),
					Delete: &s3.Delete{
						Objects: batchObjectID3,
					},
				}).Return(&s3.DeleteObjectsOutput{}, nil)
			},

			wantErr: nil,
		},
		"should wrap up error if fail to list objects": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().ListObjectVersions(&s3.ListObjectVersionsInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, errors.New("some error"))
			},

			wantErr: fmt.Errorf("list objects for bucket mockBucket: some error"),
		},
		"should not invoke DeleteObjects if bucket is empty": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().ListObjectVersions(gomock.Any()).Return(&s3.ListObjectVersionsOutput{
					IsTruncated: aws.Bool(false),
				}, nil)
				m.EXPECT().DeleteObjects(gomock.Any()).Times(0)
			},
			wantErr: nil,
		},
		"should wrap up error if fail to delete objects": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().ListObjectVersions(&s3.ListObjectVersionsInput{
					Bucket: aws.String("mockBucket"),
				}).Return(&s3.ListObjectVersionsOutput{
					IsTruncated: aws.Bool(false),
					Versions:    batchObject2,
				}, nil)
				m.EXPECT().DeleteObjects(&s3.DeleteObjectsInput{
					Bucket: aws.String("mockBucket"),
					Delete: &s3.Delete{
						Objects: batchObjectID2,
					},
				}).Return(nil, errors.New("some error"))
			},

			wantErr: fmt.Errorf("delete objects from bucket mockBucket: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockS3Client := mocks.NewMocks3API(ctrl)
			tc.mockS3Client(mockS3Client)

			service := S3{
				s3Client: mockS3Client,
			}

			gotErr := service.EmptyBucket(tc.inBucket)

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})

	}
}
