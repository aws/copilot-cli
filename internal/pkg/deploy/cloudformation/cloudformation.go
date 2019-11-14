// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy archer resources with AWS CloudFormation.
package cloudformation

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/gobuffalo/packd"
)

const (
	projectTagKey = "ecs-project"
	envTagKey     = "ecs-environment"
	appTagKey     = "ecs-application"
)

type stackConfiguration interface {
	StackName() string
	Template() (string, error)
	Parameters() []*cloudformation.Parameter
	Tags() []*cloudformation.Tag
}

// regionalClientProvider lets us make cross region describe calls
// in one CloudFormation struct. We can dynamically generate clients
// configured for a specific region.
type regionalClientProvider interface {
	Client(string) cloudformationiface.CloudFormationAPI
}

type cfClientBuilder struct {
	session *session.Session
}

func (cf cfClientBuilder) Client(region string) cloudformationiface.CloudFormationAPI {
	return cloudformation.New(cf.session, &aws.Config{Region: aws.String(region)})
}

// CloudFormation wraps the CloudFormationAPI interface
type CloudFormation struct {
	regionalClientProvider regionalClientProvider
	client                 cloudformationiface.CloudFormationAPI
	box                    packd.Box
}

// New returns a configured CloudFormation client.
func New(sess *session.Session) CloudFormation {
	cb := cfClientBuilder{
		session: sess,
	}
	return CloudFormation{
		regionalClientProvider: cb,
		client:                 cb.Client(*sess.Config.Region),
		box:                    templates.Box(),
	}
}

// streamResourceEvents sends a list of ResourceEvent every 3 seconds to the events channel.
// The events channel is closed only when the done channel receives a message.
// If an error occurs while describing stack events, it is ignored so that the stream is not interrupted.
func (cf CloudFormation) streamResourceEvents(done <-chan struct{}, events chan []deploy.ResourceEvent, stackName string) {
	sendStatusUpdates := func() {
		// Send a list of ResourceEvent to events if there was no error.
		cfEvents, err := cf.describeStackEvents(stackName)
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

// describeStackEvents gathers all stack resource events in **chronological** order.
// If an error occurs while collecting events, returns a wrapped error.
func (cf CloudFormation) describeStackEvents(stackName string) ([]*cloudformation.StackEvent, error) {
	var nextToken *string
	var events []*cloudformation.StackEvent
	for {
		out, err := cf.client.DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
			NextToken: nextToken,
			StackName: aws.String(stackName),
		})
		if err != nil {
			return nil, fmt.Errorf("desribe stack events for stack %s: %w", stackName, err)
		}
		for _, event := range out.StackEvents {
			events = append(events, event)
		}
		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	// Reverse the events so that they're returned in chronological order.
	// Taken from https://github.com/golang/go/wiki/SliceTricks#reversing.
	for i := len(events)/2 - 1; i >= 0; i-- {
		opp := len(events) - 1 - i
		events[i], events[opp] = events[opp], events[i]
	}
	return events, nil
}

func (cf CloudFormation) waitForStackCreation(stackConfig stackConfiguration) (*cloudformation.Stack, error) {
	describeStackInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackConfig.StackName()),
	}

	if err := cf.client.WaitUntilStackCreateComplete(describeStackInput); err != nil {
		return nil, fmt.Errorf("failed to create stack %s: %w", stackConfig.StackName(), err)
	}

	return cf.describeStack(describeStackInput)
}

func (cf CloudFormation) describeStack(describeStackInput *cloudformation.DescribeStacksInput) (*cloudformation.Stack, error) {
	return cf.describeStackWithClient(describeStackInput, cf.client)
}

// describeStackWithClient let's us use a preconfigured client to make calls to CloudFormation.
// This is useful when we need to make cross-region calls.
func (cf CloudFormation) describeStackWithClient(describeStackInput *cloudformation.DescribeStacksInput,
	client cloudformationiface.CloudFormationAPI) (*cloudformation.Stack, error) {
	describeStackOutput, err := client.DescribeStacks(describeStackInput)
	if err != nil {
		return nil, err
	}

	if len(describeStackOutput.Stacks) == 0 {
		return nil, fmt.Errorf("failed to find a stack named %s", *describeStackInput.StackName)
	}

	return describeStackOutput.Stacks[0], nil
}

func (cf CloudFormation) deploy(stackConfig stackConfiguration) error {
	template, err := stackConfig.Template()
	if err != nil {
		return fmt.Errorf("template creation: %w", err)
	}

	in, err := createChangeSetInput(stackConfig.StackName(), template, withCreateChangeSetType(), withTags(stackConfig.Tags()), withParameters(stackConfig.Parameters()))
	if err != nil {
		return err
	}

	if err := cf.deployChangeSet(in); err != nil {
		if stackExists(err) {
			// Explicitly return a StackAlreadyExists error for the caller to decide if they want to ignore the
			// operation or fail the program.
			return &ErrStackAlreadyExists{
				stackName: stackConfig.StackName(),
				parentErr: err,
			}
		}
		return err
	}
	return nil
}

func (cf CloudFormation) deployChangeSet(in *cloudformation.CreateChangeSetInput) error {
	set, err := cf.createChangeSet(in)
	if err != nil {
		return err
	}
	if err := set.waitForCreation(); err != nil {
		return err
	}
	if err := set.execute(); err != nil {
		return err
	}
	return nil
}

func (cf CloudFormation) createChangeSet(in *cloudformation.CreateChangeSetInput) (*changeSet, error) {
	out, err := cf.client.CreateChangeSet(in)
	if err != nil {
		return nil, fmt.Errorf("failed to create changeSet for stack %s: %w", *in.StackName, err)
	}
	return &changeSet{
		name:    aws.StringValue(out.Id),
		stackID: aws.StringValue(out.StackId),
		c:       cf.client,
	}, nil
}

// stackExists returns true if the underlying error is a stack already exists error.
func stackExists(err error) bool {
	currentErr := err
	for {
		if currentErr == nil {
			break
		}
		if aerr, ok := currentErr.(awserr.Error); ok {
			switch aerr.Code() {
			case "ValidationError":
				// A ValidationError occurs if we tried to create the stack with a change set.
				if strings.Contains(aerr.Message(), "already exists") {
					return true
				}
			case cloudformation.ErrCodeAlreadyExistsException:
				// An AlreadyExists error occurs if we tried to create the stack with the CreateStack API.
				return true
			}
		}
		currentErr = errors.Unwrap(currentErr)
	}
	return false
}

// stackExists returns true if the underlying error is a stack already exists error.
func stackSetExists(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case cloudformation.ErrCodeNameAlreadyExistsException:
			// An ErrCodeNameAlreadyExistsException occurs when a stack set already exists.
			return true
		}
	}
	return false
}
