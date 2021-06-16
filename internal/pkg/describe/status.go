// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/progress"

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
	ServiceRunningTasks(clusterName, serviceName string) ([]*awsecs.Task, error)
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
	Service                  awsecs.ServiceStatus
	DesiredRunningTasks      []awsecs.TaskStatus      `json:"tasks"`
	Alarms                   []cloudwatch.AlarmStatus `json:"alarms"`
	StoppedTasks             []awsecs.TaskStatus      `json:"stoppedTasks"`
	TargetHealthDescriptions []taskTargetHealth       `json:"targetHealthDescriptions"`

	rendererConfigurer rendererConfigurer // The Renderer interface is not mock friendly. We abstract away the code with an intermediate interface so that it can be mocked.
}

type rendererConfigurer interface {
	SummaryBarRenderer(length int, data []int, representations []string, emptyRepresentation string) (progress.Renderer, error)
}

type barRendererConfigurer struct{}

// SummaryBarRenderer configures and returns a summary bar renderer.
func (c *barRendererConfigurer) SummaryBarRenderer(length int, data []int, representations []string, emptyRepresentation string) (progress.Renderer, error) {
	renderer, err := progress.NewSummaryBarComponent(length, data, representations, emptyRepresentation)
	if err != nil {
		return nil, fmt.Errorf("set up summary bar renderer: %w", err)
	}
	return renderer, nil
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

type taskTargetHealth struct {
	HealthStatus   elbv2.HealthStatus `json:"healthStatus"`
	TaskID         string             `json:"taskID"` // TaskID is empty if the target cannot be traced to a task.
	TargetGroupARN string             `json:"targetGroup"`
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
		Service:                  service.ServiceStatus(),
		DesiredRunningTasks:      taskStatus,
		Alarms:                   alarms,
		StoppedTasks:             stoppedTaskStatus,
		TargetHealthDescriptions: tasksTargetHealth,
		rendererConfigurer:       &barRendererConfigurer{},
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

	fmt.Fprint(writer, color.Bold.Sprint("Task Summary\n\n"))
	writer.Flush()
	s.writeTaskSummary(writer)
	writer.Flush()

	if len(s.StoppedTasks) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nStopped Tasks\n\n"))
		writer.Flush()
		s.writeStoppedTasks(writer)
		writer.Flush()
	}

	if len(s.DesiredRunningTasks) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nTasks\n\n"))
		writer.Flush()
		s.writeRunningTasks(writer)
		writer.Flush()
	}

	if len(s.Alarms) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nAlarms\n\n"))
		writer.Flush()
		s.writeAlarms(writer)
		writer.Flush()
	}
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

func (s *ecsServiceStatus) writeTaskSummary(writer io.Writer) {
	// NOTE: all the `bar` need to be fully colored. Observe how all the second parameter for all `summaryBar` function
	// is a list of strings that are colored (e.g. `[]string{color.Green.Sprint("■"), color.Grey.Sprint("□")}`)
	// This is because if the some of the bar is partially colored, tab writer will behave unexpectedly.
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

	s.writeRunningTasksSummary(writer, primaryDeployment, activeDeployments)
	s.writeDeploymentsSummary(writer, primaryDeployment, activeDeployments)
	s.writeHealthSummary(writer, primaryDeployment, activeDeployments)
	s.writeCapacityProvidersSummary(writer)
}

func (s *ecsServiceStatus) writeRunningTasksSummary(writer io.Writer, primaryDeployment awsecs.Deployment, activeDeployments []awsecs.Deployment) {
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

	renderer, err := s.rendererConfigurer.SummaryBarRenderer(10, barData, barRepresentations, color.Green.Sprint("░"))
	if err != nil {
		return
	}
	fmt.Fprintf(writer, "  %s\t", header)
	if _, err := renderer.Render(writer); err != nil {
		fmt.Fprintf(writer, strings.Repeat(" ", 10))
	}
	stringSummary := fmt.Sprintf("%d/%d desired tasks are running", s.Service.RunningCount, s.Service.DesiredCount)
	fmt.Fprintf(writer, "\t%s\n", stringSummary)
}

