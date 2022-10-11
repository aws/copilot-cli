// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"

	"github.com/aws/aws-sdk-go/aws/awserr"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	envOutputPublicLoadBalancerDNSName   = "PublicLoadBalancerDNSName"
	envOutputInternalLoadBalancerDNSName = "InternalLoadBalancerDNSName"
	envOutputSubdomain                   = "EnvironmentSubdomain"
	envOutputCloudFrontDomainName        = "CloudFrontDomainName"
	envOutputPublicALBAccessible         = "PublicALBAccessible"

	svcStackResourceALBTargetGroupLogicalID    = "TargetGroup"
	svcStackResourceNLBTargetGroupLogicalID    = "NLBTargetGroup"
	svcStackResourceHTTPSListenerRuleLogicalID = "HTTPSListenerRule"
	svcStackResourceHTTPListenerRuleLogicalID  = "HTTPListenerRule"
	svcStackResourceListenerRuleResourceType   = "AWS::ElasticLoadBalancingV2::ListenerRule"
	svcOutputPublicNLBDNSName                  = "PublicNetworkLoadBalancerDNSName"
)

type envDescriber interface {
	ServiceDiscoveryEndpoint() (string, error)
	Params() (map[string]string, error)
	Outputs() (map[string]string, error)
}

type lbDescriber interface {
	ListenerRuleHostHeaders(ruleARN string) ([]string, error)
}

// LBWebServiceDescriber retrieves information about a load balanced web service.
type LBWebServiceDescriber struct {
	app             string
	svc             string
	enableResources bool

	store                    DeployedEnvServicesLister
	initECSServiceDescribers func(string) (ecsDescriber, error)
	initEnvDescribers        func(string) (envDescriber, error)
	initLBDescriber          func(string) (lbDescriber, error)
	ecsServiceDescribers     map[string]ecsDescriber
	envDescriber             map[string]envDescriber
}

// NewLBWebServiceDescriber instantiates a load balanced service describer.
func NewLBWebServiceDescriber(opt NewServiceConfig) (*LBWebServiceDescriber, error) {
	describer := &LBWebServiceDescriber{
		app:                  opt.App,
		svc:                  opt.Svc,
		enableResources:      opt.EnableResources,
		store:                opt.DeployStore,
		ecsServiceDescribers: make(map[string]ecsDescriber),
		envDescriber:         make(map[string]envDescriber),
	}
	describer.initLBDescriber = func(envName string) (lbDescriber, error) {
		env, err := opt.ConfigStore.GetEnvironment(opt.App, envName)
		if err != nil {
			return nil, fmt.Errorf("get environment %s: %w", envName, err)
		}
		sess, err := sessions.ImmutableProvider().FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return nil, err
		}
		return elbv2.New(sess), nil
	}
	describer.initECSServiceDescribers = func(env string) (ecsDescriber, error) {
		if describer, ok := describer.ecsServiceDescribers[env]; ok {
			return describer, nil
		}
		svcDescr, err := newECSServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		}, env)
		if err != nil {
			return nil, err
		}
		describer.ecsServiceDescribers[env] = svcDescr
		return svcDescr, nil
	}
	describer.initEnvDescribers = func(env string) (envDescriber, error) {
		if describer, ok := describer.envDescriber[env]; ok {
			return describer, nil
		}
		envDescr, err := NewEnvDescriber(NewEnvDescriberConfig{
			App:         opt.App,
			Env:         env,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return nil, err
		}
		describer.envDescriber[env] = envDescr
		return envDescr, nil
	}
	return describer, nil
}

