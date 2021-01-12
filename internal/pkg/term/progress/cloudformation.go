// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/stream"
)

// StackSubscriber is the interface to subscribe channels to a CloudFormation stack stream event.
type StackSubscriber interface {
	Subscribe(channels ...chan stream.StackEvent)
}

// ListeningStackRenderer returns a tree component that listens for CloudFormation
// resource events from a stack mutated with a changeSet until the streamer stops.
func ListeningStackRenderer(streamer StackSubscriber, stackName, description string, changes []Renderer, opts RenderOptions) Renderer {
	return &treeComponent{
		Root:     ListeningResourceRenderer(streamer, stackName, description, opts),
		Children: changes,
	}
}

// ListeningResourceRenderer returns a tab-separated component that listens for
// CloudFormation stack events for a particular resource.
func ListeningResourceRenderer(streamer StackSubscriber, logicalID, description string, opts RenderOptions) Renderer {
	comp := &regularResourceComponent{
		logicalID:   logicalID,
		description: description,
		statuses:    []stackStatus{notStartedStackStatus},
		stopWatch:   newStopWatch(),
		stream:      make(chan stream.StackEvent),
		padding:     opts.Padding,
		separator:   '\t',
	}
	streamer.Subscribe(comp.stream)
	go comp.Listen()
	return comp
}

// regularResourceComponent can display a simple CloudFormation stack resource event.
type regularResourceComponent struct {
	logicalID   string        // The LogicalID defined in the template for the resource.
	description string        // The human friendly explanation of the resource.
	statuses    []stackStatus // In-order history of the CloudFormation status of the resource throughout the deployment.
	stopWatch   *stopWatch    // Timer to measure how long the operation takes to complete.

	padding   int  // Leading spaces before rendering the resource.
	separator rune // Character used to separate columns of text.

	stream chan stream.StackEvent
	mu     sync.Mutex
}

// Listen updates the resource's status if a CloudFormation stack resource event is received.
func (c *regularResourceComponent) Listen() {
	for ev := range c.stream {
		if c.logicalID != ev.LogicalResourceID {
			continue
		}
		updateComponentStatus(&c.mu, &c.statuses, ev)
		updateComponentTimer(&c.mu, c.statuses, c.stopWatch)
	}
}

// Render prints the resource as a singleLineComponent and returns the number of lines written and the error if any.
func (c *regularResourceComponent) Render(out io.Writer) (numLines int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	components := stackResourceComponents(c.description, c.separator, c.statuses, c.stopWatch, c.padding)
	return renderComponents(out, components)
}

func updateComponentStatus(mu *sync.Mutex, statuses *[]stackStatus, event stream.StackEvent) {
	mu.Lock()
	defer mu.Unlock()

	*statuses = append(*statuses, stackStatus{
		value:  cloudformation.StackStatus(event.ResourceStatus),
		reason: event.ResourceStatusReason,
	})
}

func updateComponentTimer(mu *sync.Mutex, statuses []stackStatus, sw *stopWatch) {
	mu.Lock()
	defer mu.Unlock()

	// There is always at least two elements {notStartedStatus, <new event>}
	curStatus, nextStatus := statuses[len(statuses)-2], statuses[len(statuses)-1]
	switch {
	case nextStatus.value.InProgress():
		// Reset and start only if the status moved to in progress from a static state.
		// It's possible that CloudFormation sends multiple "CREATE_IN_PROGRESS" events back to back, we don't want to reset the timer then.
		if !curStatus.value.InProgress() {
			return
		}
		sw.reset()
		sw.start()
	default:
		if curStatus == notStartedStackStatus {
			// The resource went from [not started] to a finished state immediately.
			// So start the timer and then immediately finish it.
			sw.start()
		}
		sw.stop()
	}
}

func stackResourceComponents(description string, separator rune, statuses []stackStatus, sw *stopWatch, padding int) []Renderer {
	columns := []string{fmt.Sprintf("- %s", description), prettifyLatestStackStatus(statuses), prettifyElapsedTime(sw)}
	components := []Renderer{
		&singleLineComponent{
			Text:    strings.Join(columns, string(separator)),
			Padding: padding,
		},
	}

	for _, failureReason := range failureReasons(statuses) {
		for _, text := range splitByLength(failureReason, maxCellLength) {
			components = append(components, &singleLineComponent{
				Text:    strings.Join([]string{colorFailureReason(text), "", ""}, string(separator)),
				Padding: padding + nestedComponentPadding,
			})
		}
	}
	return components
}
