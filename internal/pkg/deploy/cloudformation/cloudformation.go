// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy CLI concepts with AWS CloudFormation.
package cloudformation

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/templates"
	"github.com/gobuffalo/packd"
)

// StackConfiguration represents the set of methods needed to deploy a cloudformation stack.
type StackConfiguration interface {
	StackName() string
	Template() (string, error)
	Parameters() ([]*sdkcloudformation.Parameter, error)
	Tags() []*sdkcloudformation.Tag
}

type cfnClient interface {
	Create(*cloudformation.Stack) error
	CreateAndWait(*cloudformation.Stack) error
	WaitForCreate(stackName string) error
	Update(*cloudformation.Stack) error
	UpdateAndWait(*cloudformation.Stack) error
	Delete(stackName string) error
	DeleteAndWait(stackName string) error
	Describe(stackName string) (*cloudformation.StackDescription, error)
	Events(stackName string) ([]cloudformation.StackEvent, error)
}

type stackSetClient interface {
	Create(name, template string, opts ...stackset.CreateOrUpdateOption) error
	CreateInstancesAndWait(name string, accounts, regions []string) error
	UpdateAndWait(name, template string, opts ...stackset.CreateOrUpdateOption) error
	Describe(name string) (stackset.Description, error)
	InstanceSummaries(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error)
	Delete(name string) error
}

// CloudFormation wraps the CloudFormationAPI interface
type CloudFormation struct {
	cfnClient      cfnClient
	regionalClient func(region string) cfnClient
	appStackSet    stackSetClient
	box            packd.Box
}

// New returns a configured CloudFormation client.
func New(sess *session.Session) CloudFormation {
	return CloudFormation{
		cfnClient: cloudformation.New(sess),
		regionalClient: func(region string) cfnClient {
			return cloudformation.New(sess.Copy(&aws.Config{
				Region: aws.String(region),
			}))
		},
		appStackSet: stackset.New(sess),
		box:         templates.Box(),
	}
}

// streamResourceEvents sends a list of ResourceEvent every 3 seconds to the events channel.
// The events channel is closed only when the done channel receives a message.
// If an error occurs while describing stack events, it is ignored so that the stream is not interrupted.
func (cf CloudFormation) streamResourceEvents(done <-chan struct{}, events chan []deploy.ResourceEvent, stackName string) {
	sendStatusUpdates := func() {
		// Send a list of ResourceEvent to events if there was no error.
		cfEvents, err := cf.cfnClient.Events(stackName)
		if err != nil {
			return
		}
		var transformedEvents []deploy.ResourceEvent
		for _, cfEvent := range cfEvents {
			transformedEvents = append(transformedEvents, deploy.ResourceEvent{
				Resource: deploy.Resource{
					LogicalName: aws.StringValue(cfEvent.LogicalResourceId),
					Type:        aws.StringValue(cfEvent.ResourceType),
				},
				Status: aws.StringValue(cfEvent.ResourceStatus),
				// CFN error messages end with a '.' and only the first sentence is useful, the rest is error codes.
				StatusReason: strings.Split(aws.StringValue(cfEvent.ResourceStatusReason), ".")[0],
			})
		}
		events <- transformedEvents
	}
	for {
		timeout := time.After(3 * time.Second)
		select {
		case <-timeout:
			sendStatusUpdates()
		case <-done:
			sendStatusUpdates() // Send last batch of updates.
			close(events)       // Close the channel to let receivers know that there won't be any more events.
			return              // Exit for-loop.
		}
	}
}

func toStack(config StackConfiguration) (*cloudformation.Stack, error) {
	template, err := config.Template()
	if err != nil {
		return nil, err
	}
	stack := cloudformation.NewStack(config.StackName(), template)
	stack.Parameters, err = config.Parameters()
	if err != nil {
		return nil, err
	}
	stack.Tags = config.Tags()
	return stack, nil
}

func toMap(tags []*sdkcloudformation.Tag) map[string]string {
	m := make(map[string]string)
	for _, t := range tags {
		m[aws.StringValue(t.Key)] = aws.StringValue(t.Value)
	}
	return m
}
