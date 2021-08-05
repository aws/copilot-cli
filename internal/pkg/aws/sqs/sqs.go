// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package sqs provides a client to make API requests to Amazon SQS Service.
package sqs

import (
	"encoding/json"
	"fmt"
	"strconv"

	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

const (
	sqsResourceType = "sqs"
)

const (
	// AppTagKey is tag key for Copilot app.
	AppTagKey = "copilot-application"
	// EnvTagKey is tag key for Copilot env.
	EnvTagKey = "copilot-environment"
	// ServiceTagKey is tag key for Copilot svc.
	ServiceTagKey = "copilot-service"
)

type api interface {
	GetQueueAttributes(input *sqs.GetQueueAttributesInput) (*sqs.GetQueueAttributesOutput, error)
}

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*rg.Resource, error)
}

// SQS wraps an Amazon SQS client.
type SQS struct {
	client   api
	rgClient resourceGetter
}

// QueueAttributes contains a Cloudwatch queue attributes.
type QueueAttributes struct {
	Name                                  string         `json:"name"`
	ARN                                   string         `json:"arn"`
	ApproximateNumberOfMessages           int            `json:"messages"`
	ApproximateNumberOfMessagesDelayed    int            `json:"messagesDelayed"`
	ApproximateNumberOfMessagesNotVisible int            `json:"messagesNotVisible"`
	RedrivePolicy                         *RedrivePolicy `json:"redrivePolicy"`
}

// RedrivePolicy contains the fields needed to unmarshal a SQS Queue redrive policy.
type RedrivePolicy struct {
	DeadLetterTargetArn string `json:"deadLetterTargetArn"`
	MaxReceiveCount     int    `json:"maxReceiveCount"`
}

// New returns a SQS struct configured against the input session.
func New(s *session.Session) *SQS {
	return &SQS{
		client:   sqs.New(s),
		rgClient: rg.New(s),
	}
}

// QueueAttributes returns relevant queue attributes, such as messages and redrive policy
func (sq *SQS) QueueAttributes(queues []*Queue) ([]QueueAttributes, error) {
	if len(queues) == 0 {
		return nil, nil
	}

	var queueAtts []QueueAttributes
	for _, queue := range queues {
		attOut, err := sq.client.GetQueueAttributes(
			&sqs.GetQueueAttributesInput{
				QueueUrl: &queue.url,
				AttributeNames: []*string{
					aws.String("ApproximateNumberOfMessages"),
					aws.String("ApproximateNumberOfMessagesDelayed"),
					aws.String("ApproximateNumberOfMessagesNotVisible"),
					aws.String("QueueArn"),
					aws.String("RedrivePolicy"),
				},
			})
		if err != nil {
			return nil, fmt.Errorf("list SQS queue attributes: %w", err)
		}
		redrive, err := parseDLQPolicy(aws.StringValue(attOut.Attributes["RedrivePolicy"]))
		if err != nil {
			return nil, fmt.Errorf("list SQS queue redrive policy: %w", err)
		}
		messages, err := strconv.Atoi(aws.StringValue(attOut.Attributes["ApproximateNumberOfMessages"]))
		if err != nil {
			return nil, err
		}
		messagesDelayed, err := strconv.Atoi(aws.StringValue(attOut.Attributes["ApproximateNumberOfMessagesDelayed"]))
		if err != nil {
			return nil, err
		}
		messagesNotVisible, err := strconv.Atoi(aws.StringValue(attOut.Attributes["ApproximateNumberOfMessagesNotVisible"]))
		if err != nil {
			return nil, err
		}

		queueAtts = append(queueAtts, QueueAttributes{
			Name:                                  queue.name,
			ARN:                                   aws.StringValue(attOut.Attributes["QueueArn"]),
			RedrivePolicy:                         redrive,
			ApproximateNumberOfMessages:           messages,
			ApproximateNumberOfMessagesDelayed:    messagesDelayed,
			ApproximateNumberOfMessagesNotVisible: messagesNotVisible,
		})

	}

	return queueAtts, nil
}

// SQSQueues lists out SQS queues deployed by copilot for a given service.
func (sq *SQS) SQSQueues(appName, envName, svcName string) ([]QueueAttributes, error) {
	queueResources, err := sq.rgClient.GetResourcesByTags(sqsResourceType, map[string]string{
		AppTagKey:     appName,
		EnvTagKey:     envName,
		ServiceTagKey: svcName,
	})
	if err != nil {
		return nil, fmt.Errorf("get SQS Queues for environment %s: %w", envName, err)
	}

	var sqsQueues []*Queue
	for _, q := range queueResources {
		// TODO: if we add env-level SQS queues, remove this check.
		// If the queue doesn't have a specific workload tag, don't return it.
		if _, ok := q.Tags[ServiceTagKey]; !ok {
			continue
		}
		queue, err := NewQueue(q.ARN, appName, envName, q.Tags[ServiceTagKey])
		// If there's an error parsing the queue, don't include it in the list of queues
		if err != nil {
			continue
		}
		sqsQueues = append(sqsQueues, queue)
	}

	return sq.QueueAttributes(sqsQueues)
}

// parseDLQPolicy parses the DLQ policy for the ARN of the DLQ
func parseDLQPolicy(policy string) (*RedrivePolicy, error) {
	if policy == "" {
		return nil, nil
	}

	var redrivePolicy RedrivePolicy
	err := json.Unmarshal([]byte(policy), &redrivePolicy)
	if err != nil {
		return nil, err
	}

	return &redrivePolicy, nil
}
