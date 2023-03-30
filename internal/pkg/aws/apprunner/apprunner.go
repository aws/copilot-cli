// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package apprunner provides a client to make API requests to AppRunner Service.
package apprunner

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apprunner"
)

const (
	fmtAppRunnerServiceLogGroupName     = "/aws/apprunner/%s/%s/service"
	fmtAppRunnerApplicationLogGroupName = "/aws/apprunner/%s/%s/application"

	// App Runner Statuses
	opStatusSucceeded = "SUCCEEDED"
	opStatusFailed    = "FAILED"
	svcStatusPaused   = "PAUSED"
	svcStatusRunning  = "RUNNING"

	// App Runner ImageRepositoryTypes
	repositoryTypeECR       = "ECR"
	repositoryTypeECRPublic = "ECR_PUBLIC"

	// EndpointsID is the ID to look up the App Runner service endpoint.
	EndpointsID = apprunner.EndpointsID
)

type api interface {
	DescribeService(input *apprunner.DescribeServiceInput) (*apprunner.DescribeServiceOutput, error)
	ListOperations(input *apprunner.ListOperationsInput) (*apprunner.ListOperationsOutput, error)
	ListServices(input *apprunner.ListServicesInput) (*apprunner.ListServicesOutput, error)
	PauseService(input *apprunner.PauseServiceInput) (*apprunner.PauseServiceOutput, error)
	ResumeService(input *apprunner.ResumeServiceInput) (*apprunner.ResumeServiceOutput, error)
	StartDeployment(input *apprunner.StartDeploymentInput) (*apprunner.StartDeploymentOutput, error)
	DescribeObservabilityConfiguration(input *apprunner.DescribeObservabilityConfigurationInput) (*apprunner.DescribeObservabilityConfigurationOutput, error)
	DescribeVpcIngressConnection(input *apprunner.DescribeVpcIngressConnectionInput) (*apprunner.DescribeVpcIngressConnectionOutput, error)
}

// AppRunner wraps an AWS AppRunner client.
type AppRunner struct {
	client api
}

