// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dustin/go-humanize/english"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

var (
	fmtSvcDiscoveryEndpointWithPort = "%s.%s:%s" // Format string of the form {svc}.{endpoint}:{port}
)

// ReachableService represents a service describe that has an endpoint.
type ReachableService interface {
	URI(env string) (string, error)
}

// NewReachableService returns a ReachableService based on the type of the service.
func NewReachableService(app, svc string, store ConfigStoreSvc) (ReachableService, error) {
	cfg, err := store.GetWorkload(app, svc)
	if err != nil {
		return nil, err
	}
	in := NewServiceConfig{
		App:         app,
		Svc:         svc,
		ConfigStore: store,
	}
	switch cfg.Type {
	case manifest.LoadBalancedWebServiceType:
		return NewLBWebServiceDescriber(in)
	case manifest.RequestDrivenWebServiceType:
		return NewRDWebServiceDescriber(in)
	case manifest.BackendServiceType:
		return NewBackendServiceDescriber(in)
	default:
		return nil, fmt.Errorf("service %s is of type %s which cannot be reached over the network", svc, cfg.Type)
	}
}

// URI returns the LBWebServiceURI to identify this service uniquely given an environment name.
func (d *LBWebServiceDescriber) URI(envName string) (string, error) {
	svcDescr, err := d.initECSServiceDescribers(envName)
	if err != nil {
		return "", err
	}
	envDescr, err := d.initEnvDescribers(envName)
	if err != nil {
		return "", err
	}
	var (
		albEnabled bool
		nlbEnabled bool
	)
	resources, err := svcDescr.ServiceStackResources()
	if err != nil {
		return "", fmt.Errorf("get stack resources for service %s: %w", d.svc, err)
	}
	for _, resource := range resources {
		if resource.LogicalID == svcStackResourceALBTargetGroupLogicalID {
			albEnabled = true
		}
		if resource.LogicalID == svcStackResourceNLBTargetGroupLogicalID {
			nlbEnabled = true
		}
	}

	var uri LBWebServiceURI
	if albEnabled {
		albURI, err := d.albURI(envName, svcDescr, envDescr)
		if err != nil {
			return "", err
		}
		uri.albURI = albURI
	}

	if nlbEnabled {
		nlbURI, err := d.nlbURI(envName, svcDescr, envDescr)
		if err != nil {
			return "", err
		}
		uri.nlbURI = nlbURI
	}

	return uri.String(), nil
}

func (d *LBWebServiceDescriber) albURI(envName string, svcDescr ecsDescriber, envDescr envDescriber) (albURI, error) {
	envParams, err := envDescr.Params()
	if err != nil {
		return albURI{}, fmt.Errorf("get stack parameters for environment %s: %w", envName, err)
	}
	envOutputs, err := envDescr.Outputs()
	if err != nil {
		return albURI{}, fmt.Errorf("get stack outputs for environment %s: %w", envName, err)
	}
	svcParams, err := svcDescr.Params()
	if err != nil {
		return albURI{}, fmt.Errorf("get stack parameters for service %s: %w", d.svc, err)
	}
	uri := albURI{
		DNSNames: []string{envOutputs[envOutputPublicLoadBalancerDNSName]},
		Path:     svcParams[stack.LBWebServiceRulePathParamKey],
	}
	isHTTPS, ok := svcParams[svcParamHTTPSEnabled]
	if ok && isHTTPS == "true" {
		dnsName := fmt.Sprintf("%s.%s", d.svc, envOutputs[envOutputSubdomain])
		uri.DNSNames = []string{dnsName}
		uri.HTTPS = true
	}
	aliases := envParams[stack.EnvParamAliasesKey]
	if aliases != "" {
		value := make(map[string][]string)
		if err := json.Unmarshal([]byte(aliases), &value); err != nil {
			return albURI{}, err
		}
		if value[d.svc] != nil {
			uri.DNSNames = value[d.svc]
		}
	}
	d.svcParams = svcParams
	return uri, nil
}

