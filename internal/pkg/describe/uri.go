// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"strings"

	"github.com/dustin/go-humanize/english"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

type URIAccessType int

const (
	URIAccessTypeNone URIAccessType = iota
	URIAccessTypeInternet
	URIAccessTypeInternal
	URIAccessTypeServiceDiscovery
)

var (
	fmtSvcDiscoveryEndpointWithPort = "%s.%s:%s" // Format string of the form {svc}.{endpoint}:{port}
)

// ReachableService represents a service describer that has an endpoint.
type ReachableService interface {
	URI(env string) (string, URIAccessType, error)
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
func (d *LBWebServiceDescriber) URI(envName string) (string, URIAccessType, error) {
	svcDescr, err := d.initECSServiceDescribers(envName)
	if err != nil {
		return "", URIAccessTypeNone, err
	}
	envDescr, err := d.initEnvDescribers(envName)
	if err != nil {
		return "", URIAccessTypeNone, err
	}
	var albEnabled, nlbEnabled bool
	resources, err := svcDescr.ServiceStackResources()
	if err != nil {
		return "", URIAccessTypeNone, fmt.Errorf("get stack resources for service %s: %w", d.svc, err)
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
		albDescr := &albDescriber{
			svc:             d.svc,
			env:             envName,
			svcDescriber:    svcDescr,
			envDescriber:    envDescr,
			initLBDescriber: d.initLBDescriber,
			envDNSNameKey:   envOutputPublicLoadBalancerDNSName,
		}
		albURI, err := albDescr.uri()
		if err != nil {
			return "", URIAccessTypeNone, err
		}
		uri.albURI = albURI
	}

	if nlbEnabled {
		nlbURI, err := d.nlbURI(envName, svcDescr, envDescr)
		if err != nil {
			return "", URIAccessTypeNone, err
		}
		uri.nlbURI = nlbURI
	}

	return uri.String(), URIAccessTypeInternet, nil
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
func (d *BackendServiceDescriber) URI(envName string) (string, URIAccessType, error) {
	svcDescr, err := d.initECSServiceDescribers(envName)
	if err != nil {
		return "", URIAccessTypeNone, err
	}
	envDescr, err := d.initEnvDescribers(envName)
	if err != nil {
		return "", URIAccessTypeNone, err
	}
	resources, err := svcDescr.ServiceStackResources()
	if err != nil {
		return "", URIAccessTypeNone, fmt.Errorf("get stack resources for service %s: %w", d.svc, err)
	}
	for _, res := range resources {
		if res.LogicalID == svcStackResourceALBTargetGroupLogicalID {
			albDescr := &albDescriber{
				svc:             d.svc,
				env:             envName,
				svcDescriber:    svcDescr,
				envDescriber:    envDescr,
				initLBDescriber: d.initLBDescriber,
				envDNSNameKey:   envOutputInternalLoadBalancerDNSName,
			}
			albURI, err := albDescr.uri()
			if err != nil {
				return "", URIAccessTypeNone, err
			}
			return english.OxfordWordSeries(albURI.strings(), "or"), URIAccessTypeInternal, nil
		}
	}

	svcStackParams, err := svcDescr.Params()
	if err != nil {
		return "", URIAccessTypeNone, fmt.Errorf("get stack parameters for environment %s: %w", envName, err)
	}
	port := svcStackParams[stack.WorkloadContainerPortParamKey]
	if port == stack.NoExposedContainerPort {
		return BlankServiceDiscoveryURI, URIAccessTypeNone, nil
	}
	endpoint, err := envDescr.ServiceDiscoveryEndpoint()
	if err != nil {
		return "", URIAccessTypeNone, fmt.Errorf("retrieve service discovery endpoint for environment %s: %w", envName, err)
	}
	s := serviceDiscovery{
		Service:  d.svc,
		Port:     port,
		Endpoint: endpoint,
	}
	return s.String(), URIAccessTypeServiceDiscovery, nil
}

type albDescriber struct {
	svc             string
	env             string
	svcDescriber    ecsDescriber
	envDescriber    envDescriber
	initLBDescriber func(string) (lbDescriber, error)
	envDNSNameKey   string
}

func (d *albDescriber) uri() (albURI, error) {
	svcParams, err := d.svcDescriber.Params()
	if err != nil {
		return albURI{}, fmt.Errorf("get stack parameters for service %s: %w", d.svc, err)
	}
	path := svcParams[stack.WorkloadRulePathParamKey]
	if svcParams[stack.WorkloadHTTPSParamKey] != "true" {
		envOutputs, err := d.envDescriber.Outputs()
		if err != nil {
			return albURI{}, fmt.Errorf("get stack outputs for environment %s: %w", d.env, err)
		}
		return albURI{
			DNSNames: []string{envOutputs[d.envDNSNameKey]},
			Path:     path,
		}, nil
	}
	svcResources, err := d.svcDescriber.ServiceStackResources()
	if err != nil {
		return albURI{}, fmt.Errorf("get stack resources for service %s: %w", d.svc, err)
	}
	var httpsRuleARN string
	for _, resource := range svcResources {
		if resource.LogicalID == svcStackResourceHTTPSListenerRuleLogicalID &&
			resource.Type == svcStackResourceHTTPSListenerRuleResourceType {
			httpsRuleARN = resource.PhysicalID
		}
	}
	lbDescr, err := d.initLBDescriber(d.env)
	if err != nil {
		return albURI{}, nil
	}
	dnsNames, err := lbDescr.ListenerRuleHostHeaders(httpsRuleARN)
	if err != nil {
		return albURI{}, fmt.Errorf("get host headers for listener rule %s: %w", httpsRuleARN, err)
	}
	return albURI{
		HTTPS:    true,
		DNSNames: dnsNames,
		Path:     path,
	}, nil
}

// URI returns the WebServiceURI to identify this service uniquely given an environment name.
func (d *RDWebServiceDescriber) URI(envName string) (string, URIAccessType, error) {
	describer, err := d.initAppRunnerDescriber(envName)
	if err != nil {
		return "", URIAccessTypeNone, err
	}

	serviceURL, err := describer.ServiceURL()
	if err != nil {
		return "", URIAccessTypeNone, fmt.Errorf("get outputs for service %s: %w", d.svc, err)
	}

	return serviceURL, URIAccessTypeInternet, nil
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
	uris := u.albURI.strings()
	for _, dnsName := range u.nlbURI.DNSNames {
		uris = append(uris, fmt.Sprintf("%s:%s", dnsName, u.nlbURI.Port))
	}
	return english.OxfordWordSeries(uris, "or")
}

func (u *albURI) strings() []string {
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
		uris = append(uris, protocol+dnsName+path)
	}
	return uris
}

type serviceDiscovery struct {
	Service  string
	Endpoint string
	Port     string
}

func (s *serviceDiscovery) String() string {
	return fmt.Sprintf(fmtSvcDiscoveryEndpointWithPort, s.Service, s.Endpoint, s.Port)
}
