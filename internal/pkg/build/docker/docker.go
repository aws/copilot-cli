// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
)

// Service wraps a runner.
type Service struct {
	runner runner
}

type runner interface {
	Run(name string, args []string, options ...command.Option) error
}

// New returns a Service.
func New() Service {
	return Service{
		runner: command.New(),
	}
}

// Build will run a `docker build` command with the input uri, tag, and Dockerfile image path.
func (s Service) Build(uri, imageTag, path string) error {
	imageName := imageName(uri, imageTag)

	err := s.runner.Run("docker", []string{"build", "-t", imageName, path})

	if err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	return nil
}

// Login will run a `docker login` command against the Service repository URI with the input uri and auth data.
func (s Service) Login(uri, username, password string) error {
	err := s.runner.Run("docker",
		[]string{"login", "-u", username, "--password-stdin", uri},
		command.Stdin(password))

	if err != nil {
		return fmt.Errorf("authenticate to ECR: %w", err)
	}

	return nil
}

// Push will run `docker push` command against the Service repository URI with the input uri and image tag.
func (s Service) Push(uri, imageTag string) error {
	path := imageName(uri, imageTag)

	err := s.runner.Run("docker", []string{"push", path})

	if err != nil {
		// TODO: improve the error handling here.
		// if you try to push an *existing* image that has Digest A and tag T then no error (also no image push).
		// if you try to push an *existing* image that has Digest B and tag T (that belongs to another image Digest A) then docker spits out an unclear error.
		log.Warningf("the image with tag %s may already exist.\n", imageTag)

		return fmt.Errorf("docker push: %w", err)
	}

	return nil
}

func imageName(uri, tag string) string {
	return fmt.Sprintf("%s:%s", uri, tag)
}
