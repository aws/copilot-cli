// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// CreateService instantiates a new service within an existing application. Skip if
// the service already exists in the application.
func (s *Store) CreateService(svc *archer.Application) error {
	if _, err := s.GetApplication(svc.Project); err != nil {
		return err
	}

	servicePath := fmt.Sprintf(fmtSvcParamPath, svc.Project, svc.Name)
	data, err := marshal(svc)
	if err != nil {
		return fmt.Errorf("serializing service %s: %w", svc.Name, err)
	}

	_, err = s.ssmClient.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(servicePath),
		Description: aws.String(fmt.Sprintf("Copilot Service %s", svc.Name)),
		Type:        aws.String(ssm.ParameterTypeString),
		Value:       aws.String(data),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterAlreadyExists:
				return nil
			}
		}
		return fmt.Errorf("create service %s in application %s: %w", svc.Name, svc.Project, err)
	}
	return nil
}

// GetService gets a service belonging to a particular application by name. If no svc is found
// it returns ErrNoSuchService.
func (s *Store) GetService(appName, svcName string) (*archer.Application, error) {
	svcPath := fmt.Sprintf(fmtSvcParamPath, appName, svcName)
	svcParam, err := s.ssmClient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(svcPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil, &ErrNoSuchService{
					ApplicationName: appName,
					ServiceName:     svcName,
				}
			}
		}
		return nil, fmt.Errorf("get service %s in application %s: %w", svcName, appName, err)
	}

	var svc archer.Application
	err = json.Unmarshal([]byte(*svcParam.Parameter.Value), &svc)
	if err != nil {
		return nil, fmt.Errorf("read details for service %s in application %s: %w", svcName, appName, err)
	}
	return &svc, nil
}

// ListServices returns all services belonging to a particular application.
func (s *Store) ListServices(appName string) ([]*archer.Application, error) {
	var services []*archer.Application

	servicesPath := fmt.Sprintf(rootSvcParamPath, appName)
	serializedSvcs, err := s.listParams(servicesPath)
	if err != nil {
		return nil, fmt.Errorf("list services for application %s: %w", appName, err)
	}
	for _, serializedSvc := range serializedSvcs {
		var svc archer.Application
		if err := json.Unmarshal([]byte(*serializedSvc), &svc); err != nil {
			return nil, fmt.Errorf("read service details for application %s: %w", appName, err)
		}

		services = append(services, &svc)
	}
	return services, nil
}

// DeleteService removes a service from SSM.
// If the service does not exist in the store or is successfully deleted then returns nil. Otherwise, returns an error.
func (s *Store) DeleteService(appName, svcName string) error {
	paramName := fmt.Sprintf(fmtSvcParamPath, appName, svcName)
	_, err := s.ssmClient.DeleteParameter(&ssm.DeleteParameterInput{
		Name: aws.String(paramName),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil
			}
		}
		return fmt.Errorf("delete service %s from application %s: %w", svcName, appName, err)
	}
	return nil
}