func (d *LBWebServiceDescriber) nlbURI(envName string, svcDescr ecsDescriber, envDescr envDescriber) (nlbURI, error) {
	svcParams, err := svcDescr.Params()
	if err != nil {
		return nlbURI{}, fmt.Errorf("get stack parameters for service %s: %w", d.svc, err)
	}
	port, ok := svcParams[stack.LBWebServiceNLBPortParamKey]
	if !ok {
		return nlbURI{}, nil
	}
	uri := nlbURI{
		Port: port,
	}
	dnsDelegated, ok := svcParams[stack.LBWebServiceDNSDelegatedParamKey]
	if !ok || dnsDelegated != "true" {
		svcOutputs, err := svcDescr.Outputs()
		if err != nil {
			return nlbURI{}, fmt.Errorf("get stack outputs for service %s: %w", d.svc, err)
		}
		uri.DNSNames = []string{svcOutputs[svcOutputPublicNLBDNSName]}
		return uri, nil
	}

	aliases, ok := svcParams[stack.LBWebServiceNLBAliasesParamKey]
	if ok && aliases != "" {
		uri.DNSNames = strings.Split(aliases, ",")
		return uri, nil
	}
	envOutputs, err := envDescr.Outputs()
	if err != nil {
		return nlbURI{}, fmt.Errorf("get stack outputs for environment %s: %w", envName, err)
	}
	uri.DNSNames = []string{fmt.Sprintf("%s-nlb.%s", d.svc, envOutputs[envOutputSubdomain])}
	return uri, nil
}

// URI returns the service discovery namespace and is used to make
// BackendServiceDescriber have the same signature as WebServiceDescriber.
func (d *BackendServiceDescriber) URI(envName string) (string, error) {
	svcDescr, err := d.initECSServiceDescribers(envName)
	if err != nil {
		return "", err
	}
	svcStackParams, err := svcDescr.Params()
	if err != nil {
		return "", fmt.Errorf("get stack parameters for environment %s: %w", envName, err)
	}
	port := svcStackParams[stack.LBWebServiceContainerPortParamKey]
	if port == stack.NoExposedContainerPort {
		return BlankServiceDiscoveryURI, nil
	}
	envDescr, err := d.initEnvDescribers(envName)
	if err != nil {
		return "", err
	}
	endpoint, err := envDescr.ServiceDiscoveryEndpoint()
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
	describer, err := d.initAppRunnerDescriber(envName)
	if err != nil {
		return "", err
	}

	serviceURL, err := describer.ServiceURL()
	if err != nil {
		return "", fmt.Errorf("get outputs for service %s: %w", d.svc, err)
	}

	return serviceURL, nil
}

// LBWebServiceURI represents the unique identifier to access a load balanced web service.
type LBWebServiceURI struct {
	albURI albURI
	nlbURI nlbURI
}

type albURI struct {
	HTTPS    bool
	DNSNames []string // The environment's subdomain if the service is served on HTTPS. Otherwise, the public application load balancer's DNS.
	Path     string   // Empty if the service is served on HTTPS. Otherwise, the pattern used to match the service.
}

type nlbURI struct {
	DNSNames []string
	Port     string
}

func (u *LBWebServiceURI) String() string {
	var uris []string
	for _, dnsName := range u.albURI.DNSNames {
		protocol := "http://"
		if u.albURI.HTTPS {
			protocol = "https://"
		}
		path := ""
		if u.albURI.Path != "/" {
			path = fmt.Sprintf("/%s", u.albURI.Path)
		}
		uris = append(uris, fmt.Sprintf("%s%s%s", protocol, dnsName, path))
	}

	for _, dnsName := range u.nlbURI.DNSNames {
		uris = append(uris, fmt.Sprintf("%s:%s", dnsName, u.nlbURI.Port))
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
