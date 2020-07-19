// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package repository provides support for building and pushing images to a repository.
package repository

import (
    "fmt"
    "github.com/aws/copilot-cli/internal/pkg/aws/ecr"
)

// ContainerLoginBuildPusher provides support for logging in to repositories, building images and pushing images to repositories.
type ContainerLoginBuildPusher interface {
    Build(uri, path, imageTag string, additionalTags ...string) error
    Login(uri, username, password string) error
    Push(uri, imageTag string, additionalTags ...string) error
}

// Registry gets information of repositories.
type Registry interface {
    GetRepository(name string) (string, error)
    GetECRAuth() (ecr.Auth, error)
}

// Repository builds and pushes images to an ECR repository.
type Repository struct {
    repositoryName string
    registry Registry

    uri string
}

// New instantiates a new Repository.
func New(name string, registry Registry) (*Repository, error){
    uri, err := registry.GetRepository(name)
    if err != nil {
        return nil, fmt.Errorf("get repository URI: %w", err)
    }

    return &Repository{
        repositoryName: name,
        uri:            uri,
        registry:       registry,
    }, nil
}

// BuildAndPush builds the image from Dockerfile and pushes it to the repository with tags.
func (r *Repository) BuildAndPush(docker ContainerLoginBuildPusher, dockerfilePath string, tag string, additionalTags ...string) error {
    if err := docker.Build(r.uri, dockerfilePath, tag, additionalTags...); err != nil {
        return fmt.Errorf("build Dockerfile at %s: %w", dockerfilePath, err)
    }

    auth, err := r.registry.GetECRAuth()
    if err != nil {
        return fmt.Errorf("get auth: %w", err)
    }

    if err := docker.Login(r.uri, auth.Username, auth.Password); err != nil {
        return fmt.Errorf("login to repo %s: %w", r.repositoryName, err)
    }

    if err := docker.Push(r.uri, tag, additionalTags...); err != nil {
        return  fmt.Errorf("push to repo %s: %w", r.repositoryName, err)
    }
    return  nil
}

// URI returns the uri of the repository.
func (r *Repository) URI() string{
    return r.uri
}
