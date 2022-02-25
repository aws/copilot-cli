// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"sort"

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
)

const fmtAppRunnerSvcLogGroupName = "/aws/apprunner/%s/%s/service"

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

type appRunnerServiceDescriber interface {
	Service() (*apprunner.Service, error)
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

	svcDescriber appRunnerServiceDescriber
	eventsGetter logGetter
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
	ecsSvcDescriber, err := NewAppRunnerServiceDescriber(NewServiceConfig{
		App: opt.App,
		Env: opt.Env,
		Svc: opt.Svc,

		ConfigStore: opt.ConfigStore,
	})
	if err != nil {
		return nil, err
	}

	return &appRunnerStatusDescriber{
		app:          opt.App,
		env:          opt.Env,
		svc:          opt.Svc,
		svcDescriber: ecsSvcDescriber,
		eventsGetter: cloudwatchlogs.New(ecsSvcDescriber.sess),
	}, nil
}

// Describe returns status of an ECS service.
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
	}, nil
}

func (s *ecsStatusDescriber) ecsServiceAutoscalingAlarms(cluster, service string) ([]cloudwatch.AlarmStatus, error) {
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

// Describe returns status of an AppRunner service.
func (a *appRunnerStatusDescriber) Describe() (HumanJSONStringer, error) {
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
