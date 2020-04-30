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

// CreateApplication instantiates a new application, validates its uniqueness and stores it in SSM.
func (s *Store) CreateApplication(application *archer.Project) error {
	applicationPath := fmt.Sprintf(fmtApplicationPath, application.Name)
	application.Version = schemaVersion

	data, err := marshal(application)
	if err != nil {
		return fmt.Errorf("serializing application %s: %w", application.Name, err)
	}

	_, err = s.ssmClient.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(applicationPath),
		Description: aws.String("Copilot Application"),
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
		return fmt.Errorf("create application %s: %w", application.Name, err)
	}
	return nil
}

// GetApplication fetches an application by name. If it can't be found, return a ErrNoSuchApplication
func (s *Store) GetApplication(applicationName string) (*archer.Project, error) {
	applicationPath := fmt.Sprintf(fmtApplicationPath, applicationName)
	applicationParam, err := s.ssmClient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(applicationPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				account, region := s.getCallerAccountAndRegion()
				return nil, &ErrNoSuchApplication{
					ApplicationName: applicationName,
					AccountID:       account,
					Region:          region,
				}
			}
		}
		return nil, fmt.Errorf("get application %s: %w", applicationName, err)
	}

	var application archer.Project
	if err := json.Unmarshal([]byte(*applicationParam.Parameter.Value), &application); err != nil {
		return nil, fmt.Errorf("read details for application %s: %w", applicationName, err)
	}
	return &application, nil
}

// ListApplications returns the list of existing applications in the customer's account and region.
func (s *Store) ListApplications() ([]*archer.Project, error) {
	var applications []*archer.Project
	serializedApplications, err := s.listParams(rootApplicationPath)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	for _, serializedApplication := range serializedApplications {
		var application archer.Project
		if err := json.Unmarshal([]byte(*serializedApplication), &application); err != nil {
			return nil, fmt.Errorf("read application details: %w", err)
		}

		applications = append(applications, &application)
	}
	return applications, nil
}

// DeleteApplication deletes the SSM parameter related to the application.
func (s *Store) DeleteApplication(name string) error {
	paramName := fmt.Sprintf(fmtApplicationPath, name)

	_, err := s.ssmClient.DeleteParameter(&ssm.DeleteParameterInput{
		Name: aws.String(paramName),
	})

	if err != nil {
		awserr, ok := err.(awserr.Error)
		if !ok {
			return err
		}

		if awserr.Code() == ssm.ErrCodeParameterNotFound {
			return nil
		}

		return fmt.Errorf("delete SSM param %s: %w", paramName, awserr)
	}

	return nil
}
