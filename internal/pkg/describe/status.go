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
	awsECS "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	maxAlarmStatusColumnWidth = 30
)

type alarmStatusGetter interface {
	AlarmsWithTags(tags map[string]string) ([]cloudwatch.AlarmStatus, error)
	AlarmStatus(alarms []string) ([]cloudwatch.AlarmStatus, error)
}

type ecsServiceGetter interface {
	ServiceTasks(clusterName, serviceName string) ([]*awsECS.Task, error)
	Service(clusterName, serviceName string) (*awsECS.Service, error)
}

type serviceDescriber interface {
	DescribeService(app, env, svc string) (*ecs.ServiceDesc, error)
}

type autoscalingAlarmNamesGetter interface {
	ECSServiceAlarmNames(cluster, service string) ([]string, error)
}

// ServiceStatus retrieves status of a service.
type ServiceStatus struct {
	app string
	env string
	svc string

	svcDescriber serviceDescriber
	ecsSvc       ecsServiceGetter
	cwSvc        alarmStatusGetter
	aasSvc       autoscalingAlarmNamesGetter
}

// ServiceStatusDesc contains the status for a service.
type ServiceStatusDesc struct {
	Service awsECS.ServiceStatus
	Tasks   []awsECS.TaskStatus      `json:"tasks"`
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
		app:          opt.App,
		env:          opt.Env,
		svc:          opt.Svc,
		svcDescriber: ecs.New(sess),
		cwSvc:        cloudwatch.New(sess),
		ecsSvc:       awsECS.New(sess),
		aasSvc:       aas.New(sess),
	}, nil
}

// Describe returns status of a service.
func (s *ServiceStatus) Describe() (*ServiceStatusDesc, error) {
	svcDesc, err := s.svcDescriber.DescribeService(s.app, s.env, s.svc)
	if err != nil {
		return nil, fmt.Errorf("get ECS service description for %s: %w", s.svc, err)
	}
	service, err := s.ecsSvc.Service(svcDesc.ClusterName, svcDesc.Name)
	if err != nil {
		return nil, fmt.Errorf("get service %s: %w", svcDesc.Name, err)
	}
	var taskStatus []awsECS.TaskStatus
	for _, task := range svcDesc.Tasks {
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
	autoscalingAlarms, err := s.ecsServiceAutoscalingAlarms(svcDesc.ClusterName, svcDesc.Name)
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
	headers := []string{"ID", "Image Digest", "Last Status", "Started At", "Stopped At", "Capacity Provider", "Health Status"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, task := range s.Tasks {
		fmt.Fprint(writer, task.HumanString())
	}
	fmt.Fprint(writer, color.Bold.Sprint("\nAlarms\n\n"))
	writer.Flush()
	headers = []string{"Name", "Condition", "Last Updated", "Health"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
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
