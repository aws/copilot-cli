// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"sort"

	awsS3 "github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/s3"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/aas"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
)

const (
	fmtAppRunnerSvcLogGroupName = "/aws/apprunner/%s/%s/service"
	autoscalingAlarmType        = "Auto Scaling"
	rollbackAlarmType           = "Rollback"
)

type targetHealthGetter interface {
	TargetsHealth(targetGroupARN string) ([]*elbv2.TargetHealth, error)
}

type alarmStatusGetter interface {
	AlarmsWithTags(tags map[string]string) ([]cloudwatch.AlarmStatus, error)
	AlarmStatuses(...cloudwatch.DescribeAlarmOpts) ([]cloudwatch.AlarmStatus, error)
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

type autoscalingAlarmNamesGetter interface {
	ECSServiceAlarmNames(cluster, service string) ([]string, error)
}

type ecsStatusDescriber struct {
	app string
	env string
	svc string

	svcDescriber       serviceDescriber
	ecsSvcGetter       ecsServiceGetter
	cwSvcGetter        alarmStatusGetter
	aasSvcGetter       autoscalingAlarmNamesGetter
	targetHealthGetter targetHealthGetter
}

type appRunnerStatusDescriber struct {
	app string
	env string
	svc string

	svcDescriber apprunnerDescriber
	eventsGetter logGetter
}

type staticSiteStatusDescriber struct {
	app string
	env string
	svc string

	initS3Client func(string) (bucketDataGetter, bucketNameGetter, error)
}

// NewServiceStatusConfig contains fields that initiates ServiceStatus struct.
type NewServiceStatusConfig struct {
	App         string
	Env         string
	Svc         string
	ConfigStore ConfigStoreSvc
}

// NewECSStatusDescriber instantiates a new ecsStatusDescriber struct.
func NewECSStatusDescriber(opt *NewServiceStatusConfig) (*ecsStatusDescriber, error) {
	env, err := opt.ConfigStore.GetEnvironment(opt.App, opt.Env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", opt.Env, err)
	}
	sess, err := sessions.ImmutableProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("session for role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
	}
	return &ecsStatusDescriber{
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

// NewAppRunnerStatusDescriber instantiates a new appRunnerStatusDescriber struct.
func NewAppRunnerStatusDescriber(opt *NewServiceStatusConfig) (*appRunnerStatusDescriber, error) {
	appRunnerSvcDescriber, err := newAppRunnerServiceDescriber(NewServiceConfig{
		App:         opt.App,
		Env:         opt.Env,
		Svc:         opt.Svc,
		ConfigStore: opt.ConfigStore,
	})
	if err != nil {
		return nil, err
	}

	return &appRunnerStatusDescriber{
		app:          opt.App,
		env:          opt.Env,
		svc:          opt.Svc,
		svcDescriber: appRunnerSvcDescriber,
		eventsGetter: cloudwatchlogs.New(appRunnerSvcDescriber.sess),
	}, nil
}

// NewStaticSiteStatusDescriber instantiates a new staticSiteStatusDescriber struct.
func NewStaticSiteStatusDescriber(opt *NewServiceStatusConfig) (*staticSiteStatusDescriber, error) {
	describer := &staticSiteStatusDescriber{
		app: opt.App,
		env: opt.Env,
		svc: opt.Svc,
	}
	describer.initS3Client = func(env string) (bucketDataGetter, bucketNameGetter, error) {
		environment, err := opt.ConfigStore.GetEnvironment(opt.App, env)
		if err != nil {
			return nil, nil, fmt.Errorf("get environment %s: %w", env, err)
		}
		sess, err := sessions.ImmutableProvider().FromRole(environment.ManagerRoleARN, environment.Region)
		if err != nil {
			return nil, nil, err
		}
		return awsS3.New(sess), s3.New(sess), nil
	}
	return describer, nil
}

// Describe returns the status of an ECS service.
func (s *ecsStatusDescriber) Describe() (HumanJSONStringer, error) {
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
	// Using a map then converting it to a slice to avoid duplication.
	alarms := make(map[string]cloudwatch.AlarmStatus)
	taggedAlarms, err := s.cwSvcGetter.AlarmsWithTags(map[string]string{
		deploy.AppTagKey:     s.app,
		deploy.EnvTagKey:     s.env,
		deploy.ServiceTagKey: s.svc,
	})
	if err != nil {
		return nil, fmt.Errorf("get tagged CloudWatch alarms: %w", err)
	}
	for _, alarm := range taggedAlarms {
		alarms[alarm.Name] = alarm
	}
	autoscalingAlarms, err := s.ecsServiceAutoscalingAlarms(svcDesc.ClusterName, svcDesc.Name)
	if err != nil {
		return nil, err
	}
	for _, alarm := range autoscalingAlarms {
		alarms[alarm.Name] = alarm
	}
	rollbackAlarms, err := s.ecsServiceRollbackAlarms(s.app, s.env, s.svc)
	if err != nil {
		return nil, err
	}
	for _, alarm := range rollbackAlarms {
		alarms[alarm.Name] = alarm
	}
	alarmList := make([]cloudwatch.AlarmStatus, len(alarms))
	var i int
	for _, v := range alarms {
		alarmList[i] = v
		i++
	}
	// Sort by alarm type, then alarm name within type categories.
	sort.SliceStable(alarmList, func(i, j int) bool { return alarmList[i].Name < alarmList[j].Name })
	sort.SliceStable(alarmList, func(i, j int) bool { return alarmList[i].Type < alarmList[j].Type })
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
		Alarms:                   alarmList,
		StoppedTasks:             stoppedTaskStatus,
		TargetHealthDescriptions: tasksTargetHealth,
	}, nil
}

// Describe returns the status of an AppRunner service.
func (a *appRunnerStatusDescriber) Describe() (HumanJSONStringer, error) {
	svc, err := a.svcDescriber.Service()
	if err != nil {
		return nil, fmt.Errorf("get App Runner service description for App Runner service %s in environment %s: %w", a.svc, a.env, err)
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

// Describe returns the status of a Static Site service.
func (d *staticSiteStatusDescriber) Describe() (HumanJSONStringer, error) {
	dataGetter, nameGetter, err := d.initS3Client(d.env)
	if err != nil {
		return nil, err
	}
	bucketName, err := nameGetter.BucketName(d.app, d.env, d.svc)
	if err != nil {
		return nil, fmt.Errorf("get bucket name for %q Static Site service in %q environment: %w", d.svc, d.env, err)
	}
	size, count, err := dataGetter.BucketSizeAndCount(bucketName)
	if err != nil {
		return nil, fmt.Errorf("get size and count data for %q S3 bucket: %w", bucketName, err)
	}
	return &staticSiteServiceStatus{
		BucketName: bucketName,
		Size:       size,
		Count:      count,
	}, nil
}

func (s *ecsStatusDescriber) ecsServiceAutoscalingAlarms(cluster, service string) ([]cloudwatch.AlarmStatus, error) {
	alarmNames, err := s.aasSvcGetter.ECSServiceAlarmNames(cluster, service)
	if err != nil {
		return nil, fmt.Errorf("retrieve auto scaling alarm names for ECS service %s/%s: %w", cluster, service, err)
	}
	if len(alarmNames) == 0 {
		return nil, nil
	}
	alarms, err := s.cwSvcGetter.AlarmStatuses(cloudwatch.WithNames(alarmNames))
	if err != nil {
		return nil, fmt.Errorf("get auto scaling CloudWatch alarms: %w", err)
	}
	for i := range alarms {
		alarms[i].Type = autoscalingAlarmType
	}
	return alarms, nil
}

func (s *ecsStatusDescriber) ecsServiceRollbackAlarms(app, env, svc string) ([]cloudwatch.AlarmStatus, error) {
	// This will not fetch imported alarms, as we filter by the Copilot-generated prefix of alarm names. This will also not fetch Copilot-generated alarms with names exceeding 255 characters, due to the balanced truncating of `TruncateAlarmName`.
	alarms, err := s.cwSvcGetter.AlarmStatuses(cloudwatch.WithPrefix(fmt.Sprintf("%s-%s-%s-CopilotRollback", app, env, svc)))
	if err != nil {
		return nil, fmt.Errorf("get Copilot-created CloudWatch alarms: %w", err)
	}
	for i := range alarms {
		alarms[i].Type = rollbackAlarmType
	}
	return alarms, nil
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
