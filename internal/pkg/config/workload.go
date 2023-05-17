// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
)

// Workload represents a deployable long-running service or task.
type Workload struct {
	App  string `json:"app"`  // Name of the app this workload belongs to.
	Name string `json:"name"` // Name of the workload, which must be unique within an app.
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

	_, err = s.ssm.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(wkldPath),
		Description: aws.String(fmt.Sprintf("Copilot %s %s", wkld.Type, wkld.Name)),
		Type:        aws.String(ssm.ParameterTypeString),
		Value:       aws.String(data),
		Tags: []*ssm.Tag{
			{
				Key:   aws.String("copilot-application"),
				Value: aws.String(wkld.App),
			},
			{
				Key:   aws.String("copilot-service"),
				Value: aws.String(wkld.Name),
			},
		},
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
	param, err := s.getWorkloadParam(appName, svcName)
	if err != nil {
		var errNoSuchWkld *errNoSuchWorkload
		if errors.As(err, &errNoSuchWkld) {
			return nil, &ErrNoSuchService{
				App:  appName,
				Name: svcName,
			}
		}
		return nil, err
	}

	var svc Workload
	err = json.Unmarshal(param, &svc)
	if err != nil {
		return nil, fmt.Errorf("read configuration for service %s in application %s: %w", svcName, appName, err)
	}
	if !manifestinfo.IsTypeAService(svc.Type) {
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
	param, err := s.getWorkloadParam(appName, jobName)
	if err != nil {
		var errNoSuchWkld *errNoSuchWorkload
		if errors.As(err, &errNoSuchWkld) {
			return nil, &ErrNoSuchJob{
				App:  appName,
				Name: jobName,
			}
		}
		return nil, err
	}

	var job Workload
	err = json.Unmarshal(param, &job)
	if err != nil {
		return nil, fmt.Errorf("read configuration for job %s in application %s: %w", jobName, appName, err)
	}
	if !manifestinfo.IsTypeAJob(job.Type) {
		return nil, &ErrNoSuchJob{
			App:  appName,
			Name: jobName,
		}
	}
	return &job, nil
}

// GetWorkload gets a workload belonging to an application by name.
func (s *Store) GetWorkload(appName, name string) (*Workload, error) {
	param, err := s.getWorkloadParam(appName, name)
	if err != nil {
		return nil, err
	}
	var wl Workload
	err = json.Unmarshal(param, &wl)
	if err != nil {
		return nil, fmt.Errorf("read configuration for %s in application %s: %w", name, appName, err)
	}
	return &wl, nil
}

func (s *Store) getWorkloadParam(appName, name string) ([]byte, error) {
	wlPath := fmt.Sprintf(fmtWkldParamPath, appName, name)
	wlParam, err := s.ssm.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(wlPath),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil, &errNoSuchWorkload{
					App:  appName,
					Name: name,
				}
			}
		}
		return nil, err
	}
	return []byte(*wlParam.Parameter.Value), nil
}

// ListServices returns all services belonging to a particular application.
func (s *Store) ListServices(appName string) ([]*Workload, error) {
	wklds, err := s.listWorkloads(appName)
	if err != nil {
		return nil, fmt.Errorf("read service configuration for application %s: %w", appName, err)
	}

	var services []*Workload
	for _, wkld := range wklds {
		if manifestinfo.IsTypeAService(wkld.Type) {
			services = append(services, wkld)
		}
	}

	return services, nil
}

// ListJobs returns all jobs belonging to a particular application.
func (s *Store) ListJobs(appName string) ([]*Workload, error) {
	wklds, err := s.listWorkloads(appName)
	if err != nil {
		return nil, fmt.Errorf("read job configuration for application %s: %w", appName, err)
	}

	var jobs []*Workload
	for _, wkld := range wklds {
		if manifestinfo.IsTypeAJob(wkld.Type) {
			jobs = append(jobs, wkld)
		}
	}

	return jobs, nil
}

// ListWorkloads returns all workloads belonging to a particular application.
func (s *Store) ListWorkloads(appName string) ([]*Workload, error) {
	wklds, err := s.listWorkloads(appName)
	if err != nil {
		return nil, fmt.Errorf("read workload configuration for application %s: %w", appName, err)
	}

	return wklds, nil
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
	_, err := s.ssm.DeleteParameter(&ssm.DeleteParameterInput{
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
