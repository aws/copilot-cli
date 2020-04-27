// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	sess "github.com/aws/aws-sdk-go/aws/session"
)

type EnvDescription struct {
	Environment  *archer.Environment   `json:"environment"`
	Applications []*archer.Application `json:"applications"`
	Tags         map[string]string     `json:"tags,omitempty"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	env  *archer.Environment
	apps []*archer.Application

	store        storeSvc
	sessProvider *sess.Session
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
		env:          env,
		store:        svc,
		apps:         apps,
		sessProvider: sess,
	}, nil
}

// Describe returns info about a project's environment.
func (e *EnvDescriber) Describe() (*EnvDescription, error) {
	var tags map[string]string
	return &EnvDescription{
		Environment:  e.env,
		Applications: e.apps,
		Tags:         tags,
	}, nil
}

// JSONString returns the stringified WebApp struct with json format.
func (w *EnvDescription) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified WebApp struct with human readable format.
func (w *EnvDescription) HumanString() string {
	// TODO
	return ""
}
