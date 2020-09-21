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

// Workload represents a deployable long running service or task.
type Workload struct {
	App  string `json:"app"`  // Name of the app this workload belongs to.
	Name string `json:"name"` // Name of the workload, which must be unique within a app.
	Type string `json:"type"` // Type of the workload (ex: Load Balanced Web Server, etc)
}

// CreateWorkload instantiates a new service or job within an existing application. Skip if
// the workload already exists in the application.
func (s *Store) CreateWorkload(wkld *Workload) error {
	if _, err := s.GetApplication(wkld.App); err != nil {
		return err
	}

	servicePath := fmt.Sprintf(fmtSvcParamPath, wkld.App, wkld.Name)
	data, err := marshal(wkld)
	if err != nil {
		return fmt.Errorf("serializing workload %s: %w", wkld.Name, err)
	}

	_, err = s.ssmClient.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(servicePath),
		Description: aws.String(fmt.Sprintf("Copilot Workload %s", wkld.Name)),
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
		return fmt.Errorf("create workload %s in application %s: %w", wkld.Name, wkld.App, err)
	}
	return nil
}

// GetWorkload gets a workload belonging to a particular application by name. If no job or svc is found
// it returns ErrNoSuchWorkload.
func (s *Store) GetWorkload(appName, wkldName string) (*Workload, error) {
	svcPath := fmt.Sprintf(fmtSvcParamPath, appName, wkldName)
	svcParam, err := s.ssmClient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(svcPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil, &ErrNoSuchWorkload{
					ApplicationName: appName,
					ServiceName:     wkldName,
				}
			}
		}
		return nil, fmt.Errorf("get workload %s in application %s: %w", wkldName, appName, err)
	}

	var svc Workload
	err = json.Unmarshal([]byte(*svcParam.Parameter.Value), &svc)
	if err != nil {
		return nil, fmt.Errorf("read configuration for workload %s in application %s: %w", wkldName, appName, err)
	}
	return &svc, nil
}

// ListWorkloads returns all services belonging to a particular application.
func (s *Store) ListWorkloads(appName string) ([]*Workload, error) {
	var services []*Workload

	servicesPath := fmt.Sprintf(rootSvcParamPath, appName)
	serializedSvcs, err := s.listParams(servicesPath)
	if err != nil {
		return nil, fmt.Errorf("list workloads for application %s: %w", appName, err)
	}
	for _, serializedSvc := range serializedSvcs {
		var svc Workload
		if err := json.Unmarshal([]byte(*serializedSvc), &svc); err != nil {
			return nil, fmt.Errorf("read workload configuration for application %s: %w", appName, err)
		}

		services = append(services, &svc)
	}
	return services, nil
}

// DeleteWorkload removes a service from SSM.
// If the service does not exist in the store or is successfully deleted then returns nil. Otherwise, returns an error.
func (s *Store) DeleteWorkload(appName, wkldName string) error {
	paramName := fmt.Sprintf(fmtSvcParamPath, appName, wkldName)
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
		return fmt.Errorf("delete workload %s from application %s: %w", wkldName, appName, err)
	}
	return nil
}