func (s *ecsServiceStatus) writeDeploymentsSummary(writer io.Writer, primaryDeployment awsecs.Deployment, activeDeployments []awsecs.Deployment) {
	if len(activeDeployments) <= 0 {
		return
	}

	// Show "Deployments" section only if there are "ACTIVE" deployments in addition to the "PRIMARY" deployment.
	// This is because if there aren't any "ACTIVE" deployment, then this section would have been showing the same
	// information as the "Running" section.
	header := "Deployments"
	fmt.Fprintf(writer, "  %s\t", header)
	s.writeDeployment(writer, primaryDeployment, []string{color.Green.Sprint("█"), color.Green.Sprint("░")})

	for _, deployment := range activeDeployments {
		fmt.Fprint(writer, "  \t")
		s.writeDeployment(writer, deployment, []string{color.Blue.Sprint("█"), color.Blue.Sprint("░")})
	}
}

func (s *ecsServiceStatus) writeDeployment(writer io.Writer, deployment awsecs.Deployment, representations []string) {
	var revisionInfo string
	revision, err := awsecs.TaskDefinitionVersion(deployment.TaskDefinition)
	if err == nil {
		revisionInfo = fmt.Sprintf(" (rev %d)", revision)
	}

	renderer, err := s.rendererConfigurer.SummaryBarRenderer(10, []int{(int)(deployment.RunningCount), (int)(deployment.DesiredCount) - (int)(deployment.RunningCount)}, representations, "")
	if err != nil {
		return
	}
	if _, err := renderer.Render(writer); err != nil {
		fmt.Fprintf(writer, strings.Repeat(" ", 10))
	}

	stringSummary := fmt.Sprintf("%d/%d running tasks for %s%s",
		deployment.RunningCount,
		deployment.DesiredCount,
		strings.ToLower(deployment.Status),
		revisionInfo)
	fmt.Fprintf(writer, "\t%s\n", stringSummary)
}

func (s *ecsServiceStatus) writeHealthSummary(writer io.Writer, primaryDeployment awsecs.Deployment, activeDeployments []awsecs.Deployment) {
	revision, _ := awsecs.TaskDefinitionVersion(primaryDeployment.TaskDefinition)
	primaryTasks := s.tasksOfRevision(revision)

	shouldShowHTTPHealth := anyTasksInAnyTargetGroup(primaryTasks, s.TargetHealthDescriptions)
	shouldShowContainerHealth := isContainerHealthCheckEnabled(primaryTasks)
	if !shouldShowHTTPHealth && !shouldShowContainerHealth {
		return
	}

	header := "Health"

	var revisionInfo string
	if len(activeDeployments) > 0 {
		revisionInfo = fmt.Sprintf(" (rev %d)", revision)
	}

	if shouldShowHTTPHealth {
		healthyCount := countHealthyHTTPTasks(primaryTasks, s.TargetHealthDescriptions)
		renderer, err := s.rendererConfigurer.SummaryBarRenderer(10,
			[]int{healthyCount, (int)(primaryDeployment.DesiredCount) - healthyCount},
			[]string{color.Green.Sprint("█"), color.Green.Sprint("░")},
			color.Grey.Sprintf("░"))
		if err != nil {
			return
		}

		fmt.Fprintf(writer, "  %s\t", header)
		if _, err := renderer.Render(writer); err != nil {
			fmt.Fprintf(writer, strings.Repeat(" ", 10))
		}
		stringSummary := fmt.Sprintf("%d/%d passes HTTP health checks%s", healthyCount, primaryDeployment.DesiredCount, revisionInfo)
		fmt.Fprintf(writer, "\t%s\n", stringSummary)
		header = ""
	}

	if shouldShowContainerHealth {
		healthyCount, _, _ := containerHealthBreakDownByCount(primaryTasks)
		renderer, err := s.rendererConfigurer.SummaryBarRenderer(10,
			[]int{healthyCount, (int)(primaryDeployment.DesiredCount) - healthyCount},
			[]string{color.Green.Sprint("█"), color.Green.Sprint("░")},
			color.Grey.Sprintf("░"))
		if err != nil {
			return
		}
		fmt.Fprintf(writer, "  %s\t", header)
		if _, err := renderer.Render(writer); err != nil {
			fmt.Fprintf(writer, strings.Repeat(" ", 10))
		}
		stringSummary := fmt.Sprintf("%d/%d passes container health checks%s", healthyCount, primaryDeployment.DesiredCount, revisionInfo)
		fmt.Fprintf(writer, "\t%s\n", stringSummary)
	}
}

