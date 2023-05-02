// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package delete

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/delete/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestStaticSite_CleanResources(t *testing.T) {
	type moks struct {
		bucketResourceGetter *mocks.MockbucketResourceGetter
		bucketEmptier        *mocks.MockbucketEmptier
	}

	tests := map[string]struct {
		mocks    func(m *moks)
		expected string
	}{
		"error getting bucket": {
			mocks: func(m *moks) {
				m.bucketResourceGetter.EXPECT().BucketName("app", "env", "wkld").Return("", errors.New("some error"))
			},
			expected: "get bucket name: some error",
		},
		"error emptying bucket": {
			mocks: func(m *moks) {
				m.bucketResourceGetter.EXPECT().BucketName("app", "env", "wkld").Return("bucket", nil)
				m.bucketEmptier.EXPECT().EmptyBucket("bucket").Return(errors.New("some error"))
			},
			expected: "empty bucket: some error",
		},
		"happy path": {
			mocks: func(m *moks) {
				m.bucketResourceGetter.EXPECT().BucketName("app", "env", "wkld").Return("bucket", nil)
				m.bucketEmptier.EXPECT().EmptyBucket("bucket").Return(nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &moks{
				bucketResourceGetter: mocks.NewMockbucketResourceGetter(ctrl),
				bucketEmptier:        mocks.NewMockbucketEmptier(ctrl),
			}
			tc.mocks(m)

			deleter := &StaticSiteDeleter{
				BucketResourceGetter: m.bucketResourceGetter,
				BucketEmptier:        m.bucketEmptier,
			}

			err := deleter.CleanResources("app", "env", "wkld")
			if tc.expected != "" {
				require.EqualError(t, err, tc.expected)
				return
			}
			require.NoError(t, err)
		})
	}
}
