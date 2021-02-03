// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	defaultSidecarPort = "80"
)

// convertSidecar converts the manifest sidecar configuration into a format parsable by the templates pkg.
func convertSidecar(s manifest.Sidecar) ([]*template.SidecarOpts, error) {
	if s.Sidecars == nil {
		return nil, nil
	}
	var sidecars []*template.SidecarOpts
	for name, config := range s.Sidecars {
		port, protocol, err := parsePortMapping(config.Port)
		if err != nil {
			return nil, err
		}
		sidecars = append(sidecars, &template.SidecarOpts{
			Name:       aws.String(name),
			Image:      config.Image,
			Port:       port,
			Protocol:   protocol,
			CredsParam: config.CredsParam,
			Secrets:    config.Secrets,
			Variables:  config.Variables,
		})
	}
	return sidecars, nil
}

// Valid sidecar portMapping example: 2000/udp, or 2000 (default to be tcp).
func parsePortMapping(s *string) (port *string, protocol *string, err error) {
	if s == nil {
		// default port for sidecar container to be 80.
		return aws.String(defaultSidecarPort), nil, nil
	}
	portProtocol := strings.Split(*s, "/")
	switch len(portProtocol) {
	case 1:
		return aws.String(portProtocol[0]), nil, nil
	case 2:
		return aws.String(portProtocol[0]), aws.String(portProtocol[1]), nil
	default:
		return nil, nil, fmt.Errorf("cannot parse port mapping from %s", *s)
	}
}

// convertAutoscaling converts the service's Auto Scaling configuration into a format parsable
// by the templates pkg.
func convertAutoscaling(a manifest.Autoscaling) (*template.AutoscalingOpts, error) {
	if a.IsEmpty() {
		return nil, nil
	}
	min, max, err := a.Range.Parse()
	if err != nil {
		return nil, err
	}
	autoscalingOpts := template.AutoscalingOpts{
		MinCapacity: &min,
		MaxCapacity: &max,
	}
	if a.CPU != nil {
		autoscalingOpts.CPU = aws.Float64(float64(*a.CPU))
	}
	if a.Memory != nil {
		autoscalingOpts.Memory = aws.Float64(float64(*a.Memory))
	}
	if a.Requests != nil {
		autoscalingOpts.Requests = aws.Float64(float64(*a.Requests))
	}
	if a.ResponseTime != nil {
		responseTime := float64(*a.ResponseTime) / float64(time.Second)
		autoscalingOpts.ResponseTime = aws.Float64(responseTime)
	}
	return &autoscalingOpts, nil
}

// convertHTTPHealthCheck converts the ALB health check configuration into a format parsable by the templates pkg.
func convertHTTPHealthCheck(hc manifest.HealthCheckArgsOrString) template.HTTPHealthCheckOpts {
	opts := template.HTTPHealthCheckOpts{
		HealthCheckPath:    manifest.DefaultHealthCheckPath,
		HealthyThreshold:   hc.HealthCheckArgs.HealthyThreshold,
		UnhealthyThreshold: hc.HealthCheckArgs.UnhealthyThreshold,
	}
	if hc.HealthCheckArgs.Path != nil {
		opts.HealthCheckPath = *hc.HealthCheckArgs.Path
	} else if hc.HealthCheckPath != nil {
		opts.HealthCheckPath = *hc.HealthCheckPath
	}
	if hc.HealthCheckArgs.Interval != nil {
		opts.Interval = aws.Int64(int64(hc.HealthCheckArgs.Interval.Seconds()))
	}
	if hc.HealthCheckArgs.Timeout != nil {
		opts.Timeout = aws.Int64(int64(hc.HealthCheckArgs.Timeout.Seconds()))
	}
	return opts
}

func convertLogging(lc *manifest.Logging) *template.LogConfigOpts {
	if lc == nil {
		return nil
	}
	return logConfigOpts(lc)
}
func logConfigOpts(lc *manifest.Logging) *template.LogConfigOpts {
	return &template.LogConfigOpts{
		Image:          lc.LogImage(),
		ConfigFile:     lc.ConfigFile,
		EnableMetadata: lc.GetEnableMetadata(),
		Destination:    lc.Destination,
		SecretOptions:  lc.SecretOptions,
	}
}
