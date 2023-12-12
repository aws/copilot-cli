// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"

	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template/override"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	enabled  = "ENABLED"
	disabled = "DISABLED"
)

// SQS Queue field values.
const (
	sqsDedupeScopeMessageGroup              = "messageGroup"
	sqsFIFOThroughputLimitPerMessageGroupId = "perMessageGroupId"
)

// Default values for EFS options
const (
	defaultRootDirectory   = "/"
	defaultIAM             = disabled
	defaultReadOnly        = true
	defaultWritePermission = false
	defaultNLBProtocol     = manifest.TCP
)

// Supported capacityproviders for Fargate services
const (
	capacityProviderFargateSpot = "FARGATE_SPOT"
	capacityProviderFargate     = "FARGATE"
)

// MinimumHealthyPercent and MaximumPercent configurations as per deployment strategy.
const (
	minHealthyPercentRecreate = 0
	maxPercentRecreate        = 100
	minHealthyPercentDefault  = 100
	maxPercentDefault         = 200
)

var (
	taskDefOverrideRulePrefixes = []string{"Resources", "TaskDefinition", "Properties"}
	subnetPlacementForTemplate  = map[manifest.PlacementString]string{
		manifest.PrivateSubnetPlacement: template.PrivateSubnetsPlacement,
		manifest.PublicSubnetPlacement:  template.PublicSubnetsPlacement,
	}
)

func convertPortMappings(exposedPorts []manifest.ExposedPort) []*template.PortMapping {
	portMapping := make([]*template.PortMapping, len(exposedPorts))
	for idx, exposedPort := range exposedPorts {
		portMapping[idx] = &template.PortMapping{
			ContainerPort: exposedPort.Port,
			Protocol:      exposedPort.Protocol,
			ContainerName: exposedPort.ContainerName,
		}
	}
	return portMapping
}

// convertSidecars converts the manifest sidecar configuration into a format parsable by the templates pkg.
func convertSidecars(s map[string]*manifest.SidecarConfig, exposedPorts map[string][]manifest.ExposedPort, rc RuntimeConfig) ([]*template.SidecarOpts, error) {
	var sidecars []*template.SidecarOpts
	if s == nil {
		return nil, nil
	}

	// Sort the sidecars so that the order is consistent and the integration test won't be flaky.
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, name := range keys {
		config := s[name]
		var imageURI string
		if image, ok := rc.PushedImages[name]; ok {
			imageURI = image.URI()
		}
		if uri, hasLocation := config.ImageURI(); hasLocation {
			imageURI = uri
		}
		entrypoint, err := convertEntryPoint(config.EntryPoint)
		if err != nil {
			return nil, err
		}
		command, err := convertCommand(config.Command)
		if err != nil {
			return nil, err
		}
		mp := convertSidecarMountPoints(config.MountPoints)
		sidecars = append(sidecars, &template.SidecarOpts{
			Name:       name,
			Image:      aws.String(imageURI),
			Essential:  config.Essential,
			CredsParam: config.CredsParam,
			Secrets:    convertSecrets(config.Secrets),
			Variables:  convertEnvVars(config.Variables),
			Storage: template.SidecarStorageOpts{
				MountPoints: mp,
			},
			DockerLabels: config.DockerLabels,
			DependsOn:    convertDependsOn(config.DependsOn),
			EntryPoint:   entrypoint,
			HealthCheck:  convertContainerHealthCheck(config.HealthCheck),
			Command:      command,
			PortMappings: convertPortMappings(exposedPorts[name]),
		})
	}
	return sidecars, nil
}

func convertContainerHealthCheck(hc manifest.ContainerHealthCheck) *template.ContainerHealthCheck {
	if hc.IsEmpty() {
		return nil
	}
	// Make sure that unset fields in the healthcheck gets a default value.
	hc.ApplyIfNotSet(manifest.NewDefaultContainerHealthCheck())
	return &template.ContainerHealthCheck{
		Command:     hc.Command,
		Interval:    aws.Int64(int64(hc.Interval.Seconds())),
		Retries:     aws.Int64(int64(aws.IntValue(hc.Retries))),
		StartPeriod: aws.Int64(int64(hc.StartPeriod.Seconds())),
		Timeout:     aws.Int64(int64(hc.Timeout.Seconds())),
	}
}

func convertHostedZone(alias manifest.Alias, defaultHostedZone *string) (template.AliasesForHostedZone, error) {
	aliasesFor := make(map[string][]string)
	if len(alias.AdvancedAliases) != 0 {
		for _, alias := range alias.AdvancedAliases {
			if alias.HostedZone != nil {
				if isDuplicateAliasEntry(aliasesFor[*alias.HostedZone], aws.StringValue(alias.Alias)) {
					continue
				}
				aliasesFor[*alias.HostedZone] = append(aliasesFor[*alias.HostedZone], *alias.Alias)
				continue
			}
			if defaultHostedZone != nil {
				if isDuplicateAliasEntry(aliasesFor[*defaultHostedZone], aws.StringValue(alias.Alias)) {
					continue
				}
				aliasesFor[*defaultHostedZone] = append(aliasesFor[*defaultHostedZone], *alias.Alias)
			}
		}
		return aliasesFor, nil
	}
	if defaultHostedZone == nil {
		return aliasesFor, nil
	}
	aliases, err := alias.ToStringSlice()
	if err != nil {
		return nil, err
	}

	for _, alias := range aliases {
		if isDuplicateAliasEntry(aliasesFor[*defaultHostedZone], alias) {
			continue
		}
		aliasesFor[*defaultHostedZone] = append(aliasesFor[*defaultHostedZone], alias)
	}

	return aliasesFor, nil
}

func isDuplicateAliasEntry(aliasList []string, alias string) bool {
	for _, entry := range aliasList {
		if entry == alias {
			return true
		}
	}
	return false
}

