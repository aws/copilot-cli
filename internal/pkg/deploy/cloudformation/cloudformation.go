// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy CLI concepts with AWS CloudFormation.
package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"

	"github.com/aws/copilot-cli/internal/pkg/aws/codestar"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/templates"
	"github.com/gobuffalo/packd"
	"golang.org/x/sync/errgroup"
)

const (
	// waitForStackTimeout is how long we're willing to wait for a stack to go from in progress to a complete state.
	waitForStackTimeout = 1*time.Hour + 30*time.Minute

	// CloudFormation resource types.
	ecsServiceResourceType    = "AWS::ECS::Service"
	envControllerResourceType = "Custom::EnvControllerFunction"
)

// StackConfiguration represents the set of methods needed to deploy a cloudformation stack.
type StackConfiguration interface {
	StackName() string
	Template() (string, error)
	Parameters() ([]*sdkcloudformation.Parameter, error)
	Tags() []*sdkcloudformation.Tag
}

type ecsClient interface {
	stream.ECSServiceDescriber
}

type cfnClient interface {
	// Methods augmented by the aws wrapper struct.
	Create(*cloudformation.Stack) (string, error)
	CreateAndWait(*cloudformation.Stack) error
	WaitForCreate(ctx context.Context, stackName string) error
	Update(*cloudformation.Stack) (string, error)
	UpdateAndWait(*cloudformation.Stack) error
	WaitForUpdate(ctx context.Context, stackName string) error
	Delete(stackName string) error
	DeleteAndWait(stackName string) error
	DeleteAndWaitWithRoleARN(stackName, roleARN string) error
	Describe(stackName string) (*cloudformation.StackDescription, error)
	DescribeChangeSet(changeSetID, stackName string) (*cloudformation.ChangeSetDescription, error)
	TemplateBody(stackName string) (string, error)
	TemplateBodyFromChangeSet(changeSetID, stackName string) (string, error)
	Events(stackName string) ([]cloudformation.StackEvent, error)
	ListStacksWithTags(tags map[string]string) ([]cloudformation.StackDescription, error)
	ErrorEvents(stackName string) ([]cloudformation.StackEvent, error)
	Outputs(stack *cloudformation.Stack) (map[string]string, error)

	// Methods vended by the aws sdk struct.
	DescribeStackEvents(*sdkcloudformation.DescribeStackEventsInput) (*sdkcloudformation.DescribeStackEventsOutput, error)
}

type codeStarClient interface {
	WaitUntilConnectionStatusAvailable(ctx context.Context, connectionARN string) error
}

type codePipelineClient interface {
	RetryStageExecution(pipelineName, stageName string) error
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
	codeStarClient codeStarClient
	cpClient       codePipelineClient
	ecsClient      ecsClient
	regionalClient func(region string) cfnClient
	appStackSet    stackSetClient
	box            packd.Box
}

// New returns a configured CloudFormation client.
func New(sess *session.Session) CloudFormation {
	client := CloudFormation{
		cfnClient:      cloudformation.New(sess),
		codeStarClient: codestar.New(sess),
		cpClient:       codepipeline.New(sess),
		ecsClient:      ecs.New(sess),
		regionalClient: func(region string) cfnClient {
			return cloudformation.New(sess.Copy(&aws.Config{
				Region: aws.String(region),
			}))
		},
		appStackSet: stackset.New(sess),
		box:         templates.Box(),
	}
	return client
}

// errorEvents returns the list of status reasons of failed resource events
func (cf CloudFormation) errorEvents(stackName string) ([]string, error) {
	events, err := cf.cfnClient.ErrorEvents(stackName)
	if err != nil {
		return nil, err
	}
	var reasons []string
	for _, event := range events {
		// CFN error messages end with a '. (Service' and only the first sentence is useful, the rest is error codes.
		reasons = append(reasons, strings.Split(aws.StringValue(event.ResourceStatusReason), ". (Service")[0])
	}
	return reasons, nil
}

type renderStackChangesInput struct {
	w                progress.FileWriter
	stackName        string
	stackDescription string
	createChangeSet  func() (string, error)
}