// Describe returns info of a web service.
func (d *LBWebServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for application %s: %w", d.app, err)
	}

	var routes []*WebServiceRoute
	var configs []*ECSServiceConfig
	var svcDiscoveries serviceDiscoveries
	var svcConnects serviceConnects
	var envVars []*containerEnvVar
	var secrets []*secret
	for _, env := range environments {
		svcDescr, err := d.initECSServiceDescribers(env)
		if err != nil {
			return nil, err
		}
		uri, err := d.URI(env)
		if err != nil {
			return nil, fmt.Errorf("retrieve service URI: %w", err)
		}
		routes = append(routes, &WebServiceRoute{
			Environment: env,
			URL:         uri.URI,
		})
		containerPlatform, err := svcDescr.Platform()
		if err != nil {
			return nil, fmt.Errorf("retrieve platform: %w", err)
		}
		webSvcEnvVars, err := svcDescr.EnvVars()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment variables: %w", err)
		}
		svcParams, err := svcDescr.Params()
		if err != nil {
			return nil, fmt.Errorf("get stack parameters for service %s: %w", d.svc, err)
		}
		configs = append(configs, &ECSServiceConfig{
			ServiceConfig: &ServiceConfig{
				Environment: env,
				Port:        svcParams[cfnstack.WorkloadTargetPortParamKey],
				CPU:         svcParams[cfnstack.WorkloadTaskCPUParamKey],
				Memory:      svcParams[cfnstack.WorkloadTaskMemoryParamKey],
				Platform:    dockerengine.PlatformString(containerPlatform.OperatingSystem, containerPlatform.Architecture),
			},
			Tasks: svcParams[cfnstack.WorkloadTaskCountParamKey],
		})
		envDescr, err := d.initEnvDescribers(env)
		if err != nil {
			return nil, err
		}
		if err := svcDiscoveries.collectServiceDiscoveryEndpoints(
			envDescr, d.svc, env, svcParams[cfnstack.WorkloadTargetPortParamKey]); err != nil {
			return nil, err
		}
		if err := svcConnects.collectServiceConnectEndpoints(svcDescr, env); err != nil {
			return nil, err
		}
		envVars = append(envVars, flattenContainerEnvVars(env, webSvcEnvVars)...)
		webSvcSecrets, err := svcDescr.Secrets()
		if err != nil {
			return nil, fmt.Errorf("retrieve secrets: %w", err)
		}
		secrets = append(secrets, flattenSecrets(env, webSvcSecrets)...)
	}
	resources := make(map[string][]*stack.Resource)
	if d.enableResources {
		for _, env := range environments {
			svcDescr, err := d.initECSServiceDescribers(env)
			if err != nil {
				return nil, err
			}
			stackResources, err := svcDescr.ServiceStackResources()
			if err != nil {
				return nil, fmt.Errorf("retrieve service resources: %w", err)
			}
			resources[env] = stackResources
		}
	}

	return &webSvcDesc{
		ecsSvcDesc: ecsSvcDesc{
			Service:          d.svc,
			Type:             manifest.LoadBalancedWebServiceType,
			App:              d.app,
			Configurations:   configs,
			Routes:           routes,
			ServiceDiscovery: svcDiscoveries,
			ServiceConnect:   svcConnects,
			Variables:        envVars,
			Secrets:          secrets,
			Resources:        resources,

			environments: environments,
		},
	}, nil
}

// Manifest returns the contents of the manifest used to deploy a load balanced web service stack.
// If the Manifest metadata doesn't exist in the stack template, then returns ErrManifestNotFoundInTemplate.
func (d *LBWebServiceDescriber) Manifest(env string) ([]byte, error) {
	cfn, err := d.initECSServiceDescribers(env)
	if err != nil {
		return nil, err
	}
	return cfn.Manifest()
}

type secret struct {
	Name        string `json:"name"`
	Container   string `json:"container"`
	Environment string `json:"environment"`
	ValueFrom   string `json:"valueFrom"`
}

type secrets []*secret

