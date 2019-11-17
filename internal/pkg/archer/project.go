// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Project is a named collection of environments and apps.
type Project struct {
	Name      string `json:"name"`    // Name of a project. Must be unique amongst other projects in the same account.
	AccountID string `json:"account"` // AccountID this project is mastered in.
	Domain    string `json:"domain"`  // Existing domain name in Route53. An empty domain name means the user does not have one.
	Version   string `json:"version"` // The version of the project layout in the underlying datastore (e.g. SSM).
}

// RequiresDNSDelegation returns true if we have to set up DNS Delegation resources
func (p *Project) RequiresDNSDelegation() bool {
	return p.Domain != ""
}

// ProjectRegionalResources represent project resources that are regional.
type ProjectRegionalResources struct {
	Region         string            // The region these resources are in.
	KMSKeyARN      string            // A KMS Key ARN for encrypting Pipeline artifacts.
	S3Bucket       string            // S3 bucket for Pipeline artifacts.
	RepositoryURLs map[string]string // The image repository URLs by app name.
}

// ProjectStore is an interface for creating and listing projects.
type ProjectStore interface {
	ProjectLister
	ProjectGetter
	ProjectCreator
}

// ProjectLister lists all the projects in the underlying project manager.
type ProjectLister interface {
	ListProjects() ([]*Project, error)
}

// ProjectCreator creates a project in the underlying project manager.
type ProjectCreator interface {
	CreateProject(project *Project) error
}

// ProjectGetter fetches an individual project from the underlying project manager.
type ProjectGetter interface {
	GetProject(projectName string) (*Project, error)
}

// ProjectResourceStore fetches resources related to the project.
type ProjectResourceStore interface {
	// GetRegionalProjectResources fetches all regional resources in a project.
	GetRegionalProjectResources(project *Project) ([]*ProjectRegionalResources, error)
	// GetProjectResourcesByRegion fetches the project regional resources for a particular region.
	GetProjectResourcesByRegion(project *Project, region string) (*ProjectRegionalResources, error)
}
