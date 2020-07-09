// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
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

// CloudWatch wraps an Amazon CloudWatch client.
type CloudWatch struct {
	cwClient api
	rgClient resourceGetter
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
	return &CloudWatch{
		cwClient: cloudwatch.New(s),
		rgClient: rg.New(s),
	}
}

// GetAlarmsWithTags returns all the CloudWatch alarms that have the resource tags.
func (cw *CloudWatch) GetAlarmsWithTags(tags map[string]string) ([]AlarmStatus, error) {
	var alarmNames []*string

	resources, err := cw.rgClient.GetResourcesByTags(cloudwatchResourceType, tags)
	if err != nil {
		return nil, err
	}

	for _, resource := range resources {
		name, err := cw.getAlarmName(resource.ARN)
		if err != nil {
			return nil, err
		}
		alarmNames = append(alarmNames, name)
	}

	// Return an empty array since DescribeAlarms will return all alarms if "AlarmNames" is an empty array.
	if len(alarmNames) == 0 {
		return []AlarmStatus{}, nil
	}
	var alarmStatus []AlarmStatus
	alarmResp := &cloudwatch.DescribeAlarmsOutput{}
	for {
		alarmResp, err = cw.cwClient.DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
			AlarmNames: alarmNames,
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

// getAlarmName gets the alarm name given a specific alarm ARN.
// For example: arn:aws:cloudwatch:us-west-2:1234567890:alarm:SDc-ReadCapacityUnitsLimit-BasicAlarm
// returns SDc-ReadCapacityUnitsLimit-BasicAlarm
func (cw *CloudWatch) getAlarmName(alarmArn string) (*string, error) {
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
