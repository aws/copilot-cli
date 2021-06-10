// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/aas"
	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	maxAlarmStatusColumnWidth   = 30
	fmtAppRunnerSvcLogGroupName = "/aws/apprunner/%s/%s/service"
	defaultServiceLogsLimit     = 10
	shortTaskIDLength           = 8
)

type targetHealthGetter interface {
	TargetsHealth(targetGroupARN string) ([]*elbv2.TargetHealth, error)
}

type alarmStatusGetter interface {
	AlarmsWithTags(tags map[string]string) ([]cloudwatch.AlarmStatus, error)
	AlarmStatus(alarms []string) ([]cloudwatch.AlarmStatus, error)
}

type logGetter interface {
	LogEvents(opts cloudwatchlogs.LogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
}

type ecsServiceGetter interface {
	ServiceTasks(clusterName, serviceName string) ([]*awsecs.Task, error)
	Service(clusterName, serviceName string) (*awsecs.Service, error)
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

	svcDescriber       serviceDescriber
	ecsSvcGetter       ecsServiceGetter
	cwSvcGetter        alarmStatusGetter
	aasSvcGetter       autoscalingAlarmNamesGetter
	targetHealthGetter targetHealthGetter
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
	Service             awsecs.ServiceStatus
	DesiredRunningTasks []awsecs.TaskStatus      `json:"tasks"`
	Alarms              []cloudwatch.AlarmStatus `json:"alarms"`
	StoppedTasks        []awsecs.TaskStatus      `json:"stoppedTasks"`
	TasksTargetHealth   []taskTargetHealth       `json:"targetsHealth"`
}

// appRunnerServiceStatus contains the status for an AppRunner service.
type appRunnerServiceStatus struct {
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
		app:                opt.App,
		env:                opt.Env,
		svc:                opt.Svc,
		svcDescriber:       ecs.New(sess),
		cwSvcGetter:        cloudwatch.New(sess),
		ecsSvcGetter:       awsecs.New(sess),
		aasSvcGetter:       aas.New(sess),
		targetHealthGetter: elbv2.New(sess),
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

	var taskStatus []awsecs.TaskStatus
	for _, task := range svcDesc.Tasks {
		status, err := task.TaskStatus()
		if err != nil {
			return nil, fmt.Errorf("get status for task %s: %w", aws.StringValue(task.TaskArn), err)
		}
		taskStatus = append(taskStatus, *status)
	}

	var stoppedTaskStatus []awsecs.TaskStatus
	for _, task := range svcDesc.StoppedTasks {
		status, err := task.TaskStatus()
		if err != nil {
			return nil, fmt.Errorf("get status for stopped task %s: %w", aws.StringValue(task.TaskArn), err)
		}
		stoppedTaskStatus = append(stoppedTaskStatus, *status)
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

	var tasksTargetHealth []taskTargetHealth
	targetGroupsARN := service.TargetGroups()
	for _, groupARN := range targetGroupsARN {
		targetsHealth, err := s.targetHealthGetter.TargetsHealth(groupARN)
		if err != nil {
			continue
		}
		tasksTargetHealth = append(tasksTargetHealth, targetHealthForTasks(targetsHealth, svcDesc.Tasks, groupARN)...)
	}
	sort.SliceStable(tasksTargetHealth, func(i, j int) bool {
		if tasksTargetHealth[i].TargetGroupARN == tasksTargetHealth[j].TargetGroupARN {
			return tasksTargetHealth[i].TaskID < tasksTargetHealth[j].TaskID
		}
		return tasksTargetHealth[i].TargetGroupARN < tasksTargetHealth[j].TargetGroupARN
	})

	return &ecsServiceStatus{
		Service:             service.ServiceStatus(),
		DesiredRunningTasks: taskStatus,
		Alarms:              alarms,
		StoppedTasks:        stoppedTaskStatus,
		TasksTargetHealth:   tasksTargetHealth,
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
	return &appRunnerServiceStatus{
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

// JSONString returns the stringified appRunnerServiceStatus struct with json format.
func (a *appRunnerServiceStatus) JSONString() (string, error) {
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
	writer := tabwriter.NewWriter(&b, statusMinCellWidth, tabWidth, statusCellPaddingWidth, paddingChar, noAdditionalFormatting)

	s.writeTaskSummary(writer)
	s.writeStoppedTasks(writer)
	s.writeRunningTasks(writer)
	s.writeAlarms(writer)
	writer.Flush()
	return b.String()
}

// HumanString returns the stringified appRunnerServiceStatus struct with human readable format.
func (a *appRunnerServiceStatus) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, statusCellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("Service Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, " Status %s \n", statusColor(a.Service.Status))
	fmt.Fprint(writer, color.Bold.Sprint("\nLast deployment\n\n"))
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

func (s *ecsServiceStatus) writeTaskSummary(writer *tabwriter.Writer) {
	// NOTE: all the `bar` need to be fully colored. Observe how all the second parameter for all `summaryBar` function
	// is a list of strings that are colored (e.g. `[]string{color.Green.Sprint("■"), color.Grey.Sprint("□")}`)
	// This is because if the some of the bar is partially colored, tab writer will behave unexpectedly.

	fmt.Fprint(writer, color.Bold.Sprint("Task Summary\n\n"))
	writer.Flush()

	var primaryDeployment awsecs.Deployment
	var activeDeployments []awsecs.Deployment
	for _, d := range s.Service.Deployments {
		switch d.Status {
		case awsecs.ServiceDeploymentStatusPrimary:
			primaryDeployment = d // There is at most one primary deployment
		case awsecs.ServiceDeploymentStatusActive:
			activeDeployments = append(activeDeployments, d)
		}
	}

	// Write summary of running tasks.
	header := "Running"
	var (
		barData            []int
		barRepresentations []string
	)
	if len(activeDeployments) > 0 {
		var runningPrimary, runningActive int
		for _, d := range activeDeployments {
			runningActive += (int)(d.RunningCount)
		}

		runningPrimary = (int)(primaryDeployment.RunningCount)
		barData = []int{runningPrimary, runningActive}
		barRepresentations = []string{color.Green.Sprint("█"), color.Blue.Sprint("█")}
	} else {
		barData = []int{(int)(s.Service.RunningCount), (int)(s.Service.DesiredCount) - (int)(s.Service.RunningCount)}
		barRepresentations = []string{color.Green.Sprint("█"), color.Green.Sprint("░")}
	}
	bar := summaryBar(barData, barRepresentations)
	stringSummary := fmt.Sprintf("%d/%d desired tasks are running", s.Service.RunningCount, s.Service.DesiredCount)
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", header, bar, stringSummary)

	// Write summary of primary deployment and active deployments.
	if len(activeDeployments) > 0 {
		// Show "Deployments" section only if there are "ACTIVE" deployments in addition to the "PRIMARY" deployment.
		// This is because if there aren't any "ACTIVE" deployment, then this section would have been showing the same
		// information as the "Running" section.
		header := "Deployments"
		bar, stringSummary := summaryOfDeployment(primaryDeployment, []string{color.Green.Sprint("█"), color.Green.Sprint("░")})
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", header, bar, stringSummary)
		for _, deployment := range activeDeployments {
			bar, stringSummary := summaryOfDeployment(deployment, []string{color.Blue.Sprint("█"), color.Blue.Sprint("░")})
			fmt.Fprintf(writer, "  %s\t%s\t%s\n", "", bar, stringSummary)
		}
	}

	// Write summary of HTTP health and container health of tasks in primary deployment.
	revision, _ := awsecs.TaskDefinitionVersion(primaryDeployment.TaskDefinition)
	primaryTasks := s.tasksOfRevision(revision)
	shouldShowHTTPHealth := anyTaskATarget(primaryTasks, s.TasksTargetHealth)
	shouldShowContainerHealth := !allContainerHealthEmpty(primaryTasks)
	if shouldShowHTTPHealth || shouldShowContainerHealth {
		header := "Health"

		var revisionInfo string
		if len(activeDeployments) > 0 {
			revisionInfo = fmt.Sprintf(" (rev %d)", revision)
		}

		if shouldShowHTTPHealth {
			healthyCount := healthyHTTPTaskCountInTasks(primaryTasks, s.TasksTargetHealth)
			bar := summaryBar(
				[]int{
					healthyCount,
					(int)(primaryDeployment.DesiredCount) - healthyCount,
				},
				[]string{color.Green.Sprint("█"), color.Green.Sprint("░")})
			stringSummary := fmt.Sprintf("%d/%d passes HTTP health checks%s", healthyCount, primaryDeployment.DesiredCount, revisionInfo)
			fmt.Fprintf(writer, "  %s\t%s\t%s\n", header, bar, stringSummary)
			header = ""
		}

		if shouldShowContainerHealth {
			healthyCount, _, _ := containerHealthDataForTasks(primaryTasks)
			bar := summaryBar(
				[]int{
					healthyCount,
					(int)(primaryDeployment.DesiredCount) - healthyCount,
				},
				[]string{color.Green.Sprint("█"), color.Green.Sprint("░")})
			stringSummary := fmt.Sprintf("%d/%d passes container health checks%s", healthyCount, primaryDeployment.DesiredCount, revisionInfo)
			fmt.Fprintf(writer, "  %s\t%s\t%s\n", header, bar, stringSummary)
		}
	}

	// Write summary of capacity providers.
	if !allCapacityProviderEmpty(s.DesiredRunningTasks) {
		header := "Capacity Provider"
		fargate, spot, empty := capacityProviderDataForTasks(s.DesiredRunningTasks)
		bar := summaryBar([]int{fargate + empty, spot}, []string{color.Grey.Sprintf("▒"), color.Grey.Sprintf("▓")})
		var cpSummaries []string
		if fargate+empty != 0 {
			// We consider those with empty capacity provider field as "FARGATE"
			cpSummaries = append(cpSummaries, fmt.Sprintf("%d/%d on Fargate", fargate+empty, s.Service.RunningCount))
		}
		if spot != 0 {
			cpSummaries = append(cpSummaries, fmt.Sprintf("%d/%d on Fargate Spot", spot, s.Service.RunningCount))
		}
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", header, bar, strings.Join(cpSummaries, ", "))
	}
	writer.Flush()
}

func (s *ecsServiceStatus) writeStoppedTasks(writer *tabwriter.Writer) {
	if len(s.StoppedTasks) <= 0 {
		return
	}

	fmt.Fprint(writer, color.Bold.Sprint("\nStopped Tasks\n\n"))
	writer.Flush()

	headers := []string{"Reason", "Task Count", "Sample Task IDs"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))

	reasonToTasks := make(map[string][]string)
	for _, task := range s.StoppedTasks {
		reasonToTasks[task.StoppedReason] = append(reasonToTasks[task.StoppedReason], shortTaskID(task.ID))
	}
	for reason, ids := range reasonToTasks {
		sampleIDs := ids
		if len(sampleIDs) > 5 {
			sampleIDs = sampleIDs[:5]
		}
		printWithMaxWidth(writer, "  %s\t%s\t%s\n", 30, reason, strconv.Itoa(len(ids)), strings.Join(sampleIDs, ","))
	}
}

func (s *ecsServiceStatus) writeRunningTasks(writer *tabwriter.Writer) {
	if len(s.DesiredRunningTasks) == 0 {
		return
	}

	fmt.Fprint(writer, color.Bold.Sprint("\nTasks\n\n"))
	writer.Flush()

	shouldShowHTTPHealth := anyTaskATarget(s.DesiredRunningTasks, s.TasksTargetHealth)
	shouldShowCapacityProvider := !allCapacityProviderEmpty(s.DesiredRunningTasks)
	shouldShowContainerHealth := !allContainerHealthEmpty(s.DesiredRunningTasks)

	taskToHealth := summarizeHTTPHealthForTasks(s.TasksTargetHealth)

	headers := []string{"ID", "Status", "Revision", "Started At"}

	var opts []ecsTaskStatusConfigOpts
	if shouldShowCapacityProvider {
		opts = append(opts, withCapProviderShown)
		headers = append(headers, "Capacity")
	}

	if shouldShowContainerHealth {
		opts = append(opts, withContainerHealthShow)
		headers = append(headers, "Cont. Health")
	}

	if shouldShowHTTPHealth {
		headers = append(headers, "HTTP Health")
	}

	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, task := range s.DesiredRunningTasks {
		taskStatus := fmt.Sprint((ecsTaskStatus)(task).humanString(opts...))
		if shouldShowHTTPHealth {
			var httpHealthState string
			if states, ok := taskToHealth[task.ID]; !ok || len(states) == 0 {
				httpHealthState = "-"
			} else {
				// sometimes a task can have multiple target health states (although rare)
				httpHealthState = strings.Join(states, ",")
			}
			taskStatus = fmt.Sprintf("%s\t%s", taskStatus, strings.ToUpper(httpHealthState))
		}
		fmt.Fprintf(writer, "  %s\n", taskStatus)
	}
	writer.Flush()
}

func (s *ecsServiceStatus) writeAlarms(writer *tabwriter.Writer) {
	if len(s.Alarms) <= 0 {
		return
	}

	fmt.Fprint(writer, color.Bold.Sprint("\nAlarms\n\n"))
	writer.Flush()
	headers := []string{"Name", "Condition", "Last Updated", "Health"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, alarm := range s.Alarms {
		updatedTimeSince := humanizeTime(alarm.UpdatedTimes)
		printWithMaxWidth(writer, "  %s\t%s\t%s\t%s\n", maxAlarmStatusColumnWidth, alarm.Name, alarm.Condition, updatedTimeSince, alarmHealthColor(alarm.Status))
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\n", "", "", "", "")
	}
}

type ecsTaskStatus awsecs.TaskStatus

// Example output:
//   6ca7a60d          RUNNING             42            19 hours ago       -              UNKNOWN
func (ts ecsTaskStatus) humanString(opts ...ecsTaskStatusConfigOpts) string {
	config := &ecsTaskStatusConfig{}
	for _, opt := range opts {
		opt(config)
	}

	var statusString string

	shortID := "-"
	if ts.ID != "" {
		shortID = shortTaskID(ts.ID)
	}
	statusString += fmt.Sprint(shortID)
	statusString += fmt.Sprintf("\t%s", ts.LastStatus)

	revision := "-"
	v, err := awsecs.TaskDefinitionVersion(ts.TaskDefinition)
	if err == nil {
		revision = strconv.Itoa(v)
	}
	statusString += fmt.Sprintf("\t%s", revision)

	startedSince := "-"
	if !ts.StartedAt.IsZero() {
		startedSince = humanizeTime(ts.StartedAt)
	}
	statusString += fmt.Sprintf("\t%s", startedSince)

	if config.shouldShowCapProvider {
		cp := "FARGATE (Launch type)"
		if ts.CapacityProvider != "" {
			cp = ts.CapacityProvider
		}
		statusString += fmt.Sprintf("\t%s", cp)
	}

	if config.shouldShowContainerHealth {
		ch := "-"
		if ts.Health != "" {
			ch = ts.Health
		}
		statusString += fmt.Sprintf("\t%s", ch)
	}
	return statusString
}

type ecsTaskStatusConfigOpts func(config *ecsTaskStatusConfig)

type ecsTaskStatusConfig struct {
	shouldShowCapProvider     bool
	shouldShowContainerHealth bool
}

func withCapProviderShown(config *ecsTaskStatusConfig) {
	config.shouldShowCapProvider = true
}

func withContainerHealthShow(config *ecsTaskStatusConfig) {
	config.shouldShowContainerHealth = true
}

type taskTargetHealth struct {
	HealthStatus   elbv2.HealthStatus `json:"healthStatus"`
	TaskID         string             `json:"taskID"`
	TargetGroupARN string             `json:"targetGroup"`
}

// targetHealthForTasks finds the target health in a target group, if any, for each task.
func targetHealthForTasks(targetsHealth []*elbv2.TargetHealth, tasks []*awsecs.Task, targetGroupARN string) []taskTargetHealth {
	var out []taskTargetHealth

	// Create a set of target health to be matched against the tasks' private IP addresses.
	// An IP target's ID is the IP address.
	targetsHealthSet := make(map[string]*elbv2.TargetHealth)
	for _, th := range targetsHealth {
		targetsHealthSet[th.TargetID()] = th
	}

	// For each task, check if it is a target by matching its private IP address against targetsHealthSet.
	// If it is a target, we try to add it to the output.
	for _, task := range tasks {
		ip, err := task.PrivateIP()
		if err != nil {
			continue
		}

		// Check if the IP is a target
		th, ok := targetsHealthSet[ip]
		if !ok {
			continue
		}

		if taskID, err := awsecs.TaskID(aws.StringValue(task.TaskArn)); err == nil {
			out = append(out, taskTargetHealth{
				TaskID:         taskID,
				HealthStatus:   *th.HealthStatus(),
				TargetGroupARN: targetGroupARN,
			})
		}
	}

	return out
}

type valueWithIndex struct {
	value int
	index int
}

// summaryBar returns a summary bar given data and the string representations of each data category.
// For example, data[0] will be represented by representations[0] in the summary bar.
// If len(representations) < len(data), the default representation "□" is used for all data category with missing representation.
func summaryBar(data []int, representations []string, emptyRepresentation ...string) string {
	const summaryBarLength = 10
	defaultRepresentation := color.Grey.Sprint("░")

	// The index is recorded so that we can later output the summary bar in the original order.
	var dataWithIndices []valueWithIndex
	for idx, dt := range data {
		dataWithIndices = append(dataWithIndices, valueWithIndex{
			value: dt,
			index: idx,
		})
	}

	portionsWithIndices := calculatePortions(dataWithIndices, summaryBarLength)
	if portionsWithIndices == nil {
		return fmt.Sprint(strings.Repeat(defaultRepresentation, summaryBarLength))
	}

	sort.SliceStable(portionsWithIndices, func(i, j int) bool {
		return portionsWithIndices[i].index < portionsWithIndices[j].index
	})

	var bar string
	for _, p := range portionsWithIndices {
		if p.value >= summaryBarLength {
			// If a data category's portion exceeds the summary bar length (this happens only when the some of the data have negative value)
			// returns the bar filled with that data category
			bar += fmt.Sprint(strings.Repeat(representations[p.index], summaryBarLength))
			return bar
		}
		bar += fmt.Sprint(strings.Repeat(representations[p.index], p.value))
	}
	return bar
}

func calculatePortions(valuesWithIndices []valueWithIndex, length int) []valueWithIndex {
	type decWithPortion struct {
		dec     float64
		portion valueWithIndex
	}

	var sum int
	for _, pwi := range valuesWithIndices {
		sum += pwi.value
	}
	if sum == 0 {
		return nil
	}

	var decPartsToPortion []decWithPortion
	for _, pwi := range valuesWithIndices {
		// For each value, calculate its portion out of `length`, record the decimal part and then take the floor.
		// The floored result is roughly the value's portion out of `length`.
		// The portion will be calibrated later according to the decimal part.
		outOfLength := (float64)(pwi.value) / (float64)(sum) * (float64)(length)
		_, decPart := math.Modf(outOfLength)

		decPartsToPortion = append(decPartsToPortion, decWithPortion{
			dec: decPart,
			portion: valueWithIndex{
				value: (int)(math.Floor(outOfLength)),
				index: pwi.index,
			},
		})
	}

	// Calculate the sum of the floored portion and see how far we are from `length`.
	var floorSum int
	for _, floorPortion := range decPartsToPortion {
		floorSum += floorPortion.portion.value
	}
	extra := length - floorSum

	// Sort by decimal places from larger to smaller.
	sort.SliceStable(decPartsToPortion, func(i, j int) bool {
		return decPartsToPortion[i].dec > decPartsToPortion[j].dec
	})

	// Distribute extra values first to portions with larger decimal places.
	var out []valueWithIndex
	for _, d := range decPartsToPortion {
		if extra > 0 {
			d.portion.value += 1
			extra -= 1
		}
		out = append(out, d.portion)
	}

	return out
}

func shortTaskID(id string) string {
	if len(id) >= shortTaskIDLength {
		return id[:shortTaskIDLength]
	}
	return id
}

func summaryOfDeployment(deployment awsecs.Deployment, representations []string) (string, string) {
	var revisionInfo string
	revision, err := awsecs.TaskDefinitionVersion(deployment.TaskDefinition)
	if err == nil {
		revisionInfo = fmt.Sprintf(" (rev %d)", revision)
	}

	bar := summaryBar([]int{
		(int)(deployment.RunningCount),
		(int)(deployment.DesiredCount) - (int)(deployment.RunningCount)},
		representations)
	stringSummary := fmt.Sprintf("%d/%d running tasks for %s%s",
		deployment.RunningCount,
		deployment.DesiredCount,
		strings.ToLower(deployment.Status),
		revisionInfo)

	return bar, stringSummary
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
