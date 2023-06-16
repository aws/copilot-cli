// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

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
					require.Equal(t, aws.StringValue(in.Key), "src/mockIndex.html")
				}).Return(nil, errors.New("some error"))
			},
			wantError: fmt.Errorf("upload src/mockIndex.html to bucket mockBucket: some error"),
		},
		"should upload to the s3 bucket": {
			mockS3ManagerClient: func(m *mocks.Mocks3ManagerAPI) {
				m.EXPECT().Upload(gomock.Any()).Do(func(in *s3manager.UploadInput, _ ...func(*s3manager.Uploader)) {
					b, err := io.ReadAll(in.Body)
					require.NoError(t, err)
					require.Equal(t, "bar", string(b))
					require.Equal(t, "mockBucket", aws.StringValue(in.Bucket))
					require.Equal(t, "src/mockIndex.html", aws.StringValue(in.Key))
					require.Equal(t, "text/html; charset=utf-8", aws.StringValue(in.ContentType))
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

			gotURL, gotErr := service.Upload("mockBucket", "src/mockIndex.html", bytes.NewBuffer([]byte("bar")))

			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantError.Error())
			} else {
				require.Equal(t, gotErr, nil)
				require.Equal(t, gotURL, tc.wantedURL)
			}
		})
	}
}

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
						Quiet:   aws.Bool(true),
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
						Quiet:   aws.Bool(true),
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
						Quiet:   aws.Bool(true),
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
						Quiet:   aws.Bool(true),
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
						Quiet:   aws.Bool(true),
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
				}).Return(nil, awserr.New(errCodeNotFound, "message", nil))
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

			wantErr: fmt.Errorf("unable to determine the existence of bucket %s: %w", "mockBucket",
				awserr.New("Unknown", "message", nil)),
		},
		"some objects failed to delete": {
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
						Quiet:   aws.Bool(true),
					},
				}).Return(&s3.DeleteObjectsOutput{
					Errors: []*s3.Error{
						{
							Key:     aws.String("mock/key"),
							Message: aws.String("some error"),
						},
					},
				}, nil)
			},
			wantErr: fmt.Errorf(`1/10 objects failed to delete
first failed on key "mock/key": some error`),
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
				return
			}
			require.NoError(t, gotErr)
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
		"return error S3 URI": {
			inURL:     "s3://",
			wantError: fmt.Errorf("cannot parse S3 URI s3:// into bucket name and key"),
		},
		"parses S3 URI": {
			inURL:            "s3://amplify-demo-dev-94628-deployment/auth/amplify-meta.json",
			wantedBucketName: "amplify-demo-dev-94628-deployment",
			wantedKey:        "auth/amplify-meta.json",
		},
		"parses object URL": {
			inURL:            "https://stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r.s3-us-west-2.amazonaws.com/scripts/dns-cert-validator/dd2278811c3",
			wantedBucketName: "stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r",
			wantedKey:        "scripts/dns-cert-validator/dd2278811c3",
		},
		"parses bucket URL": {
			inURL:            "https://stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r.s3-us-west-2.amazonaws.com",
			wantedBucketName: "stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r",
			wantedKey:        "",
		},
		"parses object URL with dots": {
			inURL:            "https://bucket.with.dots.in.name.s3.us-west-2.amazonaws.com/scripts/dns-cert-validator/dd2278811c3",
			wantedBucketName: "bucket.with.dots.in.name",
			wantedKey:        "scripts/dns-cert-validator/dd2278811c3",
		},
		"parses legacy object URL with dots": {
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
				require.NoError(t, tc.wantError)
				require.Equal(t, tc.wantedBucketName, gotBucketName)
				require.Equal(t, tc.wantedKey, gotKey)
			}
		})
	}
}

