// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
)

const (
	backedAppManifestPath = "applications/backend-app/manifest.yml"
)

// BackendAppProps represents the configuration needed to create a backend application.
type BackendAppProps struct {
	AppProps
	Port        uint16
	HealthCheck *ContainerHealthCheck // Optional healthcheck configuration.
}

// BackendApp holds the configuration to create a backend application manifest.
type BackendApp struct {
	App          `yaml:",inline"`
	Image        imageWithPortAndHealthcheck `yaml:",flow"`
	TaskConfig   `yaml:",inline"`
	Environments map[string]TaskConfig `yaml:",flow"`

	parser template.Parser
}

type imageWithPortAndHealthcheck struct {
	AppImageWithPort `yaml:",inline"`
	HealthCheck      *ContainerHealthCheck `yaml:"healthcheck"`
}

// ContainerHealthCheck holds the configuration to determine if the application container is healthy.
// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-healthcheck.html
type ContainerHealthCheck struct {
	Command     []string       `yaml:"command"`
	Interval    *time.Duration `yaml:"interval"`
	Retries     *int           `yaml:"retries"`
	Timeout     *time.Duration `yaml:"timeout"`
	StartPeriod *time.Duration `yaml:"start_period"`
}

// NewBackendApp applies the props to a default backend app configuration with
// minimal task sizes, single replica, no healthcheck, and then returns it.
func NewBackendApp(props BackendAppProps) *BackendApp {
	app := newDefaultBackendApp()
	var healthCheck *ContainerHealthCheck
	if props.HealthCheck != nil {
		// Create the healthcheck field only if the caller specified a healthcheck.
		healthCheck = newDefaultContainerHealthCheck()
		healthCheck.apply(props.HealthCheck)
	}
	// Apply overrides.
	app.Name = props.AppName
	app.Image.Build = props.Dockerfile
	app.Image.Port = props.Port
	app.Image.HealthCheck = healthCheck
	app.parser = template.New()
	return app
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (a *BackendApp) MarshalBinary() ([]byte, error) {
	content, err := a.parser.Parse(backedAppManifestPath, *a, template.WithFuncs(map[string]interface{}{
		"fmtSlice": func(elems []string) string {
			return fmt.Sprintf("[%s]", strings.Join(elems, ", "))
		},
		"quoteSlice": func(elems []string) []string {
			quotedElems := make([]string, len(elems))
			for i, el := range elems {
				quotedElems[i] = strconv.Quote(el)
			}
			return quotedElems
		},
	}))
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// DockerfilePath returns the image build path.
func (a *BackendApp) DockerfilePath() string {
	return a.Image.Build
}

// ApplyEnv returns the application configuration with environment overrides.
// If the environment passed in does not have any overrides then we return the default values.
func (a *BackendApp) ApplyEnv(envName string) TaskConfig {
	target, ok := a.Environments[envName]
	if !ok {
		return a.TaskConfig
	}
	return a.TaskConfig.copyAndApply(target)
}

// newDefaultBackendApp returns a backend application with minimal task sizes and a single replica.
func newDefaultBackendApp() *BackendApp {
	return &BackendApp{
		App: App{
			Type: BackendApplication,
		},
		TaskConfig: TaskConfig{
			CPU:    256,
			Memory: 512,
			Count:  intp(1),
		},
	}
}

// newDefaultContainerHealthCheck returns container health check configuration
// that's identical to a load balanced web application's defaults.
func newDefaultContainerHealthCheck() *ContainerHealthCheck {
	return &ContainerHealthCheck{
		Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
		Interval:    durationp(10 * time.Second),
		Retries:     intp(2),
		Timeout:     durationp(5 * time.Second),
		StartPeriod: durationp(0 * time.Second),
	}
}

// apply overrides the healthcheck's fields if other has them set.
func (hc *ContainerHealthCheck) apply(other *ContainerHealthCheck) {
	if other.Command != nil {
		hc.Command = other.Command
	}
	if other.Interval != nil {
		hc.Interval = other.Interval
	}
	if other.Retries != nil {
		hc.Retries = other.Retries
	}
	if other.Timeout != nil {
		hc.Timeout = other.Timeout
	}
	if other.StartPeriod != nil {
		hc.StartPeriod = other.StartPeriod
	}
}

// applyIfNotSet changes the healthcheck's fields only if they were not set and the other healthcheck has them set.
func (hc *ContainerHealthCheck) applyIfNotSet(other *ContainerHealthCheck) {
	if hc.Command == nil && other.Command != nil {
		hc.Command = other.Command
	}
	if hc.Interval == nil && other.Interval != nil {
		hc.Interval = other.Interval
	}
	if hc.Retries == nil && other.Retries != nil {
		hc.Retries = other.Retries
	}
	if hc.Timeout == nil && other.Timeout != nil {
		hc.Timeout = other.Timeout
	}
	if hc.StartPeriod == nil && other.StartPeriod != nil {
		hc.StartPeriod = other.StartPeriod
	}
}
