// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package sqs provides a client to make API requests to Amazon SQS Service.
package sqs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/endpoints"
)

var SQSQueueURLPattern = "%s/%s/%s"
var sqsServiceName = "sqs"
var fmtCopilotResourceNamePrefix = "%s-%s-%s-"

var errInvalidARN = errors.New("invalid ARN format")
var errInvalidQueueARN = errors.New("ARN is not a Copilot SQS queue")
var errInvalidARNSQSService = errors.New("ARN is not a SQS queue")
var errInvalidComponent = errors.New("Queue must be initialized with valid app, env, and workload")

// Queue holds information about a Copilot SQS Queue and its ARN, name, and URL.
type Queue struct {
	awsARN arn.ARN
	app    string
	env    string
	wkld   string

	name string
	url  string
}

// NewQueue returns a new SQS Queue.
func NewQueue(inputARN string, app, env, wkld string) (*Queue, error) {
	q := &Queue{
		app:  app,
		env:  env,
		wkld: wkld,
	}
	parsedARN, err := arn.Parse(inputARN)
	if err != nil {
		return nil, errInvalidARN
	}
	q.awsARN = parsedARN
	if err := q.validateAndExtractName(); err != nil {
		return nil, err
	}
	if err := q.validateAndExtractURL(); err != nil {
		return nil, err
	}

	return q, nil
}

// Name returns the name of the given SQS Queue, stripped of its "app-env-wkld" prefix.
func (q Queue) Name() string {
	return q.name
}

// ARN returns the ARN of the given SQS Queue
func (q Queue) ARN() string {
	return q.awsARN.String()
}

// Workload returns the workload of the given SQS Queue
func (q Queue) Workload() string {
	return q.wkld
}

// validateAndExtractName determines whether the given ARN is a Copilot-valid SQS Queue ARN and
// returns the parsed components of the ARN.
func (q *Queue) validateAndExtractName() error {
	if len(q.wkld) == 0 || len(q.env) == 0 || len(q.app) == 0 {
		return errInvalidComponent
	}

	if q.awsARN.Service != sqsServiceName {
		return errInvalidARNSQSService
	}

	// Should not include subscriptions to SQS queue.
	if len(strings.Split(q.awsARN.Resource, ":")) != 1 {
		return errInvalidARNSQSService
	}

	// Check that the queue name has the correct app-env-workload prefix.
	prefix := fmt.Sprintf(fmtCopilotResourceNamePrefix, q.app, q.env, q.wkld)
	if !strings.HasPrefix(q.awsARN.Resource, prefix) {
		return errInvalidQueueARN
	}
	// Check that the queue name has a postfix AFTER that prefix.
	if len(q.awsARN.Resource)-len(prefix) == 0 {
		return errInvalidQueueARN
	}

	q.name = q.awsARN.Resource[len(prefix):]

	return nil
}

// validateAndExtractURL determines the URL for the SQS Queue.
func (q *Queue) validateAndExtractURL() error {
	sqsEndpoint, err := endpoints.DefaultResolver().EndpointFor(endpoints.SqsServiceID, q.awsARN.Region)
	if err != nil {
		return err
	}

	q.url = fmt.Sprintf(SQSQueueURLPattern, sqsEndpoint.URL, q.awsARN.AccountID, q.awsARN.Resource)
	return nil
}
