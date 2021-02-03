// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"strings"

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
