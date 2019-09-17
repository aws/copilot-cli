// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Project is a named collection of environments.
type Project struct {
	Name    string `json:"name"`    // Name of a project. Must be unique amongst other projects in the same account
	Version string `json:"version"` // The version of the project layout.
}

// ProjectStore is an interface for creating and listing projects
type ProjectStore interface {
	ProjectLister
	ProjectCreator
}

// ProjectLister lists all the projects in the underlying project manager
type ProjectLister interface {
	ListProjects() ([]*Project, error)
}

// ProjectCreator creates a project in the underlying project manager
type ProjectCreator interface {
	CreateProject(project *Project) error
}
