// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package aas provides a client to make API requests to Application Auto Scaling.
package aas

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	aas "github.com/aws/aws-sdk-go/service/applicationautoscaling"

	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	// ECS service resource ID format: service/${clusterName}/${serviceName}.
	fmtECSResourceID    = "service/%s/%s"
	ecsServiceNamespace = "ecs"
)

type api interface {
	DescribeScalingPolicies(input *aas.DescribeScalingPoliciesInput) (*aas.DescribeScalingPoliciesOutput, error)
}

// ApplicationAutoscaling wraps an Amazon Application Auto Scaling client.
type ApplicationAutoscaling struct {
	client api
}

// New returns a ApplicationAutoscaling struct configured against the input session.
func New(s *session.Session) *ApplicationAutoscaling {
	return &ApplicationAutoscaling{
		client: aas.New(s),
	}
}

// ECSServiceAlarmNames returns names of the CloudWatch alarms associated with the
// scaling policies attached to the ECS service.
func (a *ApplicationAutoscaling) ECSServiceAlarmNames(cluster, service string) ([]string, error) {
	resourceID := fmt.Sprintf(fmtECSResourceID, cluster, service)
	var alarms []string
	var err error
	resp := &aas.DescribeScalingPoliciesOutput{}
	for {
		resp, err = a.client.DescribeScalingPolicies(&aas.DescribeScalingPoliciesInput{
			ResourceId:       aws.String(resourceID),
			ServiceNamespace: aws.String(ecsServiceNamespace),
			NextToken:        resp.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describe scaling policies for ECS service %s/%s: %w", cluster, service, err)
		}
		for _, policy := range resp.ScalingPolicies {
			for _, alarm := range policy.Alarms {
				alarms = append(alarms, aws.StringValue(alarm.AlarmName))
			}
		}
		if resp.NextToken == nil {
			break
		}
	}
	return alarms, nil
}
