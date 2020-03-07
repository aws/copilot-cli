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
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	artifactDirName = "manual"
)

type s3ManagerClient interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

type s3Client interface {
	ListObjectVersions(input *s3.ListObjectVersionsInput) (*s3.ListObjectVersionsOutput, error)
	DeleteObjects(input *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error)
}

// Service wraps an Amazon Simple Storage Service client.
type Service struct {
	s3ManagerSvc s3ManagerClient
	s3Svc        s3Client
}

// New returns a Service configured against the input session.
func New(s *session.Session) *Service {
	return &Service{
		s3ManagerSvc: s3manager.NewUploader(s),
		s3Svc:        s3.New(s),
	}
}

// PutArtifact uploads data to a S3 bucket under a random path that ends with the file name
// and returns its url.
func (s *Service) PutArtifact(bucket, fileName string, data io.Reader) (string, error) {
	id := time.Now().Unix()
	key := path.Join(artifactDirName, strconv.FormatInt(id, 10), fileName)
	resp, err := s.s3ManagerSvc.Upload(&s3manager.UploadInput{
		Body:   data,
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("put %s to bucket %s: %w", key, bucket, err)
	}

	return resp.Location, nil
}

// EmptyBucket deletes all objects within the bucket.
func (s *Service) EmptyBucket(bucket string) error {
	var listResp *s3.ListObjectVersionsOutput
	var err error
	listParams := &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
	}
	// Remove all versions of all objects.
	for {
		listResp, err = s.s3Svc.ListObjectVersions(listParams)
		if err != nil {
			return fmt.Errorf("list objects for bucket %s: %w", bucket, err)
		}
		var objectsToDelete []*s3.ObjectIdentifier
		for _, object := range listResp.Versions {
			objectsToDelete = append(objectsToDelete, &s3.ObjectIdentifier{
				Key:       object.Key,
				VersionId: object.VersionId,
			})
		}
		// After deleting other versions, remove delete markers version.
		// For info on "delete marker": https://docs.aws.amazon.com/AmazonS3/latest/dev/DeleteMarker.html
		for _, deleteMarker := range listResp.DeleteMarkers {
			objectsToDelete = append(objectsToDelete, &s3.ObjectIdentifier{
				Key:       deleteMarker.Key,
				VersionId: deleteMarker.VersionId,
			})
		}
		_, err = s.s3Svc.DeleteObjects(&s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3.Delete{
				Objects: objectsToDelete,
			},
		})
		if !aws.BoolValue(listResp.IsTruncated) {
			return nil
		}
		listParams.KeyMarker = listResp.NextKeyMarker
		listParams.VersionIdMarker = listResp.NextVersionIdMarker
	}
}
