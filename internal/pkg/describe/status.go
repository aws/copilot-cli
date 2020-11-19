// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/aws/aas"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	ecsServiceResourceType    = "ecs:service"
	maxAlarmStatusColumnWidth = 30
)

type alarmStatusGetter interface {
	AlarmsWithTags(tags map[string]string) ([]cloudwatch.AlarmStatus, error)
	AlarmStatus(alarms []string) ([]cloudwatch.AlarmStatus, error)
}

type resourcesGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*rg.Resource, error)
}

type ecsServiceGetter interface {
	ServiceTasks(clusterName, serviceName string) ([]*ecs.Task, error)
	Service(clusterName, serviceName string) (*ecs.Service, error)
}

type autoscalingAlarmNamesGetter interface {
	ECSServiceAlarmNames(cluster, service string) ([]string, error)
}

// ServiceStatus retrieves status of a service.
type ServiceStatus struct {
	app string
	env string
	svc string

	ecsSvc ecsServiceGetter
	cwSvc  alarmStatusGetter
	aasSvc autoscalingAlarmNamesGetter
	rgSvc  resourcesGetter
}

// ServiceStatusDesc contains the status for a service.
type ServiceStatusDesc struct {
	Service ecs.ServiceStatus
	Tasks   []ecs.TaskStatus         `json:"tasks"`
	Alarms  []cloudwatch.AlarmStatus `json:"alarms"`
}

// NewServiceStatusConfig contains fields that initiates ServiceStatus struct.
type NewServiceStatusConfig struct {
	App         string
	Env         string
	Svc         string
	ConfigStore ConfigStoreSvc
}

// NewServiceStatus instantiates a new ServiceStatus struct.
func NewServiceStatus(opt *NewServiceStatusConfig) (*ServiceStatus, error) {
	env, err := opt.ConfigStore.GetEnvironment(opt.App, opt.Env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", opt.Env, err)
	}
	sess, err := sessions.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("session for role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
	}
	return &ServiceStatus{
		app:    opt.App,
		env:    opt.Env,
		svc:    opt.Svc,
		rgSvc:  rg.New(sess),
		cwSvc:  cloudwatch.New(sess),
		ecsSvc: ecs.New(sess),
		aasSvc: aas.New(sess),
	}, nil
}

func (s *ServiceStatus) getServiceArn() (*ecs.ServiceArn, error) {
	svcResources, err := s.rgSvc.GetResourcesByTags(ecsServiceResourceType, map[string]string{
		deploy.AppTagKey:     s.app,
		deploy.EnvTagKey:     s.env,
		deploy.ServiceTagKey: s.svc,
	})
	if err != nil {
		return nil, err
	}
	if len(svcResources) == 0 {
		return nil, fmt.Errorf("cannot find service arn in service stack resource")
	}
	serviceArn := ecs.ServiceArn(svcResources[0].ARN)
	return &serviceArn, nil
}

// Describe returns status of a service.
func (s *ServiceStatus) Describe() (*ServiceStatusDesc, error) {
	serviceArn, err := s.getServiceArn()
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
	service, err := s.ecsSvc.Service(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get service %s: %w", serviceName, err)
	}
	tasks, err := s.ecsSvc.ServiceTasks(clusterName, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get tasks for service %s: %w", serviceName, err)
	}
	var taskStatus []ecs.TaskStatus
	for _, task := range tasks {
		status, err := task.TaskStatus()
		if err != nil {
			return nil, fmt.Errorf("get status for task %s: %w", *task.TaskArn, err)
		}
		taskStatus = append(taskStatus, *status)
	}
	var alarms []cloudwatch.AlarmStatus
	taggedAlarms, err := s.cwSvc.AlarmsWithTags(map[string]string{
		deploy.AppTagKey:     s.app,
		deploy.EnvTagKey:     s.env,
		deploy.ServiceTagKey: s.svc,
	})
	if err != nil {
		return nil, fmt.Errorf("get tagged CloudWatch alarms: %w", err)
	}
	alarms = append(alarms, taggedAlarms...)
	autoscalingAlarms, err := s.ecsServiceAutoscalingAlarms(clusterName, serviceName)
	if err != nil {
		return nil, err
	}
	alarms = append(alarms, autoscalingAlarms...)
	return &ServiceStatusDesc{
		Service: service.ServiceStatus(),
		Tasks:   taskStatus,
		Alarms:  alarms,
	}, nil
}