func Test_ParseARN(t *testing.T) {
	type wanted struct {
		bucket string
		key    string
		err    error
	}
	testCases := map[string]struct {
		in     string
		wanted wanted
	}{
		"bad bad arn": {
			in:     "i am not an arn at all",
			wanted: wanted{err: errors.New("invalid S3 ARN")},
		},
		"parses an S3 bucket arn": {
			in:     "arn:aws:s3:::amplify-demo-dev-94628-deployment",
			wanted: wanted{bucket: "amplify-demo-dev-94628-deployment"},
		},
		"parses an S3 object arn": {
			in:     "arn:aws:s3:::amplify-demo-dev-94628-deployment/studio-backend/auth/demo6a968da2/build/parameters.json",
			wanted: wanted{bucket: "amplify-demo-dev-94628-deployment", key: "studio-backend/auth/demo6a968da2/build/parameters.json"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			bucket, key, err := ParseARN(tc.in)
			if tc.wanted.err != nil {
				require.ErrorContains(t, err, tc.wanted.err.Error())
				return
			}
			require.Equal(t, tc.wanted.bucket, bucket)
			require.Equal(t, tc.wanted.key, key)
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
		"Formats a virtual-hosted-style URL with no key": {
			region: "us-west-2",
			bucket: "mybucket",

			wanted: "https://mybucket.s3.us-west-2.amazonaws.com",
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

func TestS3_BucketTree(t *testing.T) {
	type s3Mocks struct {
		s3API        *mocks.Mocks3API
		s3ManagerAPI *mocks.Mocks3ManagerAPI
	}
	mockBucket := aws.String("bucketName")
	delimiter := aws.String("/")
	nonexistentError := awserr.New(errCodeNotFound, "msg", errors.New("some error"))
	mockContinuationToken := "next"

	firstResp := s3.ListObjectsV2Output{
		CommonPrefixes: []*s3.CommonPrefix{
			{Prefix: aws.String("Images")},
			{Prefix: aws.String("css")},
			{Prefix: aws.String("top")},
		},
		Contents: []*s3.Object{
			{Key: aws.String("README.md")},
			{Key: aws.String("error.html")},
			{Key: aws.String("index.html")},
		},
		Delimiter: delimiter,
		KeyCount:  aws.Int64(14),
		MaxKeys:   aws.Int64(1000),
		Name:      mockBucket,
	}
	imagesResp := s3.ListObjectsV2Output{
		Contents: []*s3.Object{
			{Key: aws.String("firstImage.PNG")},
			{Key: aws.String("secondImage.PNG")},
		},
		Delimiter: delimiter,
		KeyCount:  aws.Int64(14),
		MaxKeys:   aws.Int64(1000),
		Name:      mockBucket,
	}
	cssResp := s3.ListObjectsV2Output{
		Contents: []*s3.Object{
			{Key: aws.String("Style.css")},
			{Key: aws.String("bootstrap.min.css")},
		},
		Delimiter: delimiter,
		KeyCount:  aws.Int64(14),
		MaxKeys:   aws.Int64(1000),
		Name:      mockBucket,
	}
	topResp := s3.ListObjectsV2Output{
		CommonPrefixes: []*s3.CommonPrefix{
			{Prefix: aws.String("middle")},
		},
		Delimiter: delimiter,
		KeyCount:  aws.Int64(14),
		MaxKeys:   aws.Int64(1000),
		Name:      mockBucket,
	}
	middleResp := s3.ListObjectsV2Output{
		Contents: []*s3.Object{
			{Key: aws.String("bottom.html")},
		},
		Delimiter: delimiter,
		KeyCount:  aws.Int64(14),
		MaxKeys:   aws.Int64(1000),
		Name:      mockBucket,
	}
	testCases := map[string]struct {
		setupMocks func(mocks s3Mocks)

		wantTree string
		wantErr  error
	}{
		"should return all objects within the bucket as a tree string": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					ContinuationToken: nil,
					Delimiter:         delimiter,
					Prefix:            nil,
				}).Return(&firstResp, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:    mockBucket,
					Delimiter: delimiter,
					Prefix:    aws.String("Images"),
				}).Return(&imagesResp, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:    mockBucket,
					Delimiter: delimiter,
					Prefix:    aws.String("css"),
				}).Return(&cssResp, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:    mockBucket,
					Delimiter: delimiter,
					Prefix:    aws.String("top"),
				}).Return(&topResp, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:    mockBucket,
					Delimiter: delimiter,
					Prefix:    aws.String("middle"),
				}).Return(&middleResp, nil)
			},
			wantTree: `.
├── README.md
├── error.html
├── index.html
├── Images
│   ├── firstImage.PNG
│   └── secondImage.PNG
├── css
│   ├── Style.css
│   └── bootstrap.min.css
└── top
    └── middle
        └── bottom.html
`,
		},
		"should handle multiple pages of objects": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					ContinuationToken: nil,
					Delimiter:         delimiter,
					Prefix:            nil,
				}).Return(
					&s3.ListObjectsV2Output{
						Contents: []*s3.Object{
							{Key: aws.String("README.md")},
						},
						Delimiter:             delimiter,
						KeyCount:              aws.Int64(14),
						MaxKeys:               aws.Int64(1000),
						Name:                  mockBucket,
						NextContinuationToken: &mockContinuationToken,
					}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					ContinuationToken: &mockContinuationToken,
					Delimiter:         delimiter,
					Prefix:            nil,
				}).Return(
					&s3.ListObjectsV2Output{
						Contents: []*s3.Object{
							{Key: aws.String("READMETOO.md")},
						},
						Delimiter:             delimiter,
						KeyCount:              aws.Int64(14),
						MaxKeys:               aws.Int64(1000),
						Name:                  mockBucket,
						NextContinuationToken: nil,
					}, nil)
			},
			wantTree: `.
├── README.md
└── READMETOO.md
`,
		},
		"return nil if bucket doesn't exist": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nonexistentError)
			},
		},
		"return err if cannot determine if bucket exists": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, errors.New("some error"))
			},
			wantErr: errors.New("some error"),
		},
		"should wrap error if fail to list objects": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					ContinuationToken: nil,
					Delimiter:         delimiter,
					Prefix:            nil,
				}).Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("list objects for bucket bucketName: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockS3Client := mocks.NewMocks3API(ctrl)

			mockS3Manager := mocks.NewMocks3ManagerAPI(ctrl)

			s3mocks := s3Mocks{
				s3API:        mockS3Client,
				s3ManagerAPI: mockS3Manager,
			}
			service := S3{
				s3Client: mockS3Client,
			}
			tc.setupMocks(s3mocks)

			gotTree, gotErr := service.BucketTree(aws.StringValue(mockBucket))
			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
				return
			}
			require.NoError(t, gotErr)
			require.Equal(t, tc.wantTree, gotTree)
		})

	}
}

