// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/dustin/go-humanize/english"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	maxServiceEventsToDisplay = 5 // Total number of events we want to display at most for ECS service events.
)

const (
	maxStoppedTasksToDisplay = 2
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
	alarms       []cloudwatch.AlarmStatus
	stoppedTasks []ecs.Task

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
		c.stoppedTasks = ev.StoppedTasks
		if len(c.stoppedTasks) > maxStoppedTasksToDisplay {
			c.stoppedTasks = c.stoppedTasks[:maxStoppedTasksToDisplay]
		}
		c.failureMsgs = append(c.failureMsgs, ev.LatestFailureEvents...)
		if len(c.failureMsgs) > c.maxLenFailureMsgs {
			c.failureMsgs = c.failureMsgs[len(c.failureMsgs)-c.maxLenFailureMsgs:]
		}
		c.alarms = ev.Alarms
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

	nl, err = c.renderStoppedTasks(buf)
	if err != nil {
		return 0, err
	}
	numLines += nl

	nl, err = c.renderFailureMsgs(buf)
	if err != nil {
		return 0, err
	}
	numLines += nl

	nl, err = c.renderAlarms(buf)
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

func (c *rollingUpdateComponent) renderAlarms(out io.Writer) (numLines int, err error) {
	if len(c.alarms) == 0 {
		return 0, nil
	}
	header := []string{"Name", "State"}
	var rows [][]string
	for _, a := range c.alarms {
		rows = append(rows, []string{
			a.Name,
			prettifyAlarmState(a.Status),
		})
	}
	table := newTableComponent(color.Faint.Sprintf("Alarms"), header, rows)
	table.Padding = c.padding
	components := []Renderer{
		&singleLineComponent{}, // Add an empty line before rendering alarms table.
		table,
	}
	return renderComponents(out, components)
}

type stoppedTasksInfo struct {
	ids    []string
	reason string
}

func (c *rollingUpdateComponent) renderStoppedTasks(out io.Writer) (numLines int, err error) {
	if len(c.stoppedTasks) == 0 {
		return 0, nil
	}
	header := []string{"TaskId", "CurrentStatus", "DesiredStatus"}
	var rows [][]string
	title := fmt.Sprintf("Latest %d %s stopped reason", len(c.stoppedTasks), english.PluralWord(len(c.stoppedTasks), "task", "tasks"))
	title = fmt.Sprintf("%s%s", color.DullRed.Sprintf("✘ "), color.Faint.Sprintf(title))
	childComponents := []Renderer{
		&singleLineComponent{}, // Add an empty line before rendering task stopped events.
		&singleLineComponent{
			Text:    title,
			Padding: c.padding,
		},
	}

	taskInfoSlice := make([]stoppedTasksInfo, 0, len(c.stoppedTasks))
	for _, st := range c.stoppedTasks {
		id, err := ecs.TaskID(aws.StringValue(st.TaskArn))
		if err != nil {
			return 0, err
		}
		// Check if there is already an entry with the same task stopped reason.
		var found bool
		for i, taskInfo := range taskInfoSlice {
			if taskInfo.reason == aws.StringValue(st.StoppedReason) {
				taskInfoSlice[i].ids = append(taskInfoSlice[i].ids, ecs.ShortTaskID(id))
				found = true
				break
			}
		}

		// If not found, create a new entry
		if !found {
			stInfo := stoppedTasksInfo{
				reason: aws.StringValue(st.StoppedReason),
				ids:    []string{ecs.ShortTaskID(id)},
			}
			taskInfoSlice = append(taskInfoSlice, stInfo)
		}

		rows = append(rows, []string{
			ecs.ShortTaskID(id),
			aws.StringValue(st.LastStatus),
			aws.StringValue(st.DesiredStatus),
		})
	}
	for _, info := range taskInfoSlice {
		for i, truncatedReason := range splitByLength(fmt.Sprintf("[%s]: %s", strings.Join(info.ids, ","), info.reason), maxCellLength) {
			pretty := fmt.Sprintf("  %s", truncatedReason)
			if i == 0 {
				pretty = fmt.Sprintf("- %s", truncatedReason)
			}
			childComponents = append(childComponents, &singleLineComponent{
				Text:    pretty,
				Padding: c.padding + nestedComponentPadding,
			})
		}
	}
	table := newTableComponent(color.Faint.Sprintf("Latest %d stopped %s", len(c.stoppedTasks), english.PluralWord(len(c.stoppedTasks), "task", "tasks")), header, rows)
	table.Padding = c.padding
	var debug debugStoppedTaskReason
	childComponents = append(childComponents,
		&singleLineComponent{
			Text:    debug.RecommendActions(),
			Padding: c.padding + nestedComponentPadding,
		})
	treeComponent := treeComponent{
		Root:     table,
		Children: childComponents,
	}
	nl, err := treeComponent.Render(out)
	if err != nil {
		return 0, fmt.Errorf("render deployments table: %w", err)
	}
	return nl, err
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
	parsed, _ := ecs.ParseServiceArn(arn)
	// Errors can't happen on valid ARNs.
	return parsed.ClusterName(), parsed.ServiceName()
}

type debugStoppedTaskReason struct{}

func (d *debugStoppedTaskReason) RecommendActions() string {
	return fmt.Sprintf(`To troubleshoot the task stopped reason:
  1. You can run %s to see the logs of the last Stopped Task.
  2. You can follow this article https://repost.aws/knowledge-center/ecs-task-stopped.`, color.HighlightCode("copilot svc logs --previous"))
}