func (s *ServiceStatus) ecsServiceAutoscalingAlarms(cluster, service string) ([]cloudwatch.AlarmStatus, error) {
	alarmNames, err := s.aasSvc.ECSServiceAlarmNames(cluster, service)
	if err != nil {
		return nil, fmt.Errorf("retrieve auto scaling alarm names for ECS service %s/%s: %w", cluster, service, err)
	}
	alarms, err := s.cwSvc.AlarmStatus(alarmNames)
	if err != nil {
		return nil, fmt.Errorf("get auto scaling CloudWatch alarms: %w", err)
	}
	return alarms, nil
}

// JSONString returns the stringified ServiceStatusDesc struct with json format.
func (s *ServiceStatusDesc) JSONString() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal services: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified ServiceStatusDesc struct with human readable format.
func (s *ServiceStatusDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, statusCellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("Service Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s %v / %v running tasks (%v pending)\n", statusColor(s.Service.Status),
		s.Service.RunningCount, s.Service.DesiredCount, s.Service.DesiredCount-s.Service.RunningCount)
	fmt.Fprint(writer, color.Bold.Sprint("\nLast Deployment\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Updated At", humanizeTime(s.Service.LastDeploymentAt))
	fmt.Fprintf(writer, "  %s\t%s\n", "Task Definition", s.Service.TaskDefinition)
	fmt.Fprint(writer, color.Bold.Sprint("\nTask Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\t%s\n", "ID", "Image Digest", "Last Status", "Started At", "Stopped At", "Health Status")
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\t%s\n", strings.Repeat("-", len("ID")), strings.Repeat("-", len("Image Digest")), strings.Repeat("-", len("Last Status")), strings.Repeat("-", len("Started At")), strings.Repeat("-", len("Stopped At")), strings.Repeat("-", len("Health Status")))
	for _, task := range s.Tasks {
		fmt.Fprint(writer, task.HumanString())
	}
	fmt.Fprint(writer, color.Bold.Sprint("\nAlarms\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\n", "Name", "Condition", "Last Updated", "Health")
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\n", strings.Repeat("-", len("Name")), strings.Repeat("-", len("Condition")), strings.Repeat("-", len("Last Updated")), strings.Repeat("-", len("Health")))
	for _, alarm := range s.Alarms {
		updatedTimeSince := humanizeTime(alarm.UpdatedTimes)
		printWithMaxWidth(writer, "  %s\t%s\t%s\t%s\n", maxAlarmStatusColumnWidth, alarm.Name, alarm.Condition, updatedTimeSince, alarmHealthColor(alarm.Status))
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\n", "", "", "", "")
	}
	writer.Flush()
	return b.String()
}

func printWithMaxWidth(w *tabwriter.Writer, format string, width int, members ...string) {
	columns := make([][]string, len(members))
	maxNumOfLinesPerCol := 0
	for ind, member := range members {
		var column []string
		builder := new(strings.Builder)
		// https://stackoverflow.com/questions/25686109/split-string-by-length-in-golang
		for i, r := range []rune(member) {
			builder.WriteRune(r)
			if i > 0 && (i+1)%width == 0 {
				column = append(column, builder.String())
				builder.Reset()
			}
		}
		if builder.String() != "" {
			column = append(column, builder.String())
		}
		maxNumOfLinesPerCol = int(math.Max(float64(len(column)), float64(maxNumOfLinesPerCol)))
		columns[ind] = column
	}
	for i := 0; i < maxNumOfLinesPerCol; i++ {
		args := make([]interface{}, len(columns))
		for ind, memberSlice := range columns {
			if i >= len(memberSlice) {
				args[ind] = ""
				continue
			}
			args[ind] = memberSlice[i]
		}
		fmt.Fprintf(w, format, args...)
	}
}

func alarmHealthColor(status string) string {
	switch status {
	case "OK":
		return color.Green.Sprint(status)
	case "ALARM":
		return color.Red.Sprint(status)
	case "INSUFFICIENT_DATA":
		return color.Yellow.Sprint(status)
	default:
		return status
	}
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