func (cf CloudFormation) newRenderWorkloadInput(w progress.FileWriter, stack *cloudformation.Stack) *renderStackChangesInput {
	in := &renderStackChangesInput{
		w:                w,
		stackName:        stack.Name,
		stackDescription: fmt.Sprintf("Creating the infrastructure for stack %s", stack.Name),
	}
	in.createChangeSet = func() (changeSetID string, err error) {
		spinner := progress.NewSpinner(w)
		label := fmt.Sprintf("Proposing infrastructure changes for stack %s", stack.Name)
		spinner.Start(label)
		changeSetID, err = cf.cfnClient.Create(stack)
		if err == nil {
			// Successfully created the change set to create the stack.
			spinner.Stop(log.Ssuccessf("%s\n", label))
			return changeSetID, nil
		}

		var errAlreadyExists *cloudformation.ErrStackAlreadyExists
		if !errors.As(err, &errAlreadyExists) {
			// Unexpected error trying to create a stack.
			spinner.Stop(log.Serrorf("%s\n", label))
			return "", cf.handleStackError(stack.Name, err)
		}

		// We have to create an update stack change set instead.
		in.stackDescription = fmt.Sprintf("Updating the infrastructure for stack %s", stack.Name)
		changeSetID, err = cf.cfnClient.Update(stack)
		if err != nil {
			msg := log.Serrorf("%s\n", label)
			var errChangeSetEmpty *cloudformation.ErrChangeSetEmpty
			if errors.As(err, &errChangeSetEmpty) {
				msg = fmt.Sprintf("- No new infrastructure changes for stack %s\n", stack.Name)
			}
			spinner.Stop(msg)
			return "", cf.handleStackError(stack.Name, err)
		}
		spinner.Stop(log.Ssuccessf("%s\n", label))
		return changeSetID, nil
	}
	return in
}

func (cf CloudFormation) renderStackChanges(in *renderStackChangesInput) error {
	changeSetID, err := in.createChangeSet()
	if err != nil {
		return err
	}
	waitCtx, cancelWait := context.WithTimeout(context.Background(), waitForStackTimeout)
	defer cancelWait()
	g, ctx := errgroup.WithContext(waitCtx)

	renderer, err := cf.createChangeSetRenderer(g, ctx, changeSetID, in.stackName, in.stackDescription, progress.RenderOptions{})
	if err != nil {
		return err
	}
	g.Go(func() error {
		return progress.Render(ctx, progress.NewTabbedFileWriter(in.w), renderer)
	})
	if err := g.Wait(); err != nil {
		return err
	}
	if err := cf.errOnFailedStack(in.stackName); err != nil {
		return err
	}
	return nil
}

func (cf CloudFormation) createChangeSetRenderer(group *errgroup.Group, ctx context.Context, changeSetID, stackName, description string, opts progress.RenderOptions) (progress.DynamicRenderer, error) {
	changeSet, err := cf.cfnClient.DescribeChangeSet(changeSetID, stackName)
	if err != nil {
		return nil, err
	}
	body, err := cf.cfnClient.TemplateBodyFromChangeSet(changeSetID, stackName)
	if err != nil {
		return nil, err
	}
	descriptions, err := cloudformation.ParseTemplateDescriptions(body)
	if err != nil {
		return nil, fmt.Errorf("parse cloudformation template for resource descriptions: %w", err)
	}

	streamer := stream.NewStackStreamer(cf.cfnClient, stackName, changeSet.CreationTime)
	children, err := cf.changeRenderers(changeRenderersInput{
		g:                  group,
		ctx:                ctx,
		stackName:          stackName,
		stackStreamer:      streamer,
		changes:            changeSet.Changes,
		changeSetTimestamp: changeSet.CreationTime,
		descriptions:       descriptions,
		opts:               progress.NestedRenderOptions(opts),
	})
	if err != nil {
		return nil, err
	}
	renderer := progress.ListeningChangeSetRenderer(streamer, stackName, description, children, opts)
	group.Go(func() error {
		return stream.Stream(ctx, streamer)
	})
	return renderer, nil
}

type changeRenderersInput struct {
	g                  *errgroup.Group             // Group that all goroutines belong.
	ctx                context.Context             // Context associated with the group.
	stackName          string                      // Name of the stack.
	stackStreamer      progress.StackSubscriber    // Streamer for the stack where changes belong.
	changes            []*sdkcloudformation.Change // List of changes that will be applied to the stack.
	changeSetTimestamp time.Time                   // ChangeSet creation time.
	descriptions       map[string]string           // Descriptions for the logical IDs of the changes.
	opts               progress.RenderOptions      // Display options that should be applied to the changes.
}

