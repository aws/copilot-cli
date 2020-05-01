// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	backedSvcManifestPath = "services/backend/manifest.yml"
)

// BackendSvcProps represents the configuration needed to create a backend service.
type BackendSvcProps struct {
	SvcProps
	Port        uint16
	HealthCheck *ContainerHealthCheck // Optional healthcheck configuration.
}

// BackendSvc holds the configuration to create a backend service manifest.
type BackendSvc struct {
	Svc          `yaml:",inline"`
	Image        imageWithPortAndHealthcheck `yaml:",flow"`
	TaskConfig   `yaml:",inline"`
	Environments map[string]TaskConfig `yaml:",flow"`

	parser template.Parser
}

type imageWithPortAndHealthcheck struct {
	SvcImageWithPort `yaml:",inline"`
	HealthCheck      *ContainerHealthCheck `yaml:"healthcheck"`
}

// ContainerHealthCheck holds the configuration to determine if the service container is healthy.
// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-healthcheck.html
type ContainerHealthCheck struct {
	Command     []string       `yaml:"command"`
	Interval    *time.Duration `yaml:"interval"`
	Retries     *int           `yaml:"retries"`
	Timeout     *time.Duration `yaml:"timeout"`
	StartPeriod *time.Duration `yaml:"start_period"`
}

// NewBackendSvc applies the props to a default backend service configuration with
// minimal task sizes, single replica, no healthcheck, and then returns it.
func NewBackendSvc(props BackendSvcProps) *BackendSvc {
	svc := newDefaultBackendSvc()
	var healthCheck *ContainerHealthCheck
	if props.HealthCheck != nil {
		// Create the healthcheck field only if the caller specified a healthcheck.
		healthCheck = newDefaultContainerHealthCheck()
		healthCheck.apply(props.HealthCheck)
	}
	// Apply overrides.
	svc.Name = props.SvcName
	svc.Image.Build = props.Dockerfile
	svc.Image.Port = props.Port
	svc.Image.HealthCheck = healthCheck
	svc.parser = template.New()
	return svc
}

// MarshalBinary serializes the manifest object into a binary YAML document.
// Implements the encoding.BinaryMarshaler interface.
func (a *BackendSvc) MarshalBinary() ([]byte, error) {
	content, err := a.parser.Parse(backedSvcManifestPath, *a, template.WithFuncs(map[string]interface{}{
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
func (a *BackendSvc) DockerfilePath() string {
	return a.Image.Build
}

// ApplyEnv returns the service manifest with environment overrides.
// If the environment passed in does not have any overrides then it returns itself.
func (a *BackendSvc) ApplyEnv(envName string) *BackendSvc {
	target, ok := a.Environments[envName]
	if !ok {
		return a
	}
	return &BackendSvc{
		Svc: a.Svc,
		Image: imageWithPortAndHealthcheck{
			SvcImageWithPort: a.Image.SvcImageWithPort,
			HealthCheck:      a.Image.HealthCheck,
		},
		TaskConfig: a.TaskConfig.copyAndApply(target),
	}
}

// newDefaultBackendSvc returns a backend service with minimal task sizes and a single replica.
func newDefaultBackendSvc() *BackendSvc {
	return &BackendSvc{
		Svc: Svc{
			Type: BackendService,
		},
		TaskConfig: TaskConfig{
			CPU:    256,
			Memory: 512,
			Count:  intp(1),
		},
	}
}

// newDefaultContainerHealthCheck returns container health check configuration
// that's identical to a load balanced web service's defaults.
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

// HealthCheckOpts converts the image's healthcheck configuration into a format parsable by the templates pkg.
func (i imageWithPortAndHealthcheck) HealthCheckOpts() *ecs.HealthCheck {
	if i.HealthCheck == nil {
		return nil
	}
	return &ecs.HealthCheck{
		Command:     aws.StringSlice(i.HealthCheck.Command),
		Interval:    aws.Int64(int64(i.HealthCheck.Interval.Seconds())),
		Retries:     aws.Int64(int64(*i.HealthCheck.Retries)),
		StartPeriod: aws.Int64(int64(i.HealthCheck.StartPeriod.Seconds())),
		Timeout:     aws.Int64(int64(i.HealthCheck.Timeout.Seconds())),
	}
}
