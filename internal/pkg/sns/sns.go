// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package sns provides a client to retrieve Copilot SNS information.
package sns

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

const (
	fmtSNSTopicNamePrefix = "%s-%s-%s-"
	topicResourceType     = "sns:topic"
	snsServiceName        = "sns"
)

var (
	errInvalidTopicARN = errors.New("invalid topic ARN")
)

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error)
}

type Topic struct {
	ARN  string
	App  string
	Env  string
	Wkld string
}

func (t Topic) Name() (string, error) {
	parsedARN, err := t.parse()
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf(fmtSNSTopicNamePrefix, t.App, t.Env, t.Wkld)
	if strings.HasPrefix(parsedARN.Resource, prefix) {
		return parsedARN.Resource[len(prefix):], nil
	}
	return "", errInvalidTopicARN
}

func (t Topic) ID() (string, error) {
	parsedARN, err := t.parse()
	if err != nil {
		return "", err
	}
	return parsedARN.Resource, nil
}

func (t Topic) parse() (*arn.ARN, error) {
	parsedARN, err := arn.Parse(string(t.ARN))
	if err != nil {
		return nil, errInvalidTopicARN
	}

	if parsedARN.Service != snsServiceName {
		return nil, errInvalidTopicARN
	}

	if len(strings.Split(parsedARN.Resource, ":")) != 1 {
		return nil, errInvalidTopicARN
	}
	return &parsedARN, nil
}

type Client struct {
	rgGetter resourceGetter
}

func New(sess *session.Session) *Client {
	return &Client{
		rgGetter: resourcegroups.New(sess),
	}
}

func (c Client) ListAppEnvTopics(app, env string) ([]Topic, error) {
	topics, err := c.rgGetter.GetResourcesByTags(topicResourceType, map[string]string{
		deploy.AppTagKey: app,
		deploy.EnvTagKey: env,
	})

	if err != nil {
		return nil, fmt.Errorf("get SNS topic resources for environment %s: %w", env, err)
	}

	if len(topics) == 0 {
		return nil, nil
	}

	var out []Topic
	for _, r := range topics {
		// TODO: if we add env-level SNS topics, remove this check.
		// If the topic doesn't have a specific workload tag, don't return it.
		if _, ok := r.Tags[deploy.ServiceTagKey]; !ok {
			continue
		}
		out = append(out, Topic{
			ARN:  r.ARN,
			App:  app,
			Env:  env,
			Wkld: r.Tags[deploy.ServiceTagKey],
		})
	}

	return out, nil
}
