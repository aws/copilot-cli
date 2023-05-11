// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package clean

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type bucketResourceGetterDouble struct {
	BucketNameFn func(app, env, wkld string) (string, error)
}

func (b *bucketResourceGetterDouble) BucketName(app, env, wkld string) (string, error) {
	return b.BucketNameFn(app, env, wkld)
}

type bucketEmptierDouble struct {
	EmptyBucketFn func(bucket string) error
}

func (b *bucketEmptierDouble) EmptyBucket(bucket string) error {
	return b.EmptyBucketFn(bucket)
}

func TestStaticSite_CleanResources(t *testing.T) {
	tests := map[string]struct {
		cleaner  *StaticSiteCleaner
		expected string
	}{
		"error getting bucket": {
			cleaner: &StaticSiteCleaner{
				bucketResourceGetter: &bucketResourceGetterDouble{
					BucketNameFn: func(app, env, wkld string) (string, error) {
						return "", errors.New("some error")
					},
				},
			},
			expected: "get bucket name: some error",
		},
		"error emptying bucket": {
			cleaner: &StaticSiteCleaner{
				bucketResourceGetter: &bucketResourceGetterDouble{
					BucketNameFn: func(_, _, _ string) (string, error) {
						return "bucket", nil
					},
				},
				bucketEmptier: &bucketEmptierDouble{
					EmptyBucketFn: func(_ string) error {
						return errors.New("some error")
					},
				},
			},
			expected: "empty bucket: some error",
		},
		"happy path": {
			cleaner: &StaticSiteCleaner{
				bucketResourceGetter: &bucketResourceGetterDouble{
					BucketNameFn: func(_, _, _ string) (string, error) {
						return "bucket", nil
					},
				},
				bucketEmptier: &bucketEmptierDouble{
					EmptyBucketFn: func(_ string) error {
						return nil
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.cleaner.Clean()
			if tc.expected != "" {
				require.EqualError(t, err, tc.expected)
				return
			}
			require.NoError(t, err)
		})
	}
}
