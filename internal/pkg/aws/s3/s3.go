// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package s3 contains utility functions for Amazon Simple Storage Service Client.
package s3

import (
	"fmt"
	"io"
	"path"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	artifactDirName = "manual"
)

type s3Client interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

// Service wraps an Amazon Simple Storage Service client.
type Service struct {
	s3Svc s3Client
}

// New returns a Service configured against the input session.
func New(s *session.Session) *Service {
	return &Service{
		s3Svc: s3manager.NewUploader(s),
	}
}

// PutArtifact uploads data to a S3 bucket under a random path that ends with the file name
// and returns its url.
func (s *Service) PutArtifact(bucket, fileName string, data io.Reader) (string, error) {
	id := time.Now().Unix()
	key := path.Join(artifactDirName, strconv.FormatInt(id, 10), fileName)
	resp, err := s.s3Svc.Upload(&s3manager.UploadInput{
		Body:   data,
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("put %s to bucket %s: %w", key, bucket, err)
	}

	return resp.Location, nil
}
