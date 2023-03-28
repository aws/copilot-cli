// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package dockerengine provides functionality to interact with the Docker server.
package dockerengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

// Cmd is the interface implemented by external commands.
type Cmd interface {
	Run(name string, args []string, options ...exec.CmdOption) error
}

// Operating systems and architectures supported by docker.
const (
	OSLinux   = "linux"
	OSWindows = "windows"

	ArchAMD64 = "amd64"
	ArchX86   = "x86_64"
	ArchARM   = "arm"
	ArchARM64 = "arm64"
)

const (
	credStoreECRLogin = "ecr-login" // set on `credStore` attribute in docker configuration file
)

// CmdClient represents the docker client to interact with the server via external commands.
type CmdClient struct {
	runner Cmd
	// Override in unit tests.
	buf       *bytes.Buffer
	homePath  string
	lookupEnv func(string) (string, bool)
}

// New returns CmdClient to make requests against the Docker daemon via external commands.
func New(cmd Cmd) CmdClient {
	return CmdClient{
		runner:    cmd,
		homePath:  userHomeDirectory(),
		lookupEnv: os.LookupEnv,
	}
}

// BuildArguments holds the arguments that can be passed while building a container.
type BuildArguments struct {
	URI        string            // Required. Location of ECR Repo. Used to generate image name in conjunction with tag.
	Tags       []string          // Required. List of tags to apply to the image.
	Dockerfile string            // Required. Dockerfile to pass to `docker build` via --file flag.
	Context    string            // Optional. Build context directory to pass to `docker build`.
	Target     string            // Optional. The target build stage to pass to `docker build`.
	CacheFrom  []string          // Optional. Images to consider as cache sources to pass to `docker build`
	Platform   string            // Optional. OS/Arch to pass to `docker build`.
	Args       map[string]string // Optional. Build args to pass via `--build-arg` flags. Equivalent to ARG directives in dockerfile.
	Labels     map[string]string // Required. Set metadata for an image.
}

type dockerConfig struct {
	CredsStore  string            `json:"credsStore,omitempty"`
	CredHelpers map[string]string `json:"credHelpers,omitempty"`
}

// Build will run a `docker build` command for the given ecr repo URI and build arguments.
func (c CmdClient) Build(in *BuildArguments) error {

	// Tags must not be empty to build an docker image.
	if len(in.Tags) == 0 {
		return &errEmptyImageTags{
			uri: in.URI,
		}
	}
	dfDir := in.Context
	if dfDir == "" { // Context wasn't specified use the Dockerfile's directory as context.
		dfDir = filepath.Dir(in.Dockerfile)
	}

	args := []string{"build"}

	// Add additional image tags to the docker build call.
	for _, tag := range in.Tags {
		args = append(args, "-t", imageName(in.URI, tag))
	}

	// Add cache from options.
	for _, imageFrom := range in.CacheFrom {
		args = append(args, "--cache-from", imageFrom)
	}

	// Add target option.
	if in.Target != "" {
		args = append(args, "--target", in.Target)
	}

	// Add platform option.
	if in.Platform != "" {
		args = append(args, "--platform", in.Platform)
	}

	// Plain display if we're in a CI environment.
	if ci, _ := c.lookupEnv("CI"); ci == "true" {
		args = append(args, "--progress", "plain")
	}

	// Add the "args:" override section from manifest to the docker build call.
	// Collect the keys in a slice to sort for test stability.
	var keys []string
	for k := range in.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, in.Args[k]))
	}

	// Add Labels to docker build call.
	// Collect the keys in a slice to sort for test stability.
	var labelKeys []string
	for k := range in.Labels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)
	for _, k := range labelKeys {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, in.Labels[k]))
	}

	args = append(args, dfDir, "-f", in.Dockerfile)
	// If host platform is not linux/amd64, show the user how the container image is being built; if the build fails (if their docker server doesn't have multi-platform-- and therefore `--platform` capability, for instance) they may see why.
	if in.Platform != "" {
		log.Infof("Building your container image: docker %s\n", strings.Join(args, " "))
	}
	if err := c.runner.Run("docker", args); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	return nil
}

// Login runs a `docker login` command against the specified Docker registry using the input credentials.
// The function returns the standard output and standard error message from the `docker login` command as a string.
// If the login process fails, an error is returned from the `docker login` command.
func (c CmdClient) Login(uri, username, password string) (string, error) {
	buf := new(strings.Builder)
	err := c.runner.Run("docker",
		[]string{"login", "-u", username, "--password-stdin", uri},
		exec.Stdin(strings.NewReader(password)), exec.Stdout(buf), exec.Stderr(buf))

	if err != nil {
		return buf.String(), fmt.Errorf("authenticate to ECR: %w", err)
	}

	return buf.String(), nil
}

// Push pushes the images with the specified tags and ecr repository URI, and returns the image digest on success.
func (c CmdClient) Push(uri string, tags ...string) (digest string, err error) {
	images := []string{}
	for _, tag := range tags {
		images = append(images, imageName(uri, tag))
	}
	var args []string
	if ci, _ := c.lookupEnv("CI"); ci == "true" {
		args = append(args, "--quiet")
	}

	for _, img := range images {
		if err := c.runner.Run("docker", append([]string{"push", img}, args...)); err != nil {
			return "", fmt.Errorf("docker push %s: %w", img, err)
		}
	}
	buf := new(strings.Builder)
	// The container image will have the same digest regardless of the associated tag.
	// Pick the first tag and get the image's digest.
	// For Main container we call  docker inspect --format '{{json (index .RepoDigests 0)}}' uri:latest
	// For Sidecar container images we call docker inspect --format '{{json (index .RepoDigests 0)}}' uri:<sidecarname>-latest
	if err := c.runner.Run("docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", imageName(uri, tags[0])}, exec.Stdout(buf)); err != nil {
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
func (c CmdClient) CheckDockerEngineRunning() error {
	if _, err := osexec.LookPath("docker"); err != nil {
		return ErrDockerCommandNotFound
	}
	buf := &bytes.Buffer{}
	err := c.runner.Run("docker", []string{"info", "-f", "'{{json .}}'"}, exec.Stdout(buf))
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
func (c CmdClient) GetPlatform() (os, arch string, err error) {
	if _, err := osexec.LookPath("docker"); err != nil {
		return "", "", ErrDockerCommandNotFound
	}
	buf := &bytes.Buffer{}
	err = c.runner.Run("docker", []string{"version", "-f", "'{{json .Server}}'"}, exec.Stdout(buf))
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
	return fmt.Sprintf("%s:%s", uri, tag)
}

// IsEcrCredentialHelperEnabled return true if ecr-login is enabled either globally or registry level
func (c CmdClient) IsEcrCredentialHelperEnabled(uri string) bool {
	// Make sure the program is able to obtain the home directory
	splits := strings.Split(uri, "/")
	if c.homePath == "" || len(splits) == 0 {
		return false
	}

	// Look into the default locations
	pathsToTry := []string{filepath.Join(".docker", "config.json"), ".dockercfg"}
	for _, path := range pathsToTry {
		content, err := os.ReadFile(filepath.Join(c.homePath, path))
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

// PlatformString returns a specified of the format <os>/<arch>.
func PlatformString(os, arch string) string {
	return fmt.Sprintf("%s/%s", os, arch)
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

type errEmptyImageTags struct {
	uri string
}

func (e *errEmptyImageTags) Error() string {
	return fmt.Sprintf("tags to reference an image should not be empty for building and pushing into the ECR repository %s", e.uri)
}
