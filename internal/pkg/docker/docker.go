// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package docker provides an interface to the system's Docker daemon.
package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/command"
)

// Runner represents a command that can be run.
type Runner struct {
	runner
}

type runner interface {
	Run(name string, args []string, options ...command.Option) error
}

// New returns a Runner.
func New() Runner {
	return Runner{
		runner: command.New(),
	}
}

// BuildArguments holds the arguments we can pass in as flags from the manifest.
type BuildArguments struct {
	URI            string            // Required. Location of ECR Repo. Used to generate image name in conjunction with tag.
	ImageTag       string            // Required. Tag to pass to `docker build` via -t flag. Usually Git commit short ID.
	Dockerfile     string            // Required. Dockerfile to pass to `docker build` via --file flag.
	Context        string            // Optional. Build context directory to pass to `docker build`
	Args           map[string]string // Optional. Build args to pass via `--build-arg` flags. Equivalent to ARG directives in dockerfile.
	AdditionalTags []string          // Optional. Additional image tags to pass to docker.
}

// Build will run a `docker build` command with the input uri, tag, and Dockerfile path.
func (r Runner) Build(in BuildArguments) error {
	dfDir := in.Context
	if dfDir == "" { // Context wasn't specified use the Dockerfile's directory as context.
		dfDir = filepath.Dir(in.Dockerfile)
	} 

	args := []string{"build"}

	// Add additional image tags to the docker build call.
	for _, tag := range append(buildInput.AdditionalTags, buildInput.ImageTag) {
		args = append(args, "-t", imageName(buildInput.URI, tag))
	}

	// Add the "args:" override section from manifest to the docker build call
	for k, v := range buildInput.Args {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, dfDir, "-f", buildInput.Dockerfile)

	err := r.Run("docker", args)
	if err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	return nil
}

// Login will run a `docker login` command against the Service repository URI with the input uri and auth data.
func (r Runner) Login(uri, username, password string) error {
	err := r.Run("docker",
		[]string{"login", "-u", username, "--password-stdin", uri},
		command.Stdin(strings.NewReader(password)))

	if err != nil {
		return fmt.Errorf("authenticate to ECR: %w", err)
	}

	return nil
}

// Push will run `docker push` command against the repository URI with the input uri and image tags.
func (r Runner) Push(uri, imageTag string, additionalTags ...string) error {
	for _, imageTag := range append(additionalTags, imageTag) {
		path := imageName(uri, imageTag)

		err := r.Run("docker", []string{"push", path})
		if err != nil {
			return fmt.Errorf("docker push %s: %w", path, err)
		}
	}

	return nil
}

func imageName(uri, tag string) string {
	return fmt.Sprintf("%s:%s", uri, tag)
}
