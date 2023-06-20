// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package s3 provides a client to make API requests to Amazon Simple Storage Service.
package s3

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
	"github.com/xlab/treeprint"
)

const (
	// EndpointsID is the ID to look up the S3 service endpoint.
	EndpointsID = s3.EndpointsID

	// Error codes.
	errCodeNotFound = "NotFound"

	// Object location prefixes.
	s3URIPrefix = "s3://"

	// Delimiter for ListObjectsV2Input.
	slashDelimiter = "/"
)

type s3ManagerAPI interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

type s3API interface {
	ListObjectVersions(input *s3.ListObjectVersionsInput) (*s3.ListObjectVersionsOutput, error)
	ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
	DeleteObjects(input *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error)
	HeadBucket(input *s3.HeadBucketInput) (*s3.HeadBucketOutput, error)
}

// NamedBinary is a named binary to be uploaded.
type NamedBinary interface {
	Name() string
	Content() []byte
}

// CompressAndUploadFunc is invoked to zip multiple template contents and upload them to an S3 bucket under the specified key.
type CompressAndUploadFunc func(key string, objects ...NamedBinary) (url string, err error)

// S3 wraps an Amazon Simple Storage Service client.
type S3 struct {
	s3Manager s3ManagerAPI
	s3Client  s3API
}

// New returns an S3 client configured against the input session.
func New(s *session.Session) *S3 {
	return &S3{
		s3Manager: s3manager.NewUploader(s),
		s3Client:  s3.New(s),
	}
}

// Upload uploads a file to an S3 bucket under the specified key.
// Per s3's recommendation https://docs.aws.amazon.com/AmazonS3/latest/userguide/about-object-ownership.html:
// The bucket owner, in addition to the object owner, is granted full control.
func (s *S3) Upload(bucket, key string, data io.Reader) (string, error) {
	return s.upload(bucket, key, data)
}

// EmptyBucket deletes all objects within the bucket.
func (s *S3) EmptyBucket(bucket string) error {
	var listResp *s3.ListObjectVersionsOutput
	var err error

	bucketExists, err := s.bucketExists(bucket)
	if err != nil {
		return fmt.Errorf("unable to determine the existence of bucket %s: %w", bucket, err)
	}

	if !bucketExists {
		return nil
	}

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
		delResp, err := s.s3Client.DeleteObjects(&s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3.Delete{
				Objects: objectsToDelete,
				Quiet:   aws.Bool(true), // we don't care about success values
			},
		})
		switch {
		case err != nil:
			return fmt.Errorf("delete objects from bucket %s: %w", bucket, err)
		case len(delResp.Errors) > 0:
			return errors.Join(
				fmt.Errorf("%d/%d objects failed to delete", len(delResp.Errors), len(objectsToDelete)),
				fmt.Errorf("first failed on key %q: %s", aws.StringValue(delResp.Errors[0].Key), aws.StringValue(delResp.Errors[0].Message)),
			)
		}
		if !aws.BoolValue(listResp.IsTruncated) {
			return nil
		}
		listParams.KeyMarker = listResp.NextKeyMarker
		listParams.VersionIdMarker = listResp.NextVersionIdMarker
	}
}

// ParseURL parses URLs or s3 URIs and returns the bucket name and the optional key.
// For example, the URL: "https://stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r.s3-us-west-2.amazonaws.com/scripts/dns-cert-validator/dd2278811c3"
// or alternatively, the s3 URI: "s3://stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r/scripts/dns-cert-validator/dd2278811c3"
// Returns "stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r" and  "scripts/dns-cert-validator/dd2278811c3"
func ParseURL(url string) (bucket string, key string, err error) {
	if strings.HasPrefix(url, s3URIPrefix) {
		return parseS3URI(url)
	}
	return parseHTTPURI(url)
}