// convertDependsOn converts image and sidecar depends on fields to have upper case statuses.
func convertDependsOn(d manifest.DependsOn) map[string]string {
	if d == nil {
		return nil
	}
	dependsOn := make(map[string]string)
	for name, status := range d {
		dependsOn[name] = strings.ToUpper(status)
	}
	return dependsOn
}

func convertAdvancedCount(a manifest.AdvancedCount) (*template.AdvancedCount, error) {
	if a.IsEmpty() {
		return nil, nil
	}
	autoscaling, err := convertAutoscaling(a)
	if err != nil {
		return nil, err
	}
	return &template.AdvancedCount{
		Spot:        a.Spot,
		Autoscaling: autoscaling,
		Cps:         convertCapacityProviders(a),
	}, nil
}

// convertCapacityProviders transforms the manifest fields into a format
// parsable by the templates pkg.
func convertCapacityProviders(a manifest.AdvancedCount) []*template.CapacityProviderStrategy {
	if a.Spot == nil && a.Range.RangeConfig.SpotFrom == nil {
		return nil
	}
	var cps []*template.CapacityProviderStrategy
	// if Spot specified as count, then weight on Spot CPS should be 1
	cps = append(cps, &template.CapacityProviderStrategy{
		Weight:           aws.Int(1),
		CapacityProvider: capacityProviderFargateSpot,
	})
	rc := a.Range.RangeConfig
	// Return if only spot is specified as count
	if rc.SpotFrom == nil {
		return cps
	}
	// Scaling with spot
	spotFrom := aws.IntValue(rc.SpotFrom)
	min := aws.IntValue(rc.Min)
	// If spotFrom value is greater than or equal to the autoscaling min,
	// then the base value on the Fargate capacity provider must be set
	// to one less than spotFrom
	if spotFrom >= min {
		base := spotFrom - 1
		if base < 0 {
			base = 0
		}
		fgCapacity := &template.CapacityProviderStrategy{
			Base:             aws.Int(base),
			Weight:           aws.Int(0),
			CapacityProvider: capacityProviderFargate,
		}
		cps = append(cps, fgCapacity)
	}
	return cps
}

// convertCooldown converts a service manifest cooldown struct into a format parsable
// by the templates pkg.
func convertCooldown(c manifest.Cooldown) template.Cooldown {
	if c.IsEmpty() {
		return template.Cooldown{}
	}

	cooldown := template.Cooldown{}

	if c.ScaleInCooldown != nil {
		scaleInTime := float64(*c.ScaleInCooldown) / float64(time.Second)
		cooldown.ScaleInCooldown = aws.Float64(scaleInTime)
	}
	if c.ScaleOutCooldown != nil {
		scaleOutTime := float64(*c.ScaleOutCooldown) / float64(time.Second)
		cooldown.ScaleOutCooldown = aws.Float64(scaleOutTime)
	}

	return cooldown
}

// convertScalingCooldown handles the logic of converting generalized and specific cooldowns set
// into the scaling cooldown used in the Auto Scaling configuration.
func convertScalingCooldown(specCooldown, genCooldown manifest.Cooldown) template.Cooldown {
	cooldown := convertCooldown(genCooldown)

	specTemplateCooldown := convertCooldown(specCooldown)
	if specCooldown.ScaleInCooldown != nil {
		cooldown.ScaleInCooldown = specTemplateCooldown.ScaleInCooldown
	}
	if specCooldown.ScaleOutCooldown != nil {
		cooldown.ScaleOutCooldown = specTemplateCooldown.ScaleOutCooldown
	}

	return cooldown
}

