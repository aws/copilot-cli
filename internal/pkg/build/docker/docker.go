// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
)

// TODO: wrap the `os/exec` stuff in a `command` package to make mocking and testing easier on this package.

// Service wraps a repository URI endpoint.
type Service struct {
	repositoryURI string
}

// New returns a Service configured with the input URI.
func New(uri string) Service {
	return Service{
		repositoryURI: uri,
	}
}

// Build will `os/exec` a `docker build` command with the input tag and Dockerfile image path.
func (s Service) Build(imageTag, path string) error {
	imageName := fmt.Sprintf("%s:%s", s.repositoryURI, imageTag)

	cmd := newCommand("docker", "build", "-t", imageName, path)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	return nil
}

// Login will `os/exec` a `docker login` command against the Service repository URI with the input auth data.
func (s Service) Login(auth ecr.Auth) error {
	cmd := newCommand("docker", "login", "-u", auth.Username, "--password-stdin", s.repositoryURI)
	cmd.Stdin = strings.NewReader(auth.Password)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("authenticate to ECR: %w", err)
	}

	return nil
}

// Push will `os/exec` a `docker push` command against the Service repository URI with the input image tag.
func (s Service) Push(imageTag string) error {
	path := s.repositoryURI + ":" + imageTag

	cmd := newCommand("docker", "push", path)

	if err := cmd.Run(); err != nil {
		// TODO: improve the error handling here.
		// if you try to push an *existing* image that has Digest A and tag T then no error (also no image push).
		// if you try to push an *existing* image that has Digest B and tag T (that belongs to another image Digest A) then docker spits out an unclear error.
		log.Warningf("the image with tag %s may already exist.\n", imageTag)

		return fmt.Errorf("docker push: %w", err)
	}

	return nil
}

func newCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	// NOTE: Stdout and Stderr must both be set otherwise command output pipes to os.DevNull
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	return cmd
}
