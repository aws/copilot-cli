// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/progress/summarybar"

	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/term/color"

	fcolor "github.com/fatih/color"
)

const (
	maxAlarmStatusColumnWidth = 30
	defaultServiceLogsLimit   = 10
	shortTaskIDLength         = 8
	summaryBarWidth           = 10
	emptyRep                  = "░"
)

var (
	summaryBarWidthConfig    = summarybar.WithWidth(summaryBarWidth)
	summaryBarEmptyRepConfig = summarybar.WithEmptyRep(emptyRep)
)

// ecsServiceStatus contains the status for an ECS service.
type ecsServiceStatus struct {
	Service                  awsecs.ServiceStatus
	DesiredRunningTasks      []awsecs.TaskStatus      `json:"tasks"`
	Alarms                   []cloudwatch.AlarmStatus `json:"alarms"`
	StoppedTasks             []awsecs.TaskStatus      `json:"stoppedTasks"`
	TargetHealthDescriptions []taskTargetHealth       `json:"targetHealthDescriptions"`
}

// appRunnerServiceStatus contains the status for an App Runner service.
type appRunnerServiceStatus struct {
	Service   apprunner.Service
	LogEvents []*cloudwatchlogs.Event
}

// staticSiteServiceStatus contains the status for a Static Site service.
type staticSiteServiceStatus struct {
	BucketName string `json:"bucketName"`
	Size       string `json:"totalSize"`
	Count      int    `json:"totalObjects"`
}

type taskTargetHealth struct {
	HealthStatus   elbv2.HealthStatus `json:"healthStatus"`
	TaskID         string             `json:"taskID"` // TaskID is empty if the target cannot be traced to a task.
	TargetGroupARN string             `json:"targetGroup"`
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

// JSONString returns the stringified staticSiteServiceStatus struct with json format.
func (s *staticSiteServiceStatus) JSONString() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal services: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified ecsServiceStatus struct in human-readable format.
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

// HumanString returns the stringified appRunnerServiceStatus struct in human-readable format.
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

// HumanString returns the stringified staticSiteServiceStatus struct in human-readable format.
func (s *staticSiteServiceStatus) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, statusCellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("Bucket Summary\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  Bucket Name     %s\n", s.BucketName)
	fmt.Fprintf(writer, "  Total Objects   %s\n", strconv.Itoa(s.Count))
	fmt.Fprintf(writer, "  Total Size      %s\n", s.Size)
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
	// By default, we want to show the primary running task vs. primary desired tasks.
	data := []summarybar.Datum{
		{
			Value:          (int)(s.Service.RunningCount),
			Representation: color.Green.Sprint("█"),
		},
		{
			Value:          (int)(s.Service.DesiredCount) - (int)(s.Service.RunningCount),
			Representation: color.Green.Sprint("░"),
		},
	}
	// If there is one or more active deployments, show the primary running tasks vs. active running tasks instead.
	if len(activeDeployments) > 0 {
		var runningPrimary, runningActive int
		for _, d := range activeDeployments {
			runningActive += (int)(d.RunningCount)
		}
		runningPrimary = (int)(primaryDeployment.RunningCount)
		data = []summarybar.Datum{
			{
				Value:          runningPrimary,
				Representation: color.Green.Sprint("█"),
			},
			{
				Value:          runningActive,
				Representation: color.Blue.Sprint("█"),
			},
		}
	}
	renderer := summarybar.New(data, summaryBarWidthConfig, summaryBarEmptyRepConfig)
	fmt.Fprintf(writer, "  %s\t", "Running")
	_, _ = renderer.Render(writer)
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

	s.writeDeployment(writer, primaryDeployment, color.Green)
	for _, deployment := range activeDeployments {
		fmt.Fprint(writer, "  \t")
		s.writeDeployment(writer, deployment, color.Blue)
	}
}

