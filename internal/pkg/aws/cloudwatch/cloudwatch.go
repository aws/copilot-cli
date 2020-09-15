// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudwatch provides a client to make API requests to Amazon CloudWatch Service.
package cloudwatch

import (
	"fmt"
	"strings"
	"time"

	aas "github.com/aws/copilot-cli/internal/pkg/aws/applicationautoscaling"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

const (
	cloudwatchResourceType = "cloudwatch:alarm"
	compositeAlarmType     = "Composite"
	metricAlarmType        = "Metric"
)

type api interface {
	DescribeAlarms(input *cloudwatch.DescribeAlarmsInput) (*cloudwatch.DescribeAlarmsOutput, error)
}

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*rg.Resource, error)
}

type autoscalingAlarmNamesGetter interface {
	ECSServiceAlarmNames(cluster, service string) ([]string, error)
}

// CloudWatch wraps an Amazon CloudWatch client.
type CloudWatch struct {
	client api
	// Optional client.
	rgClient  resourceGetter
	assClient autoscalingAlarmNamesGetter
	// Optional client init.
	initRgClient  func()
	initAssclient func()
}

// AlarmStatus contains CloudWatch alarm status.
type AlarmStatus struct {
	Arn          string    `json:"arn"`
	Name         string    `json:"name"`
	Reason       string    `json:"reason"`
	Status       string    `json:"status"`
	Type         string    `json:"type"`
	UpdatedTimes time.Time `json:"updatedTimes"`
}

// New returns a CloudWatch struct configured against the input session.
func New(s *session.Session) *CloudWatch {
	cw := &CloudWatch{
		client: cloudwatch.New(s),
	}
	cw.initRgClient = func() {
		cw.rgClient = rg.New(s)
	}
	cw.initAssclient = func() {
		cw.assClient = aas.New(s)
	}
	return cw
}

// ECSServiceAutoscalingAlarms returns the CloudWatch alarms associated with the
// auto scaling policies attached to the ECS service.
func (cw *CloudWatch) ECSServiceAutoscalingAlarms(cluster, service string) ([]AlarmStatus, error) {
	cw.initAssclient()
	alarmNames, err := cw.assClient.ECSServiceAlarmNames(cluster, service)
	if err != nil {
		return nil, fmt.Errorf("retrieve auto scaling alarm names for ECS service %s/%s: %w", cluster, service, err)
	}
	return cw.alarmStatus(alarmNames)
}

// AlarmsWithTags returns all the CloudWatch alarms that have the resource tags.
func (cw *CloudWatch) AlarmsWithTags(tags map[string]string) ([]AlarmStatus, error) {
	cw.initRgClient()
	var alarmNames []string
	resources, err := cw.rgClient.GetResourcesByTags(cloudwatchResourceType, tags)
	if err != nil {
		return nil, err
	}
	for _, resource := range resources {
		name, err := getAlarmName(resource.ARN)
		if err != nil {
			return nil, err
		}
		alarmNames = append(alarmNames, aws.StringValue(name))
	}
	return cw.alarmStatus(alarmNames)
}

func (cw *CloudWatch) alarmStatus(alarms []string) ([]AlarmStatus, error) {
	if len(alarms) == 0 {
		return []AlarmStatus{}, nil
	}
	var alarmStatus []AlarmStatus
	var err error
	alarmResp := &cloudwatch.DescribeAlarmsOutput{}
	for {
		alarmResp, err = cw.client.DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
			AlarmNames: aws.StringSlice(alarms),
			NextToken:  alarmResp.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describe CloudWatch alarms: %w", err)
		}
		alarmStatus = append(alarmStatus, cw.compositeAlarmsStatus(alarmResp.CompositeAlarms)...)
		alarmStatus = append(alarmStatus, cw.metricAlarmsStatus(alarmResp.MetricAlarms)...)
		if alarmResp.NextToken == nil {
			break
		}
	}
	return alarmStatus, nil
}

func (cw *CloudWatch) compositeAlarmsStatus(alarms []*cloudwatch.CompositeAlarm) []AlarmStatus {
	var alarmStatusList []AlarmStatus
	for _, alarm := range alarms {
		alarmStatusList = append(alarmStatusList, AlarmStatus{
			Arn:          aws.StringValue(alarm.AlarmArn),
			Name:         aws.StringValue(alarm.AlarmName),
			Reason:       aws.StringValue(alarm.StateReason),
			Status:       aws.StringValue(alarm.StateValue),
			Type:         compositeAlarmType,
			UpdatedTimes: *alarm.StateUpdatedTimestamp,
		})
	}
	return alarmStatusList
}

func (cw *CloudWatch) metricAlarmsStatus(alarms []*cloudwatch.MetricAlarm) []AlarmStatus {
	var alarmStatusList []AlarmStatus
	for _, alarm := range alarms {
		alarmStatusList = append(alarmStatusList, AlarmStatus{
			Arn:          aws.StringValue(alarm.AlarmArn),
			Name:         aws.StringValue(alarm.AlarmName),
			Reason:       aws.StringValue(alarm.StateReason),
			Status:       aws.StringValue(alarm.StateValue),
			Type:         metricAlarmType,
			UpdatedTimes: *alarm.StateUpdatedTimestamp,
		})
	}
	return alarmStatusList
}

// getAlarmName gets the alarm name given a specific alarm ARN.
// For example: arn:aws:cloudwatch:us-west-2:1234567890:alarm:SDc-ReadCapacityUnitsLimit-BasicAlarm
// returns SDc-ReadCapacityUnitsLimit-BasicAlarm
func getAlarmName(alarmArn string) (*string, error) {
	resp, err := arn.Parse(alarmArn)
	if err != nil {
		return nil, fmt.Errorf("parse alarm ARN %s: %w", alarmArn, err)
	}
	alarmNameList := strings.Split(resp.Resource, ":")
	if len(alarmNameList) != 2 {
		return nil, fmt.Errorf("cannot parse alarm ARN resource %s", resp.Resource)
	}
	return aws.String(alarmNameList[1]), nil
}
