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

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatch"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
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
	projectName   string
	envName       string
	appName       string
	storeSvc      envGetter
	describer     serviceArnGetter
	ecsSvc        ecsServiceGetter
	cwSvc         alarmStatusGetter
	initecsSvc    func(*WebAppStatus, *archer.Environment) error
	initcwSvc     func(*WebAppStatus, *archer.Environment) error
	initDescriber func(*WebAppStatus, string) error
}

// WebAppStatusDesc contains the status for a web application.
type WebAppStatusDesc struct {
	Service ecs.ServiceStatus        `json:",flow"`
	Tasks   []ecs.TaskStatus         `json:"tasks"`
	Alarms  []cloudwatch.AlarmStatus `json:"alarms"`
}

// NewWebAppStatus initinstantiatesiate a new WebAppStatus struct.
func NewWebAppStatus(projectName, envName, appName string) (*WebAppStatus, error) {
	ssmStore, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	return &WebAppStatus{
		projectName: projectName,
		envName:     envName,
		appName:     appName,
		storeSvc:    ssmStore,
		initecsSvc: func(w *WebAppStatus, env *archer.Environment) error {
			sess, err := session.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
			if err != nil {
				return fmt.Errorf("session for role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
			}
			w.ecsSvc = ecs.New(sess)
			return nil
		},
		initcwSvc: func(w *WebAppStatus, env *archer.Environment) error {
			sess, err := session.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
			if err != nil {
				return fmt.Errorf("session for role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
			}
			w.cwSvc = cloudwatch.New(sess)
			return nil
		},
		initDescriber: func(w *WebAppStatus, appName string) error {
			d, err := NewWebAppDescriber(projectName, appName)
			if err != nil {
				return fmt.Errorf("creating describer for application %s in project %s: %w", appName, projectName, err)
			}
			w.describer = d
			return nil
		},
	}, nil
}

// Describe returns status of a web app application.
func (w *WebAppStatus) Describe() (*WebAppStatusDesc, error) {
	if err := w.configSvc(); err != nil {
		return nil, err
	}
	serviceArn, err := w.describer.GetServiceArn(w.envName)
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
	service, err := w.ecsSvc.Service(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get ECS service %s: %w", serviceName, err)
	}
	tasks, err := w.ecsSvc.ServiceTasks(clusterName, serviceName)
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
	alarms, err := w.cwSvc.GetAlarmsWithTags(map[string]string{
		stack.ProjectTagKey: w.projectName,
		stack.EnvTagKey:     w.envName,
		stack.AppTagKey:     w.appName,
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

func (w *WebAppStatus) configSvc() error {
	if err := w.initDescriber(w, w.appName); err != nil {
		return err
	}
	env, err := w.storeSvc.GetEnvironment(w.projectName, w.envName)
	if err != nil {
		return fmt.Errorf("get environment %s: %w", w.envName, err)
	}
	if err := w.initcwSvc(w, env); err != nil {
		return err
	}
	return w.initecsSvc(w, env)
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
	fmt.Fprintf(writer, "  %s\t%s\n", "Updated At", humanize.Time(time.Unix(w.Service.LastDeployment, 0)))
	fmt.Fprintf(writer, "  %s\t%s\n", "Task Definition", w.Service.TaskDefinition)
	fmt.Fprintf(writer, color.Bold.Sprint("\nTask Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\t%s\n", "ID", "Image Digest", "Last Status", "Desired Status", "Started At", "Stopped At")
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