// convertAutoscaling converts the service's Auto Scaling configuration into a format parsable
// by the templates pkg.
func convertAutoscaling(a manifest.AdvancedCount) (*template.AutoscalingOpts, error) {
	if a.IsEmpty() {
		return nil, nil
	}
	if a.Spot != nil {
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

	if a.CPU.Value != nil {
		autoscalingOpts.CPU = aws.Float64(float64(*a.CPU.Value))
	}
	if a.CPU.ScalingConfig.Value != nil {
		autoscalingOpts.CPU = aws.Float64(float64(*a.CPU.ScalingConfig.Value))
	}
	if a.Memory.Value != nil {
		autoscalingOpts.Memory = aws.Float64(float64(*a.Memory.Value))
	}
	if a.Memory.ScalingConfig.Value != nil {
		autoscalingOpts.Memory = aws.Float64(float64(*a.Memory.ScalingConfig.Value))
	}
	if a.Requests.Value != nil {
		autoscalingOpts.Requests = aws.Float64(float64(*a.Requests.Value))
	}
	if a.Requests.ScalingConfig.Value != nil {
		autoscalingOpts.Requests = aws.Float64(float64(*a.Requests.ScalingConfig.Value))
	}
	if a.ResponseTime.Value != nil {
		responseTime := float64(*a.ResponseTime.Value) / float64(time.Second)
		autoscalingOpts.ResponseTime = aws.Float64(responseTime)
	}
	if a.ResponseTime.ScalingConfig.Value != nil {
		responseTime := float64(*a.ResponseTime.ScalingConfig.Value) / float64(time.Second)
		autoscalingOpts.ResponseTime = aws.Float64(responseTime)
	}

	autoscalingOpts.CPUCooldown = convertScalingCooldown(a.CPU.ScalingConfig.Cooldown, a.Cooldown)
	autoscalingOpts.MemCooldown = convertScalingCooldown(a.Memory.ScalingConfig.Cooldown, a.Cooldown)
	autoscalingOpts.ReqCooldown = convertScalingCooldown(a.Requests.ScalingConfig.Cooldown, a.Cooldown)
	autoscalingOpts.RespTimeCooldown = convertScalingCooldown(a.ResponseTime.ScalingConfig.Cooldown, a.Cooldown)
	autoscalingOpts.QueueDelayCooldown = convertScalingCooldown(a.QueueScaling.Cooldown, a.Cooldown)

	if !a.QueueScaling.IsEmpty() {
		acceptableBacklog, err := a.QueueScaling.AcceptableBacklogPerTask()
		if err != nil {
			return nil, err
		}
		autoscalingOpts.QueueDelay = &template.AutoscalingQueueDelayOpts{
			AcceptableBacklogPerTask: acceptableBacklog,
		}
	}
	return &autoscalingOpts, nil
}

// convertHTTPHealthCheck converts the ALB health check configuration into a format parsable by the templates pkg.
func convertHTTPHealthCheck(hc *manifest.HealthCheckArgsOrString) template.HTTPHealthCheckOpts {
	opts := template.HTTPHealthCheckOpts{
		HealthCheckPath:    manifest.DefaultHealthCheckPath,
		GracePeriod:        manifest.DefaultHealthCheckGracePeriod,
		HealthyThreshold:   hc.Advanced.HealthyThreshold,
		UnhealthyThreshold: hc.Advanced.UnhealthyThreshold,
		SuccessCodes:       aws.StringValue(hc.Advanced.SuccessCodes),
	}

	if hc.IsZero() {
		return opts
	}
	if hc.IsBasic() {
		opts.HealthCheckPath = convertPath(hc.Basic)
		return opts
	}

	if hc.Advanced.Path != nil {
		opts.HealthCheckPath = convertPath(*hc.Advanced.Path)
	}
	if hc.Advanced.Port != nil {
		opts.Port = strconv.Itoa(aws.IntValue(hc.Advanced.Port))
	}
	if hc.Advanced.Interval != nil {
		opts.Interval = aws.Int64(int64(hc.Advanced.Interval.Seconds()))
	}
	if hc.Advanced.Timeout != nil {
		opts.Timeout = aws.Int64(int64(hc.Advanced.Timeout.Seconds()))
	}
	if hc.Advanced.GracePeriod != nil {
		opts.GracePeriod = int64(hc.Advanced.GracePeriod.Seconds())
	}
	return opts
}

// convertNLBHealthCheck converts the NLB health check configuration into a format parsable by the templates pkg.
func convertNLBHealthCheck(nlbHC *manifest.NLBHealthCheckArgs) template.NLBHealthCheck {
	hc := template.NLBHealthCheck{
		HealthyThreshold:   nlbHC.HealthyThreshold,
		UnhealthyThreshold: nlbHC.UnhealthyThreshold,
		GracePeriod:        aws.Int64(int64(manifest.DefaultHealthCheckGracePeriod)),
	}
	if nlbHC.Port != nil {
		hc.Port = strconv.Itoa(aws.IntValue(nlbHC.Port))
	}
	if nlbHC.Timeout != nil {
		hc.Timeout = aws.Int64(int64(nlbHC.Timeout.Seconds()))
	}
	if nlbHC.Interval != nil {
		hc.Interval = aws.Int64(int64(nlbHC.Interval.Seconds()))
	}
	if nlbHC.GracePeriod != nil {
		hc.GracePeriod = aws.Int64(int64(nlbHC.GracePeriod.Seconds()))
	}
	return hc
}

type networkLoadBalancerConfig struct {
	settings *template.NetworkLoadBalancer

	// If a domain is associated these values are not empty.
	appDNSDelegationRole *string
	appDNSName           *string
}

func convertELBAccessLogsConfig(mft *manifest.Environment) *template.ELBAccessLogs {
	elbAccessLogsArgs, isELBAccessLogsSet := mft.ELBAccessLogs()
	if !isELBAccessLogsSet {
		return nil
	}

	if elbAccessLogsArgs == nil {
		return &template.ELBAccessLogs{}
	}

	return &template.ELBAccessLogs{
		BucketName: aws.StringValue(elbAccessLogsArgs.BucketName),
		Prefix:     aws.StringValue(elbAccessLogsArgs.Prefix),
	}
}

// convertFlowLogsConfig converts the VPC FlowLog configuration into a format parsable by the templates pkg.
func convertFlowLogsConfig(mft *manifest.Environment) (*template.VPCFlowLogs, error) {
	vpcFlowLogs := mft.EnvironmentConfig.Network.VPC.FlowLogs
	if vpcFlowLogs.IsZero() {
		return nil, nil
	}
	retentionInDays := aws.Int(14)
	if vpcFlowLogs.Advanced.Retention != nil {
		retentionInDays = vpcFlowLogs.Advanced.Retention
	}
	return &template.VPCFlowLogs{
		Retention: retentionInDays,
	}, nil

}

func convertEnvSecurityGroupCfg(mft *manifest.Environment) (*template.SecurityGroupConfig, error) {
	securityGroupConfig, isSecurityConfigSet := mft.EnvSecurityGroup()
	if !isSecurityConfigSet {
		return nil, nil
	}
	var ingress = make([]template.SecurityGroupRule, len(securityGroupConfig.Ingress))
	var egress = make([]template.SecurityGroupRule, len(securityGroupConfig.Egress))
	for idx, ingressValue := range securityGroupConfig.Ingress {
		ingress[idx].IpProtocol = ingressValue.IpProtocol
		ingress[idx].CidrIP = ingressValue.CidrIP
		if fromPort, toPort, err := ingressValue.GetPorts(); err != nil {
			return nil, err
		} else {
			ingress[idx].ToPort = toPort
			ingress[idx].FromPort = fromPort
		}
	}
	for idx, egressValue := range securityGroupConfig.Egress {
		egress[idx].IpProtocol = egressValue.IpProtocol
		egress[idx].CidrIP = egressValue.CidrIP
		if fromPort, toPort, err := egressValue.GetPorts(); err != nil {
			return nil, err
		} else {
			egress[idx].ToPort = toPort
			egress[idx].FromPort = fromPort
		}
	}
	return &template.SecurityGroupConfig{
		Ingress: ingress,
		Egress:  egress,
	}, nil
}

func (s *LoadBalancedWebService) convertALBListener() (*template.ALBListener, error) {
	rrConfig := s.manifest.HTTPOrBool
	if rrConfig.Disabled() || rrConfig.IsEmpty() {
		return nil, nil
	}
	var rules []template.ALBListenerRule
	var aliasesFor template.AliasesForHostedZone
	for _, routingRule := range rrConfig.RoutingRules() {
		httpRedirect := true
		if routingRule.RedirectToHTTPS != nil {
			httpRedirect = aws.BoolValue(routingRule.RedirectToHTTPS)
		}
		rule, err := routingRuleConfigConverter{
			rule:            routingRule,
			manifest:        s.manifest,
			httpsEnabled:    s.httpsEnabled,
			redirectToHTTPS: httpRedirect,
		}.convert()
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
		aliasesFor, err = convertHostedZone(rrConfig.Main.Alias, rrConfig.Main.HostedZone)
		if err != nil {
			return nil, err
		}
	}
	return &template.ALBListener{
		Rules:             rules,
		IsHTTPS:           s.httpsEnabled,
		HostedZoneAliases: aliasesFor,
	}, nil
}

func (s *BackendService) convertALBListener() (*template.ALBListener, error) {
	rrConfig := s.manifest.HTTP
	if rrConfig.IsEmpty() {
		return nil, nil
	}
	var rules []template.ALBListenerRule
	var hostedZoneAliases template.AliasesForHostedZone
	for _, routingRule := range rrConfig.RoutingRules() {
		rule, err := routingRuleConfigConverter{
			rule:            routingRule,
			manifest:        s.manifest,
			httpsEnabled:    s.httpsEnabled,
			redirectToHTTPS: s.httpsEnabled,
		}.convert()
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
		hostedZoneAliases, err = convertHostedZone(rrConfig.Main.Alias, rrConfig.Main.HostedZone)
		if err != nil {
			return nil, err
		}
	}

	return &template.ALBListener{
		Rules:             rules,
		IsHTTPS:           s.httpsEnabled,
		MainContainerPort: s.manifest.MainContainerPort(),
		HostedZoneAliases: hostedZoneAliases,
	}, nil
}

func (s *BackendService) convertGracePeriod() *int64 {
	if s.manifest.HTTP.Main.HealthCheck.Advanced.GracePeriod != nil {
		return aws.Int64(int64(s.manifest.HTTP.Main.HealthCheck.Advanced.GracePeriod.Seconds()))
	}
	return aws.Int64(int64(manifest.DefaultHealthCheckGracePeriod))
}

type loadBalancerTargeter interface {
	MainContainerPort() string
	ExposedPorts() (manifest.ExposedPortsIndex, error)
}

type routingRuleConfigConverter struct {
	rule            manifest.RoutingRule
	manifest        loadBalancerTargeter
	httpsEnabled    bool
	redirectToHTTPS bool
}

// convertPath attempts to standardize manifest paths on '/path' or '/' patterns.
//   - If the path starts with a / (including '/'), return it unmodified.
//   - Otherwise, prepend a leading '/' character.
//
// CFN health check and path patterns expect a leading '/', so we do that here instead of in the template.
//
// Empty strings, if they make it to this point, are converted to '/'.
func convertPath(path string) string {
	if path == "" {
		return "/"
	}
	if path[0] == '/' {
		return path
	}
	return "/" + path
}

func (conv routingRuleConfigConverter) convert() (*template.ALBListenerRule, error) {
	var aliases []string
	var err error

	exposedPorts, err := conv.manifest.ExposedPorts()
	if err != nil {
		return nil, err
	}
	targetContainer, targetPort, err := conv.rule.Target(exposedPorts)
	if err != nil {
		return nil, err
	}

	if conv.httpsEnabled {
		aliases, err = convertAlias(conv.rule.Alias)
		if err != nil {
			return nil, err
		}
	}

	config := &template.ALBListenerRule{
		Path:                convertPath(aws.StringValue(conv.rule.Path)),
		TargetContainer:     targetContainer,
		TargetPort:          targetPort,
		Aliases:             aliases,
		HTTPHealthCheck:     convertHTTPHealthCheck(&conv.rule.HealthCheck),
		AllowedSourceIps:    convertAllowedSourceIPs(conv.rule.AllowedSourceIps),
		Stickiness:          strconv.FormatBool(aws.BoolValue(conv.rule.Stickiness)),
		HTTPVersion:         aws.StringValue(convertHTTPVersion(conv.rule.ProtocolVersion)),
		RedirectToHTTPS:     conv.redirectToHTTPS,
		DeregistrationDelay: convertDeregistrationDelay(conv.rule.DeregistrationDelay),
	}
	return config, nil
}

func convertDeregistrationDelay(delay *time.Duration) *int64 {
	if delay == nil {
		return aws.Int64(int64(manifest.DefaultDeregistrationDelay))
	}
	return aws.Int64(int64(delay.Seconds()))
}

type nlbListeners []template.NetworkLoadBalancerListener

// isCertRequired returns true if any of the NLB listeners have protocol as TLS set.
func (ls nlbListeners) isCertRequired() bool {
	for _, listener := range ls {
		if listener.Protocol == manifest.TLS {
			return true
		}
	}
	return false
}

func (s *LoadBalancedWebService) convertNetworkLoadBalancer() (networkLoadBalancerConfig, error) {
	nlbConfig := s.manifest.NLBConfig
	if nlbConfig.IsEmpty() {
		return networkLoadBalancerConfig{}, nil
	}
	exposedPorts, err := s.manifest.ExposedPorts()
	if err != nil {
		return networkLoadBalancerConfig{}, err
	}
	listeners := make(nlbListeners, len(nlbConfig.NLBListeners()))
	for idx, listener := range nlbConfig.NLBListeners() {
		// Parse targetContainer and targetPort for the Network Load Balancer targets.
		targetContainer, targetPort, err := listener.Target(exposedPorts)
		if err != nil {
			return networkLoadBalancerConfig{}, err
		}

		// Parse listener port and protocol.
		port, protocol, err := manifest.ParsePortMapping(listener.Port)
		if err != nil {
			return networkLoadBalancerConfig{}, err
		}

		if protocol == nil {
			protocol = aws.String(defaultNLBProtocol)
		}

		listeners[idx] = template.NetworkLoadBalancerListener{
			Port:                aws.StringValue(port),
			Protocol:            strings.ToUpper(aws.StringValue(protocol)),
			TargetContainer:     targetContainer,
			TargetPort:          targetPort,
			SSLPolicy:           listener.SSLPolicy,
			HealthCheck:         convertNLBHealthCheck(&listener.HealthCheck),
			Stickiness:          listener.Stickiness,
			DeregistrationDelay: convertDeregistrationDelay(listener.DeregistrationDelay),
		}
	}

	aliases, err := convertAlias(nlbConfig.Aliases)
	if err != nil {
		return networkLoadBalancerConfig{}, fmt.Errorf(`convert "nlb.alias" to string slice: %w`, err)
	}

	config := networkLoadBalancerConfig{
		settings: &template.NetworkLoadBalancer{
			Listener:            listeners,
			Aliases:             aliases,
			MainContainerPort:   s.manifest.MainContainerPort(),
			CertificateRequired: listeners.isCertRequired(),
		},
	}

	if s.dnsDelegationEnabled {
		dnsDelegationRole, dnsName := convertAppInformation(s.appInfo)
		config.appDNSName = dnsName
		config.appDNSDelegationRole = dnsDelegationRole
	}
	return config, nil
}

func (s *LoadBalancedWebService) convertGracePeriod() *int64 {
	if s.manifest.HTTPOrBool.Main.HealthCheck.Advanced.GracePeriod != nil {
		return aws.Int64(int64(s.manifest.HTTPOrBool.Main.HealthCheck.Advanced.GracePeriod.Seconds()))
	}
	if s.manifest.NLBConfig.Listener.HealthCheck.GracePeriod != nil {
		return aws.Int64(int64(s.manifest.NLBConfig.Listener.HealthCheck.GracePeriod.Seconds()))
	}
	return aws.Int64(int64(manifest.DefaultHealthCheckGracePeriod))
}

func (s *LoadBalancedWebService) convertImportedALB() (*template.ImportedALB, error) {
	if s.importedALB == nil {
		return nil, nil
	}
	var listeners []template.LBListener
	for _, listener := range s.importedALB.Listeners {
		listeners = append(listeners, template.LBListener{
			ARN:      listener.ARN,
			Port:     listener.Port,
			Protocol: listener.Protocol,
		})
	}
	var securityGroups []template.LBSecurityGroup
	for _, sg := range s.importedALB.SecurityGroups {
		securityGroups = append(securityGroups, template.LBSecurityGroup{
			ID: sg,
		})
	}
	return &template.ImportedALB{
		Name:           s.importedALB.Name,
		ARN:            s.importedALB.ARN,
		DNSName:        s.importedALB.DNSName,
		HostedZoneID:   s.importedALB.HostedZoneID,
		Listeners:      listeners,
		SecurityGroups: securityGroups,
	}, nil
}

func convertExecuteCommand(e *manifest.ExecuteCommand) *template.ExecuteCommandOpts {
	if e.Config.IsEmpty() && !aws.BoolValue(e.Enable) {
		return nil
	}
	return &template.ExecuteCommandOpts{}
}

func convertAllowedSourceIPs(allowedSourceIPs []manifest.IPNet) []string {
	var sourceIPs []string
	for _, ipNet := range allowedSourceIPs {
		sourceIPs = append(sourceIPs, string(ipNet))
	}
	return sourceIPs
}

func convertServiceConnectServer(s manifest.ServiceConnectBoolOrArgs, target *manifest.ServiceConnectTargetContainer) *template.ServiceConnectServer {
	if target == nil || target.Port == "" || target.Port == template.NoExposedContainerPort {
		return nil
	}

	return &template.ServiceConnectServer{
		Name:  target.Container,
		Port:  target.Port,
		Alias: aws.StringValue(s.Alias),
	}
}

func convertLogging(lc manifest.Logging) *template.LogConfigOpts {
	if lc.IsEmpty() {
		return nil
	}
	return &template.LogConfigOpts{
		Image:          lc.LogImage(),
		ConfigFile:     lc.ConfigFile,
		EnableMetadata: lc.GetEnableMetadata(),
		Destination:    lc.Destination,
		SecretOptions:  convertSecrets(lc.SecretOptions),
		Variables:      convertEnvVars(lc.Variables),
		Secrets:        convertSecrets(lc.Secrets),
	}
}

func convertTaskDefOverrideRules(inRules []manifest.OverrideRule) []override.Rule {
	var res []override.Rule
	suffixStr := strings.Join(taskDefOverrideRulePrefixes, override.PathSegmentSeparator)
	for _, r := range inRules {
		res = append(res, override.Rule{
			Path:  strings.Join([]string{suffixStr, r.Path}, override.PathSegmentSeparator),
			Value: r.Value,
		})
	}
	return res
}

// convertStorageOpts converts a manifest Storage field into template data structures which can be used
// to execute CFN templates
func convertStorageOpts(wlName *string, in manifest.Storage) *template.StorageOpts {
	if in.IsEmpty() {
		return nil
	}
	return &template.StorageOpts{
		Ephemeral:         convertEphemeral(in.Ephemeral),
		ReadonlyRootFS:    in.ReadonlyRootFS,
		Volumes:           convertVolumes(in.Volumes),
		MountPoints:       convertMountPoints(in.Volumes),
		EFSPerms:          convertEFSPermissions(in.Volumes),
		ManagedVolumeInfo: convertManagedFSInfo(wlName, in.Volumes),
	}
}

func convertEphemeral(in *int) *int {
	// Min value for extensible ephemeral storage is 21; if customer specifies 20, which is the default size,
	// we shouldn't let CF error out. Instead, we'll just omit it from the config.
	if aws.IntValue(in) == 20 {
		return nil
	}
	return in
}

// convertSidecarMountPoints is used to convert from manifest to template objects.
func convertSidecarMountPoints(in []manifest.SidecarMountPoint) []*template.MountPoint {
	if len(in) == 0 {
		return nil
	}
	var output []*template.MountPoint
	for _, smp := range in {
		output = append(output, convertMountPoint(smp.SourceVolume, smp.ContainerPath, smp.ReadOnly))
	}
	return output
}

func convertMountPoint(sourceVolume, containerPath *string, readOnly *bool) *template.MountPoint {
	// readOnly defaults to true.
	oReadOnly := aws.Bool(defaultReadOnly)
	if readOnly != nil {
		oReadOnly = readOnly
	}
	return &template.MountPoint{
		ReadOnly:      oReadOnly,
		ContainerPath: containerPath,
		SourceVolume:  sourceVolume,
	}
}

func convertMountPoints(input map[string]*manifest.Volume) []*template.MountPoint {
	if len(input) == 0 {
		return nil
	}
	var output []*template.MountPoint
	for name, volume := range input {
		output = append(output, convertMountPoint(aws.String(name), volume.ContainerPath, volume.ReadOnly))
	}
	return output
}

func convertEFSPermissions(input map[string]*manifest.Volume) []*template.EFSPermission {
	var output []*template.EFSPermission
	for _, volume := range input {
		// If there's no EFS configuration, we don't need to generate any permissions.
		if volume.EmptyVolume() {
			continue
		}
		// If EFS is explicitly disabled, we don't need to generate permisisons.
		if volume.EFS.Disabled() {
			continue
		}
		// Managed FS permissions are rendered separately in the template.
		if volume.EFS.UseManagedFS() {
			continue
		}

		// Write defaults to false.
		write := defaultWritePermission
		if volume.ReadOnly != nil {
			write = !aws.BoolValue(volume.ReadOnly)
		}
		accessPointID := volume.EFS.Advanced.AuthConfig.AccessPointID
		output = append(output, &template.EFSPermission{
			Write:         write,
			AccessPointID: accessPointID,
			FilesystemID:  convertFileSystemID(volume.EFS.Advanced),
		})
	}
	return output
}

func convertManagedFSInfo(wlName *string, input map[string]*manifest.Volume) *template.ManagedVolumeCreationInfo {
	var output *template.ManagedVolumeCreationInfo
	for name, volume := range input {
		if volume.EmptyVolume() || !volume.EFS.UseManagedFS() {
			continue
		}
		uid := volume.EFS.Advanced.UID
		gid := volume.EFS.Advanced.GID
		if uid == nil && gid == nil {
			crc := aws.Uint32(getRandomUIDGID(wlName))
			uid = crc
			gid = crc
		}
		output = &template.ManagedVolumeCreationInfo{
			Name:    aws.String(name),
			DirName: wlName,
			UID:     uid,
			GID:     gid,
		}
	}
	return output
}

// getRandomUIDGID returns the 32-bit checksum of the service name for use as CreationInfo in the EFS Access Point.
// See https://stackoverflow.com/a/14210379/5890422 for discussion of the possibility of collisions in CRC32 with
// small numbers of hashes.
func getRandomUIDGID(name *string) uint32 {
	return crc32.ChecksumIEEE([]byte(aws.StringValue(name)))
}

func convertVolumes(input map[string]*manifest.Volume) []*template.Volume {
	var output []*template.Volume
	for name, volume := range input {
		// Volumes can contain either:
		//   a) an EFS configuration, which must be valid
		//   b) no EFS configuration, in which case the volume is created using task scratch storage in order to share
		//      data between containers.

		// If EFS is not configured, just add the name to create an empty volume and continue.
		if volume.EmptyVolume() {
			output = append(
				output,
				&template.Volume{
					Name: aws.String(name),
				},
			)
			continue
		}

		// If we're using managed EFS, continue.
		if volume.EFS.UseManagedFS() {
			continue
		}

		// Convert EFS configuration to template struct.
		output = append(
			output,
			&template.Volume{
				Name: aws.String(name),
				EFS:  convertEFSConfiguration(volume.EFS.Advanced),
			},
		)
	}
	return output
}

func convertEFSConfiguration(in manifest.EFSVolumeConfiguration) *template.EFSVolumeConfiguration {
	// Set default values correctly.
	rootDir := in.RootDirectory
	if aws.StringValue(rootDir) == "" {
		rootDir = aws.String(defaultRootDirectory)
	}
	// Set default values for IAM and AccessPointID
	iam := aws.String(defaultIAM)
	if in.AuthConfig.IsEmpty() {
		return &template.EFSVolumeConfiguration{
			Filesystem:    convertFileSystemID(in),
			RootDirectory: rootDir,
			IAM:           iam,
		}
	}
	// AuthConfig exists; check the properties.
	if aws.BoolValue(in.AuthConfig.IAM) {
		iam = aws.String(enabled)
	}

	return &template.EFSVolumeConfiguration{
		Filesystem:    convertFileSystemID(in),
		RootDirectory: rootDir,
		IAM:           iam,
		AccessPointID: in.AuthConfig.AccessPointID,
	}
}

func convertFileSystemID(in manifest.EFSVolumeConfiguration) template.FileSystemID {
	if in.FileSystemID.Plain != nil {
		return template.PlainFileSystemID(aws.StringValue(in.FileSystemID.Plain))
	}
	return template.ImportedFileSystemID(aws.StringValue(in.FileSystemID.FromCFN.Name))
}

func convertNetworkConfig(network manifest.NetworkConfig) template.NetworkOpts {
	if network.IsEmpty() {
		return template.NetworkOpts{
			AssignPublicIP: template.EnablePublicIP,
			SubnetsType:    template.PublicSubnetsPlacement,
		}
	}
	opts := template.NetworkOpts{
		AssignPublicIP: template.EnablePublicIP,
		SubnetsType:    template.PublicSubnetsPlacement,
	}
	inSGs := network.VPC.SecurityGroups.GetIDs()
	outSGs := make([]template.SecurityGroup, len(inSGs))
	for i, sg := range inSGs {
		if sg.Plain != nil {
			outSGs[i] = template.PlainSecurityGroup(aws.StringValue(sg.Plain))
		} else {
			outSGs[i] = template.ImportedSecurityGroup(aws.StringValue(sg.FromCFN.Name))
		}
	}
	opts.SecurityGroups = outSGs
	opts.DenyDefaultSecurityGroup = network.VPC.SecurityGroups.IsDefaultSecurityGroupDenied()

	placement := network.VPC.Placement
	if placement.IsEmpty() {
		return opts
	}
	if placement.PlacementString != nil {
		if *placement.PlacementString == manifest.PrivateSubnetPlacement {
			opts.AssignPublicIP = template.DisablePublicIP
		}
		opts.SubnetsType = subnetPlacementForTemplate[*placement.PlacementString]
		return opts
	}
	opts.AssignPublicIP = template.DisablePublicIP
	opts.SubnetsType = ""
	opts.SubnetIDs = placement.PlacementArgs.Subnets.IDs
	return opts
}

func convertRDWSNetworkConfig(network manifest.RequestDrivenWebServiceNetworkConfig) template.NetworkOpts {
	opts := template.NetworkOpts{}
	if network.IsEmpty() {
		return opts
	}
	placement := network.VPC.Placement
	if placement.IsEmpty() {
		return opts
	}
	if placement.PlacementString != nil {
		opts.SubnetsType = subnetPlacementForTemplate[*placement.PlacementString]
		return opts
	}
	opts.SubnetIDs = placement.PlacementArgs.Subnets.IDs
	return opts
}

func convertAlias(alias manifest.Alias) ([]string, error) {
	out, err := alias.ToStringSlice()
	if err != nil {
		return nil, fmt.Errorf(`convert "http.alias" to string slice: %w`, err)
	}
	return out, nil
}

func convertEntryPoint(entrypoint manifest.EntryPointOverride) ([]string, error) {
	out, err := entrypoint.ToStringSlice()
	if err != nil {
		return nil, fmt.Errorf(`convert "entrypoint" to string slice: %w`, err)
	}
	return out, nil
}

func convertDeploymentControllerConfig(in manifest.DeploymentControllerConfig) template.DeploymentConfigurationOpts {
	out := template.DeploymentConfigurationOpts{
		MinHealthyPercent: minHealthyPercentDefault,
		MaxPercent:        maxPercentDefault,
	}
	if strings.EqualFold(aws.StringValue(in.Rolling), manifest.ECSRecreateRollingUpdateStrategy) {
		out.MinHealthyPercent = minHealthyPercentRecreate
		out.MaxPercent = maxPercentRecreate
	}
	return out
}

func convertDeploymentConfig(in manifest.DeploymentConfig) template.DeploymentConfigurationOpts {
	out := convertDeploymentControllerConfig(in.DeploymentControllerConfig)
	out.Rollback = template.RollingUpdateRollbackConfig{
		AlarmNames:        in.RollbackAlarms.Basic,
		CPUUtilization:    in.RollbackAlarms.Advanced.CPUUtilization,
		MemoryUtilization: in.RollbackAlarms.Advanced.MemoryUtilization,
	}
	return out
}

func convertWorkerDeploymentConfig(in manifest.WorkerDeploymentConfig) template.DeploymentConfigurationOpts {
	out := convertDeploymentControllerConfig(in.DeploymentControllerConfig)
	out.Rollback = template.RollingUpdateRollbackConfig{
		AlarmNames:        in.WorkerRollbackAlarms.Basic,
		CPUUtilization:    in.WorkerRollbackAlarms.Advanced.CPUUtilization,
		MemoryUtilization: in.WorkerRollbackAlarms.Advanced.MemoryUtilization,
		MessagesDelayed:   in.WorkerRollbackAlarms.Advanced.MessagesDelayed,
	}
	return out
}

func convertCommand(command manifest.CommandOverride) ([]string, error) {
	out, err := command.ToStringSlice()
	if err != nil {
		return nil, fmt.Errorf(`convert "command" to string slice: %w`, err)
	}
	return out, nil
}

func convertPublish(topics []manifest.Topic, accountID, region, app, env, svc string) (*template.PublishOpts, error) {
	if len(topics) == 0 {
		return nil, nil
	}
	partition, err := partitions.Region(region).Partition()
	if err != nil {
		return nil, err
	}
	var publishers template.PublishOpts
	// convert the topics to template Topics
	for _, topic := range topics {
		var fifoConfig *template.FIFOTopicConfig
		if topic.FIFO.IsEnabled() {
			fifoConfig = &template.FIFOTopicConfig{}
			if !topic.FIFO.Advanced.IsEmpty() {
				fifoConfig = &template.FIFOTopicConfig{
					ContentBasedDeduplication: topic.FIFO.Advanced.ContentBasedDeduplication,
				}
			}
		}
		publishers.Topics = append(publishers.Topics, &template.Topic{
			Name:            topic.Name,
			FIFOTopicConfig: fifoConfig,
			AccountID:       accountID,
			Partition:       partition.ID(),
			Region:          region,
			App:             app,
			Env:             env,
			Svc:             svc,
		})
	}

	return &publishers, nil
}

func convertSubscribe(s *manifest.WorkerService) (*template.SubscribeOpts, error) {
	if s.Subscribe.Topics == nil {
		return nil, nil
	}
	var subscriptions template.SubscribeOpts
	for _, sb := range s.Subscriptions() {
		ts, err := convertTopicSubscription(sb)
		if err != nil {
			return nil, err
		}
		subscriptions.Topics = append(subscriptions.Topics, ts)
	}
	subscriptions.Queue = convertQueue(s.Subscribe.Queue)
	return &subscriptions, nil
}

func convertTopicSubscription(t manifest.TopicSubscription) (
	*template.TopicSubscription, error) {
	filterPolicy, err := convertFilterPolicy(t.FilterPolicy)
	if err != nil {
		return nil, err
	}
	if aws.BoolValue(t.Queue.Enabled) {
		return &template.TopicSubscription{
			Name:         t.Name,
			Service:      t.Service,
			Queue:        &template.SQSQueue{},
			FilterPolicy: filterPolicy,
		}, nil
	}
	return &template.TopicSubscription{
		Name:         t.Name,
		Service:      t.Service,
		Queue:        convertQueue(t.Queue.Advanced),
		FilterPolicy: filterPolicy,
	}, nil
}

func convertFilterPolicy(filterPolicy map[string]interface{}) (*string, error) {
	if len(filterPolicy) == 0 {
		return nil, nil
	}
	bytes, err := json.Marshal(filterPolicy)
	if err != nil {
		return nil, fmt.Errorf(`convert "filter_policy" to a JSON string: %w`, err)
	}
	return aws.String(string(bytes)), nil
}

func convertQueue(in manifest.SQSQueue) *template.SQSQueue {
	if in.IsEmpty() {
		return nil
	}

	queue := &template.SQSQueue{
		Retention:  convertRetention(in.Retention),
		Delay:      convertDelay(in.Delay),
		Timeout:    convertTimeout(in.Timeout),
		DeadLetter: convertDeadLetter(in.DeadLetter),
	}

	if !in.FIFO.IsEnabled() {
		return queue
	}

	if aws.BoolValue(in.FIFO.Enable) {
		queue.FIFOQueueConfig = &template.FIFOQueueConfig{}
		return queue
	}

	if !in.FIFO.Advanced.IsEmpty() {
		queue.FIFOQueueConfig = &template.FIFOQueueConfig{
			ContentBasedDeduplication: in.FIFO.Advanced.ContentBasedDeduplication,
			DeduplicationScope:        in.FIFO.Advanced.DeduplicationScope,
			FIFOThroughputLimit:       in.FIFO.Advanced.FIFOThroughputLimit,
		}
		if aws.BoolValue(in.FIFO.Advanced.HighThroughputFifo) {
			queue.FIFOQueueConfig.FIFOThroughputLimit = aws.String(sqsFIFOThroughputLimitPerMessageGroupId)
			queue.FIFOQueueConfig.DeduplicationScope = aws.String(sqsDedupeScopeMessageGroup)
		}
	}
	return queue
}

func convertTime(t *time.Duration) *int64 {
	if t == nil {
		return nil
	}
	return aws.Int64(int64(t.Seconds()))
}

func convertRetention(t *time.Duration) *int64 {
	return convertTime(t)
}

func convertDelay(t *time.Duration) *int64 {
	return convertTime(t)
}

func convertTimeout(t *time.Duration) *int64 {
	return convertTime(t)
}

func convertDeadLetter(d manifest.DeadLetterQueue) *template.DeadLetterQueue {
	if d.IsEmpty() {
		return nil
	}
	return &template.DeadLetterQueue{
		Tries: d.Tries,
	}
}

func convertAppInformation(app deploy.AppInformation) (delegationRole *string, domain *string) {
	role := app.DNSDelegationRole()
	if role != "" {
		delegationRole = &role
	}
	if app.Domain != "" {
		domain = &app.Domain
	}
	return
}

func convertPlatform(platform manifest.PlatformArgsOrString) template.RuntimePlatformOpts {
	if platform.IsEmpty() {
		return template.RuntimePlatformOpts{}
	}

	os := template.OSLinux
	switch platform.OS() {
	case manifest.OSWindows, manifest.OSWindowsServer2019Core:
		os = template.OSWindowsServer2019Core
	case manifest.OSWindowsServer2019Full:
		os = template.OSWindowsServer2019Full
	case manifest.OSWindowsServer2022Core:
		os = template.OSWindowsServer2022Core
	case manifest.OSWindowsServer2022Full:
		os = template.OSWindowsServer2022Full
	}

	arch := template.ArchX86
	if manifest.IsArmArch(platform.Arch()) {
		arch = template.ArchARM64
	}
	return template.RuntimePlatformOpts{
		OS:   os,
		Arch: arch,
	}
}

func convertHTTPVersion(protocolVersion *string) *string {
	if protocolVersion == nil {
		return nil
	}
	pv := strings.ToUpper(*protocolVersion)
	return &pv
}

func convertEnvVars(variables map[string]manifest.Variable) map[string]template.Variable {
	if len(variables) == 0 {
		return nil
	}
	m := make(map[string]template.Variable, len(variables))
	for name, variable := range variables {
		if variable.RequiresImport() {
			m[name] = template.ImportedVariable(variable.Value())
			continue
		}
		m[name] = template.PlainVariable(variable.Value())
	}
	return m
}

// convertSecrets converts the manifest Secrets into a format parsable by the templates pkg.
func convertSecrets(secrets map[string]manifest.Secret) map[string]template.Secret {
	if len(secrets) == 0 {
		return nil
	}
	m := make(map[string]template.Secret, len(secrets))
	var tplSecret template.Secret
	for name, mftSecret := range secrets {
		switch {
		case mftSecret.IsSecretsManagerName():
			tplSecret = template.SecretFromSecretsManager(mftSecret.Value())
		case mftSecret.RequiresImport():
			tplSecret = template.SecretFromImportedSSMOrARN(mftSecret.Value())
		default:
			tplSecret = template.SecretFromPlainSSMOrARN(mftSecret.Value())
		}
		m[name] = tplSecret
	}
	return m
}

func convertCustomResources(urlForFunc map[string]string) (map[string]template.S3ObjectLocation, error) {
	out := make(map[string]template.S3ObjectLocation)
	for fn, url := range urlForFunc {
		bucket, key, err := s3.ParseURL(url)
		if err != nil {
			return nil, fmt.Errorf("convert custom resource %q url: %w", fn, err)
		}
		out[fn] = template.S3ObjectLocation{
			Bucket: bucket,
			Key:    key,
		}
	}
	return out, nil
}

type uploadableCRs []*customresource.CustomResource

func (in uploadableCRs) convert() []uploadable {
	out := make([]uploadable, len(in))
	for i, cr := range in {
		out[i] = cr
	}
	return out
}
