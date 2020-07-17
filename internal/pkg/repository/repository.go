// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// package repository provides support for building and pushing images to a repository.
package repository

import (
    "fmt"
    "github.com/aws/copilot-cli/internal/pkg/aws/ecr"
)

// DockerService provides support for logging in to repositories, building images and pushing images to repositories.
type DockerService interface {
    Build(uri, path, imageTag string, additionalTags ...string) error
    Login(uri, username, password string) error
    Push(uri, imageTag string, additionalTags ...string) error
}

// RepositoryGetter gets information of ECR repositories.
type ECRRepositoryGetter interface {
    GetRepository(name string) (string, error)
    GetECRAuth() (ecr.Auth, error)
}

// ECRRepository builds and pushes images to an ECR repository.
type ECRRepository struct {
    repositoryName string

    repositoryGetter ECRRepositoryGetter
    docker           DockerService

    uri string
}

// NewECRRepository instantiates a new ECRRepository.
func NewECRRepository(name string, repositoryGetter ECRRepositoryGetter, docker DockerService) (*ECRRepository, error){
    repo := ECRRepository{
        repositoryName:   name,

        repositoryGetter: repositoryGetter,
        docker:           docker,
    }

    uri, err := repositoryGetter.GetRepository(name)
    if err != nil {
        return nil, fmt.Errorf("get ECR repository URI: %w", err)
    }

    repo.uri = uri
    return &repo, nil
}

// BuildAndPushToRepo builds the image from Dockerfile and pushes it to the ECR repository with tags.
func (r *ECRRepository) BuildAndPushToRepo(dockerfilePath string, tag string, additionalTags ...string) error {
    if err := r.docker.Build(r.uri, dockerfilePath, tag, additionalTags...); err != nil {
        return fmt.Errorf("build Dockerfile at %s: %w", dockerfilePath, err)
    }

    auth, err := r.repositoryGetter.GetECRAuth()
    if err != nil {
        return fmt.Errorf("get ECR auth: %w", err)
    }

    if err := r.docker.Login(r.uri, auth.Username, auth.Password); err != nil {
        return fmt.Errorf("login to repo %s: %w", r.repositoryName, err)
    }

    if err := r.docker.Push(r.uri, tag, additionalTags...); err != nil {
        return  fmt.Errorf("push to repo %s: %w", r.repositoryName, err)
    }
    return  nil
}

// URI returns the uri of the repository.
func (r *ECRRepository) URI() string{
    return r.uri
}
