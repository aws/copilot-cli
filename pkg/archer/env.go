// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// EnvironmentStore is an interface for creating and listing envss
type EnvironmentStore interface {
	ListEnvironments(projectName string) ([]*Environment, error)
	CreateEnvironment(environment *Environment) error
	GetEnvironment(projectName string, environmentName string) (*Environment, error)
}

// Environment represents the configuration of a particular Environment in a Project. It includes
// the location of the Environment (account and region), the name of the environment, as well as the project
// the environment belongs to.
type Environment struct {
	Project   string `json:"project"`   // Name of the project this environment belongs to.
	Name      string `json:"name"`      // Name of the environment, must be unique within a project.
	Region    string `json:"region"`    // Name of the region this environment is stored in.
	AccountID string `json:"accountID"` // Account ID of the account this environment is stored in.
	Prod      bool   `json:"prod"`      // Whether or not this environment is a production environment.
}

// ListEnvironments lists all the environments in a particular project
func (archer *Archer) ListEnvironments(project string) ([]*Environment, error) {
	return archer.envStore.ListEnvironments(project)
}

// CreateEnvironment creates an environment in an existing projects
func (archer *Archer) CreateEnvironment(environment *Environment) error {
	return archer.envStore.CreateEnvironment(environment)
}

// GetEnvironment gets a particular environment
func (archer *Archer) GetEnvironment(project string, name string) (*Environment, error) {
	//TODO decorate this environment with additional infrastrastructure
	// from tagris
	return archer.envStore.GetEnvironment(project, name)
}
