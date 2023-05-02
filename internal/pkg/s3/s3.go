// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package s3 provides a client to retrieve Copilot S3 information.
package s3

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

const (
	bucketType = "s3:bucket"
)

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error)
}

// Client retrieves Copilot S3 service information from AWS.
type Client struct {
	rgGetter resourceGetter
}

// New inits a new Client.
func New(sess *session.Session) *Client {
	return &Client{
		rgGetter: resourcegroups.New(sess),
	}
}

// BucketName returns the bucket name given the Copilot app, env, and Static Site service name.
func (c Client) BucketName(app, env, svc string) (string, error) {
	tags := tags(map[string]string{
		deploy.AppTagKey:     app,
		deploy.EnvTagKey:     env,
		deploy.ServiceTagKey: svc,
	})
	buckets, err := c.rgGetter.GetResourcesByTags(bucketType, tags)
	if err != nil {
		return "", fmt.Errorf("get S3 bucket with tags %s: %w", tags.String(), err)
	}
	if len(buckets) == 0 {
		return "", &ErrNotFound{tags}
	}
	if len(buckets) > 1 {
		return "", fmt.Errorf("more than one S3 bucket with tags %s", tags.String())
	}
	bucketName, _, err := s3.ParseARN(buckets[0].ARN)
	if err != nil {
		return "", fmt.Errorf("parse ARN %s: %w", buckets[0].ARN, err)
	}
	return bucketName, nil
}

type tags map[string]string

func (tags tags) String() string {
	serialized := make([]string, len(tags))
	var i = 0
	for k, v := range tags {
		serialized[i] = fmt.Sprintf("%q=%q", k, v)
		i += 1
	}
	sort.SliceStable(serialized, func(i, j int) bool { return serialized[i] < serialized[j] })
	return strings.Join(serialized, ",")
}

// ErrNotFound is returned when no bucket is found
// matching the given tags.
type ErrNotFound struct {
	tags tags
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("no S3 bucket found with tags %s", e.tags.String())
}
