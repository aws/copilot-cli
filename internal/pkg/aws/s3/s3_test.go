// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/s3/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestService_PutArtifact(t *testing.T) {
	buf := &bytes.Buffer{}
	fmt.Fprint(buf, "some data")
	timeNow := strconv.FormatInt(time.Now().Unix(), 10)
	testCases := map[string]struct {
		inBucket     string
		inFileName   string
		inData       *bytes.Buffer
		mockS3Client func(m *mocks.Mocks3Client)

		wantErr  error
		wantPath string
	}{
		"should put artifact to s3 bucket and return the path": {
			inBucket:   "mockBucket",
			inData:     buf,
			inFileName: "my-app.addons.stack.yml",
			mockS3Client: func(m *mocks.Mocks3Client) {
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
			mockS3Client: func(m *mocks.Mocks3Client) {
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

			mockS3Client := mocks.NewMocks3Client(ctrl)
			tc.mockS3Client(mockS3Client)

			service := Service{
				s3Svc: mockS3Client,
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