func (s *ecsServiceStatus) writeDeployment(writer io.Writer, deployment awsecs.Deployment, repColor *fcolor.Color) {
	var revisionInfo string
	revision, err := awsecs.TaskDefinitionVersion(deployment.TaskDefinition)
	if err == nil {
		revisionInfo = fmt.Sprintf(" (rev %d)", revision)
	}

	data := []summarybar.Datum{
		{
			Value:          (int)(deployment.RunningCount),
			Representation: repColor.Sprint("█"),
		},
		{
			Value:          (int)(deployment.DesiredCount) - (int)(deployment.RunningCount),
			Representation: repColor.Sprint("░"),
		},
	}
	renderer := summarybar.New(data, summaryBarWidthConfig, summaryBarEmptyRepConfig)
	_, _ = renderer.Render(writer)
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

	var revisionInfo string
	if len(activeDeployments) > 0 {
		revisionInfo = fmt.Sprintf(" (rev %d)", revision)
	}

	header := "Health"
	if shouldShowHTTPHealth {
		healthyCount := countHealthyHTTPTasks(primaryTasks, s.TargetHealthDescriptions)

		data := []summarybar.Datum{
			{
				Value:          healthyCount,
				Representation: color.Green.Sprint("█"),
			},
			{
				Value:          (int)(primaryDeployment.DesiredCount) - healthyCount,
				Representation: color.Green.Sprint("░"),
			},
		}
		renderer := summarybar.New(data, summaryBarWidthConfig, summaryBarEmptyRepConfig)
		fmt.Fprintf(writer, "  %s\t", "Health")
		_, _ = renderer.Render(writer)
		stringSummary := fmt.Sprintf("%d/%d passes HTTP health checks%s", healthyCount, primaryDeployment.DesiredCount, revisionInfo)
		fmt.Fprintf(writer, "\t%s\n", stringSummary)
		header = ""
	}

	if shouldShowContainerHealth {
		healthyCount, _, _ := containerHealthBreakDownByCount(primaryTasks)

		data := []summarybar.Datum{
			{
				Value:          healthyCount,
				Representation: color.Green.Sprint("█"),
			},
			{
				Value:          (int)(primaryDeployment.DesiredCount) - healthyCount,
				Representation: color.Green.Sprint("░"),
			},
		}

		renderer := summarybar.New(data, summaryBarWidthConfig, summaryBarEmptyRepConfig)
		fmt.Fprintf(writer, "  %s\t", header)
		_, _ = renderer.Render(writer)
		stringSummary := fmt.Sprintf("%d/%d passes container health checks%s", healthyCount, primaryDeployment.DesiredCount, revisionInfo)
		fmt.Fprintf(writer, "\t%s\n", stringSummary)
	}
}

func (s *ecsServiceStatus) writeCapacityProvidersSummary(writer io.Writer) {
	if !isCapacityProvidersEnabled(s.DesiredRunningTasks) {
		return
	}

	fargate, spot, empty := runningCapacityProvidersBreakDownByCount(s.DesiredRunningTasks)
	data := []summarybar.Datum{
		{
			Value:          fargate + empty,
			Representation: color.Grey.Sprintf("▒"),
		},
		{
			Value:          spot,
			Representation: color.Grey.Sprintf("▓"),
		},
	}
	renderer := summarybar.New(data, summaryBarWidthConfig, summaryBarEmptyRepConfig)
	fmt.Fprintf(writer, "  %s\t", "Capacity Provider")
	_, _ = renderer.Render(writer)

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
	headers := []string{"Name", "Type", "Condition", "Last Updated", "Health"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, alarm := range s.Alarms {
		updatedTimeSince := humanizeTime(alarm.UpdatedTimes)
		printWithMaxWidth(writer, "  %s\t%s\t%s\t%s\t%s\n", maxAlarmStatusColumnWidth, alarm.Name, alarm.Type, alarm.Condition, updatedTimeSince, alarmHealthColor(alarm.Status))
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\n", "", "", "", "", "")
	}
}

type ecsTaskStatus awsecs.TaskStatus

// Example output:
//
//	6ca7a60d          RUNNING             42            19 hours ago       -              UNKNOWN
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
