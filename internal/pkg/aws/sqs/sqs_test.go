// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package sqs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/sqs/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type sqsMocks struct {
	sq *mocks.Mockapi
	rg *mocks.MockresourceGetter
}

func TestSQS_SQSQueues(t *testing.T) {
	const (
		svcName      = "mockSvc"
		envName      = "mockEnv"
		appName      = "mockApp"
		mockQueueArn = "arn:aws:sqs:us-west-2:1234567890:mockApp-mockEnv-mockSvc-queuename"
		mockQueueURL = "https://sqs.us-west-2.amazonaws.com/1234567890/mockApp-mockEnv-mockSvc-queuename"
	)
	mockError := errors.New("some error")
	testTags := map[string]string{
		AppTagKey:     appName,
		EnvTagKey:     envName,
		ServiceTagKey: svcName,
	}

	testCases := map[string]struct {
		setupMocks func(m sqsMocks)

		wantErr       error
		wantQueueAtts []QueueAttributes
	}{
		"errors if failed to search resources": {
			setupMocks: func(m sqsMocks) {
				m.rg.EXPECT().GetResourcesByTags(sqsResourceType, gomock.Eq(testTags)).Return(nil, mockError)
			},

			wantErr: fmt.Errorf("get SQS Queues for environment mockEnv: %w", mockError),
		},
		"errors if failed to get sqs queues because of invalid ARN": {
			setupMocks: func(m sqsMocks) {
				m.rg.EXPECT().GetResourcesByTags(sqsResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{{ARN: "badArn"}}, nil)
			},

			wantErr: fmt.Errorf("parse queue ARN badArn: arn: invalid prefix"),
		},
		"errors if failed to list SQS queue attributes": {
			setupMocks: func(m sqsMocks) {
				gomock.InOrder(
					m.rg.EXPECT().GetResourcesByTags(sqsResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{
						{
							ARN: mockQueueArn,
							Tags: map[string]string{
								AppTagKey:     "mockApp",
								EnvTagKey:     "mockEnv",
								ServiceTagKey: "mockSvc",
							},
						}}, nil),
					m.sq.EXPECT().GetQueueAttributes(&sqs.GetQueueAttributesInput{
						QueueUrl: aws.String(mockQueueURL),
						AttributeNames: []*string{
							aws.String("ApproximateNumberOfMessages"),
							aws.String("ApproximateNumberOfMessagesDelayed"),
							aws.String("ApproximateNumberOfMessagesNotVisible"),
							aws.String("QueueArn"),
							aws.String("RedrivePolicy"),
						},
					}).Return(nil, mockError),
				)
			},

			wantErr: fmt.Errorf("list SQS queue attributes: some error"),
		},
		"return if no queues found": {
			setupMocks: func(m sqsMocks) {
				m.rg.EXPECT().GetResourcesByTags(sqsResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{}, nil)
			},

			wantQueueAtts: nil,
		},
		"success with DLQ": {
			setupMocks: func(m sqsMocks) {
				gomock.InOrder(
					m.rg.EXPECT().GetResourcesByTags(sqsResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{
						{
							ARN: mockQueueArn,
							Tags: map[string]string{
								AppTagKey:     "mockApp",
								EnvTagKey:     "mockEnv",
								ServiceTagKey: "mockSvc",
							},
						}}, nil),
					m.sq.EXPECT().GetQueueAttributes(&sqs.GetQueueAttributesInput{
						QueueUrl: aws.String(mockQueueURL),
						AttributeNames: []*string{
							aws.String("ApproximateNumberOfMessages"),
							aws.String("ApproximateNumberOfMessagesDelayed"),
							aws.String("ApproximateNumberOfMessagesNotVisible"),
							aws.String("QueueArn"),
							aws.String("RedrivePolicy"),
						},
					}).Return(&sqs.GetQueueAttributesOutput{
						Attributes: map[string]*string{
							"ApproximateNumberOfMessages":           aws.String("30"),
							"ApproximateNumberOfMessagesDelayed":    aws.String("5"),
							"ApproximateNumberOfMessagesNotVisible": aws.String("3"),
							"QueueArn":                              aws.String(mockQueueArn),
							"RedrivePolicy":                         aws.String("{\"deadLetterTargetArn\":\"arn:aws:sqs:us-west-2:1234567890:mockApp-mockEnv-mockSvc-DeadLetterQueue\",\"maxReceiveCount\":10}"),
						},
					}, nil),
				)
			},

			wantQueueAtts: []QueueAttributes{
				{
					Name:                                  "queuename",
					ApproximateNumberOfMessages:           30,
					ApproximateNumberOfMessagesDelayed:    5,
					ApproximateNumberOfMessagesNotVisible: 3,
					ARN:                                   mockQueueArn,
					RedrivePolicy: &RedrivePolicy{
						DeadLetterTargetArn: "arn:aws:sqs:us-west-2:1234567890:mockApp-mockEnv-mockSvc-DeadLetterQueue",
						MaxReceiveCount:     10,
					},
				},
			},
		},
		"success without DLQ": {
			setupMocks: func(m sqsMocks) {
				gomock.InOrder(
					m.rg.EXPECT().GetResourcesByTags(sqsResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{
						{
							ARN: mockQueueArn,
							Tags: map[string]string{
								AppTagKey:     "mockApp",
								EnvTagKey:     "mockEnv",
								ServiceTagKey: "mockSvc",
							},
						}}, nil),
					m.sq.EXPECT().GetQueueAttributes(&sqs.GetQueueAttributesInput{
						QueueUrl: aws.String(mockQueueURL),
						AttributeNames: []*string{
							aws.String("ApproximateNumberOfMessages"),
							aws.String("ApproximateNumberOfMessagesDelayed"),
							aws.String("ApproximateNumberOfMessagesNotVisible"),
							aws.String("QueueArn"),
							aws.String("RedrivePolicy"),
						},
					}).Return(&sqs.GetQueueAttributesOutput{
						Attributes: map[string]*string{
							"ApproximateNumberOfMessages":           aws.String("30"),
							"ApproximateNumberOfMessagesDelayed":    aws.String("5"),
							"ApproximateNumberOfMessagesNotVisible": aws.String("3"),
							"QueueArn":                              aws.String(mockQueueArn),
							"RedrivePolicy":                         aws.String(""),
						},
					}, nil),
				)
			},

			wantQueueAtts: []QueueAttributes{
				{
					Name:                                  "queuename",
					ApproximateNumberOfMessages:           30,
					ApproximateNumberOfMessagesDelayed:    5,
					ApproximateNumberOfMessagesNotVisible: 3,
					ARN:                                   mockQueueArn,
					RedrivePolicy:                         nil,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSQSClient := mocks.NewMockapi(ctrl)
			mockrgClient := mocks.NewMockresourceGetter(ctrl)
			mocks := sqsMocks{
				sq: mockSQSClient,
				rg: mockrgClient,
			}

			tc.setupMocks(mocks)

			sqsSvc := SQS{
				client:   mockSQSClient,
				rgClient: mockrgClient,
			}

			gotQueueAtt, gotErr := sqsSvc.SQSQueues(appName, envName, svcName)

			if gotErr != nil {
				require.EqualError(t, tc.wantErr, gotErr.Error())
			} else {
				require.Equal(t, tc.wantQueueAtts, gotQueueAtt)
			}
		})

	}
}