func (s secrets) humanString(w io.Writer) {
	headers := []string{"Name", "Container", "Environment", "Value From"}
	fmt.Fprintf(w, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(w, "  %s\n", strings.Join(underline(headers), "\t"))
	sort.SliceStable(s, func(i, j int) bool { return s[i].Environment < s[j].Environment })
	sort.SliceStable(s, func(i, j int) bool { return s[i].Container < s[j].Container })
	sort.SliceStable(s, func(i, j int) bool { return s[i].Name < s[j].Name })
	if len(s) > 0 {
		valueFrom := s[0].ValueFrom
		if _, err := arn.Parse(s[0].ValueFrom); err != nil {
			// If the valueFrom is not an ARN, preface it with "parameter/"
			valueFrom = fmt.Sprintf("parameter/%s", s[0].ValueFrom)
		}
		fmt.Fprintf(w, "  %s\n", strings.Join([]string{s[0].Name, s[0].Container, s[0].Environment, valueFrom}, "\t"))
	}
	for prev, cur := 0, 1; cur < len(s); prev, cur = prev+1, cur+1 {
		valueFrom := s[cur].ValueFrom
		if _, err := arn.Parse(s[cur].ValueFrom); err != nil {
			// If the valueFrom is not an ARN, preface it with "parameter/"
			valueFrom = fmt.Sprintf("parameter/%s", s[cur].ValueFrom)
		}
		cols := []string{s[cur].Name, s[cur].Container, s[cur].Environment, valueFrom}
		if s[prev].Name == s[cur].Name {
			cols[0] = dittoSymbol
		}
		if s[prev].Container == s[cur].Container {
			cols[1] = dittoSymbol
		}
		if s[prev].Environment == s[cur].Environment {
			cols[2] = dittoSymbol
		}
		if s[prev].ValueFrom == s[cur].ValueFrom {
			cols[3] = dittoSymbol
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(cols, "\t"))
	}
}

func underline(headings []string) []string {
	var lines []string
	for _, heading := range headings {
		line := strings.Repeat("-", len(heading))
		lines = append(lines, line)
	}
	return lines
}

// WebServiceRoute contains serialized route parameters for a web service.
type WebServiceRoute struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
}

// ServiceDiscovery contains serialized service discovery info for an ECS service.
type ServiceDiscovery struct {
	Environment []string `json:"environment"`
	// Namespace is not an accurate name but unfortunately we can't take it back.
	Namespace string `json:"namespace"`
}

type serviceDiscoveries []*ServiceDiscovery

func (sds *serviceDiscoveries) collectServiceDiscoveryEndpoints(descr envDescriber, svc, env, port string) error {
	endpoint, err := descr.ServiceDiscoveryEndpoint()
	if err != nil {
		return err
	}
	sds.appendServiceDiscovery(serviceDiscovery{
		Service:  svc,
		Port:     port,
		Endpoint: endpoint,
	}, env)
	return nil
}

func (sds *serviceDiscoveries) appendServiceDiscovery(sd serviceDiscovery, env string) {
	exist := false
	for _, svcDiscovery := range *sds {
		if svcDiscovery.Namespace == sd.String() {
			svcDiscovery.Environment = append(svcDiscovery.Environment, env)
			exist = true
		}
	}
	if !exist {
		*sds = append(*sds, &ServiceDiscovery{
			Environment: []string{env},
			Namespace:   sd.String(),
		})
	}
	return
}

// ServiceConnect contains serialized service connect info for an ECS service.
type ServiceConnect struct {
	Environment []string `json:"environment"`
	DNSName     string   `json:"dnsName"`
}

type serviceConnects []*ServiceConnect

func (scs *serviceConnects) collectServiceConnectEndpoints(descr ecsDescriber, env string) error {
	scDNSNames, err := descr.ServiceConnectDNSNames()
	if err != nil {
		return fmt.Errorf("retrieve service connect DNS names: %w", err)
	}
	for _, dnsName := range scDNSNames {
		scs.appendServiceConnect(dnsName, env)
	}
	return nil
}

func (scs *serviceConnects) appendServiceConnect(dnsName string, env string) {
	exist := false
	for _, svcConnect := range *scs {
		if svcConnect.DNSName == dnsName {
			svcConnect.Environment = append(svcConnect.Environment, env)
			exist = true
		}
	}
	if !exist {
		*scs = append(*scs, &ServiceConnect{
			Environment: []string{env},
			DNSName:     dnsName,
		})
	}
	return
}

type serviceEndpoints struct {
	discoveries serviceDiscoveries
	connects    serviceConnects
}

