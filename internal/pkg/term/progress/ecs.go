// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	maxServiceEventsToDisplay = 5 // Total number of events we want to display at most for ECS service events.
)

// ECSServiceSubscriber is the interface to subscribe channels to ECS service descriptions.
type ECSServiceSubscriber interface {
	Subscribe() <-chan stream.ECSService
}

// ListeningRollingUpdateRenderer renders ECS rolling update deployments.
func ListeningRollingUpdateRenderer(streamer ECSServiceSubscriber, opts RenderOptions) DynamicRenderer {
	c := &rollingUpdateComponent{
		padding:           opts.Padding,
		maxLenFailureMsgs: maxServiceEventsToDisplay,
		stream:            streamer.Subscribe(),
		done:              make(chan struct{}),
	}
	go c.Listen()
	return c
}

type rollingUpdateComponent struct {
	// Data to render.
	deployments  []stream.ECSDeployment
	failureMsgs  []string

	// Style configuration for the component.
	padding           int
	maxLenFailureMsgs int

	stream <-chan stream.ECSService // Channel where deployment events are received.
	done   chan struct{}            // Channel that's closed when there are no more events to listen on.
	mu     sync.Mutex               // Lock used to mutate data to render.
}

// Listen updates the deployment statuses and failure event messages as events are streamed.
func (c *rollingUpdateComponent) Listen() {
	for ev := range c.stream {
		c.mu.Lock()
		c.deployments = ev.Deployments
		c.failureMsgs = append(c.failureMsgs, ev.LatestFailureEvents...)
		if len(c.failureMsgs) > c.maxLenFailureMsgs {
			c.failureMsgs = c.failureMsgs[len(c.failureMsgs)-c.maxLenFailureMsgs:]
		}
		c.mu.Unlock()
	}
	close(c.done)
}

// Render prints first the deployments as a tableComponent and then the failure messages as singleLineComponents.
func (c *rollingUpdateComponent) Render(out io.Writer) (numLines int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	buf := new(bytes.Buffer)

	nl, err := c.renderDeployments(buf)
	if err != nil {
		return 0, err
	}
	numLines += nl

	nl, err = c.renderFailureMsgs(buf)
	if err != nil {
		return 0, err
	}
	numLines += nl

	if _, err := buf.WriteTo(out); err != nil {
		return 0, fmt.Errorf("render rolling update component to writer: %w", err)
	}
	return numLines, nil
}

// Done returns a channel that's closed when there are no more events to listen.
func (c *rollingUpdateComponent) Done() <-chan struct{} {
	return c.done
}

func (c *rollingUpdateComponent) renderDeployments(out io.Writer) (numLines int, err error) {
	header := []string{"", "Revision", "Rollout", "Desired", "Running", "Failed", "Pending"}
	var rows [][]string
	for _, d := range c.deployments {
		rows = append(rows, []string{
			d.Status,
			d.TaskDefRevision,
			prettifyRolloutStatus(d.RolloutState),
			strconv.Itoa(d.DesiredCount),
			strconv.Itoa(d.RunningCount),
			strconv.Itoa(d.FailedCount),
			strconv.Itoa(d.PendingCount),
		})
	}
	table := newTableComponent(color.Faint.Sprintf("Deployments"), header, rows)
	table.Padding = c.padding
	nl, err := table.Render(out)
	if err != nil {
		return 0, fmt.Errorf("render deployments table: %w", err)
	}
	return nl, err
}

func (c *rollingUpdateComponent) renderFailureMsgs(out io.Writer) (numLines int, err error) {
	if len(c.failureMsgs) == 0 {
		return 0, nil
	}

	title := "Latest failure event"
	if l := len(c.failureMsgs); l > 1 {
		title = fmt.Sprintf("Latest %d failure events", l)
	}
	title = fmt.Sprintf("%s%s", color.DullRed.Sprintf("✘ "), color.Faint.Sprintf(title))
	components := []Renderer{
		&singleLineComponent{}, // Add an empty line before rendering failure events.
		&singleLineComponent{
			Text:    title,
			Padding: c.padding,
		},
	}

	for _, msg := range reverseStrings(c.failureMsgs) {
		for i, truncatedMsg := range splitByLength(msg, maxCellLength) {
			pretty := fmt.Sprintf("  %s", truncatedMsg)
			if i == 0 {
				pretty = fmt.Sprintf("- %s", truncatedMsg)
			}
			components = append(components, &singleLineComponent{
				Text:    pretty,
				Padding: c.padding + nestedComponentPadding,
			})
		}
	}
	return renderComponents(out, components)
}

func reverseStrings(arr []string) []string {
	reversed := make([]string, len(arr))
	copy(reversed, arr)

	for i := len(reversed)/2 - 1; i >= 0; i-- {
		opp := len(reversed) - 1 - i
		reversed[i], reversed[opp] = reversed[opp], reversed[i]
	}
	return reversed
}

// parseServiceARN returns the cluster name and service name from a service ARN.
func parseServiceARN(arn string) (cluster, service string) {
	parsed := ecs.ServiceArn(arn)
	// Errors can't happen on valid ARNs.
	cluster, _ = parsed.ClusterName()
	service, _ = parsed.ServiceName()
	return cluster, service
}
