// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/s3/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type clientMocks struct {
	resourceGetter *mocks.MockresourceGetter
}

func TestClient_Service(t *testing.T) {
	const (
		mockApp    = "mockApp"
		mockEnv    = "mockEnv"
		mockSvc    = "mockSvc"
		mockARN    = "arn:aws:s3:us-west-2:1234567890:myBucket"
		mockBadARN = "badARN"
	)
	mockError := errors.New("some error")
	getRgInput := map[string]string{
		deploy.AppTagKey:     mockApp,
		deploy.EnvTagKey:     mockEnv,
		deploy.ServiceTagKey: mockSvc,
	}

	tests := map[string]struct {
		setupMocks func(mocks clientMocks)

		wantedError error
		wanted      string
	}{
		"error if fail to get bucket resource": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(bucketType, getRgInput).
						Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf(`get S3 bucket with tags "copilot-application"="mockApp","copilot-environment"="mockEnv","copilot-service"="mockSvc": some error`),
		},
		"error if got 0 bucket": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(bucketType, getRgInput).
						Return([]*resourcegroups.Resource{}, nil),
				)
			},
			wantedError: fmt.Errorf(`no S3 bucket found with tags "copilot-application"="mockApp","copilot-environment"="mockEnv","copilot-service"="mockSvc"`),
		},
		"error if got more than 1 bucket": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(bucketType, getRgInput).
						Return([]*resourcegroups.Resource{
							{}, {},
						}, nil),
				)
			},
			wantedError: fmt.Errorf(`more than one S3 bucket with tags "copilot-application"="mockApp","copilot-environment"="mockEnv","copilot-service"="mockSvc"`),
		},
		"fail to parse ARN": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(bucketType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockBadARN},
						}, nil),
				)
			},
			wantedError: fmt.Errorf("parse ARN badARN: invalid S3 ARN: arn: invalid prefix"),
		},
		"success": {
			setupMocks: func(m clientMocks) {
				gomock.InOrder(
					m.resourceGetter.EXPECT().GetResourcesByTags(bucketType, getRgInput).
						Return([]*resourcegroups.Resource{
							{ARN: mockARN},
						}, nil),
				)
			},
			wanted: "myBucket",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockRgGetter := mocks.NewMockresourceGetter(ctrl)
			mocks := clientMocks{
				resourceGetter: mockRgGetter,
			}

			test.setupMocks(mocks)

			client := Client{
				rgGetter: mockRgGetter,
			}

			// WHEN
			get, err := client.BucketName(mockApp, mockEnv, mockSvc)

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, get, test.wanted)
			}
		})
	}
}
