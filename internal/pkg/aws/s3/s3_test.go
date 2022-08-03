// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestS3_ZipAndUpload(t *testing.T) {
	testCases := map[string]struct {
		mockS3ManagerClient func(m *mocks.Mocks3ManagerAPI)

		wantedURL string
		wantError error
	}{
		"return error if upload fails": {
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(gomock.Any()).Do(func(in *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) {
					require.Equal(t, aws.StringValue(in.Bucket), "mockBucket")
					require.Equal(t, aws.StringValue(in.Key), "mockFileName")
				}).Return(nil, errors.New("some error"))
			},
			wantError: fmt.Errorf("upload mockFileName to bucket mockBucket: some error"),
		},
		"should upload to the s3 bucket": {
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(gomock.Any()).Do(func(in *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) {
					b, err := ioutil.ReadAll(in.Body)
					require.NoError(t, err)
					reader, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
					require.NoError(t, err)
					for _, f := range reader.File {
						require.Equal(t, f.Name, "foo")
						rc, err := f.Open()
						require.NoError(t, err)
						buf := &bytes.Buffer{}
						_, err = io.CopyN(buf, rc, 3)
						require.NoError(t, err)
						require.Equal(t, buf.String(), "bar")
						rc.Close()
						fmt.Println()
					}
					require.Equal(t, aws.StringValue(in.Bucket), "mockBucket")
					require.Equal(t, aws.StringValue(in.Key), "mockFileName")
					require.Equal(t, s3.ObjectCannedACLBucketOwnerFullControl, aws.StringValue(in.ACL))
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

func TestS3_Upload(t *testing.T) {
	testCases := map[string]struct {
		mockS3ManagerClient func(m *mocks.Mocks3ManagerAPI)

		wantedURL string
		wantError error
	}{
		"return error if upload fails": {
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(gomock.Any()).Do(func(in *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) {
					require.Equal(t, aws.StringValue(in.Bucket), "mockBucket")
					require.Equal(t, aws.StringValue(in.Key), "mockFileName")
				}).Return(nil, errors.New("some error"))
			},
			wantError: fmt.Errorf("upload mockFileName to bucket mockBucket: some error"),
		},
		"should upload to the s3 bucket": {
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(gomock.Any()).Do(func(in *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) {
					b, err := ioutil.ReadAll(in.Body)
					require.NoError(t, err)
					require.Equal(t, "bar", string(b))
					require.Equal(t, "mockBucket", aws.StringValue(in.Bucket))
					require.Equal(t, "mockFileName", aws.StringValue(in.Key))
					require.Equal(t, s3.ObjectCannedACLBucketOwnerFullControl, aws.StringValue(in.ACL))
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

			gotURL, gotErr := service.Upload("mockBucket", "mockFileName", bytes.NewBuffer([]byte("bar")))

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
				m.EXPECT().HeadBucket(&s3.HeadBucketInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, nil)
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
				m.EXPECT().HeadBucket(&s3.HeadBucketInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, nil)
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
				m.EXPECT().HeadBucket(&s3.HeadBucketInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, nil)
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
				m.EXPECT().HeadBucket(&s3.HeadBucketInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, nil)
				m.EXPECT().ListObjectVersions(&s3.ListObjectVersionsInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, errors.New("some error"))
			},

			wantErr: fmt.Errorf("list objects for bucket mockBucket: some error"),
		},
		"should not invoke DeleteObjects if bucket is empty": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().HeadBucket(&s3.HeadBucketInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, nil)
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
				m.EXPECT().HeadBucket(&s3.HeadBucketInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, nil)
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
		"should not proceed with deletion as the bucket doesnt exists": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().HeadBucket(&s3.HeadBucketInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, awserr.New(notFound, "message", nil))
			},

			wantErr: nil,
		},
		"should throw error while perform bucket exists check": {
			inBucket: "mockBucket",
			mockS3Client: func(m *mocks.Mocks3API) {
				m.EXPECT().HeadBucket(&s3.HeadBucketInput{
					Bucket: aws.String("mockBucket"),
				}).Return(nil, awserr.New("Unknown", "message", nil))
			},

			wantErr: fmt.Errorf("unable to determine the existance of bucket %s: %w", "mockBucket",
				awserr.New("Unknown", "message", nil)),
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

func TestS3_ParseURL(t *testing.T) {
	testCases := map[string]struct {
		inURL string

		wantedBucketName string
		wantedKey        string
		wantError        error
	}{
		"return error if fail to parse": {
			inURL:     "badURL",
			wantError: fmt.Errorf("cannot parse S3 URL badURL into bucket name and key"),
		},
		"success": {
			inURL:            "https://stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r.s3-us-west-2.amazonaws.com/scripts/dns-cert-validator/dd2278811c3",
			wantedBucketName: "stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r",
			wantedKey:        "scripts/dns-cert-validator/dd2278811c3",
		},
		"success with dots": {
			inURL:            "https://bucket.with.dots.in.name.s3.us-west-2.amazonaws.com/scripts/dns-cert-validator/dd2278811c3",
			wantedBucketName: "bucket.with.dots.in.name",
			wantedKey:        "scripts/dns-cert-validator/dd2278811c3",
		},
		"success with dots legacy URL": {
			inURL:            "https://bucket.with.dots.in.name.s3-us-west-2.amazonaws.com/scripts/dns-cert-validator/dd2278811c3",
			wantedBucketName: "bucket.with.dots.in.name",
			wantedKey:        "scripts/dns-cert-validator/dd2278811c3",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotBucketName, gotKey, gotErr := ParseURL(tc.inURL)

			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantError.Error())
			} else {
				require.Equal(t, gotErr, nil)
				require.Equal(t, tc.wantedBucketName, gotBucketName)
				require.Equal(t, tc.wantedKey, gotKey)
			}
		})
	}
}

func TestURL(t *testing.T) {
	testCases := map[string]struct {
		region string
		bucket string
		key    string

		wanted string
	}{
		// See https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-bucket-intro.html#virtual-host-style-url-ex
		"Formats a virtual-hosted-style URL": {
			region: "us-west-2",
			bucket: "mybucket",
			key:    "puppy.jpg",

			wanted: "https://mybucket.s3.us-west-2.amazonaws.com/puppy.jpg",
		},
		"Formats the URL for a region in the aws-cn partition": {
			region: "cn-north-1",
			bucket: "mybucket",
			key:    "puppy.jpg",

			wanted: "https://mybucket.s3.cn-north-1.amazonaws.cn/puppy.jpg",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, URL(tc.region, tc.bucket, tc.key))
		})
	}
}

func TestS3_FormatARN(t *testing.T) {
	testCases := map[string]struct {
		inPartition string
		inLocation  string

		wantedARN string
	}{
		"success": {
			inPartition: "aws",
			inLocation:  "stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf/manual/1638391936/env",

			wantedARN: "arn:aws:s3:::stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf/manual/1638391936/env",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotARN := FormatARN(tc.inPartition, tc.inLocation)

			require.Equal(t, gotARN, tc.wantedARN)
		})
	}
}
