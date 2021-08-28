// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"encoding/json"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/dustin/go-humanize/english"
)

var (
	fmtSvcDiscoveryEndpointWithPort = "%s.%s:%s" // Format string of the form {svc}.{endpoint}:{port}
)

// URI returns the LBWebServiceURI to identify this service uniquely given an environment name.
func (d *LBWebServiceDescriber) URI(envName string) (string, error) {
	err := d.initDescribers(envName)
	if err != nil {
		return "", err
	}

	envParams, err := d.envDescriber[envName].Params()
	if err != nil {
		return "", fmt.Errorf("get stack parameters for environment %s: %w", envName, err)
	}
	envOutputs, err := d.envDescriber[envName].Outputs()
	if err != nil {
		return "", fmt.Errorf("get stack outputs for environment %s: %w", envName, err)
	}
	svcParams, err := d.svcStackDescriber[envName].Params()
	if err != nil {
		return "", fmt.Errorf("get stack parameters for service %s: %w", d.svc, err)
	}

	uri := &LBWebServiceURI{
		DNSNames: []string{envOutputs[envOutputPublicLoadBalancerDNSName]},
		Path:     svcParams[stack.LBWebServiceRulePathParamKey],
	}
	_, isHTTPS := envOutputs[envOutputSubdomain]
	if isHTTPS {
		dnsName := fmt.Sprintf("%s.%s", d.svc, envOutputs[envOutputSubdomain])
		uri.DNSNames = []string{dnsName}
		uri.HTTPS = true
	}
	aliases := envParams[stack.EnvParamAliasesKey]
	if aliases != "" {
		value := make(map[string][]string)
		if err := json.Unmarshal([]byte(aliases), &value); err != nil {
			return "", err
		}
		if value[d.svc] != nil {
			uri.DNSNames = value[d.svc]
		}
	}
	d.svcParams = svcParams
	return uri.String(), nil
}

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
	port := svcStackParams[stack.LBWebServiceContainerPortParamKey]
	if port == stack.NoExposedContainerPort {
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

// LBWebServiceURI represents the unique identifier to access a load balanced web service.
type LBWebServiceURI struct {
	HTTPS    bool
	DNSNames []string // The environment's subdomain if the service is served on HTTPS. Otherwise, the public load balancer's DNS.
	Path     string   // Empty if the service is served on HTTPS. Otherwise, the pattern used to match the service.
}

func (u *LBWebServiceURI) String() string {
	var uris []string
	for _, dnsName := range u.DNSNames {
		protocol := "http://"
		if u.HTTPS {
			protocol = "https://"
		}
		path := ""
		if u.Path != "/" {
			path = fmt.Sprintf("/%s", u.Path)
		}
		uris = append(uris, fmt.Sprintf("%s%s%s", protocol, dnsName, path))
	}
	return english.OxfordWordSeries(uris, "or")
}

type serviceDiscovery struct {
	Service  string
	Endpoint string
	Port     string
}

func (s *serviceDiscovery) String() string {
	return fmt.Sprintf(fmtSvcDiscoveryEndpointWithPort, s.Service, s.Endpoint, s.Port)
}
