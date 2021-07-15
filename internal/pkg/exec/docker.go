// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DockerCommand represents docker commands that can be run.
type DockerCommand struct {
	runner
	// Override in unit tests.
	buf      *bytes.Buffer
	homePath string
}

// NewDockerCommand returns a DockerCommand.
func NewDockerCommand() DockerCommand {
	return DockerCommand{
		runner:   NewCmd(),
		homePath: userHomeDirectory(),
	}
}

// BuildArguments holds the arguments we can pass in as flags from the manifest.
type BuildArguments struct {
	URI        string            // Required. Location of ECR Repo. Used to generate image name in conjunction with tag.
	Tags       []string          // Optional. List of tags to apply to the image besides "latest".
	Dockerfile string            // Required. Dockerfile to pass to `docker build` via --file flag.
	Context    string            // Optional. Build context directory to pass to `docker build`
	Target     string            // Optional. The target build stage to pass to `docker build`
	CacheFrom  []string          // Optional. Images to consider as cache sources to pass to `docker build`
	Args       map[string]string // Optional. Build args to pass via `--build-arg` flags. Equivalent to ARG directives in dockerfile.
}

type dockerConfig struct {
	CredsStore  string            `json:"credsStore,omitempty"`
	CredHelpers map[string]string `json:"credHelpers,omitempty"`
}

const (
	credStoreECRLogin = "ecr-login" // set on `credStore` attribute in docker configuration file
)

// Architectures that receive special handling.
const (
	ArmArch   = "arm"
	Arm64Arch = "arm64"
)

// Build will run a `docker build` command for the given ecr repo URI and build arguments.
func (c DockerCommand) Build(in *BuildArguments) error {
	dfDir := in.Context
	if dfDir == "" { // Context wasn't specified use the Dockerfile's directory as context.
		dfDir = filepath.Dir(in.Dockerfile)
	}

	args := []string{"build"}

	// Add additional image tags to the docker build call.
	args = append(args, "-t", in.URI)
	for _, tag := range in.Tags {
		args = append(args, "-t", imageName(in.URI, tag))
	}

	// Add cache from options
	for _, imageFrom := range in.CacheFrom {
		args = append(args, "--cache-from", imageFrom)
	}

	// Add target option
	if in.Target != "" {
		args = append(args, "--target", in.Target)
	}

	// Add the "args:" override section from manifest to the docker build call

	// Collect the keys in a slice to sort for test stability
	var keys []string
	for k := range in.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, in.Args[k]))
	}

	args = append(args, dfDir, "-f", in.Dockerfile)

	if err := c.Run("docker", args); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	return nil
}

// Login will run a `docker login` command against the Service repository URI with the input uri and auth data.
func (c DockerCommand) Login(uri, username, password string) error {
	err := c.Run("docker",
		[]string{"login", "-u", username, "--password-stdin", uri},
		Stdin(strings.NewReader(password)))

	if err != nil {
		return fmt.Errorf("authenticate to ECR: %w", err)
	}

	return nil
}

// Push pushes the images with the specified tags and ecr repository URI, and returns the image digest on success.
func (c DockerCommand) Push(uri string, tags ...string) (digest string, err error) {
	images := []string{uri}
	for _, tag := range tags {
		images = append(images, imageName(uri, tag))
	}

	for _, img := range images {
		if err := c.Run("docker", []string{"push", img}); err != nil {
			return "", fmt.Errorf("docker push %s: %w", img, err)
		}
	}
	buf := new(strings.Builder)
	if err := c.Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", uri}, Stdout(buf)); err != nil {
		return "", fmt.Errorf("inspect image digest for %s: %w", uri, err)
	}
	repoDigest := strings.Trim(strings.TrimSpace(buf.String()), `"'`) // remove new lines and quotes from output
	parts := strings.SplitAfter(repoDigest, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("parse the digest from the repo digest '%s'", repoDigest)
	}
	return parts[1], nil
}

// CheckDockerEngineRunning will run `docker info` command to check if the docker engine is running.
func (c DockerCommand) CheckDockerEngineRunning() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return ErrDockerCommandNotFound
	}
	buf := &bytes.Buffer{}
	err := c.runner.Run("docker", []string{"info", "-f", "'{{json .}}'"}, Stdout(buf))
	if err != nil {
		return fmt.Errorf("get docker info: %w", err)
	}
	// Trim redundant prefix and suffix. For example: '{"ServerErrors":["Cannot connect...}'\n returns
	// {"ServerErrors":["Cannot connect...}
	out := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(buf.String()), "'"), "'")
	type dockerEngineNotRunningMsg struct {
		ServerErrors []string `json:"ServerErrors"`
	}
	var msg dockerEngineNotRunningMsg
	if err := json.Unmarshal([]byte(out), &msg); err != nil {
		return fmt.Errorf("unmarshal docker info message: %w", err)
	}
	if len(msg.ServerErrors) == 0 {
		return nil
	}
	return &ErrDockerDaemonNotResponsive{
		msg: strings.Join(msg.ServerErrors, "\n"),
	}
}

// GetPlatform will run the `docker version` command to get the OS/Arch.
func (c DockerCommand) GetPlatform() (os, arch string, err error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return "", "", ErrDockerCommandNotFound
	}
	buf := &bytes.Buffer{}
	err = c.runner.Run("docker", []string{"version", "-f", "'{{json .Server}}'"}, Stdout(buf))
	if err != nil {
		return "", "", fmt.Errorf("run docker version: %w", err)
	}

	out := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(buf.String()), "'"), "'")
	type dockerServer struct {
		OS   string `json:"Os"`
		Arch string `json:"Arch"`
	}
	var platform dockerServer
	if err := json.Unmarshal([]byte(out), &platform); err != nil {
		return "", "", fmt.Errorf("unmarshal docker platform: %w", err)

	}
	return platform.OS, platform.Arch, nil
}

func imageName(uri, tag string) string {
	if tag == "" {
		return uri // If no tag is specified build with latest.
	}
	return fmt.Sprintf("%s:%s", uri, tag)
}

// IsEcrCredentialHelperEnabled return true if ecr-login is enabled either globally or registry level
func (c DockerCommand) IsEcrCredentialHelperEnabled(uri string) bool {
	// Make sure the program is able to obtain the home directory
	splits := strings.Split(uri, "/")
	if c.homePath == "" || len(splits) == 0 {
		return false
	}

	// Look into the default locations
	pathsToTry := []string{filepath.Join(".docker", "config.json"), ".dockercfg"}
	for _, path := range pathsToTry {
		content, err := ioutil.ReadFile(filepath.Join(c.homePath, path))
		if err != nil {
			// if we can't read the file keep going
			continue
		}

		config, err := parseCredFromDockerConfig(content)
		if err != nil {
			continue
		}

		if config.CredsStore == credStoreECRLogin || config.CredHelpers[splits[0]] == credStoreECRLogin {
			return true
		}
	}

	return false
}

func parseCredFromDockerConfig(config []byte) (*dockerConfig, error) {
	/*
			Sample docker config file
		    {
		        "credsStore" : "ecr-login",
		        "credHelpers": {
		            "dummyaccountId.dkr.ecr.region.amazonaws.com": "ecr-login"
		        }
		    }
	*/
	cred := dockerConfig{}
	err := json.Unmarshal(config, &cred)
	if err != nil {
		return nil, err
	}

	return &cred, nil
}

func userHomeDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return home
}
