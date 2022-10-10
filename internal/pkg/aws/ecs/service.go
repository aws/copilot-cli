// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecs provides a client to make API requests to Amazon Elastic Container Service.
package ecs

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	ecs "github.com/aws/copilot-cli/internal/pkg/ecssdk"
)

const (
	// ServiceDeploymentStatusPrimary is the status PRIMARY of an ECS service deployment.
	ServiceDeploymentStatusPrimary = "PRIMARY"
	// ServiceDeploymentStatusActive is the status ACTIVE of an ECS service deployment.
	ServiceDeploymentStatusActive = "ACTIVE"
)

// Service wraps up ECS Service struct.
type Service ecs.Service

// Deployment contains information of a ECS service Deployment.
type Deployment struct {
	Id             string    `json:"id"`
	DesiredCount   int64     `json:"desiredCount"`
	RunningCount   int64     `json:"runningCount"`
	UpdatedAt      time.Time `json:"updatedAt"`
	LaunchType     string    `json:"launchType"`
	TaskDefinition string    `json:"taskDefinition"`
	Status         string    `json:"status"`
}

// ServiceStatus contains the status info of a service.
type ServiceStatus struct {
	DesiredCount     int64        `json:"desiredCount"`
	RunningCount     int64        `json:"runningCount"`
	Status           string       `json:"status"`
	Deployments      []Deployment `json:"deployments"`
	LastDeploymentAt time.Time    `json:"lastDeploymentAt"` // kept to avoid breaking change
	TaskDefinition   string       `json:"taskDefinition"`   // kept to avoid breaking change
}

// ServiceStatus returns the status of the running service.
func (s *Service) ServiceStatus() ServiceStatus {
	var deployments []Deployment
	for _, dp := range s.Deployments {
		deployments = append(deployments, Deployment{
			Id:             aws.StringValue(dp.Id),
			DesiredCount:   aws.Int64Value(dp.DesiredCount),
			RunningCount:   aws.Int64Value(dp.RunningCount),
			UpdatedAt:      aws.TimeValue(dp.UpdatedAt),
			LaunchType:     aws.StringValue(dp.LaunchType),
			TaskDefinition: aws.StringValue(dp.TaskDefinition),
			Status:         aws.StringValue(dp.Status),
		})
	}

	return ServiceStatus{
		Status:           aws.StringValue(s.Status),
		DesiredCount:     aws.Int64Value(s.DesiredCount),
		RunningCount:     aws.Int64Value(s.RunningCount),
		Deployments:      deployments,
		LastDeploymentAt: aws.TimeValue(s.Deployments[0].UpdatedAt), // FIXME Service assumed to have at least one deployment
		TaskDefinition:   aws.StringValue(s.Deployments[0].TaskDefinition),
	}
}

// ServiceConnectAliases returns the ECS Service Connect client aliases for a service.
func (s *Service) ServiceConnectAliases() []string {
	if len(s.Deployments) == 0 {
		return nil
	}
	lastDeployment := s.Deployments[0]
	scConfig := lastDeployment.ServiceConnectConfiguration
	if scConfig == nil || !aws.BoolValue(scConfig.Enabled) {
		return nil
	}
	ns := aws.StringValue(scConfig.Namespace)
	var aliases []string
	for _, service := range scConfig.Services {
		defaultName := aws.StringValue(service.PortName)
		if aws.StringValue(service.DiscoveryName) != "" {
			defaultName = aws.StringValue(service.DiscoveryName)
		}
		defaultAlias := fmt.Sprintf("%s.%s", defaultName, ns)
		if len(service.ClientAliases) == 0 {
			aliases = append(aliases, defaultAlias)
			continue
		}
		for _, clientAlias := range service.ClientAliases {
			alias := defaultAlias
			if aws.StringValue(clientAlias.DnsName) != "" {
				alias = aws.StringValue(clientAlias.DnsName)
			}
			aliases = append(aliases, fmt.Sprintf("%s:%v", alias, aws.Int64Value(clientAlias.Port)))
		}
	}
	return aliases
}

// LastUpdatedAt returns the last updated time of the ECS service.
func (s *Service) LastUpdatedAt() time.Time {
	return aws.TimeValue(s.Deployments[0].UpdatedAt)
}

// TargetGroups returns the ARNs of target groups attached to the service.
func (s *Service) TargetGroups() []string {
	var targetGroupARNs []string
	for _, lb := range s.LoadBalancers {
		targetGroupARNs = append(targetGroupARNs, aws.StringValue(lb.TargetGroupArn))
	}
	return targetGroupARNs
}

// ServiceArn is the arn of an ECS service.
type ServiceArn string

// ClusterName returns the cluster name.
// For example: arn:aws:ecs:us-west-2:1234567890:service/my-project-test-Cluster-9F7Y0RLP60R7/my-project-test-myService-JSOH5GYBFAIB
// will return my-project-test-Cluster-9F7Y0RLP60R7
func (s *ServiceArn) ClusterName() (string, error) {
	serviceArn := string(*s)
	parsedArn, err := arn.Parse(serviceArn)
	if err != nil {
		return "", err
	}
	resources := strings.Split(parsedArn.Resource, "/")
	if len(resources) != 3 {
		return "", fmt.Errorf("cannot parse resource for ARN %s", serviceArn)
	}
	return resources[1], nil
}

// ServiceName returns the service name.
// For example: arn:aws:ecs:us-west-2:1234567890:service/my-project-test-Cluster-9F7Y0RLP60R7/my-project-test-myService-JSOH5GYBFAIB
// will return my-project-test-myService-JSOH5GYBFAIB
func (s *ServiceArn) ServiceName() (string, error) {
	serviceArn := string(*s)
	parsedArn, err := arn.Parse(serviceArn)
	if err != nil {
		return "", err
	}
	resources := strings.Split(parsedArn.Resource, "/")
	if len(resources) != 3 {
		return "", fmt.Errorf("cannot parse resource for ARN %s", serviceArn)
	}
	return resources[2], nil
}

// NetworkConfiguration holds service's NetworkConfiguration.
type NetworkConfiguration struct {
	AssignPublicIp string
	SecurityGroups []string
	Subnets        []string
}
