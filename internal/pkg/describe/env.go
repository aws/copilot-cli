// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"encoding/json"
	//"encoding/json"
	"fmt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"

	//"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
)

type Environment struct {
	Name       string `json:"name"`
	Production bool   `json:"production"` // add to instances used elsewhere (ie project_show)?
	Region     string `json:"region"`
	AccountID  string `json:"accountID"`
}

type EnvApp struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type EnvTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type EnvSummary struct {
	Environment  []*Environment `json:"environment"`
	Applications []*EnvApp      `json:"applications"`
	Tags         []*EnvTag      `json:"tags"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	env *archer.Environment

	store           envGetter
	ecsClient       map[string]ecsService
	stackDescribers map[string]stackDescriber
	sessProvider    sessionFromRoleProvider
}

// NewEnvDescriber instantiates an environment.
func NewEnvDescriber(project, env string) (*EnvDescriber, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := svc.GetEnvironment(project, env)
	if err != nil {
		return nil, err
	}
	return &EnvDescriber{
		env:             meta,
		store:           svc,
		stackDescribers: make(map[string]stackDescriber),
		ecsClient:       make(map[string]ecsService),
		sessProvider:    session.NewProvider(),
	}, nil
}

// JSONString returns the stringified WebApp struct with json format.
func (w *EnvSummary) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified WebApp struct with human readable format.
func (w *EnvSummary) HumanString() string {
	// TODO
	return ""
}