func (s *ecsServiceStatus) writeCapacityProvidersSummary(writer io.Writer) {
	if !isCapacityProvidersEnabled(s.DesiredRunningTasks) {
		return
	}
	header := "Capacity Provider"

	fargate, spot, empty := runningCapacityProvidersBreakDownByCount(s.DesiredRunningTasks)
	renderer, err := s.rendererConfigurer.SummaryBarRenderer(10,
		[]int{fargate + empty, spot},
		[]string{color.Grey.Sprintf("▒"), color.Grey.Sprintf("▓")},
		color.Grey.Sprintf("░"))
	if err != nil {
		return
	}

	fmt.Fprintf(writer, "  %s\t", header)
	if _, err := renderer.Render(writer); err != nil {
		fmt.Fprintf(writer, strings.Repeat(" ", 10))
	}

	var cpSummaries []string
	if fargate+empty != 0 {
		// We consider those with empty capacity provider field as "FARGATE"
		cpSummaries = append(cpSummaries, fmt.Sprintf("%d/%d on Fargate", fargate+empty, s.Service.RunningCount))
	}
	if spot != 0 {
		cpSummaries = append(cpSummaries, fmt.Sprintf("%d/%d on Fargate Spot", spot, s.Service.RunningCount))
	}
	fmt.Fprintf(writer, "\t%s\n", strings.Join(cpSummaries, ", "))
}

func (s *ecsServiceStatus) writeStoppedTasks(writer io.Writer) {
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

func (s *ecsServiceStatus) writeRunningTasks(writer io.Writer) {
	shouldShowHTTPHealth := anyTasksInAnyTargetGroup(s.DesiredRunningTasks, s.TargetHealthDescriptions)
	shouldShowCapacityProvider := isCapacityProvidersEnabled(s.DesiredRunningTasks)
	shouldShowContainerHealth := isContainerHealthCheckEnabled(s.DesiredRunningTasks)

	taskToHealth := summarizeHTTPHealthForTasks(s.TargetHealthDescriptions)

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
}

func (s *ecsServiceStatus) writeAlarms(writer io.Writer) {
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

// targetHealthForTasks finds the corresponding task, if any, for each target health in a target group.
func targetHealthForTasks(targetsHealth []*elbv2.TargetHealth, tasks []*awsecs.Task, targetGroupARN string) []taskTargetHealth {
	var out []taskTargetHealth

	// Create a map from task's private IP address to task's ID to be matched against target's ID.
	ipToTaskID := make(map[string]string)
	for _, task := range tasks {
		ip, err := task.PrivateIP()
		if err != nil {
			continue
		}

		taskID, err := awsecs.TaskID(aws.StringValue(task.TaskArn))
		if err != nil {
			continue
		}

		ipToTaskID[ip] = taskID
	}

	// For each target, check if its health check is actually checking against one of the tasks.
	// If the target is an IP target, then its ID is the IP address.
	// If a task is running on that IP address, then effectively the target's health check is checking against that task.
	for _, th := range targetsHealth {
		targetID := th.TargetID()
		taskID, ok := ipToTaskID[targetID]
		if !ok {
			taskID = ""
		}
		out = append(out, taskTargetHealth{
			HealthStatus:   *th.HealthStatus(),
			TargetGroupARN: targetGroupARN,
			TaskID:         taskID,
		})
	}
	return out
}

func shortTaskID(id string) string {
	if len(id) >= shortTaskIDLength {
		return id[:shortTaskIDLength]
	}
	return id
}

func printWithMaxWidth(w io.Writer, format string, width int, members ...string) {
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
