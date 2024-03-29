// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"

	"github.com/aws/aws-sdk-go/aws/awserr"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	envOutputPublicLoadBalancerDNSName   = "PublicLoadBalancerDNSName"
	envOutputInternalLoadBalancerDNSName = "InternalLoadBalancerDNSName"
	envOutputSubdomain                   = "EnvironmentSubdomain"
	envOutputCloudFrontDomainName        = "CloudFrontDomainName"
	envOutputPublicALBAccessible         = "PublicALBAccessible"

	svcStackResourceALBTargetGroupLogicalID             = "TargetGroup"
	svcStackResourceNLBTargetGroupLogicalID             = "NLBTargetGroup" // Deprecated. Only retained so 'svc show' can still work without redeploying service.
	svcStackResourceNLBTargetGroupV2LogicalID           = "NetworkLoadBalancerTargetGroup"
	svcStackResourceHTTPSListenerRuleLogicalID          = "HTTPSListenerRule"
	svcStackResourceHTTPListenerRuleLogicalID           = "HTTPListenerRule"
	svcStackResourceListenerRuleForImportedALBLogicalID = "HTTPListenerRuleForImportedALB"
	svcStackResourceListenerRuleResourceType            = "AWS::ElasticLoadBalancingV2::ListenerRule"
	svcOutputPublicNLBDNSName                           = "PublicNetworkLoadBalancerDNSName"
)

type envDescriber interface {
	ServiceDiscoveryEndpoint() (string, error)
	Params() (map[string]string, error)
	Outputs() (map[string]string, error)
}

type lbDescriber interface {
	ListenerRulesHostHeaders(ruleARNs []string) ([]string, error)
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
	initCWDescriber          func(string) (cwAlarmDescriber, error)
	ecsServiceDescribers     map[string]ecsDescriber
	envDescriber             map[string]envDescriber
	cwAlarmDescribers        map[string]cwAlarmDescriber
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
			Env:         env,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		})
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
	describer.initCWDescriber = func(envName string) (cwAlarmDescriber, error) {
		if describer, ok := describer.cwAlarmDescribers[envName]; ok {
			return describer, nil
		}
		env, err := opt.ConfigStore.GetEnvironment(opt.App, envName)
		if err != nil {
			return nil, fmt.Errorf("get environment %s: %w", envName, err)
		}
		sess, err := sessions.ImmutableProvider().FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return nil, err
		}
		return cloudwatch.New(sess), nil
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
	svcDiscoveries := make(serviceDiscoveries)
	svcConnects := make(serviceConnects)
	var envVars []*containerEnvVar
	var secrets []*secret
	var alarmDescriptions []*cloudwatch.AlarmDescription
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
		alarmNames, err := svcDescr.RollbackAlarmNames()
		if err != nil {
			return nil, fmt.Errorf("retrieve rollback alarm names: %w", err)
		}
		if len(alarmNames) != 0 {
			cwAlarmDescr, err := d.initCWDescriber(env)
			if err != nil {
				return nil, err
			}
			alarms, err := cwAlarmDescr.AlarmDescriptions(alarmNames)
			if err != nil {
				return nil, fmt.Errorf("retrieve alarm descriptions: %w", err)
			}
			for _, alarm := range alarms {
				alarm.Environment = env
			}
			alarmDescriptions = append(alarmDescriptions, alarms...)
		}
		envDescr, err := d.initEnvDescribers(env)
		if err != nil {
			return nil, err
		}
		if err := svcDiscoveries.collectEndpoints(
			envDescr, d.svc, env, svcParams[cfnstack.WorkloadTargetPortParamKey]); err != nil {
			return nil, err
		}
		if err := svcConnects.collectEndpoints(svcDescr, env); err != nil {
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
			stackResources, err := svcDescr.StackResources()
			if err != nil {
				return nil, fmt.Errorf("retrieve service resources: %w", err)
			}
			resources[env] = stackResources
		}
	}

	return &webSvcDesc{
		ecsSvcDesc: ecsSvcDesc{
			Service:           d.svc,
			Type:              manifestinfo.LoadBalancedWebServiceType,
			App:               d.app,
			Configurations:    configs,
			AlarmDescriptions: alarmDescriptions,
			Routes:            routes,
			ServiceDiscovery:  svcDiscoveries,
			ServiceConnect:    svcConnects,
			Variables:         envVars,
			Secrets:           secrets,
			Resources:         resources,

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

// WebServiceRoute contains serialized route parameters for a web service.
type WebServiceRoute struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
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
	if len(w.AlarmDescriptions) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nRollback Alarms\n\n"))
		writer.Flush()
		rollbackAlarms(w.AlarmDescriptions).humanString(writer)
	}
	fmt.Fprint(writer, color.Bold.Sprint("\nRoutes\n\n"))
	writer.Flush()
	headers := []string{"Environment", "URL"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, route := range w.Routes {
		fmt.Fprintf(writer, "  %s\t%s\n", route.Environment, route.URL)
	}
	if len(w.ServiceConnect) > 0 || len(w.ServiceDiscovery) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nInternal Service Endpoints\n\n"))
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
