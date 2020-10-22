// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	svcWorkloadType = "service"
	jobWorkloadType = "job"
)

// Workload represents a deployable long running service or task.
type Workload struct {
	App  string `json:"app"`  // Name of the app this workload belongs to.
	Name string `json:"name"` // Name of the workload, which must be unique within a app.
	Type string `json:"type"` // Type of the workload (ex: Load Balanced Web Service, etc)
}

// CreateService instantiates a new service within an existing application. Skip if
// the service already exists in the application.
func (s *Store) CreateService(svc *Workload) error {
	if err := s.createWorkload(svc); err != nil {
		return fmt.Errorf("create service %s in application %s: %w", svc.Name, svc.App, err)
	}
	return nil
}

// CreateJob instantiates a new job within an existing application. Skip if the job already
// exists in the application.
func (s *Store) CreateJob(job *Workload) error {
	if err := s.createWorkload(job); err != nil {
		return fmt.Errorf("create job %s in application %s: %w", job.Name, job.App, err)
	}
	return nil
}

func (s *Store) createWorkload(wkld *Workload) error {
	if _, err := s.GetApplication(wkld.App); err != nil {
		return err
	}

	wkldPath := fmt.Sprintf(fmtWkldParamPath, wkld.App, wkld.Name)
	data, err := marshal(wkld)
	if err != nil {
		return fmt.Errorf("serialize data: %w", err)
	}

	_, err = s.ssmClient.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(wkldPath),
		Description: aws.String(fmt.Sprintf("Copilot %s %s", wkld.Type, wkld.Name)),
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
		return err
	}
	return nil
}

// GetService gets a service belonging to a particular application by name. If no job or svc is found
// it returns ErrNoSuchService.
func (s *Store) GetService(appName, svcName string) (*Workload, error) {
	svcPath := fmt.Sprintf(fmtWkldParamPath, appName, svcName)
	svcParam, err := s.ssmClient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(svcPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil, &ErrNoSuchService{
					App:  appName,
					Name: svcName,
				}
			}
		}
		return nil, fmt.Errorf("get service %s in application %s: %w", svcName, appName, err)
	}

	var svc Workload
	err = json.Unmarshal([]byte(*svcParam.Parameter.Value), &svc)
	if err != nil {
		return nil, fmt.Errorf("read configuration for service %s in application %s: %w", svcName, appName, err)
	}
	if !strings.Contains(strings.ToLower(svc.Type), svcWorkloadType) {
		return nil, &ErrNoSuchService{
			App:  appName,
			Name: svcName,
		}
	}
	return &svc, nil
}

// GetJob gets a job belonging to a particular application by name. If no job by that name is found,
// it returns ErrNoSuchJob.
func (s *Store) GetJob(appName, jobName string) (*Workload, error) {
	jobPath := fmt.Sprintf(fmtWkldParamPath, appName, jobName)
	jobParam, err := s.ssmClient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(jobPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil, &ErrNoSuchJob{
					App:  appName,
					Name: jobName,
				}
			}
		}
		return nil, fmt.Errorf("get job %s in application %s: %w", jobName, appName, err)
	}

	var job Workload
	err = json.Unmarshal([]byte(*jobParam.Parameter.Value), &job)
	if err != nil {
		return nil, fmt.Errorf("read configuration for job %s in application %s: %w", jobName, appName, err)
	}
	if !strings.Contains(strings.ToLower(job.Type), jobWorkloadType) {
		return nil, &ErrNoSuchJob{
			App:  appName,
			Name: jobName,
		}
	}
	return &job, nil
}

// ListServices returns all services belonging to a particular application.
func (s *Store) ListServices(appName string) ([]*Workload, error) {
	wklds, err := s.listWorkloads(appName)
	if err != nil {
		return nil, fmt.Errorf("read service configuration for application %s: %w", appName, err)
	}

	var services []*Workload
	for _, wkld := range wklds {
		if strings.Contains(strings.ToLower(wkld.Type), svcWorkloadType) {
			services = append(services, wkld)
		}
	}

	return services, nil
}

// ListJobs returns all jobs belonging to a particular application.
func (s *Store) ListJobs(appName string) ([]*Workload, error) {
	wklds, err := s.listWorkloads(appName)
	if err != nil {
		return nil, fmt.Errorf("read service configuration for application %s: %w", appName, err)
	}

	var jobs []*Workload
	for _, wkld := range wklds {
		if strings.Contains(strings.ToLower(wkld.Type), jobWorkloadType) {
			jobs = append(jobs, wkld)
		}
	}

	return jobs, nil
}

func (s *Store) listWorkloads(appName string) ([]*Workload, error) {
	var workloads []*Workload

	workloadsPath := fmt.Sprintf(rootWkldParamPath, appName)
	serializedWklds, err := s.listParams(workloadsPath)
	if err != nil {
		return nil, err
	}
	for _, serializedWkld := range serializedWklds {
		var wkld Workload
		if err := json.Unmarshal([]byte(*serializedWkld), &wkld); err != nil {
			return nil, err
		}

		workloads = append(workloads, &wkld)
	}
	return workloads, nil
}

// DeleteService removes a service from SSM.
// If the service does not exist in the store or is successfully deleted then returns nil. Otherwise, returns an error.
func (s *Store) DeleteService(appName, svcName string) error {
	if err := s.deleteWorkload(appName, svcName); err != nil {
		return fmt.Errorf("delete service %s from application %s: %w", svcName, appName, err)
	}
	return nil
}

// DeleteJob removes a job from SSM.
// If the job does not exist in the store or is successfully deleted then returns nil. Otherwise, returns an error.
func (s *Store) DeleteJob(appName, jobName string) error {
	if err := s.deleteWorkload(appName, jobName); err != nil {
		return fmt.Errorf("delete job %s from application %s: %w", jobName, appName, err)
	}
	return nil
}

func (s *Store) deleteWorkload(appName, wkldName string) error {
	paramName := fmt.Sprintf(fmtWkldParamPath, appName, wkldName)
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
		return err
	}
	return nil
}
