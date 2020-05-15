// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	// ServiceCfnTemplateNameFormat is the base output file name when `service package`
	// is called. This is also used to render the pipeline CFN template.
	ServiceCfnTemplateNameFormat = "%s.stack.yml"
	// ServiceCfnTemplateConfigurationNameFormat is the base output configuration
	// file name when `service package` is called. It's also used to render the
	// pipeline CFN template.
	ServiceCfnTemplateConfigurationNameFormat = "%s-%s.params.json"
	// AddonsCfnTemplateNameFormat is the addons output file name when `service package`
	// is called.
	AddonsCfnTemplateNameFormat = "%s.addons.stack.yml"
)

// Service represents a deployable long running service or task.
type Service struct {
	App  string `json:"app"`  // Name of the app this service belongs to.
	Name string `json:"name"` // Name of the service, which must be unique within a app.
	Type string `json:"type"` // Type of the service (ex: Load Balanced Web Server, etc)
}

// CreateService instantiates a new service within an existing application. Skip if
// the service already exists in the application.
func (s *Store) CreateService(svc *Service) error {
	if _, err := s.GetApplication(svc.App); err != nil {
		return err
	}

	servicePath := fmt.Sprintf(fmtSvcParamPath, svc.App, svc.Name)
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
		return fmt.Errorf("create service %s in application %s: %w", svc.Name, svc.App, err)
	}
	return nil
}

// GetService gets a service belonging to a particular application by name. If no svc is found
// it returns ErrNoSuchService.
func (s *Store) GetService(appName, svcName string) (*Service, error) {
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

	var svc Service
	err = json.Unmarshal([]byte(*svcParam.Parameter.Value), &svc)
	if err != nil {
		return nil, fmt.Errorf("read configuration for service %s in application %s: %w", svcName, appName, err)
	}
	return &svc, nil
}

// ListServices returns all services belonging to a particular application.
func (s *Store) ListServices(appName string) ([]*Service, error) {
	var services []*Service

	servicesPath := fmt.Sprintf(rootSvcParamPath, appName)
	serializedSvcs, err := s.listParams(servicesPath)
	if err != nil {
		return nil, fmt.Errorf("list services for application %s: %w", appName, err)
	}
	for _, serializedSvc := range serializedSvcs {
		var svc Service
		if err := json.Unmarshal([]byte(*serializedSvc), &svc); err != nil {
			return nil, fmt.Errorf("read service configuration for application %s: %w", appName, err)
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