// New returns a Service configured against the input session.
func New(s *session.Session) *AppRunner {
	return &AppRunner{
		client: apprunner.New(s),
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

	var secrets []*EnvironmentSecret
	for k, v := range resp.Service.SourceConfiguration.ImageRepository.ImageConfiguration.RuntimeEnvironmentSecrets {
		secrets = append(secrets, &EnvironmentSecret{
			Name:  k,
			Value: aws.StringValue(v),
		})
	}
	sort.SliceStable(secrets, func(i int, j int) bool { return secrets[i].Name < secrets[j].Name })

	var observabilityConfiguration ObservabilityConfiguration
	if resp.Service.ObservabilityConfiguration != nil && aws.BoolValue(resp.Service.ObservabilityConfiguration.ObservabilityEnabled) {
		if out, err := a.client.DescribeObservabilityConfiguration(&apprunner.DescribeObservabilityConfigurationInput{
			ObservabilityConfigurationArn: resp.Service.ObservabilityConfiguration.ObservabilityConfigurationArn,
		}); err == nil {
			// NOTE: swallow the error otherwise, because observability is an optional description of the service.
			// Example error: when "EnvManagerRole" doesn't have the "apprunner:ObservabilityConfiguration" permission.
			observabilityConfiguration = ObservabilityConfiguration{
				TraceConfiguration: (*TraceConfiguration)(out.ObservabilityConfiguration.TraceConfiguration),
			}
		}
	}
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
		ImageID:              *resp.Service.SourceConfiguration.ImageRepository.ImageIdentifier,
		Port:                 *resp.Service.SourceConfiguration.ImageRepository.ImageConfiguration.Port,
		Observability:        observabilityConfiguration,
		EnvironmentSecrets:   secrets,
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

// PauseService pause the running App Runner service.
func (a *AppRunner) PauseService(svcARN string) error {
	resp, err := a.client.PauseService(&apprunner.PauseServiceInput{
		ServiceArn: aws.String(svcARN),
	})
	if err != nil {
		return fmt.Errorf("pause service operation failed: %w", err)
	}
	if resp.OperationId == nil && aws.StringValue(resp.Service.Status) == svcStatusPaused {
		return nil
	}
	if err := a.WaitForOperation(aws.StringValue(resp.OperationId), svcARN); err != nil {
		return err
	}
	return nil
}

// ResumeService resumes a paused App Runner service.
func (a *AppRunner) ResumeService(svcARN string) error {
	resp, err := a.client.ResumeService(&apprunner.ResumeServiceInput{
		ServiceArn: aws.String(svcARN),
	})
	if err != nil {
		return fmt.Errorf("resume service operation failed: %w", err)
	}
	if resp.OperationId == nil && aws.StringValue(resp.Service.Status) == svcStatusRunning {
		return nil
	}
	if err := a.WaitForOperation(aws.StringValue(resp.OperationId), svcARN); err != nil {
		return err
	}
	return nil
}

// StartDeployment initiates a manual deployment to an AWS App Runner service.
func (a *AppRunner) StartDeployment(svcARN string) (string, error) {
	out, err := a.client.StartDeployment(&apprunner.StartDeploymentInput{
		ServiceArn: aws.String(svcARN),
	})
	if err != nil {
		return "", fmt.Errorf("start new deployment: %w", err)
	}
	return aws.StringValue(out.OperationId), nil
}

// DescribeOperation return OperationSummary for given OperationId and ServiceARN.
func (a *AppRunner) DescribeOperation(operationId, svcARN string) (*apprunner.OperationSummary, error) {
	var nextToken *string
	for {
		resp, err := a.client.ListOperations(&apprunner.ListOperationsInput{
			ServiceArn: aws.String(svcARN),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list operations: %w", err)
		}
		for _, operation := range resp.OperationSummaryList {
			if aws.StringValue(operation.Id) == operationId {
				return operation, nil
			}
		}
		if resp.NextToken == nil {
			break
		}
		nextToken = resp.NextToken
	}
	return nil, fmt.Errorf("no operation found %s", operationId)
}

// WaitForOperation waits for a service operation.
func (a *AppRunner) WaitForOperation(operationId, svcARN string) error {
	for {
		resp, err := a.DescribeOperation(operationId, svcARN)
		if err != nil {
			return fmt.Errorf("error describing operation %s: %w", operationId, err)
		}
		switch status := aws.StringValue(resp.Status); status {
		case opStatusSucceeded:
			return nil
		case opStatusFailed:
			return &ErrWaitServiceOperationFailed{
				operationId: operationId,
			}
		}
		time.Sleep(3 * time.Second)
	}
}

// PrivateURL returns the url associated with a VPC Ingress Connection.
func (a *AppRunner) PrivateURL(vicARN string) (string, error) {
	resp, err := a.client.DescribeVpcIngressConnection(&apprunner.DescribeVpcIngressConnectionInput{
		VpcIngressConnectionArn: aws.String(vicARN),
	})
	if err != nil {
		return "", fmt.Errorf("describe vpc ingress connection %q: %w", vicARN, err)
	}

	return aws.StringValue(resp.VpcIngressConnection.DomainName), nil
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

// ImageIsSupported returns true if the image identifier is supported by App Runner.
func ImageIsSupported(imageIdentifier string) bool {
	return imageIsECR(imageIdentifier) || imageIsECRPublic(imageIdentifier)
}

// DetermineImageRepositoryType returns the App Runner ImageRepositoryType enum value for the provided image identifier,
// or returns an error if the imageIdentifier is not supported by App Runner or the ImageRepositoryType cannot be
// determined.
func DetermineImageRepositoryType(imageIdentifier string) (string, error) {
	if !ImageIsSupported(imageIdentifier) {
		return "", fmt.Errorf("image is not supported by App Runner: %s", imageIdentifier)
	}

	if imageIsECR(imageIdentifier) {
		return repositoryTypeECR, nil
	}

	if imageIsECRPublic(imageIdentifier) {
		return repositoryTypeECRPublic, nil
	}

	return "", fmt.Errorf("unable to determine the image repository type for image: %s", imageIdentifier)
}

func imageIsECR(imageIdentifier string) bool {
	matched, _ := regexp.Match(`^\d{12}\.dkr\.ecr\.[^\.]+\.amazonaws\.com/`, []byte(imageIdentifier))
	return matched
}

func imageIsECRPublic(imageIdentifier string) bool {
	matched, _ := regexp.Match(`^public\.ecr\.aws/`, []byte(imageIdentifier))
	return matched
}
