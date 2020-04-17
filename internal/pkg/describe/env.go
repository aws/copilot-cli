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
	env *archer.Environment

	store          envGetter
	ecsClient      map[string]ecsService
	stackDescriber stackDescriber
	sessProvider   sessionFromRoleProvider
}

// NewEnvDescriber instantiates an environment describer.
func NewEnvDescriber(project string, env string) (*EnvDescriber, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := svc.GetEnvironment(project, env)
	if err != nil {
		return nil, err
	}
	sess, err := session.NewProvider().FromRole(meta.ManagerRoleARN, meta.Region)
	if err != nil {
		return nil, fmt.Errorf("session for role %s and region %s: %w", meta.ManagerRoleARN, meta.Region, err)
	}
	return &EnvDescriber{
		env:            meta,
		store:          svc,
		stackDescriber: cloudformation.New(sess),
		ecsClient:      make(map[string]ecsService),
		sessProvider:   session.NewProvider(),
	}, nil
}

func (e *EnvDescriber) Describe() (*Environment, error) {
	return &Environment{
		EnvironmentSummary: nil,
		Applications:       nil,
		Tags:               nil,
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
