// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type EnvironmentSummary struct {
	Name         string `json:"name"`
	AccountID    string `json:"accountID"`
	Region       string `json:"region"`
	IsProduction bool   `json:"production"`
}

type Environment struct {
	*EnvironmentSummary `json:"environment"`
	Applications        []*Application    `json:"applications"`
	Tags                map[string]string `json:"tags,omitempty"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	env  *archer.Environment
	apps []*archer.Application

	store          envGetter
	ecsClient      map[string]ecsService
	stackDescriber stackDescriber
	sessProvider   sessionFromRoleProvider
}

// NewEnvDescriber instantiates an environment describer.
func NewEnvDescriber(projectName string, envName string) (*EnvDescriber, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	env, err := svc.GetEnvironment(projectName, envName)
	if err != nil {
		return nil, err
	}
	apps, err := svc.ListApplications(projectName)
	if err != nil {
		return nil, err
	}
	sess, err := session.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("assuming role for environment %s: %w", env.ManagerRoleARN, err)
	}
	return &EnvDescriber{
		env:            env,
		store:          svc,
		apps:           apps,
		stackDescriber: cloudformation.New(sess),
		ecsClient:      make(map[string]ecsService),
		sessProvider:   session.NewProvider(),
	}, nil
}

func (e *EnvDescriber) Describe() (*Environment, error) {
	var envToExpand *EnvironmentSummary
	envToExpand = &EnvironmentSummary{
		Name:         e.env.Name,
		AccountID:    e.env.AccountID,
		Region:       e.env.Region,
		IsProduction: e.env.Prod,
	}
	var appsToSerialize []*Application
	for _, app := range e.apps {
		appsToSerialize = append(appsToSerialize, &Application{
			Name: app.Name,
			Type: app.Type,
		})
	}
	var tags map[string]string
	return &Environment{
		EnvironmentSummary: envToExpand,
		Applications:       appsToSerialize,
		Tags:               tags,
	}, nil
}

// JSONString returns the stringified WebApp struct with json format.
func (w *Environment) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified WebApp struct with human readable format.
func (w *Environment) HumanString() string {
	// TODO
	return ""
}