func TestS3_BucketSizeAndCount(t *testing.T) {
	type s3Mocks struct {
		s3API        *mocks.Mocks3API
		s3ManagerAPI *mocks.Mocks3ManagerAPI
	}
	mockBucket := aws.String("bucketName")
	nonexistentError := awserr.New(errCodeNotFound, "msg", errors.New("some error"))
	mockContinuationToken := "next"

	resp := s3.ListObjectsV2Output{
		Contents: []*s3.Object{
			{
				Key:  aws.String("README.md"),
				Size: aws.Int64(111111),
			},
			{
				Key:  aws.String("error.html"),
				Size: aws.Int64(222222),
			},
			{
				Key:  aws.String("index.html"),
				Size: aws.Int64(333333),
			},
		},
		KeyCount: aws.Int64(14),
		MaxKeys:  aws.Int64(1000),
		Name:     mockBucket,
	}

	testCases := map[string]struct {
		setupMocks func(mocks s3Mocks)

		wantSize  string
		wantCount int
		wantErr   error
	}{
		"should return correct size and count": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					Delimiter:         aws.String(""),
					ContinuationToken: nil,
					Prefix:            nil,
				}).Return(&resp, nil)
			},
			wantSize:  "667 kB",
			wantCount: 3,
		},
		"should handle multiple pages of objects": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					Delimiter:         aws.String(""),
					ContinuationToken: nil,
					Prefix:            nil,
				}).Return(
					&s3.ListObjectsV2Output{
						Contents: []*s3.Object{
							{
								Key:  aws.String("README.md"),
								Size: aws.Int64(123),
							},
						},
						KeyCount:              aws.Int64(14),
						MaxKeys:               aws.Int64(1000),
						Name:                  mockBucket,
						NextContinuationToken: &mockContinuationToken,
					}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					Delimiter:         aws.String(""),
					ContinuationToken: &mockContinuationToken,
					Prefix:            nil,
				}).Return(
					&s3.ListObjectsV2Output{
						Contents: []*s3.Object{
							{
								Key:  aws.String("READMETOO.md"),
								Size: aws.Int64(321),
							},
						},
						KeyCount:              aws.Int64(14),
						MaxKeys:               aws.Int64(1000),
						Name:                  mockBucket,
						NextContinuationToken: nil,
					}, nil)
			},
			wantSize:  "444 B",
			wantCount: 2,
		},
		"empty bucket": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					Delimiter:         aws.String(""),
					ContinuationToken: nil,
					Prefix:            nil,
				}).Return(&s3.ListObjectsV2Output{
					Contents: nil,
					Name:     mockBucket,
				}, nil)
			},
			wantSize:  "0 B",
			wantCount: 0,
		},
		"return nil if bucket doesn't exist": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nonexistentError)
			},
		},
		"return err if cannot determine if bucket exists": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, errors.New("some error"))
			},
			wantErr: errors.New("some error"),
		},
		"should wrap error if fail to list objects": {
			setupMocks: func(m s3Mocks) {
				m.s3API.EXPECT().HeadBucket(&s3.HeadBucketInput{Bucket: mockBucket}).Return(&s3.HeadBucketOutput{}, nil)
				m.s3API.EXPECT().ListObjectsV2(&s3.ListObjectsV2Input{
					Bucket:            mockBucket,
					Delimiter:         aws.String(""),
					ContinuationToken: nil,
				}).Return(nil, errors.New("some error"))
			},
			wantErr: errors.New("list objects for bucket bucketName: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockS3Client := mocks.NewMocks3API(ctrl)
			mockS3Manager := mocks.NewMocks3ManagerAPI(ctrl)

			s3mocks := s3Mocks{
				s3API:        mockS3Client,
				s3ManagerAPI: mockS3Manager,
			}
			service := S3{
				s3Client: mockS3Client,
			}
			tc.setupMocks(s3mocks)

			gotSize, gotCount, gotErr := service.BucketSizeAndCount(aws.StringValue(mockBucket))
			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
				return
			}
			require.NoError(t, gotErr)
			require.Equal(t, tc.wantSize, gotSize)
			require.Equal(t, tc.wantCount, gotCount)
		})

	}
}
