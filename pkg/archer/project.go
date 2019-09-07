// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// ProjectStore is an interface for creating and listing projects
type ProjectStore interface {
	ListProjects() ([]*Project, error)
	CreateProject(project *Project) error
}

// Project is a named collection of environments.
type Project struct {
	Name    string `json:"name"`    // Name of a project. Must be unique amongst other projects in the same account
	Version string `json:"version"` // The version of the project layout.
}

// ListProjects lists all the projects belonging to a particular account and region
func (archer *Archer) ListProjects() ([]*Project, error) {
	return archer.projStore.ListProjects()
}

// CreateProject creates a project
func (archer *Archer) CreateProject(project *Project) error {
	return archer.projStore.CreateProject(project)
}