// ParseARN parses an S3 bucket or object ARN.
// For example, a bucket arn "arn:aws:s3:::my-bucket" returns "my-bucket", "", nil.
// Whereas, an object arn "arn:aws:s3:::my-bucket/key.json" returns "my-bucket, "key.json", nil
func ParseARN(arn string) (bucket, key string, err error) {
	parsedARN, err := awsarn.Parse(arn)
	if err != nil {
		return "", "", fmt.Errorf("invalid S3 ARN: %w", err)
	}
	parts := strings.SplitN(parsedARN.Resource, "/", 2)
	if len(parts) == 1 {
		return parts[0], "", nil
	}
	return parts[0], parts[1], nil
}

// URL returns a virtual-hosted–style S3 url for the object stored at key in a bucket created in the specified region.
func URL(region, bucket, key string) string {
	tld := "com"
	for cn := range endpoints.AwsCnPartition().Regions() {
		if cn == region {
			tld = "cn"
			break
		}
	}
	if key != "" {
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.%s/%s", bucket, region, tld, key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.%s", bucket, region, tld)
}

// Location returns an S3 object location URI in the format "s3://bucket/key".
func Location(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}

// FormatARN formats an S3 object ARN.
// For example: arn:aws:s3:::stackset-myapp-infrastru-pipelinebuiltartifactbuc-1nk5t9zkymh8r.s3-us-west-2.amazonaws.com/scripts/dns-cert-validator/dd2278811c3
func FormatARN(partition, location string) string {
	return fmt.Sprintf("arn:%s:s3:::%s", partition, location)
}

// BucketTree creates an ASCII tree representing the folder structure of a bucket's objects.
func (s *S3) BucketTree(bucket string) (string, error) {
	outputs, err := s.listObjects(bucket, "/")
	if err != nil || outputs == nil {
		return "", err
	}
	var contents []*s3.Object
	var prefixes []*s3.CommonPrefix
	for _, output := range outputs {
		contents = append(contents, output.Contents...)
		prefixes = append(prefixes, output.CommonPrefixes...)
	}
	tree := treeprint.New()
	// Add top-level files.
	for _, object := range contents {
		tree.AddNode(aws.StringValue(object.Key))
	}
	// Recursively add folders and their children.
	if err := s.addNodes(tree, prefixes, bucket); err != nil {
		return "", err
	}
	return tree.String(), nil
}

// BucketSizeAndCount returns the total size and number of objects in an S3 bucket.
func (s *S3) BucketSizeAndCount(bucket string) (string, int, error) {
	outputs, err := s.listObjects(bucket, "")
	if err != nil || outputs == nil {
		return "", 0, err
	}
	var size int64
	var count int
	for _, output := range outputs {
		for _, object := range output.Contents {
			size += aws.Int64Value(object.Size)
			count++
		}
	}
	return humanize.Bytes(uint64(size)), count, nil
}

func (s *S3) listObjects(bucket, delimiter string) ([]s3.ListObjectsV2Output, error) {
	exists, err := s.bucketExists(bucket)
	if err != nil || !exists {
		return nil, err
	}
	var outputs []s3.ListObjectsV2Output
	listResp := &s3.ListObjectsV2Output{}
	for {
		listParams := &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Delimiter:         aws.String(delimiter),
			ContinuationToken: listResp.NextContinuationToken,
		}
		listResp, err = s.s3Client.ListObjectsV2(listParams)
		if err != nil {
			return nil, fmt.Errorf("list objects for bucket %s: %w", bucket, err)
		}
		outputs = append(outputs, *listResp)
		if listResp.NextContinuationToken == nil {
			break
		}
	}
	return outputs, nil
}

