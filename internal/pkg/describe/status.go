// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatch"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	humanize "github.com/dustin/go-humanize"
)

const (
	shortTaskIDLength      = 8
	shortImageDigestLength = 8
)

// WebAppStatus contains the status for a web application.
type WebAppStatus struct {
	Service ecs.ServiceStatus        `json:",flow"`
	Tasks   []ecs.TaskStatus         `json:"tasks"`
	Metrics []cloudwatch.AlarmStatus `json:"metrics"`
}

// JSONString returns the stringified webAppStatus struct with json format.
func (w *WebAppStatus) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified webAppStatus struct with human readable format.
func (w *WebAppStatus) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("Service Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Status", w.Service.Status)
	fmt.Fprintf(writer, "  %s\t%v\n", "DesiredCount", w.Service.DesiredCount)
	fmt.Fprintf(writer, "  %s\t%v\n", "RunningCount", w.Service.RunningCount)
	fmt.Fprintf(writer, color.Bold.Sprint("\nTask Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\t%s\n", "ID", "ImageDigest", "LastStatus", "DesiredStatus", "StartedAt", "StoppedAt")
	for _, task := range w.Tasks {
		var digest []string
		for _, image := range task.Images {
			digest = append(digest, image.Digest[:shortImageDigestLength])
		}
		startedSince := humanize.Time(time.Unix(task.StartedAt, 0))
		stoppedSince := "-"
		if task.StoppedAt != 0 {
			stoppedSince = humanize.Time(time.Unix(task.StoppedAt, 0))
		}
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\t%s\n", task.ID[:shortTaskIDLength], strings.Join(digest, ","), task.LastStatus, task.DesiredStatus, startedSince, stoppedSince)
	}
	fmt.Fprintf(writer, color.Bold.Sprint("\nMetrics\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\n", "Name", "Health", "UpdatedTimes", "Reason")
	for _, metric := range w.Metrics {
		updatedTimeSince := humanize.Time(time.Unix(metric.UpdatedTimes, 0))
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\n", metric.Name, metric.Status, updatedTimeSince, metric.Reason)
	}
	writer.Flush()
	return b.String()
}
