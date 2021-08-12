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
	prefix string
	wkld   string

	name string
}

// NewTopic creates a new Topic struct, validating the ARN as a Copilot-managed SNS topic.
// This function will
func NewTopic(inputARN string, app, env, wkld string) (*Topic, error) {
	if app == "" || env == "" || wkld == "" {
		return nil, errInvalidComponent
	}

	parsedARN, err := arn.Parse(inputARN)
	if err != nil {
		return nil, errInvalidARN
	}

	if parsedARN.Service != snsServiceName {
		return nil, errInvalidARNService
	}

	t := &Topic{
		prefix: fmt.Sprintf(fmtSNSTopicNamePrefix, app, env, wkld),
		wkld:   wkld,
		awsARN: parsedARN,
	}

	if err = t.validateAndExtractName(); err != nil {
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

// validateAndExtractName determines whether the given ARN is a Copilot-valid SNS topic ARN.
// It extracts the topic name from the ARN resource field.
func (t *Topic) validateAndExtractName() error {
	// Check that the topic name has the correct app-env-workload prefix.
	if !strings.HasPrefix(t.awsARN.Resource, t.prefix) {
		return errInvalidTopicARN
	}
	// Check that the topic resources ID has a postfix AFTER that prefix.
	if len(t.awsARN.Resource)-len(t.prefix) == 0 {
		return errInvalidTopicARN
	}

	t.name = t.awsARN.Resource[len(t.prefix):]

	return nil
}
