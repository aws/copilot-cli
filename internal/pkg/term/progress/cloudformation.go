// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/aws/copilot-cli/internal/pkg/stream"
)

// StackSubscriber is the interface to subscribe channels to a CloudFormation stack stream event.
type StackSubscriber interface {
	Subscribe(channels ...chan stream.StackEvent)
}

// StackResourceDescription identifies a CloudFormation stack resource annotated with a human-friendly description.
type StackResourceDescription struct {
	LogicalResourceID string
	ResourceType      string
	Description       string
}

// ListeningStackRenderer returns a component that listens for CloudFormation
// resource events from a stack until the ctx is canceled.
//
// The component only listens for stack resource events for the provided changes in the stack.
// The state of changes is updated as events are published from the streamer.
func ListeningStackRenderer(ctx context.Context, stackName, description string, changes []StackResourceDescription, streamer StackSubscriber) Renderer {
	var children []Renderer
	for _, change := range changes {
		children = append(children, listeningResourceRenderer(ctx, change, streamer))
	}
	comp := &stackComponent{
		logicalID:   stackName,
		description: description,
		children:    children,
		stream:      make(chan stream.StackEvent),
	}
	streamer.Subscribe(comp.stream)
	go comp.Listen(ctx)
	return comp
}

// listeningResourceRenderer returns a component that listens for CloudFormation stack events for a particular resource.
func listeningResourceRenderer(ctx context.Context, resource StackResourceDescription, streamer StackSubscriber) Renderer {
	comp := &regularResourceComponent{
		logicalID:   resource.LogicalResourceID,
		description: resource.Description,
		stream:      make(chan stream.StackEvent),
	}
	streamer.Subscribe(comp.stream)
	go comp.Listen(ctx)
	return comp
}

// stackComponent can display a CloudFormation stack and all of its associated resources.
type stackComponent struct {
	logicalID   string     // The CloudFormation stack name.
	description string     // The human friendly explanation of the purpose of the stack.
	status      string     // The CloudFormation status of the stack.
	children    []Renderer // Resources part of the stack.

	stream chan stream.StackEvent
	mu     sync.Mutex
}

// Listen updates the stack's status if a CloudFormation stack event is received or until ctx is canceled.
func (c *stackComponent) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// TODO(efekarakus): The streamer should close(c.stream) on ctx.Done().
			// So that we can loop through remaining events `for ev := range c.stream`
			// and make sure that the latest status for the logical ID is applied.
			return
		case ev := <-c.stream:
			updateComponentStatus(&c.mu, &c.status, c.logicalID, ev)
		}
	}
}

// Render prints the CloudFormation stack's resource components in order and returns the number of lines written.
// If any sub-component's Render call fails, then writes nothing and returns an error.
func (c *stackComponent) Render(out io.Writer) (numLines int, err error) {
	r := new(allOrNothingRenderer)
	c.mu.Lock()
	r.Partial(&singleLineComponent{
		Text: fmt.Sprintf("- %s [%s]", c.description, c.status),
	})
	c.mu.Unlock()
	for _, child := range c.children {
		r.Partial(child)
	}
	return r.Render(out)
}

// regularResourceComponent can display a simple CloudFormation stack resource event.
type regularResourceComponent struct {
	logicalID   string // The LogicalID defined in the template for the resource.
	status      string // The CloudFormation status of the resource.
	description string // The human friendly explanation of the resource.

	stream chan stream.StackEvent
	mu     sync.Mutex
}

// Listen updates the resource's status if a CloudFormation stack resource event is received or until ctx is canceled.
func (c *regularResourceComponent) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-c.stream:
			updateComponentStatus(&c.mu, &c.status, c.logicalID, ev)
		}
	}
}

// Render prints the resource as a singleLineComponent and returns the number of lines written and the error if any.
func (c *regularResourceComponent) Render(out io.Writer) (numLines int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	slc := &singleLineComponent{
		Text: fmt.Sprintf("- %s [%s]", c.description, c.status),
	}
	return slc.Render(out)
}

func updateComponentStatus(mu *sync.Mutex, status *string, logicalID string, event stream.StackEvent) {
	if logicalID != event.LogicalResourceID {
		return
	}
	mu.Lock()
	*status = event.ResourceStatus
	mu.Unlock()
}
