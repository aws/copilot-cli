// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	//"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
)

type Environment struct {
	Name       string `json:"name"`
	Production bool   `json:"production"`
	Region     string `json:"region"`
	AccountID  string `json:"accountID"`
}

type EnvApp struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type EnvTags struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Env struct {
	About []*Environment `json:"about"`
	Applications []*EnvApp `json:"applications"`
	Tags []*EnvTags `json:"tags"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	env *archer.Environment

	//store           envGetter
	//ecsClient       map[string]ecsService
	//stackDescribers map[string]stackDescriber
	//sessProvider    sessionFromRoleProvider
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
		env: meta,
		//store:           svc,
		//stackDescribers: make(map[string]stackDescriber),
		//ecsClient:       make(map[string]ecsService),
		//sessProvider:    session.NewProvider(),
	}, nil
}

// JSONString returns the stringified WebApp struct with json format.
func (w *Environment) JSONString() (string, error) {
	// TODO
	return nil
}

// HumanString returns the stringified WebApp struct with human readable format.
func (w *Environment) HumanString() string {
	// TODO
	return nil
}
