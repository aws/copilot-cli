// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/color"

	"github.com/dustin/go-humanize/english"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
)

// URIAccessType represents how a URI can be accessed.
type URIAccessType int

// Supported URI Access Types.
const (
	URIAccessTypeNone URIAccessType = iota
	URIAccessTypeInternet
	URIAccessTypeInternal
	URIAccessTypeServiceDiscovery
	URIAccessTypeServiceConnect

	LBDNS = "COPILOT_LB_DNS"
)

var (
	fmtSvcDiscoveryEndpointWithPort = "%s.%s:%s" // Format string of the form {svc}.{endpoint}:{port}
)

// URI represents a uri and how it can be accessed.
type URI struct {
	URI        string
	AccessType URIAccessType
}

// ReachableService represents a service describer that has an endpoint.
type ReachableService interface {
	URI(env string) (URI, error)
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
	case manifestinfo.LoadBalancedWebServiceType:
		return NewLBWebServiceDescriber(in)
	case manifestinfo.RequestDrivenWebServiceType:
		return NewRDWebServiceDescriber(in)
	case manifestinfo.BackendServiceType:
		return NewBackendServiceDescriber(in)
	case manifestinfo.StaticSiteType:
		return NewStaticSiteDescriber(in)
	default:
		return nil, &ErrNonAccessibleServiceType{
			name:    svc,
			svcType: cfg.Type,
		}
	}
}