func (s *S3) bucketExists(bucket string) (bool, error) {
	input := &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}
	_, err := s.s3Client.HeadBucket(input)
	if err != nil {
		var aerr awserr.Error
		if errors.As(err, &aerr) && aerr.Code() == errCodeNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3) addNodes(tree treeprint.Tree, prefixes []*s3.CommonPrefix, bucket string) error {
	if len(prefixes) == 0 {
		return nil
	}
	listResp := &s3.ListObjectsV2Output{}
	var err error
	for _, prefix := range prefixes {
		var respContents []*s3.Object
		var respPrefixes []*s3.CommonPrefix
		branch := tree.AddBranch(filepath.Base(aws.StringValue(prefix.Prefix)))
		for {
			listParams := &s3.ListObjectsV2Input{
				Bucket:            aws.String(bucket),
				Delimiter:         aws.String(slashDelimiter),
				ContinuationToken: listResp.ContinuationToken,
				Prefix:            prefix.Prefix,
			}
			listResp, err = s.s3Client.ListObjectsV2(listParams)
			if err != nil {
				return fmt.Errorf("list objects for bucket %s: %w", bucket, err)
			}
			respContents = append(respContents, listResp.Contents...)
			respPrefixes = append(respPrefixes, listResp.CommonPrefixes...)
			if listResp.NextContinuationToken == nil {
				break
			}
		}
		for _, file := range respContents {
			fileName := filepath.Base(aws.StringValue(file.Key))
			branch.AddNode(fileName)
		}
		if err := s.addNodes(branch, respPrefixes, bucket); err != nil {
			return err
		}
	}
	return nil
}

func (s *S3) upload(bucket, key string, buf io.Reader) (string, error) {
	in := &s3manager.UploadInput{
		Body:        buf,
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ACL:         aws.String(s3.ObjectCannedACLBucketOwnerFullControl),
		ContentType: defaultContentTypeFromExt(key),
	}
	resp, err := s.s3Manager.Upload(in)
	if err != nil {
		return "", fmt.Errorf("upload %s to bucket %s: %w", key, bucket, err)
	}
	return resp.Location, nil
}

func defaultContentTypeFromExt(key string) *string {
	contentType := mime.TypeByExtension(filepath.Ext(key))
	if contentType == "" {
		return nil
	}
	return aws.String(contentType)
}

// parseS3URI parses the bucket name and object key from a [s3:// URI].
// For example: s3://mybucket/puppy.jpg
//
// [s3:// URI]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-bucket-intro.html#accessing-a-bucket-using-S3-format
func parseS3URI(uri string) (bucket, key string, err error) {
	parsed := strings.SplitN(strings.TrimPrefix(uri, s3URIPrefix), "/", 2)
	bucket = parsed[0]
	if bucket == "" {
		return "", "", fmt.Errorf("cannot parse S3 URI %s into bucket name and key", uri)
	}
	if len(parsed) == 2 {
		key = parsed[1]
	}
	return
}

// parseHTTPURL parses the bucket name and optional key from a [virtual-hosted-style access URL].
// For example: https://DOC-EXAMPLE-BUCKET1.s3.us-west-2.amazonaws.com/puppy.png
//
// [virtual-hosted-style access URL]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-bucket-intro.html#virtual-host-style-url-ex
func parseHTTPURI(url string) (bucket, key string, err error) {
	parsedURL := strings.SplitN(strings.TrimPrefix(url, "https://"), "/", 2)

	// go through the host backwards and find the first piece that
	// starts with 's3' - the rest of the host (to the left of 's3')
	// is the bucket name. this is to support both URL host formats:
	// <bucket>.s3-<region>.amazonaws.com and <bucket>.s3.<region>.amazonaws.com
	split := strings.Split(parsedURL[0], ".")
	bucketEndIdx := len(split) - 1
	for ; bucketEndIdx > 0; bucketEndIdx-- {
		if strings.HasPrefix(split[bucketEndIdx], "s3") {
			break
		}
	}
	bucket = strings.Join(split[:bucketEndIdx], ".")
	if bucket == "" {
		return "", "", fmt.Errorf("cannot parse S3 URL %s into bucket name and key", url)
	}
	if len(parsedURL) == 2 {
		key = parsedURL[1]
	}
	return
}
