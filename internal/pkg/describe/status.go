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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	humanize "github.com/dustin/go-humanize"
)

const (
	shortTaskIDLength      = 8
	shortImageDigestLength = 8
)

type alarmStatusGetter interface {
	GetAlarmsWithTags(tags map[string]string) ([]cloudwatch.AlarmStatus, error)
}

type ecsServiceGetter interface {
	ServiceTasks(clusterName, serviceName string) ([]*ecs.Task, error)
	Service(clusterName, serviceName string) (*ecs.Service, error)
}

type serviceArnGetter interface {
	GetServiceArn(envName string) (*ecs.ServiceArn, error)
}

// WebAppStatus retrieves status of a web app application.
type WebAppStatus struct {
	ProjectName string
	EnvName     string
	AppName     string

	Describer serviceArnGetter
	EcsSvc    ecsServiceGetter
	CwSvc     alarmStatusGetter
}

// WebAppStatusDesc contains the status for a web application.
type WebAppStatusDesc struct {
	Service ecs.ServiceStatus        `json:",flow"`
	Tasks   []ecs.TaskStatus         `json:"tasks"`
	Alarms  []cloudwatch.AlarmStatus `json:"alarms"`
}

// NewWebAppStatus initinstantiatesiate a new WebAppStatus struct.
func NewWebAppStatus(projectName, envName, appName string) (*WebAppStatus, error) {
	return &WebAppStatus{
		ProjectName: projectName,
		EnvName:     envName,
		AppName:     appName,
	}, nil
}

// Describe returns status of a web app application.
func (w *WebAppStatus) Describe() (*WebAppStatusDesc, error) {
	serviceArn, err := w.Describer.GetServiceArn(w.EnvName)
	if err != nil {
		return nil, fmt.Errorf("get service ARN: %w", err)
	}
	clusterName, err := serviceArn.ClusterName()
	if err != nil {
		return nil, fmt.Errorf("get cluster name: %w", err)
	}
	serviceName, err := serviceArn.ServiceName()
	if err != nil {
		return nil, fmt.Errorf("get service name: %w", err)
	}
	service, err := w.EcsSvc.Service(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get ECS service %s: %w", serviceName, err)
	}
	tasks, err := w.EcsSvc.ServiceTasks(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get ECS tasks for service %s: %w", serviceName, err)
	}
	var taskStatus []ecs.TaskStatus
	for _, task := range tasks {
		status, err := task.TaskStatus()
		if err != nil {
			return nil, fmt.Errorf("get status for task %s: %w", *task.TaskArn, err)
		}
		taskStatus = append(taskStatus, *status)
	}
	alarms, err := w.CwSvc.GetAlarmsWithTags(map[string]string{
		stack.ProjectTagKey: w.ProjectName,
		stack.EnvTagKey:     w.EnvName,
		stack.AppTagKey:     w.AppName,
	})
	if err != nil {
		return nil, fmt.Errorf("get CloudWatch alarms: %w", err)
	}
	return &WebAppStatusDesc{
		Service: service.ServiceStatus(),
		Tasks:   taskStatus,
		Alarms:  alarms,
	}, nil
}

// JSONString returns the stringified webAppStatus struct with json format.
func (w *WebAppStatusDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified webAppStatus struct with human readable format.
func (w *WebAppStatusDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("Service Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s %v / %v running tasks (%v pending)\n", statusColor(w.Service.Status),
		w.Service.RunningCount, w.Service.DesiredCount, w.Service.DesiredCount-w.Service.RunningCount)
	fmt.Fprintf(writer, color.Bold.Sprint("\nLast Deployment\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Updated At", humanize.Time(time.Unix(w.Service.LastDeploymentAt, 0)))
	fmt.Fprintf(writer, "  %s\t%s\n", "Task Definition", w.Service.TaskDefinition)
	fmt.Fprintf(writer, color.Bold.Sprint("\nTask Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\t%s\n", "ID", "Image Digest", "Last Status", "Desired Status", "Started At", "Stopped At")
	for _, task := range w.Tasks {
		var digest []string
		imageDigest := "-"
		for _, image := range task.Images {
			if len(image.Digest) < shortImageDigestLength {
				continue
			}
			digest = append(digest, image.Digest[:shortImageDigestLength])
		}
		if len(digest) != 0 {
			imageDigest = strings.Join(digest, ",")
		}
		startedSince := "-"
		if task.StartedAt != 0 {
			startedSince = humanize.Time(time.Unix(task.StartedAt, 0))
		}
		stoppedSince := "-"
		if task.StoppedAt != 0 {
			stoppedSince = humanize.Time(time.Unix(task.StoppedAt, 0))
		}
		shortTaskID := "-"
		if len(task.ID) >= shortTaskIDLength {
			shortTaskID = task.ID[:shortTaskIDLength]
		}
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\t%s\n", shortTaskID, imageDigest, task.LastStatus, task.DesiredStatus, startedSince, stoppedSince)
	}
	fmt.Fprintf(writer, color.Bold.Sprint("\nAlarms\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\n", "Name", "Health", "Last Updated", "Reason")
	for _, alarm := range w.Alarms {
		updatedTimeSince := humanize.Time(time.Unix(alarm.UpdatedTimes, 0))
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\n", alarm.Name, alarm.Status, updatedTimeSince, alarm.Reason)
	}
	writer.Flush()
	return b.String()
}

func statusColor(status string) string {
	switch status {
	case "ACTIVE":
		return color.Green.Sprint(status)
	case "DRAINING":
		return color.Yellow.Sprint(status)
	default:
		return color.Red.Sprint(status)
	}
}
