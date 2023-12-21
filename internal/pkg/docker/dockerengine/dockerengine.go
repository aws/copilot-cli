// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package dockerengine provides functionality to interact with the Docker server.
package dockerengine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/fatih/color"
	"golang.org/x/sync/errgroup"
)

// Cmd is the interface implemented by external commands.
type Cmd interface {
	Run(name string, args []string, options ...exec.CmdOption) error
	RunWithContext(ctx context.Context, name string, args []string, opts ...exec.CmdOption) error
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

// Health states of a Container.
const (
	noHealthcheck = "none"      // Indicates there is no healthcheck
	starting      = "starting"  // Starting indicates that the container is not yet ready
	healthy       = "healthy"   // Healthy indicates that the container is running correctly
	unhealthy     = "unhealthy" // Unhealthy indicates that the container has a problem
)

// State of a docker container.
const (
	containerStatusRunning = "running"
	containerStatusExited  = "exited"
)

// DockerCmdClient represents the docker client to interact with the server via external commands.
type DockerCmdClient struct {
	runner Cmd
	// Override in unit tests.
	buf       *bytes.Buffer
	homePath  string
	lookupEnv func(string) (string, bool)
}

// New returns CmdClient to make requests against the Docker daemon via external commands.
func New(cmd Cmd) DockerCmdClient {
	return DockerCmdClient{
		runner:    cmd,
		homePath:  userHomeDirectory(),
		lookupEnv: os.LookupEnv,
	}
}

// BuildArguments holds the arguments that can be passed while building a container.
type BuildArguments struct {
	URI               string            // Required. Location of ECR Repo. Used to generate image name in conjunction with tag.
	Tags              []string          // Required. List of tags to apply to the image.
	Dockerfile        string            // Optional. One of Dockerfile or DockerfileContent is required. Dockerfile to pass to `docker build` via --file flag.
	DockerfileContent string            // Optional. One of Dockerfile or DockerfileContent is required. Dockerfile content to pass to `docker build` via stdin.
	Context           string            // Optional. Build context directory to pass to `docker build`.
	Target            string            // Optional. The target build stage to pass to `docker build`.
	CacheFrom         []string          // Optional. Images to consider as cache sources to pass to `docker build`
	Platform          string            // Optional. OS/Arch to pass to `docker build`.
	Args              map[string]string // Optional. Build args to pass via `--build-arg` flags. Equivalent to ARG directives in dockerfile.
	Labels            map[string]string // Required. Set metadata for an image.
}

// RunOptions holds the options for running a Docker container.
type RunOptions struct {
	ImageURI             string            // Required. The image name to run.
	Secrets              map[string]string // Optional. Secrets to pass to the container as environment variables.
	EnvVars              map[string]string // Optional. Environment variables to pass to the container.
	ContainerName        string            // Optional. The name for the container.
	ContainerPorts       map[string]string // Optional. Contains host and container ports.
	Command              []string          // Optional. The command to run in the container.
	ContainerNetwork     string            // Optional. Network mode for the container.
	LogOptions           RunLogOptions     // Optional. Configure logging for output from the container
	AddLinuxCapabilities []string          // Optional. Adds linux capabilities to the container.
	Init                 bool              // Optional. Adds an init process as an entrypoint.
}

// RunLogOptions holds the logging configuration for Run().
type RunLogOptions struct {
	Color      *color.Color
	Output     io.Writer
	LinePrefix string
}

// GenerateDockerBuildArgs returns command line arguments to be passed to the Docker build command based on the provided BuildArguments.
// Returns an error if no tags are provided for building an image.
func (in *BuildArguments) GenerateDockerBuildArgs(c DockerCmdClient) ([]string, error) {
	// Tags must not be empty to build an docker image.
	if len(in.Tags) == 0 {
		return nil, &errEmptyImageTags{
			uri: in.URI,
		}
	}
	dfDir := in.Context
	// Context wasn't specified use the Dockerfile's directory as context.
	if dfDir == "" {
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

	if in.DockerfileContent != "" {
		args = append(args, "-")
	} else {
		args = append(args, dfDir, "-f", in.Dockerfile)
	}
	return args, nil
}

type dockerConfig struct {
	CredsStore  string            `json:"credsStore,omitempty"`
	CredHelpers map[string]string `json:"credHelpers,omitempty"`
}

// Build will run a `docker build` command for the given ecr repo URI and build arguments.
func (c DockerCmdClient) Build(ctx context.Context, in *BuildArguments, w io.Writer) error {
	args, err := in.GenerateDockerBuildArgs(c)
	if err != nil {
		return fmt.Errorf("generate docker build args: %w", err)
	}
	opts := []exec.CmdOption{
		exec.Stdout(w),
		exec.Stderr(w),
	}
	if in.DockerfileContent != "" {
		opts = append(opts, exec.Stdin(strings.NewReader(in.DockerfileContent)))
	}
	if err := c.runner.RunWithContext(ctx, "docker", args, opts...); err != nil {
		return fmt.Errorf("building image: %w", err)
	}
	return nil
}

// Login will run a `docker login` command against the Service repository URI with the input uri and auth data.
func (c DockerCmdClient) Login(uri, username, password string) error {
	err := c.runner.Run("docker",
		[]string{"login", "-u", username, "--password-stdin", uri},
		exec.Stdin(strings.NewReader(password)))

	if err != nil {
		return fmt.Errorf("authenticate to ECR: %w", err)
	}

	return nil
}

// Exec runs cmd in container with args and writes stderr/stdout to out.
func (c DockerCmdClient) Exec(ctx context.Context, container string, out io.Writer, cmd string, args ...string) error {
	return c.runner.RunWithContext(ctx, "docker", append([]string{
		"exec",
		container,
		cmd,
	}, args...), exec.Stdout(out), exec.Stderr(out))
}

// Push pushes the images with the specified tags and ecr repository URI, and returns the image digest on success.
func (c DockerCmdClient) Push(ctx context.Context, uri string, w io.Writer, tags ...string) (digest string, err error) {
	images := []string{}
	for _, tag := range tags {
		images = append(images, imageName(uri, tag))
	}
	var args []string
	if ci, _ := c.lookupEnv("CI"); ci == "true" {
		args = append(args, "--quiet")
	}

	for _, img := range images {
		if err := c.runner.RunWithContext(ctx, "docker", append([]string{"push", img}, args...), exec.Stdout(w), exec.Stderr(w)); err != nil {
			return "", fmt.Errorf("docker push %s: %w", img, err)
		}
	}
	buf := new(strings.Builder)
	// The container image will have the same digest regardless of the associated tag.
	// Pick the first tag and get the image's digest.
	// For Main container we call  docker inspect --format '{{json (index .RepoDigests 0)}}' uri:latest
	// For Sidecar container images we call docker inspect --format '{{json (index .RepoDigests 0)}}' uri:<sidecarname>-latest
	if err := c.runner.RunWithContext(ctx, "docker", []string{"inspect", "--format", "'{{json (index .RepoDigests 0)}}'", imageName(uri, tags[0])}, exec.Stdout(buf)); err != nil {
		return "", fmt.Errorf("inspect image digest for %s: %w", uri, err)
	}
	repoDigest := strings.Trim(strings.TrimSpace(buf.String()), `"'`) // remove new lines and quotes from output
	parts := strings.SplitAfter(repoDigest, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("parse the digest from the repo digest '%s'", repoDigest)
	}
	return parts[1], nil
}

func (in *RunOptions) generateRunArguments() []string {
	args := []string{"run"}

	if in.ContainerName != "" {
		args = append(args, "--name", in.ContainerName)
	}

	for hostPort, containerPort := range in.ContainerPorts {
		args = append(args, "--publish", fmt.Sprintf("%s:%s", hostPort, containerPort))
	}

	if in.ContainerNetwork != "" {
		args = append(args, "--network", fmt.Sprintf("container:%s", in.ContainerNetwork))
	}

	for key, value := range in.Secrets {
		args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
	}

	for key, value := range in.EnvVars {
		args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
	}

	for _, cap := range in.AddLinuxCapabilities {
		args = append(args, "--cap-add", cap)
	}

	if in.Init {
		args = append(args, "--init")
	}

	args = append(args, in.ImageURI)

	if in.Command != nil && len(in.Command) > 0 {
		args = append(args, in.Command...)
	}
	return args
}

// Run runs a Docker container with the sepcified options.
func (c DockerCmdClient) Run(ctx context.Context, options *RunOptions) error {
	type exitCodeError interface {
		ExitCode() int
	}
	// set default options
	if options.LogOptions.Color == nil {
		options.LogOptions.Color = color.New()
	}
	if options.LogOptions.Output == nil {
		options.LogOptions.Output = os.Stderr
	}

	// Ensure only one thread is writing to Output at a time
	// since we don't know if the Writer is thread safe.
	mu := &sync.Mutex{}
	g, ctx := errgroup.WithContext(ctx)
	logger := func() io.WriteCloser {
		pr, pw := io.Pipe()
		g.Go(func() error {
			scanner := bufio.NewScanner(pr)
			for scanner.Scan() {
				mu.Lock()
				options.LogOptions.Color.Fprintln(options.LogOptions.Output, options.LogOptions.LinePrefix+scanner.Text())
				mu.Unlock()
			}
			return scanner.Err()
		})
		return pw
	}

	g.Go(func() error {
		// Close loggers to ensure scanner.Scan() in the logger goroutine returns.
		// This is really only an issue in tests; os/exec.Cmd.Run() returns EOF to
		// output streams when the command exits.
		stdout := logger()
		defer stdout.Close()
		stderr := logger()
		defer stderr.Close()

		if err := c.runner.RunWithContext(ctx, "docker",
			options.generateRunArguments(),
			exec.Stdout(stdout),
			exec.Stderr(stderr),
			exec.NewProcessGroup()); err != nil {
			var ec exitCodeError
			if errors.As(err, &ec) {
				return &ErrContainerExited{
					name:     options.ContainerName,
					exitcode: ec.ExitCode(),
				}
			}
			return fmt.Errorf("running container: %w", err)
		}
		return nil
	})

	return g.Wait()
}

// IsContainerRunning checks if a specific Docker container is running.
func (c DockerCmdClient) IsContainerRunning(ctx context.Context, name string) (bool, error) {
	state, err := c.containerState(ctx, name)
	if err != nil {
		return false, err
	}
	switch state.Status {
	case containerStatusRunning:
		return true, nil
	case containerStatusExited:
		return false, &ErrContainerExited{name: name, exitcode: state.ExitCode}
	}
	return false, nil
}

// ContainerExitCode returns the exit code of a container.
func (c DockerCmdClient) ContainerExitCode(ctx context.Context, name string) (int, error) {
	state, err := c.containerState(ctx, name)
	if err != nil {
		return 0, err
	}
	if state.Status == containerStatusRunning {
		return 0, &ErrContainerNotExited{name: name}
	}
	return state.ExitCode, nil
}

// IsContainerHealthy returns true if a container health state is healthy.
func (c DockerCmdClient) IsContainerHealthy(ctx context.Context, containerName string) (bool, error) {
	state, err := c.containerState(ctx, containerName)
	if err != nil {
		return false, err
	}
	if state.Status != containerStatusRunning {
		return false, fmt.Errorf("container %q is not in %q state", containerName, containerStatusRunning)
	}
	if state.Health == nil {
		return false, fmt.Errorf("healthcheck is not configured for container %q", containerName)
	}
	switch state.Health.Status {
	case healthy:
		return true, nil
	case starting:
		return false, nil
	case unhealthy:
		return false, fmt.Errorf("container %q is %q", containerName, unhealthy)
	case noHealthcheck:
		return false, fmt.Errorf("healthcheck is not configured for container %q", containerName)
	default:
		return false, fmt.Errorf("container %q had unexpected health status %q", containerName, state.Health.Status)
	}
}

// ContainerState holds the status, exit code, and health information of a Docker container.
type ContainerState struct {
	Status   string `json:"Status"`
	ExitCode int    `json:"ExitCode"`
	Health   *struct {
		Status string `json:"Status"`
	}
}

// containerState retrieves the current state of a specified Docker container.
// It returns a ContainerState object and any error encountered during retrieval.
func (d *DockerCmdClient) containerState(ctx context.Context, containerName string) (ContainerState, error) {
	containerID, err := d.containerID(ctx, containerName)
	if err != nil {
		return ContainerState{}, err
	}
	if containerID == "" {
		return ContainerState{}, nil
	}
	buf := &bytes.Buffer{}
	if err := d.runner.RunWithContext(ctx, "docker", []string{"inspect", "--format", "{{json .State}}", containerID}, exec.Stdout(buf)); err != nil {
		return ContainerState{}, fmt.Errorf("run docker inspect: %w", err)
	}
	var containerState ContainerState
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &containerState); err != nil {
		return ContainerState{}, fmt.Errorf("unmarshal state of container %q:%w", containerName, err)
	}
	return containerState, nil
}

// containerID gets the ID of a Docker container by its name.
func (d *DockerCmdClient) containerID(ctx context.Context, containerName string) (string, error) {
	buf := &bytes.Buffer{}
	if err := d.runner.RunWithContext(ctx, "docker", []string{"ps", "-a", "-q", "--filter", "name=" + containerName}, exec.Stdout(buf)); err != nil {
		return "", fmt.Errorf("run docker ps: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

// Stop calls `docker stop` to stop a running container.
func (c DockerCmdClient) Stop(ctx context.Context, containerID string) error {
	buf := &bytes.Buffer{}
	if err := c.runner.RunWithContext(ctx, "docker", []string{"stop", containerID}, exec.Stdout(buf), exec.Stderr(buf)); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(buf.String()), err)
	}
	return nil
}

// Rm calls `docker rm` to remove a stopped container.
func (c DockerCmdClient) Rm(ctx context.Context, containerID string) error {
	buf := &bytes.Buffer{}
	if err := c.runner.RunWithContext(ctx, "docker", []string{"rm", containerID}, exec.Stdout(buf), exec.Stderr(buf)); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(buf.String()), err)
	}
	return nil
}

// CheckDockerEngineRunning will run `docker info` command to check if the docker engine is running.
func (c DockerCmdClient) CheckDockerEngineRunning() error {
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
func (c DockerCmdClient) GetPlatform() (os, arch string, err error) {
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
func (c DockerCmdClient) IsEcrCredentialHelperEnabled(uri string) bool {
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
