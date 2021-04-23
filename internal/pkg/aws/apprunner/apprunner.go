// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package apprunner provides a client to make API requests to AppRunner Service.
package apprunner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/new-sdk-go/apprunner"
)

const (
	fmtAppRunnerServiceLogGroupName     = "/aws/apprunner/%s/%s/service"
	fmtAppRunnerApplicationLogGroupName = "/aws/apprunner/%s/%s/application"
)

type api interface {
	DescribeService(input *apprunner.DescribeServiceInput) (*apprunner.DescribeServiceOutput, error)
	ListServices(input *apprunner.ListServicesInput) (*apprunner.ListServicesOutput, error)
}

// AppRunner wraps an AWS AppRunner client.
type AppRunner struct {
	client api
}

// New returns a Service configured against the input session.
func New(s *session.Session) *AppRunner {
	return &AppRunner{
		// TODO: remove endpoint override
		client: apprunner.New(s, &aws.Config{Endpoint: aws.String("https://fusion.gamma.us-east-1.bullet.aws.dev")}),
	}
}

// DescribeService returns a description of an AppRunner service given its ARN.
func (a *AppRunner) DescribeService(svcARN string) (*Service, error) {
	resp, err := a.client.DescribeService(&apprunner.DescribeServiceInput{
		ServiceArn: aws.String(svcARN),
	})
	if err != nil {
		return nil, fmt.Errorf("describe service %s: %w", svcARN, err)
	}
	var envVars []*EnvironmentVariable
	for k, v := range resp.Service.SourceConfiguration.ImageRepository.ImageConfiguration.RuntimeEnvironmentVariables {
		envVars = append(envVars, &EnvironmentVariable{
			Name:  k,
			Value: aws.StringValue(v),
		})
	}
	sort.SliceStable(envVars, func(i int, j int) bool { return envVars[i].Name < envVars[j].Name })

	return &Service{
		ServiceARN:           aws.StringValue(resp.Service.ServiceArn),
		Name:                 aws.StringValue(resp.Service.ServiceName),
		ID:                   aws.StringValue(resp.Service.ServiceId),
		Status:               aws.StringValue(resp.Service.Status),
		ServiceURL:           aws.StringValue(resp.Service.ServiceUrl),
		DateCreated:          *resp.Service.CreatedAt,
		DateUpdated:          *resp.Service.UpdatedAt,
		EnvironmentVariables: envVars,
		CPU:                  *resp.Service.InstanceConfiguration.Cpu,
		Memory:               *resp.Service.InstanceConfiguration.Memory,
		Port:                 *resp.Service.SourceConfiguration.ImageRepository.ImageConfiguration.Port,
	}, nil
}

// ServiceARN returns the ARN of an AppRunner service given its service name.
func (a *AppRunner) ServiceARN(svc string) (string, error) {
	var nextToken *string
	for {
		resp, err := a.client.ListServices(&apprunner.ListServicesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return "", fmt.Errorf("list AppRunner services: %w", err)
		}
		for _, service := range resp.ServiceSummaryList {
			if aws.StringValue(service.ServiceName) == svc {
				return aws.StringValue(service.ServiceArn), nil
			}
		}
		if resp.NextToken == nil {
			break
		}
		nextToken = resp.NextToken
	}
	return "", fmt.Errorf("no AppRunner service found for %s", svc)
}

// ParseServiceName returns the service name.
// For example: arn:aws:apprunner:us-west-2:1234567890:service/my-service/fc1098ac269245959ba78fd58bdd4bf
// will return my-service
func ParseServiceName(svcARN string) (string, error) {
	parsedARN, err := arn.Parse(svcARN)
	if err != nil {
		return "", err
	}
	resources := strings.Split(parsedARN.Resource, "/")
	if len(resources) != 3 {
		return "", fmt.Errorf("cannot parse resource for ARN %s", svcARN)
	}
	return resources[1], nil
}

// ParseServiceID returns the service id.
// For example: arn:aws:apprunner:us-west-2:1234567890:service/my-service/fc1098ac269245959ba78fd58bdd4bf
// will return fc1098ac269245959ba78fd58bdd4bf
func ParseServiceID(svcARN string) (string, error) {
	parsedARN, err := arn.Parse(svcARN)
	if err != nil {
		return "", err
	}
	resources := strings.Split(parsedARN.Resource, "/")
	if len(resources) != 3 {
		return "", fmt.Errorf("cannot parse resource for ARN %s", svcARN)
	}
	return resources[2], nil
}

// LogGroupName returns the log group name given the app runner service's name.
// An application log group is formatted as "/aws/apprunner/<svcName>/<svcID>/application".
func LogGroupName(svcARN string) (string, error) {
	svcName, err := ParseServiceName(svcARN)
	if err != nil {
		return "", fmt.Errorf("get service name: %w", err)
	}
	svcID, err := ParseServiceID(svcARN)
	if err != nil {
		return "", fmt.Errorf("get service id: %w", err)
	}
	return fmt.Sprintf(fmtAppRunnerApplicationLogGroupName, svcName, svcID), nil
}

// SystemLogGroupName returns the service log group name given the app runner service's name.
// A service log group is formatted as "/aws/apprunner/<svcName>/<svcID>/service".
func SystemLogGroupName(svcARN string) (string, error) {
	svcName, err := ParseServiceName(svcARN)
	if err != nil {
		return "", fmt.Errorf("get service name: %w", err)
	}
	svcID, err := ParseServiceID(svcARN)
	if err != nil {
		return "", fmt.Errorf("get service id: %w", err)
	}
	return fmt.Sprintf(fmtAppRunnerServiceLogGroupName, svcName, svcID), nil
}