// URI returns the LBWebServiceURI to identify this service uniquely given an environment name.
func (d *LBWebServiceDescriber) URI(envName string) (URI, error) {
	svcDescr, err := d.initECSServiceDescribers(envName)
	if err != nil {
		return URI{}, err
	}
	envDescr, err := d.initEnvDescribers(envName)
	if err != nil {
		return URI{}, err
	}
	var albEnabled, importedALB, nlbEnabled bool
	resources, err := svcDescr.StackResources()
	if err != nil {
		return URI{}, fmt.Errorf("get stack resources for service %s: %w", d.svc, err)
	}
	for _, resource := range resources {
		if strings.HasPrefix(resource.LogicalID, svcStackResourceALBTargetGroupLogicalID) {
			albEnabled = true
		}
		if strings.HasPrefix(resource.LogicalID, svcStackResourceListenerRuleForImportedALBLogicalID) {
			importedALB = true
		}
		if strings.HasPrefix(resource.LogicalID, svcStackResourceNLBTargetGroupLogicalID) {
			nlbEnabled = true
		}
		if strings.HasPrefix(resource.LogicalID, svcStackResourceNLBTargetGroupV2LogicalID) {
			nlbEnabled = true
		}
	}

	var uri LBWebServiceURI
	if albEnabled {
		uriDescr := &uriDescriber{
			svc:              d.svc,
			env:              envName,
			svcDescriber:     svcDescr,
			envDescriber:     envDescr,
			initLBDescriber:  d.initLBDescriber,
			albCFNOutputName: envOutputPublicLoadBalancerDNSName,
		}
		if importedALB {
			uriDescr.albCFNOutputName = ""
		}
		publicURI, err := uriDescr.uri()
		if err != nil {
			return URI{}, err
		}
		uri.access = publicURI
	}

	if nlbEnabled {
		nlbURI, err := d.nlbURI(envName, svcDescr, envDescr)
		if err != nil {
			return URI{}, err
		}
		uri.nlbURI = nlbURI
	}

	return URI{
		URI:        uri.String(),
		AccessType: URIAccessTypeInternet,
	}, nil
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
func (d *BackendServiceDescriber) URI(envName string) (URI, error) {
	svcDescr, err := d.initECSServiceDescribers(envName)
	if err != nil {
		return URI{}, err
	}
	envDescr, err := d.initEnvDescribers(envName)
	if err != nil {
		return URI{}, err
	}
	resources, err := svcDescr.StackResources()
	if err != nil {
		return URI{}, fmt.Errorf("get stack resources for service %s: %w", d.svc, err)
	}
	for _, res := range resources {
		if res.LogicalID == svcStackResourceALBTargetGroupLogicalID {
			uriDescr := &uriDescriber{
				svc:              d.svc,
				env:              envName,
				svcDescriber:     svcDescr,
				envDescriber:     envDescr,
				initLBDescriber:  d.initLBDescriber,
				albCFNOutputName: envOutputInternalLoadBalancerDNSName,
			}
			privateURI, err := uriDescr.uri()
			if err != nil {
				return URI{}, err
			}
			if !privateURI.HTTPS && len(privateURI.DNSNames) > 1 {
				privateURI = uriDescr.bestEffortRemoveALBDNSName(privateURI)
			}
			return URI{
				URI:        english.OxfordWordSeries(privateURI.strings(), "or"),
				AccessType: URIAccessTypeInternal,
			}, nil
		}
	}

	svcStackParams, err := svcDescr.Params()
	if err != nil {
		return URI{}, fmt.Errorf("get stack parameters for environment %s: %w", envName, err)
	}
	if !isReachableWithinVPC(svcStackParams) {
		return URI{
			URI:        BlankServiceDiscoveryURI,
			AccessType: URIAccessTypeNone,
		}, nil
	}
	scDNSNames, err := svcDescr.ServiceConnectDNSNames()
	if err != nil {
		return URI{}, fmt.Errorf("retrieve service connect DNS names: %w", err)
	}
	if len(scDNSNames) > 0 {
		return URI{
			URI:        english.OxfordWordSeries(scDNSNames, "or"),
			AccessType: URIAccessTypeServiceConnect,
		}, nil
	}
	endpoint, err := envDescr.ServiceDiscoveryEndpoint()
	if err != nil {
		return URI{}, fmt.Errorf("retrieve service discovery endpoint for environment %s: %w", envName, err)
	}
	s := serviceDiscovery{
		Service:  d.svc,
		Port:     svcStackParams[stack.WorkloadTargetPortParamKey],
		Endpoint: endpoint,
	}
	return URI{
		URI:        s.String(),
		AccessType: URIAccessTypeServiceDiscovery,
	}, nil
}

type uriDescriber struct {
	svc              string
	env              string
	svcDescriber     ecsDescriber
	envDescriber     envDescriber
	initLBDescriber  func(string) (lbDescriber, error)
	albCFNOutputName string // The DNS name for the public or private ALB.
}

func (d *uriDescriber) envDNSName(path string) (accessURI, error) {
	var dnsNames []string

	envOutputs, err := d.envDescriber.Outputs()
	if err != nil {
		return accessURI{}, fmt.Errorf("get stack outputs for environment %s: %w", d.env, err)
	}
	// Backward compatibility concern since envOutputPublicALBAccessible didn't exist before,
	// and public ALB DNS name previously was always accessible until the introduction of envOutputPublicALBAccessible.
	if accessible, ok := envOutputs[envOutputPublicALBAccessible]; !ok || accessible == "true" {
		dnsNames = append(dnsNames, envOutputs[d.albCFNOutputName])
	}
	if cfDNS, ok := envOutputs[envOutputCloudFrontDomainName]; ok {
		dnsNames = append(dnsNames, cfDNS)
	}
	return accessURI{
		DNSNames: dnsNames,
		Path:     path,
	}, nil
}

func (d *uriDescriber) uri() (accessURI, error) {
	svcParams, err := d.svcDescriber.Params()
	if err != nil {
		return accessURI{}, fmt.Errorf("get stack parameters for service %s: %w", d.svc, err)
	}

	path := svcParams[stack.WorkloadRulePathParamKey]
	httpsEnabled := svcParams[stack.WorkloadHTTPSParamKey] == "true"

	// public load balancers use the env DNS name if https is not enabled
	if d.albCFNOutputName == envOutputPublicLoadBalancerDNSName && !httpsEnabled {
		return d.envDNSName(path)
	}

	svcResources, err := d.svcDescriber.StackResources()
	if err != nil {
		return accessURI{}, fmt.Errorf("get stack resources for service %s: %w", d.svc, err)
	}

	var ruleARNs []string
	for _, resource := range svcResources {
		if resource.Type == svcStackResourceListenerRuleResourceType &&
			((httpsEnabled && strings.HasPrefix(resource.LogicalID, svcStackResourceHTTPSListenerRuleLogicalID)) ||
				(!httpsEnabled && strings.HasPrefix(resource.LogicalID, svcStackResourceHTTPListenerRuleLogicalID))) {
			ruleARNs = append(ruleARNs, resource.PhysicalID)
		}
	}

	lbDescr, err := d.initLBDescriber(d.env)
	if err != nil {
		return accessURI{}, nil
	}
	dnsNames, err := lbDescr.ListenerRulesHostHeaders(ruleARNs)
	if err != nil {
		return accessURI{}, fmt.Errorf("get host headers for listener rules %s: %w", strings.Join(ruleARNs, ","), err)
	}
	if len(dnsNames) == 0 {
		envVars, _ := d.svcDescriber.EnvVars()
		var lbDNS string
		for _, envVar := range envVars {
			if envVar.Name == LBDNS {
				lbDNS = envVar.Value
				return accessURI{
					HTTPS:    httpsEnabled,
					DNSNames: []string{lbDNS},
					Path:     path,
				}, nil
			}
		}
		return d.envDNSName(path)
	}
	return accessURI{
		HTTPS:    httpsEnabled,
		DNSNames: dnsNames,
		Path:     path,
	}, nil
}

func (d *uriDescriber) bestEffortRemoveALBDNSName(accessURI accessURI) accessURI {
	envOutputs, err := d.envDescriber.Outputs()
	if err != nil {
		return accessURI
	}
	lbDNSName := envOutputs[d.albCFNOutputName]
	for i := range accessURI.DNSNames {
		if accessURI.DNSNames[i] == lbDNSName {
			accessURI.DNSNames = append(accessURI.DNSNames[:i], accessURI.DNSNames[i+1:]...)
			break
		}
	}
	return accessURI
}

// URI returns the WebServiceURI to identify this service uniquely given an environment name.
func (d *RDWebServiceDescriber) URI(envName string) (URI, error) {
	describer, err := d.initAppRunnerDescriber(envName)
	if err != nil {
		return URI{}, err
	}

	serviceURL, err := describer.ServiceURL()
	if err != nil {
		return URI{}, fmt.Errorf("get outputs for service %q: %w", d.svc, err)
	}

	isPrivate, err := describer.IsPrivate()
	if err != nil {
		return URI{}, fmt.Errorf("check if service %q is private: %w", d.svc, err)
	}

	accessType := URIAccessTypeInternet
	if isPrivate {
		accessType = URIAccessTypeInternal
	}

	return URI{
		URI:        serviceURL,
		AccessType: accessType,
	}, nil
}

// LBWebServiceURI represents the unique identifier to access a load balanced web service.
type LBWebServiceURI struct {
	access accessURI
	nlbURI nlbURI
}

type accessURI struct {
	HTTPS    bool
	DNSNames []string // The environment's subdomain if the service is served on HTTPS. Otherwise, the public application load balancer's access point.
	Path     string   // Empty if the service is served on HTTPS. Otherwise, the pattern used to match the service.
}

type nlbURI struct {
	DNSNames []string
	Port     string
}

func (u *LBWebServiceURI) String() string {
	uris := u.access.strings()
	for _, dnsName := range u.nlbURI.DNSNames {
		uris = append(uris, color.HighlightResource(fmt.Sprintf("%s:%s", dnsName, u.nlbURI.Port)))
	}
	return english.OxfordWordSeries(uris, "or")
}

func (u *accessURI) strings() []string {
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
		uris = append(uris, color.HighlightResource(protocol+dnsName+path))
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
