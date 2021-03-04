// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package s3 provides a client to make API requests to Amazon Simple Storage Service.
package s3

import (
	"archive/zip"
	"bytes"
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

type s3ManagerAPI interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

type s3API interface {
	ListObjectVersions(input *s3.ListObjectVersionsInput) (*s3.ListObjectVersionsOutput, error)
	DeleteObjects(input *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error)
}

// S3 wraps an Amazon Simple Storage Service client.
type S3 struct {
	s3Manager s3ManagerAPI
	s3Client  s3API

	// cache variables for unit tests.
	buf *bytes.Buffer
}

// New returns an S3 client configured against the input session.
func New(s *session.Session) *S3 {
	return &S3{
		s3Manager: s3manager.NewUploader(s),
		s3Client:  s3.New(s),
	}
}

// PutArtifact uploads data to a S3 bucket under a random path that ends with
// the file name and returns its url.
func (s *S3) PutArtifact(bucket, fileName string, data io.Reader) (string, error) {
	id := time.Now().Unix()
	key := path.Join(artifactDirName, strconv.FormatInt(id, 10), fileName)
	resp, err := s.s3Manager.Upload(&s3manager.UploadInput{
		Body:   data,
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("put %s to bucket %s: %w", key, bucket, err)
	}

	return resp.Location, nil
}

// ZipAndUpload zips the file and uploads data to a S3 bucket.
func (s *S3) ZipAndUpload(bucket, name string, data map[string]string) error {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	for name, content := range data {
		f, err := w.Create(name)
		if err != nil {
			return fmt.Errorf("create zip file %s: %w", name, err)
		}
		_, err = f.Write([]byte(content))
		if err != nil {
			return fmt.Errorf("write zip file %s: %w", name, err)
		}
	}
	err := w.Close()
	if err != nil {
		return err
	}
	// For unit test override
	if s.buf == nil {
		s.buf = buf
		defer func() {
			s.buf = nil
		}()
	}
	if _, err := s.s3Manager.Upload(&s3manager.UploadInput{
		Body:   s.buf,
		Bucket: aws.String(bucket),
		Key:    aws.String(name),
	}); err != nil {
		return fmt.Errorf("upload %s to bucket %s: %w", name, bucket, err)
	}
	return nil
}

// EmptyBucket deletes all objects within the bucket.
func (s *S3) EmptyBucket(bucket string) error {
	var listResp *s3.ListObjectVersionsOutput
	var err error
	listParams := &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
	}
	// Remove all versions of all objects.
	for {
		listResp, err = s.s3Client.ListObjectVersions(listParams)
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
		if len(objectsToDelete) == 0 {
			return nil
		}
		_, err = s.s3Client.DeleteObjects(&s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3.Delete{
				Objects: objectsToDelete,
			},
		})
		if err != nil {
			return fmt.Errorf("delete objects from bucket %s: %w", bucket, err)
		}
		if !aws.BoolValue(listResp.IsTruncated) {
			return nil
		}
		listParams.KeyMarker = listResp.NextKeyMarker
		listParams.VersionIdMarker = listResp.NextVersionIdMarker
	}
}
