// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
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
// resource events from a stack until the streamer stops.
//
// The component only listens for stack resource events for the provided changes in the stack.
// The state of changes is updated as events are published from the streamer.
func ListeningStackRenderer(streamer StackSubscriber, stackName, description string, changes []StackResourceDescription) Renderer {
	var children []Renderer
	for _, change := range changes {
		children = append(children, listeningResourceRenderer(streamer, change, nestedComponentPadding))
	}
	comp := &stackComponent{
		logicalID:   stackName,
		description: description,
		children:    children,
		stream:      make(chan stream.StackEvent),
	}
	streamer.Subscribe(comp.stream)
	go comp.Listen()
	return comp
}

// listeningResourceRenderer returns a component that listens for CloudFormation stack events for a particular resource.
func listeningResourceRenderer(streamer StackSubscriber, resource StackResourceDescription, padding int) Renderer {
	comp := &regularResourceComponent{
		logicalID:   resource.LogicalResourceID,
		description: resource.Description,
		stream:      make(chan stream.StackEvent),
		padding:     padding,
	}
	streamer.Subscribe(comp.stream)
	go comp.Listen()
	return comp
}

// stackComponent can display a CloudFormation stack and all of its associated resources.
type stackComponent struct {
	logicalID   string     // The CloudFormation stack name.
	description string     // The human friendly explanation of the purpose of the stack.
	status      string     // The CloudFormation status of the stack.
	children    []Renderer // Resources part of the stack.
	padding     int        // Leading spaces before rendering the stack.

	stream chan stream.StackEvent
	mu     sync.Mutex
}

// Listen updates the stack's status if a CloudFormation stack event is received.
func (c *stackComponent) Listen() {
	for ev := range c.stream {
		updateComponentStatus(&c.mu, &c.status, c.logicalID, ev)
	}
}

// Render prints the CloudFormation stack's resource components in order and returns the number of lines written.
// If any sub-component's Render call fails, then writes nothing and returns an error.
func (c *stackComponent) Render(out io.Writer) (numLines int, err error) {
	r := new(allOrNothingRenderer)
	c.mu.Lock()
	r.Partial(&singleLineComponent{
		Text:    fmt.Sprintf("- %s\t[%s]", c.description, c.status),
		Padding: c.padding,
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
	padding     int    // Leading spaces before rendering the resource.

	stream chan stream.StackEvent
	mu     sync.Mutex
}

// Listen updates the resource's status if a CloudFormation stack resource event is received.
func (c *regularResourceComponent) Listen() {
	for ev := range c.stream {
		updateComponentStatus(&c.mu, &c.status, c.logicalID, ev)
	}
}

// Render prints the resource as a singleLineComponent and returns the number of lines written and the error if any.
func (c *regularResourceComponent) Render(out io.Writer) (numLines int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	slc := &singleLineComponent{
		Text:    fmt.Sprintf("- %s\t[%s]", c.description, c.status),
		Padding: c.padding,
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
