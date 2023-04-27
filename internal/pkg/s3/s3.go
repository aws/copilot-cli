// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package s3 provides a client to retrieve Copilot S3 information.
package s3

import (
	"fmt"

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
	buckets, err := c.rgGetter.GetResourcesByTags(bucketType, map[string]string{
		deploy.AppTagKey:     app,
		deploy.EnvTagKey:     env,
		deploy.ServiceTagKey: svc,
	})
	if err != nil {
		return "", fmt.Errorf("get S3 bucket with tags (%s, %s, %s): %w", app, env, svc, err)
	}
	if len(buckets) == 0 {
		return "", fmt.Errorf("no S3 bucket found with tags %s, %s, %s", svc, env, svc)
	}
	if len(buckets) > 1 {
		return "", fmt.Errorf("more than one S3 bucket with the name %s found in environment %s", svc, env)
	}
	bucketName, _, err := s3.ParseARN(buckets[0].ARN)
	if err != nil {
		return "", fmt.Errorf("parse ARN %s: %w", buckets[0].ARN, err)
	}
	return bucketName, nil
}
