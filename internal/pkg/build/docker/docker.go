// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecr"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
)

type command struct {
	cmd *exec.Cmd
}

func (c command) run() error {
	return c.cmd.Run()
}

func (c command) standardInput(input string) {
	c.cmd.Stdin = strings.NewReader(input)
}

type runnable interface {
	run() error
	standardInput(string)
}

// Service exists for mockability.
type Service struct {
	createCommand func(name string, args ...string) runnable
}

// New returns a Service.
func New() Service {
	return Service{
		createCommand: newCommand,
	}
}

// Build will `os/exec` a `docker build` command with the input uri, tag, and Dockerfile image path.
func (s Service) Build(uri, imageTag, path string) error {
	imageName := imageName(uri, imageTag)

	cmd := s.createCommand("docker", "build", "-t", imageName, path)

	if err := cmd.run(); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	return nil
}

// Login will `os/exec` a `docker login` command against the Service repository URI with the input uri and auth data.
func (s Service) Login(uri string, auth ecr.Auth) error {
	cmd := s.createCommand("docker", "login", "-u", auth.Username, "--password-stdin", uri)
	cmd.standardInput(auth.Password)

	if err := cmd.run(); err != nil {
		return fmt.Errorf("authenticate to ECR: %w", err)
	}

	return nil
}

// Push will `os/exec` a `docker push` command against the Service repository URI with the input uri and image tag.
func (s Service) Push(uri, imageTag string) error {
	path := imageName(uri, imageTag)

	cmd := s.createCommand("docker", "push", path)

	if err := cmd.run(); err != nil {
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

func newCommand(name string, args ...string) runnable {
	cmd := exec.Command(name, args...)
	// NOTE: Stdout and Stderr must both be set otherwise command output pipes to os.DevNull
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	return command{
		cmd: cmd,
	}
}
