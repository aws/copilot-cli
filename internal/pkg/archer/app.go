// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Application represents a deployable service or task.
type Application struct {
	Project string `json:"project"` // Name of the project this application belongs to.
	Name    string `json:"name"`    // Name of the application, which must be unique within a project.
	Type    string `json:"type"`    // Type of the application (LoadBalanced app, etc)
}

// ApplicationStore can List, Create and Get applications in an underlying project management store
type ApplicationStore interface {
	ApplicationLister
	ApplicationGetter
	ApplicationCreator
}

// ApplicationLister fetches and returns a list of application from an underlying project management store
type ApplicationLister interface {
	ListApplications(projectName string) ([]*Application, error)
}

// ApplicationGetter fetches and returns an application from an underlying project management store
type ApplicationGetter interface {
	GetApplication(projectName string, applicationName string) (*Application, error)
}

// ApplicationCreator creates an application in the underlying project management store
type ApplicationCreator interface {
	CreateApplication(app *Application) error
}
