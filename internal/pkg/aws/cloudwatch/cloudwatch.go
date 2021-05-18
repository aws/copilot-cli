// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudwatch provides a client to make API requests to Amazon CloudWatch Service.
package cloudwatch

import (
	"fmt"
	"strings"
	"time"

	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/dustin/go-humanize"
)

const (
	cloudwatchResourceType = "cloudwatch:alarm"
	compositeAlarmType     = "Composite"
	metricAlarmType        = "Metric"
)

// humanizeDuration is overridden in tests so that its output is constant as time passes.
var humanizeDuration = humanize.RelTime

type api interface {
	DescribeAlarms(input *cloudwatch.DescribeAlarmsInput) (*cloudwatch.DescribeAlarmsOutput, error)
}

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*rg.Resource, error)
}

// CloudWatch wraps an Amazon CloudWatch client.
type CloudWatch struct {
	client   api
	rgClient resourceGetter
}

// AlarmStatus contains CloudWatch alarm status.
type AlarmStatus struct {
	Arn          string    `json:"arn"`
	Name         string    `json:"name"`
	Condition    string    `json:"condition"`
	Status       string    `json:"status"`
	Type         string    `json:"type"`
	UpdatedTimes time.Time `json:"updatedTimes"`
}

// New returns a CloudWatch struct configured against the input session.
func New(s *session.Session) *CloudWatch {
	return &CloudWatch{
		client:   cloudwatch.New(s),
		rgClient: rg.New(s),
	}
}

// AlarmsWithTags returns all the CloudWatch alarms that have the resource tags.
func (cw *CloudWatch) AlarmsWithTags(tags map[string]string) ([]AlarmStatus, error) {
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
		alarmNames = append(alarmNames, name)
	}
	return cw.AlarmStatus(alarmNames)
}

// AlarmStatus returns the status of each given alarm name.
func (cw *CloudWatch) AlarmStatus(alarms []string) ([]AlarmStatus, error) {
	if len(alarms) == 0 {
		return nil, nil
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
		if alarm == nil {
			continue
		}
		alarmStatusList = append(alarmStatusList, AlarmStatus{
			Arn:          aws.StringValue(alarm.AlarmArn),
			Name:         aws.StringValue(alarm.AlarmName),
			Condition:    aws.StringValue(alarm.AlarmRule),
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
		if alarm == nil {
			continue
		}
		metricAlarm := metricAlarm(*alarm)
		alarmStatusList = append(alarmStatusList, AlarmStatus{
			Arn:          aws.StringValue(metricAlarm.AlarmArn),
			Name:         aws.StringValue(metricAlarm.AlarmName),
			Condition:    metricAlarm.condition(),
			Status:       aws.StringValue(metricAlarm.StateValue),
			Type:         metricAlarmType,
			UpdatedTimes: *metricAlarm.StateUpdatedTimestamp,
		})
	}
	return alarmStatusList
}

// getAlarmName gets the alarm name given a specific alarm ARN.
// For example: arn:aws:cloudwatch:us-west-2:1234567890:alarm:SDc-ReadCapacityUnitsLimit-BasicAlarm
// returns SDc-ReadCapacityUnitsLimit-BasicAlarm
func getAlarmName(alarmARN string) (string, error) {
	resp, err := arn.Parse(alarmARN)
	if err != nil {
		return "", fmt.Errorf("parse alarm ARN %s: %w", alarmARN, err)
	}
	alarmNameList := strings.Split(resp.Resource, ":")
	if len(alarmNameList) != 2 {
		return "", fmt.Errorf("unknown ARN resource format %s", resp.Resource)
	}
	return alarmNameList[1], nil
}
