// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package clean

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/s3"
)

type bucketResourceGetter interface {
	BucketName(app, env, wkld string) (string, error)
}

type bucketEmptier interface {
	EmptyBucket(string) error
}

// StaticSite is used to clean up resources created for a static site.
type StaticSite struct {
	app, env, svc        string
	bucketResourceGetter bucketResourceGetter
	bucketEmptier        bucketEmptier
}

// NewStaticSite returns an initialized static site.
func NewStaticSite(app, env, svc string, rg bucketResourceGetter, emptier bucketEmptier) *StaticSite {
	return &StaticSite{
		app:                  app,
		env:                  env,
		svc:                  svc,
		bucketResourceGetter: rg,
		bucketEmptier:        emptier,
	}
}

// Clean looks for the S3 bucket for the service. If no bucket is found,
// it returns no error. If a bucket is found, it is emptied.
func (s *StaticSite) Clean() error {
	bucket, err := s.bucketResourceGetter.BucketName(s.app, s.env, s.svc)
	if err != nil {
		var notFound *s3.ErrNotFound
		if errors.As(err, &notFound) {
			// bucket doesn't exist, no need to clean up
			return nil
		}
		return fmt.Errorf("get bucket name: %w", err)
	}

	if err := s.bucketEmptier.EmptyBucket(bucket); err != nil {
		return fmt.Errorf("empty bucket: %w", err)
	}
	return nil
}
