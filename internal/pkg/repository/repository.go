// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package repository provides support for building and pushing images to a repository.
package repository

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
)

// ContainerLoginBuildPusher provides support for logging in to repositories, building images and pushing images to repositories.
type ContainerLoginBuildPusher interface {
	Build(args *dockerengine.BuildArguments) error
	Login(uri, username, password string) error
	Push(uri string, tags ...string) (digest string, err error)
	IsEcrCredentialHelperEnabled(uri string) bool
}

// Registry gets information of repositories.
type Registry interface {
	RepositoryURI(name string) (string, error)
	Auth() (string, string, error)
}

// Repository builds and pushes images to a repository.
type Repository struct {
	name     string
	registry Registry
}

// New instantiates a new Repository.
func New(name string, sess *session.Session) (*Repository, error) {
	return &Repository{
		name:     name,
		registry: ecr.New(sess),
	}, nil
}

// BuildAndPush builds the image from Dockerfile and pushes it to the repository with tags.
func (r *Repository) BuildAndPush(docker ContainerLoginBuildPusher, args *dockerengine.BuildArguments) (digest string, err error) {
	if args.URI == "" {
		uri, err := r.registry.RepositoryURI(r.name)
		if err != nil {
			return "", fmt.Errorf("get repository URI: %w", err)
		}
		args.URI = uri
	}
	if err := docker.Build(args); err != nil {
		return "", fmt.Errorf("build Dockerfile at %s: %w", args.Dockerfile, err)
	}

	// Perform docker login only if credStore attribute value != ecr-login
	if !docker.IsEcrCredentialHelperEnabled(args.URI) {
		username, password, err := r.registry.Auth()
		if err != nil {
			return "", fmt.Errorf("get auth: %w", err)
		}

		if err := docker.Login(args.URI, username, password); err != nil {
			return "", fmt.Errorf("login to repo %s: %w", r.name, err)
		}
	}

	digest, err = docker.Push(args.URI, args.Tags...)
	if err != nil {
		return "", fmt.Errorf("push to repo %s: %w", r.name, err)
	}
	return digest, nil
}

// URI returns the uri of the repository.
func (r *Repository) URI() (string, error) {
	uri, err := r.registry.RepositoryURI(r.name)
	if err != nil {
		return "", fmt.Errorf("get repository URI: %w", err)
	}
	return uri, nil
}
