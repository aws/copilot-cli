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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/aas"
	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	awsECS "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	maxAlarmStatusColumnWidth   = 30
	fmtAppRunnerSvcLogGroupName = "/aws/apprunner/%s/%s/service"
	defaultServiceLogsLimit     = 10
)

type alarmStatusGetter interface {
	AlarmsWithTags(tags map[string]string) ([]cloudwatch.AlarmStatus, error)
	AlarmStatus(alarms []string) ([]cloudwatch.AlarmStatus, error)
}

type logGetter interface {
	LogEvents(opts cloudwatchlogs.LogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
}

type ecsServiceGetter interface {
	ServiceTasks(clusterName, serviceName string) ([]*awsECS.Task, error)
	Service(clusterName, serviceName string) (*awsECS.Service, error)
}

type serviceDescriber interface {
	DescribeService(app, env, svc string) (*ecs.ServiceDesc, error)
}

type apprunnerServiceDescriber interface {
	Service() (*apprunner.Service, error)
}

type autoscalingAlarmNamesGetter interface {
	ECSServiceAlarmNames(cluster, service string) ([]string, error)
}

// ECSStatusDescriber retrieves status of an ECS service.
type ECSStatusDescriber struct {
	app string
	env string
	svc string

	svcDescriber serviceDescriber
	ecsSvcGetter ecsServiceGetter
	cwSvcGetter  alarmStatusGetter
	aasSvcGetter autoscalingAlarmNamesGetter
}

// AppRunnerStatusDescriber retrieves status of an AppRunner service.
type AppRunnerStatusDescriber struct {
	app string
	env string
	svc string

	svcDescriber apprunnerServiceDescriber
	eventsGetter logGetter
}

// ecsServiceStatus contains the status for an ECS service.
type ecsServiceStatus struct {
	Service      awsECS.ServiceStatus
	Tasks        []awsECS.TaskStatus      `json:"tasks"`
	Alarms       []cloudwatch.AlarmStatus `json:"alarms"`
	StoppedTasks []awsECS.TaskStatus      `json:"stoppedTasks"`
}

// apprunnerServiceStatus contains the status for an AppRunner service.
type apprunnerServiceStatus struct {
	Service   apprunner.Service
	LogEvents []*cloudwatchlogs.Event
}

// NewServiceStatusConfig contains fields that initiates ServiceStatus struct.
type NewServiceStatusConfig struct {
	App         string
	Env         string
	Svc         string
	ConfigStore ConfigStoreSvc
}

// NewECSStatusDescriber instantiates a new ECSStatusDescriber struct.
func NewECSStatusDescriber(opt *NewServiceStatusConfig) (*ECSStatusDescriber, error) {
	env, err := opt.ConfigStore.GetEnvironment(opt.App, opt.Env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", opt.Env, err)
	}
	sess, err := sessions.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("session for role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
	}
	return &ECSStatusDescriber{
		app:          opt.App,
		env:          opt.Env,
		svc:          opt.Svc,
		svcDescriber: ecs.New(sess),
		cwSvcGetter:  cloudwatch.New(sess),
		ecsSvcGetter: awsECS.New(sess),
		aasSvcGetter: aas.New(sess),
	}, nil
}

// NewAppRunnerStatusDescriber instantiates a new AppRunnerStatusDescriber struct.
func NewAppRunnerStatusDescriber(opt *NewServiceStatusConfig) (*AppRunnerStatusDescriber, error) {
	ecsSvcDescriber, err := NewAppRunnerServiceDescriber(NewServiceConfig{
		App: opt.App,
		Env: opt.Env,
		Svc: opt.Svc,

		ConfigStore: opt.ConfigStore,
	})
	if err != nil {
		return nil, err
	}

	return &AppRunnerStatusDescriber{
		app:          opt.App,
		env:          opt.Env,
		svc:          opt.Svc,
		svcDescriber: ecsSvcDescriber,
		eventsGetter: cloudwatchlogs.New(ecsSvcDescriber.sess),
	}, nil
}

// Describe returns status of an ECS service.
func (s *ECSStatusDescriber) Describe() (HumanJSONStringer, error) {
	svcDesc, err := s.svcDescriber.DescribeService(s.app, s.env, s.svc)
	if err != nil {
		return nil, fmt.Errorf("get ECS service description for %s: %w", s.svc, err)
	}
	service, err := s.ecsSvcGetter.Service(svcDesc.ClusterName, svcDesc.Name)
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
	taggedAlarms, err := s.cwSvcGetter.AlarmsWithTags(map[string]string{
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

	var stoppedTaskStatus []awsECS.TaskStatus
	for _, task := range svcDesc.StoppedTasks {
		status, err := task.TaskStatus()
		if err != nil {
			return nil, fmt.Errorf("get status for stopped task %s: %w", *task.TaskArn, err)
		}
		stoppedTaskStatus = append(stoppedTaskStatus, *status)
	}

	return &ecsServiceStatus{
		Service:      service.ServiceStatus(),
		Tasks:        taskStatus,
		Alarms:       alarms,
		StoppedTasks: stoppedTaskStatus,
	}, nil
}

// Describe returns status of an AppRunner service.
func (a *AppRunnerStatusDescriber) Describe() (HumanJSONStringer, error) {
	svc, err := a.svcDescriber.Service()
	if err != nil {
		return nil, fmt.Errorf("get AppRunner service description for App Runner service %s in environment %s: %w", a.svc, a.env, err)
	}
	logGroupName := fmt.Sprintf(fmtAppRunnerSvcLogGroupName, svc.Name, svc.ID)
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup: logGroupName,
		Limit:    aws.Int64(defaultServiceLogsLimit),
	}
	logEventsOutput, err := a.eventsGetter.LogEvents(logEventsOpts)
	if err != nil {
		return nil, fmt.Errorf("get log events for log group %s: %w", logGroupName, err)
	}
	return &apprunnerServiceStatus{
		Service:   *svc,
		LogEvents: logEventsOutput.Events,
	}, nil
}

func (s *ECSStatusDescriber) ecsServiceAutoscalingAlarms(cluster, service string) ([]cloudwatch.AlarmStatus, error) {
	alarmNames, err := s.aasSvcGetter.ECSServiceAlarmNames(cluster, service)
	if err != nil {
		return nil, fmt.Errorf("retrieve auto scaling alarm names for ECS service %s/%s: %w", cluster, service, err)
	}
	alarms, err := s.cwSvcGetter.AlarmStatus(alarmNames)
	if err != nil {
		return nil, fmt.Errorf("get auto scaling CloudWatch alarms: %w", err)
	}
	return alarms, nil
}

// JSONString returns the stringified ecsServiceStatus struct with json format.
func (s *ecsServiceStatus) JSONString() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal services: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// JSONString returns the stringified apprunnerServiceStatus struct with json format.
func (a *apprunnerServiceStatus) JSONString() (string, error) {
	data := struct {
		ARN       string    `json:"arn"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
		Source    struct {
			ImageID string `json:"imageId"`
		} `json:"source"`
	}{
		ARN:       a.Service.ServiceARN,
		Status:    a.Service.Status,
		CreatedAt: a.Service.DateCreated,
		UpdatedAt: a.Service.DateUpdated,
		Source: struct {
			ImageID string `json:"imageId"`
		}{
			ImageID: a.Service.ImageID,
		},
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal services: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified ecsServiceStatus struct with human readable format.
func (s *ecsServiceStatus) HumanString() string {
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
	headers := []string{"ID", "Image Digest", "Last Status", "Started At", "Capacity Provider", "Health Status"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, task := range s.Tasks {
		fmt.Fprintf(writer, "  %s\n", task.HumanString())
	}
	if len(s.StoppedTasks) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nStopped Tasks\n\n"))
		writer.Flush()
		headers = []string{"ID", "Image Digest", "Last Status", "Started At", "Stopped At", "Stopped Reason"}
		fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
		fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
		for _, task := range s.StoppedTasks {
			fmt.Fprintf(writer, "  %s\n", (awsECS.StoppedTaskStatus)(task).HumanString())
		}
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

// HumanString returns the stringified apprunnerServiceStatus struct with human readable format.
func (a *apprunnerServiceStatus) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, statusCellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("Service Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, " Status %s \n", statusColor(a.Service.Status))
	fmt.Fprint(writer, color.Bold.Sprint("\nLast Deployment\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Updated At", humanizeTime(a.Service.DateUpdated))
	serviceID := fmt.Sprintf("%s/%s", a.Service.Name, a.Service.ID)
	fmt.Fprintf(writer, "  %s\t%s\n", "Service ID", serviceID)
	imageID := a.Service.ImageID
	if strings.Contains(a.Service.ImageID, "/") {
		imageID = strings.SplitAfterN(imageID, "/", 2)[1] // strip the registry.
	}
	fmt.Fprintf(writer, "  %s\t%s\n", "Source", imageID)
	writer.Flush()
	fmt.Fprint(writer, color.Bold.Sprint("\nSystem Logs\n\n"))
	writer.Flush()
	lo, _ := time.LoadLocation("UTC")
	for _, event := range a.LogEvents {
		timestamp := time.Unix(event.Timestamp/1000, 0).In(lo)
		fmt.Fprintf(writer, "  %v\t%s\n", timestamp.Format(time.RFC3339), event.Message)
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
	case "RUNNING":
		return color.Green.Sprint(status)
	case "UPDATING":
		return color.Yellow.Sprint(status)
	default:
		return color.Red.Sprint(status)
	}
}
