// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy CLI concepts with AWS CloudFormation.
package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/templates"
	"github.com/gobuffalo/packd"
	"golang.org/x/sync/errgroup"
)

// StackConfiguration represents the set of methods needed to deploy a cloudformation stack.
type StackConfiguration interface {
	StackName() string
	Template() (string, error)
	Parameters() ([]*sdkcloudformation.Parameter, error)
	Tags() []*sdkcloudformation.Tag
}

type cfnClient interface {
	// Methods augmented by the aws wrapper struct.
	Create(*cloudformation.Stack) (string, error)
	CreateAndWait(*cloudformation.Stack) error
	WaitForCreate(ctx context.Context, stackName string) error
	Update(*cloudformation.Stack) error
	UpdateAndWait(*cloudformation.Stack) error
	WaitForUpdate(ctx context.Context, stackName string) error
	Delete(stackName string) error
	DeleteAndWait(stackName string) error
	DeleteAndWaitWithRoleARN(stackName, roleARN string) error
	Describe(stackName string) (*cloudformation.StackDescription, error)
	DescribeChangeSet(changeSetID, stackName string) (*cloudformation.ChangeSetDescription, error)
	TemplateBody(stackName string) (string, error)
	Events(stackName string) ([]cloudformation.StackEvent, error)
	ListStacksWithTags(tags map[string]string) ([]cloudformation.StackDescription, error)
	ErrorEvents(stackName string) ([]cloudformation.StackEvent, error)

	// Methods vended by the aws sdk struct.
	DescribeStackEvents(*sdkcloudformation.DescribeStackEventsInput) (*sdkcloudformation.DescribeStackEventsOutput, error)
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

// errorEvents returns the list of CloudFormation Resource Events, filtered by failures and errors.
func (cf CloudFormation) errorEvents(conf StackConfiguration) ([]deploy.ResourceEvent, error) {
	events, err := cf.cfnClient.ErrorEvents(conf.StackName())
	if err != nil {
		return nil, err
	}
	var transformedEvents []deploy.ResourceEvent
	for _, cfEvent := range events {
		transformedEvents = append(transformedEvents, transformEvent(cfEvent))
	}
	return transformedEvents, nil
}

type renderStackChangesInput struct {
	w                progress.FileWriter
	stackName        string
	stackDescription string
	createChangeSet  func() (string, error)
	waitForStack     func(context.Context, string) error
}

func (cf CloudFormation) renderStackChanges(in renderStackChangesInput) error {
	changeSetID, err := in.createChangeSet()
	if err != nil {
		return err
	}
	changeSet, err := cf.cfnClient.DescribeChangeSet(changeSetID, in.stackName)
	if err != nil {
		return err
	}
	body, err := cf.cfnClient.TemplateBody(in.stackName)
	if err != nil {
		return err
	}
	descriptions, err := cloudformation.ParseTemplateDescriptions(body)
	if err != nil {
		return fmt.Errorf("parse cloudformation template for resource descriptions: %w", err)
	}

	waitCtx, cancelWait := context.WithCancel(context.Background())
	streamer := stream.NewStackStreamer(cf.cfnClient, in.stackName, changeSet.CreationTime)
	renderer := progress.ListeningStackRenderer(streamer, in.stackName, in.stackDescription, resourcesToRender(changeSet.Changes, descriptions))

	// Run the streamer, renderer, and waiter all concurrently until they all exit successfully or one of them errors.
	// When the waiter exits, the waitCtx is canceled which results in the streamer and renderer to exit.
	// When the streamer exits, the group ctx is canceled resulting in the waiter to exit and canceling the renderer.
	// When the renderer exits, it's the same flow as the streamer.
	g, ctx := errgroup.WithContext(waitCtx)
	g.Go(func() error {
		defer cancelWait()
		return in.waitForStack(ctx, in.stackName)
	})
	g.Go(func() error {
		return stream.Stream(waitCtx, streamer)
	})
	g.Go(func() error {
		return progress.Render(waitCtx, progress.NewTabbedFileWriter(in.w), renderer)
	})
	return g.Wait()
}

func transformEvent(input cloudformation.StackEvent) deploy.ResourceEvent {
	return deploy.ResourceEvent{
		Resource: deploy.Resource{
			LogicalName: aws.StringValue(input.LogicalResourceId),
			Type:        aws.StringValue(input.ResourceType),
		},
		Status: aws.StringValue(input.ResourceStatus),
		// CFN error messages end with a '. (Service' and only the first sentence is useful, the rest is error codes.
		StatusReason: strings.Split(aws.StringValue(input.ResourceStatusReason), ". (Service")[0],
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

// resourcesToRender filters changes by resources that have a description.
func resourcesToRender(changes []*sdkcloudformation.Change, descriptions map[string]string) []progress.StackResourceDescription {
	var resources []progress.StackResourceDescription
	for _, change := range changes {
		logicalID := aws.StringValue(change.ResourceChange.LogicalResourceId)
		description, ok := descriptions[logicalID]
		if !ok {
			continue
		}
		resources = append(resources, progress.StackResourceDescription{
			LogicalResourceID: logicalID,
			ResourceType:      aws.StringValue(change.ResourceChange.PhysicalResourceId),
			Description:       description,
		})
	}
	return resources
}

func stopSpinner(spinner *progress.Spinner, err error, label string) {
	if err == nil {
		spinner.Stop(log.Ssuccessf("%s\n", label))
		return
	}
	var existsErr *cloudformation.ErrStackAlreadyExists
	if errors.As(err, &existsErr) {
		spinner.Stop(log.Ssuccessf("%s\n", label))
		return
	}
	spinner.Stop(log.Serrorf("%s\n", label))
}
