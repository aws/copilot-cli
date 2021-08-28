// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"

	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

// URI returns the service discovery namespace and is used to make
// BackendServiceDescriber have the same signature as WebServiceDescriber.
func (d *BackendServiceDescriber) URI(envName string) (string, error) {
	if err := d.initDescribers(envName); err != nil {
		return "", err
	}
	svcStackParams, err := d.svcStackDescriber[envName].Params()
	if err != nil {
		return "", fmt.Errorf("get stack parameters for environment %s: %w", envName, err)
	}
	port := svcStackParams[cfnstack.LBWebServiceContainerPortParamKey]
	if port == cfnstack.NoExposedContainerPort {
		return BlankServiceDiscoveryURI, nil
	}
	endpoint, err := d.envDescriber[envName].ServiceDiscoveryEndpoint()
	if err != nil {
		return "nil", fmt.Errorf("retrieve service discovery endpoint for environment %s: %w", envName, err)
	}
	s := serviceDiscovery{
		Service:  d.svc,
		Port:     port,
		Endpoint: endpoint,
	}
	return s.String(), nil
}

// URI returns the WebServiceURI to identify this service uniquely given an environment name.
func (d *RDWebServiceDescriber) URI(envName string) (string, error) {
	err := d.initServiceDescriber(envName)
	if err != nil {
		return "", err
	}

	serviceURL, err := d.envSvcDescribers[envName].ServiceURL()
	if err != nil {
		return "", fmt.Errorf("get outputs for service %s: %w", d.svc, err)
	}

	return serviceURL, nil
}