// changeRenderers filters changes by resources that have a description and returns the appropriate progress.Renderer for each resource type.
func (cf CloudFormation) changeRenderers(in changeRenderersInput) ([]progress.Renderer, error) {
	var resources []progress.Renderer
	for _, change := range in.changes {
		logicalID := aws.StringValue(change.ResourceChange.LogicalResourceId)
		description, ok := in.descriptions[logicalID]
		if !ok {
			continue
		}
		var renderer progress.Renderer
		switch {
		case aws.StringValue(change.ResourceChange.ResourceType) == envControllerResourceType:
			r, err := cf.createEnvControllerRenderer(&envControllerRendererInput{
				g:                 in.g,
				ctx:               in.ctx,
				workloadStackName: in.stackName,
				workloadTimestamp: in.changeSetTimestamp,
				change:            change,
				description:       description,
				serviceStack:      in.stackStreamer,
				renderOpts:        in.opts,
			})
			if err != nil {
				return nil, err
			}
			renderer = r
		case aws.StringValue(change.ResourceChange.ResourceType) == ecsServiceResourceType:
			renderer = progress.ListeningECSServiceResourceRenderer(in.stackStreamer, cf.ecsClient, logicalID, description, progress.ECSServiceRendererOpts{
				Group:      in.g,
				Ctx:        in.ctx,
				RenderOpts: in.opts,
			})
		case change.ResourceChange.ChangeSetId != nil:
			// The resource change is a nested stack.
			changeSetID := aws.StringValue(change.ResourceChange.ChangeSetId)
			stackName := parseStackNameFromARN(aws.StringValue(change.ResourceChange.PhysicalResourceId))

			r, err := cf.createChangeSetRenderer(in.g, in.ctx, changeSetID, stackName, description, in.opts)
			if err != nil {
				return nil, err
			}
			renderer = r
		default:
			renderer = progress.ListeningResourceRenderer(in.stackStreamer, logicalID, description, progress.ResourceRendererOpts{
				RenderOpts: in.opts,
			})
		}
		resources = append(resources, renderer)
	}
	return resources, nil
}

type envControllerRendererInput struct {
	g                 *errgroup.Group
	ctx               context.Context
	workloadStackName string
	workloadTimestamp time.Time
	change            *sdkcloudformation.Change
	description       string
	serviceStack      progress.StackSubscriber
	renderOpts        progress.RenderOptions
}

func (cf CloudFormation) createEnvControllerRenderer(in *envControllerRendererInput) (progress.DynamicRenderer, error) {
	workload, err := cf.cfnClient.Describe(in.workloadStackName)
	if err != nil {
		return nil, err
	}
	envStackName := fmt.Sprintf("%s-%s", parseAppNameFromTags(workload.Tags), parseEnvNameFromTags(workload.Tags))
	body, err := cf.cfnClient.TemplateBody(envStackName)
	if err != nil {
		return nil, err
	}
	envResourceDescriptions, err := cloudformation.ParseTemplateDescriptions(body)
	if err != nil {
		return nil, fmt.Errorf("parse cloudformation template for resource descriptions: %w", err)
	}
	envStreamer := stream.NewStackStreamer(cf.cfnClient, envStackName, in.workloadTimestamp)
	ctx, cancel := context.WithCancel(in.ctx)
	in.g.Go(func() error {
		if err := stream.Stream(ctx, envStreamer); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		return nil
	})
	return progress.ListeningEnvControllerRenderer(progress.EnvControllerConfig{
		Description:     in.description,
		RenderOpts:      in.renderOpts,
		ActionStreamer:  in.serviceStack,
		ActionLogicalID: aws.StringValue(in.change.ResourceChange.LogicalResourceId),
		EnvStreamer:     envStreamer,
		CancelEnvStream: cancel,
		EnvStackName:    envStackName,
		EnvResources:    envResourceDescriptions,
	}), nil
}

func (cf CloudFormation) errOnFailedStack(stackName string) error {
	stack, err := cf.cfnClient.Describe(stackName)
	if err != nil {
		return err
	}
	status := aws.StringValue(stack.StackStatus)
	if cloudformation.StackStatus(status).Failure() {
		return fmt.Errorf("stack %s did not complete successfully and exited with status %s", stackName, status)
	}
	return nil
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

// parseStackNameFromARN retrieves "my-nested-stack" from an input like:
// arn:aws:cloudformation:us-west-2:123456789012:stack/my-nested-stack/d0a825a0-e4cd-xmpl-b9fb-061c69e99205
func parseStackNameFromARN(stackARN string) string {
	return strings.Split(stackARN, "/")[1]
}

func parseAppNameFromTags(tags []*sdkcloudformation.Tag) string {
	for _, t := range tags {
		if aws.StringValue(t.Key) == deploy.AppTagKey {
			return aws.StringValue(t.Value)
		}
	}
	return ""
}

func parseEnvNameFromTags(tags []*sdkcloudformation.Tag) string {
	for _, t := range tags {
		if aws.StringValue(t.Key) == deploy.EnvTagKey {
			return aws.StringValue(t.Value)
		}
	}
	return ""
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
