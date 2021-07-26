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

type Topic struct {
	ARN  string
	App  string
	Env  string
	Wkld string
}

// Name returns the name of the given SNS topic, stripped of its "app-env-wkld" prefix.
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

// ID returns the resource ID of the topic (the last element of the ARN).
func (t Topic) ID() (string, error) {
	parsedARN, err := t.parse()
	if err != nil {
		return "", err
	}
	return parsedARN.Resource, nil
}

// parse determines whether the given ARN is a Copilot-valid SNS topic ARN and returns the
// parsed components of the ARN.
func (t Topic) parse() (*arn.ARN, error) {
	parsedARN, err := arn.Parse(string(t.ARN))
	if err != nil {
		return nil, errInvalidARN
	}

	if parsedARN.Service != snsServiceName {
		return nil, errInvalidARNService
	}

	if len(strings.Split(parsedARN.Resource, ":")) != 1 {
		return nil, errInvalidARNService
	}
	return &parsedARN, nil
}
