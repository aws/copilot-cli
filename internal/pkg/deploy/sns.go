// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines workload deployment resources.
package deploy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
)

var errInvalidTopicARN = errors.New("ARN is not a Copilot SNS topic")
var errInvalidARN = errors.New("invalid ARN format")
var errInvalidARNService = errors.New("ARN is not an SNS topic")
var errInvalidComponent = errors.New("Topic must be initialized with valid app, env, and workload")

// Topic holds information about a Copilot SNS topic and its ARN, ID, and Name.
type Topic struct {
	awsARN arn.ARN
	app    string
	env    string
	wkld   string

	name string
}

// NewTopic creates a new Topic struct, validating the ARN as a Copilot-managed SNS topic.
func NewTopic(inputARN string, app, env, wkld string) (*Topic, error) {
	t := &Topic{
		app:  app,
		env:  env,
		wkld: wkld,
	}
	parsedARN, err := arn.Parse(inputARN)
	if err != nil {
		return nil, errInvalidARN
	}
	t.awsARN = parsedARN
	if err := t.validateAndExtractName(); err != nil {
		return nil, err
	}

	return t, nil
}

// Name returns the name of the given SNS topic, stripped of its "app-env-wkld" prefix.
func (t Topic) Name() string {
	return t.name
}

// ARN returns the full ARN of the SNS topic.
func (t Topic) ARN() string {
	return t.awsARN.String()
}

// Workload returns the svc or job the topic is associated with.
func (t Topic) Workload() string {
	return t.wkld
}

// validateAndExtractName determines whether the given ARN is a Copilot-valid SNS topic ARN and
// returns the parsed components of the ARN.
func (t *Topic) validateAndExtractName() error {
	if len(t.wkld) == 0 || len(t.env) == 0 || len(t.app) == 0 {
		return errInvalidComponent
	}

	if t.awsARN.Service != snsServiceName {
		return errInvalidARNService
	}

	// Should not include subscriptions to SNS topics.
	if len(strings.Split(t.awsARN.Resource, ":")) != 1 {
		return errInvalidARNService
	}

	// Check that the topic name has the correct app-env-workload prefix.
	prefix := fmt.Sprintf(fmtSNSTopicNamePrefix, t.app, t.env, t.wkld)
	if !strings.HasPrefix(t.awsARN.Resource, prefix) {
		return errInvalidTopicARN
	}
	// Check that the topic name has a postfix AFTER that prefix.
	if len(t.awsARN.Resource)-len(prefix) == 0 {
		return errInvalidTopicARN
	}

	t.name = t.awsARN.Resource[len(prefix):]

	return nil
}