func (s serviceEndpoints) humanString(w io.Writer) {
	headers := []string{"Environment", "Endpoint", "Type"}
	fmt.Fprintf(w, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(w, "  %s\n", strings.Join(underline(headers), "\t"))
	scEnvToEndpoints := make(map[string]map[string]struct{})
	for _, sc := range s.connects {
		envs := strings.Join(sc.Environment, ", ")
		if _, ok := scEnvToEndpoints[envs]; !ok {
			scEnvToEndpoints[envs] = make(map[string]struct{})
		}
		scEnvToEndpoints[envs][sc.DNSName] = struct{}{}
	}
	var scEnvs []string
	for env := range scEnvToEndpoints {
		scEnvs = append(scEnvs, env)
	}
	sort.SliceStable(scEnvs, func(i int, j int) bool { return scEnvs[i] < scEnvs[j] })
	for _, env := range scEnvs {
		var endpoints []string
		for endpoint := range scEnvToEndpoints[env] {
			endpoints = append(endpoints, endpoint)
		}
		sort.SliceStable(endpoints, func(i int, j int) bool { return endpoints[i] < endpoints[j] })
		fmt.Fprintf(w, "  %s\t%s\t%s\n", env, strings.Join(endpoints, ", "), "Service Connect")
	}
	sdEnvToEndpoints := make(map[string]map[string]struct{})
	for _, sd := range s.discoveries {
		envs := strings.Join(sd.Environment, ", ")
		if _, ok := sdEnvToEndpoints[envs]; !ok {
			sdEnvToEndpoints[envs] = make(map[string]struct{})
		}
		sdEnvToEndpoints[envs][sd.Namespace] = struct{}{}
	}
	var sdEnvs []string
	for env := range sdEnvToEndpoints {
		sdEnvs = append(sdEnvs, env)
	}
	sort.SliceStable(sdEnvs, func(i int, j int) bool { return sdEnvs[i] < sdEnvs[j] })
	for _, env := range sdEnvs {
		var endpoints []string
		for endpoint := range sdEnvToEndpoints[env] {
			endpoints = append(endpoints, endpoint)
		}
		sort.SliceStable(endpoints, func(i int, j int) bool { return endpoints[i] < endpoints[j] })
		fmt.Fprintf(w, "  %s\t%s\t%s\n", env, strings.Join(endpoints, ", "), "Service Discovery")
	}
}

// webSvcDesc contains serialized parameters for a web service.
type webSvcDesc struct {
	ecsSvcDesc
}

// JSONString returns the stringified webSvcDesc struct in json format.
func (w *webSvcDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal web service description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified webService struct in human readable format.
func (w *webSvcDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Application", w.App)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.Service)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", w.Type)
	fmt.Fprint(writer, color.Bold.Sprint("\nConfigurations\n\n"))
	writer.Flush()
	w.Configurations.humanString(writer)
	fmt.Fprint(writer, color.Bold.Sprint("\nRoutes\n\n"))
	writer.Flush()
	headers := []string{"Environment", "URL"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, route := range w.Routes {
		fmt.Fprintf(writer, "  %s\t%s\n", route.Environment, route.URL)
	}
	if len(w.ServiceConnect) > 0 || len(w.ServiceDiscovery) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nInternal Service Endpoint\n\n"))
		writer.Flush()
		endpoints := serviceEndpoints{
			discoveries: w.ServiceDiscovery,
			connects:    w.ServiceConnect,
		}
		endpoints.humanString(writer)
	}
	fmt.Fprint(writer, color.Bold.Sprint("\nVariables\n\n"))
	writer.Flush()
	w.Variables.humanString(writer)
	if len(w.Secrets) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nSecrets\n\n"))
		writer.Flush()
		w.Secrets.humanString(writer)
	}
	if len(w.Resources) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nResources\n"))
		writer.Flush()

		w.Resources.humanStringByEnv(writer, w.environments)
	}
	writer.Flush()
	return b.String()
}

func cpuToString(s string) string {
	cpuInt, _ := strconv.Atoi(s)
	cpuFloat := float64(cpuInt) / 1024
	return fmt.Sprintf("%g", cpuFloat)
}

// IsStackNotExistsErr returns true if error type is stack not exist.
func IsStackNotExistsErr(err error) bool {
	if err == nil {
		return false
	}
	aerr, ok := err.(awserr.Error)
	if !ok {
		return IsStackNotExistsErr(errors.Unwrap(err))
	}
	if aerr.Code() != "ValidationError" {
		return IsStackNotExistsErr(errors.Unwrap(err))
	}
	if !strings.Contains(aerr.Message(), "does not exist") {
		return IsStackNotExistsErr(errors.Unwrap(err))
	}
	return true
}
