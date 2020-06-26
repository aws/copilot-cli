// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDynamoDB_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, ddb *DynamoDB)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, ddb *DynamoDB) {
				m := mocks.NewMockParser(ctrl)
				ddb.parser = m
				m.EXPECT().Parse(dynamoDbAddonPath, *ddb, gomock.Any()).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, ddb *DynamoDB) {
				m := mocks.NewMockParser(ctrl)
				ddb.parser = m
				m.EXPECT().Parse(dynamoDbAddonPath, *ddb, gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			addon := &DynamoDB{}
			tc.mockDependencies(ctrl, addon)

			// WHEN
			b, err := addon.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestS3_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, s3 *S3)

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			mockDependencies: func(ctrl *gomock.Controller, s3 *S3) {
				m := mocks.NewMockParser(ctrl)
				s3.parser = m
				m.EXPECT().Parse(s3AddonPath, *s3, gomock.Any()).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("some error"),
		},
		"returns rendered content": {
			mockDependencies: func(ctrl *gomock.Controller, s3 *S3) {
				m := mocks.NewMockParser(ctrl)
				s3.parser = m
				m.EXPECT().Parse(s3AddonPath, *s3, gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("hello")}, nil)

			},

			wantedBinary: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			addon := &S3{}
			tc.mockDependencies(ctrl, addon)

			// WHEN
			b, err := addon.MarshalBinary()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}
