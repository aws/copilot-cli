// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/graph"
	"github.com/dustin/go-humanize/english"
)

const (
	// Container dependency status constants.
	dependsOnStart    = "START"
	dependsOnComplete = "COMPLETE"
	dependsOnSuccess  = "SUCCESS"
	dependsOnHealthy  = "HEALTHY"

	// Min and Max values for task ephemeral storage in GiB.
	ephemeralMinValueGiB = 20
	ephemeralMaxValueGiB = 200

	envFileExt = ".env"
)

const (
	TCPUDP = "TCP_UDP"
	tcp    = "TCP"
	udp    = "UDP"
	tls    = "TLS"
)

var validProtocols = []string{TCPUDP, tcp, udp, tls}

var (
	intRangeBandRegexp  = regexp.MustCompile(`^(\d+)-(\d+)$`)
	volumesPathRegexp   = regexp.MustCompile(`^[a-zA-Z0-9\-\.\_/]+$`)
	awsSNSTopicRegexp   = regexp.MustCompile(`^[a-zA-Z0-9_-]*$`)   // Validates that an expression contains only letters, numbers, underscores, and hyphens.
	awsNameRegexp       = regexp.MustCompile(`^[a-z][a-z0-9\-]+$`) // Validates that an expression starts with a letter and only contains letters, numbers, and hyphens.
	punctuationRegExp   = regexp.MustCompile(`[\.\-]{2,}`)         // Check for consecutive periods or dashes.
	trailingPunctRegExp = regexp.MustCompile(`[\-\.]$`)            // Check for trailing dash or dot.

	essentialContainerDependsOnValidStatuses = []string{dependsOnStart, dependsOnHealthy}
	dependsOnValidStatuses                   = []string{dependsOnStart, dependsOnComplete, dependsOnSuccess, dependsOnHealthy}

	httpProtocolVersions = []string{"GRPC", "HTTP1", "HTTP2"}

	invalidTaskDefOverridePathRegexp = []string{`Family`, `ContainerDefinitions\[\d+\].Name`}
)

// Validate returns nil if LoadBalancedWebService is configured correctly.
func (l LoadBalancedWebService) Validate() error {
	var err error
	if err = l.LoadBalancedWebServiceConfig.Validate(); err != nil {
		return err
	}
	if err = l.Workload.Validate(); err != nil {
		return err
	}
	if err = validateTargetContainer(validateTargetContainerOpts{
		mainContainerName: aws.StringValue(l.Name),
		targetContainer:   l.RoutingRule.targetContainer(),
		sidecarConfig:     l.Sidecars,
	}); err != nil {
		return fmt.Errorf("validate HTTP load balancer target: %w", err)
	}
	if err = validateTargetContainer(validateTargetContainerOpts{
		mainContainerName: aws.StringValue(l.Name),
		targetContainer:   l.NLBConfig.TargetContainer,
		sidecarConfig:     l.Sidecars,
	}); err != nil {
		return fmt.Errorf("validate network load balancer target: %w", err)
	}
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     l.Sidecars,
		imageConfig:       l.ImageConfig.Image,
		mainContainerName: aws.StringValue(l.Name),
		logging:           l.Logging,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	return nil
}

// Validate returns nil if LoadBalancedWebServiceConfig is configured correctly.
func (l LoadBalancedWebServiceConfig) Validate() error {
	var err error
	if l.RoutingRule.Disabled() && l.NLBConfig.IsEmpty() {
		return &errAtLeastOneFieldMustBeSpecified{
			missingFields: []string{"http", "nlb"},
		}
	}
	if l.RoutingRule.Disabled() && (l.Count.AdvancedCount.Requests != nil || l.Count.AdvancedCount.ResponseTime != nil) {
		return errors.New(`scaling based on "nlb" requests or response time is not supported`)
	}
	if err = l.ImageConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = l.ImageOverride.Validate(); err != nil {
		return err
	}
	if err = l.RoutingRule.Validate(); err != nil {
		return fmt.Errorf(`validate "http": %w`, err)
	}
	if err = l.TaskConfig.Validate(); err != nil {
		return err
	}
	if err = l.Logging.Validate(); err != nil {
		return fmt.Errorf(`validate "logging": %w`, err)
	}
	for k, v := range l.Sidecars {
		if err = v.Validate(); err != nil {
			return fmt.Errorf(`validate "sidecars[%s]": %w`, k, err)
		}
	}
	if err = l.Network.Validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err = l.PublishConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for ind, taskDefOverride := range l.TaskDefOverrides {
		if err = taskDefOverride.Validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if l.TaskConfig.IsWindows() {
		if err = validateWindows(validateWindowsOpts{
			execEnabled: aws.BoolValue(l.ExecuteCommand.Enable),
			efsVolumes:  l.Storage.Volumes,
		}); err != nil {
			return fmt.Errorf("validate Windows: %w", err)
		}
	}
	if l.TaskConfig.IsARM() {
		if err = validateARM(validateARMOpts{
			Spot:     l.Count.AdvancedCount.Spot,
			SpotFrom: l.Count.AdvancedCount.Range.RangeConfig.SpotFrom,
		}); err != nil {
			return fmt.Errorf("validate ARM: %w", err)
		}
	}
	if err = l.NLBConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "nlb": %w`, err)
	}
	return nil
}

// Validate returns nil if BackendService is configured correctly.
func (b BackendService) Validate() error {
	var err error
	if err = b.BackendServiceConfig.Validate(); err != nil {
		return err
	}
	if err = b.Workload.Validate(); err != nil {
		return err
	}
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     b.Sidecars,
		imageConfig:       b.ImageConfig.Image,
		mainContainerName: aws.StringValue(b.Name),
		logging:           b.Logging,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	return nil
}

// Validate returns nil if BackendServiceConfig is configured correctly.
func (b BackendServiceConfig) Validate() error {
	var err error
	if err = b.ImageConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = b.ImageOverride.Validate(); err != nil {
		return err
	}
	if err = b.TaskConfig.Validate(); err != nil {
		return err
	}
	if err = b.Logging.Validate(); err != nil {
		return fmt.Errorf(`validate "logging": %w`, err)
	}
	for k, v := range b.Sidecars {
		if err = v.Validate(); err != nil {
			return fmt.Errorf(`validate "sidecars[%s]": %w`, k, err)
		}
	}
	if err = b.Network.Validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err = b.PublishConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for ind, taskDefOverride := range b.TaskDefOverrides {
		if err = taskDefOverride.Validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if b.TaskConfig.IsWindows() {
		if err = validateWindows(validateWindowsOpts{
			execEnabled: aws.BoolValue(b.ExecuteCommand.Enable),
			efsVolumes:  b.Storage.Volumes,
		}); err != nil {
			return fmt.Errorf("validate Windows: %w", err)
		}
	}
	if b.TaskConfig.IsARM() {
		if err = validateARM(validateARMOpts{
			Spot:     b.Count.AdvancedCount.Spot,
			SpotFrom: b.Count.AdvancedCount.Range.RangeConfig.SpotFrom,
		}); err != nil {
			return fmt.Errorf("validate ARM: %w", err)
		}
	}
	return nil
}

// Validate returns nil if RequestDrivenWebService is configured correctly.
func (r RequestDrivenWebService) Validate() error {
	if err := r.RequestDrivenWebServiceConfig.Validate(); err != nil {
		return err
	}
	return r.Workload.Validate()
}

// Validate returns nil if RequestDrivenWebServiceConfig is configured correctly.
func (r RequestDrivenWebServiceConfig) Validate() error {
	var err error
	if err = r.ImageConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = r.InstanceConfig.Validate(); err != nil {
		return err
	}
	if err = r.RequestDrivenWebServiceHttpConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "http": %w`, err)
	}
	if err = r.PublishConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	if err = r.Network.Validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	return nil
}

// Validate returns nil if WorkerService is configured correctly.
func (w WorkerService) Validate() error {
	var err error
	if err = w.WorkerServiceConfig.Validate(); err != nil {
		return err
	}
	if err = w.Workload.Validate(); err != nil {
		return err
	}
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     w.Sidecars,
		imageConfig:       w.ImageConfig.Image,
		mainContainerName: aws.StringValue(w.Name),
		logging:           w.Logging,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	return nil
}

// Validate returns nil if WorkerServiceConfig is configured correctly.
func (w WorkerServiceConfig) Validate() error {
	var err error
	if err = w.ImageConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = w.ImageOverride.Validate(); err != nil {
		return err
	}
	if err = w.TaskConfig.Validate(); err != nil {
		return err
	}
	if err = w.Logging.Validate(); err != nil {
		return fmt.Errorf(`validate "logging": %w`, err)
	}
	for k, v := range w.Sidecars {
		if err = v.Validate(); err != nil {
			return fmt.Errorf(`validate "sidecars[%s]": %w`, k, err)
		}
	}
	if err = w.Network.Validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err = w.Subscribe.Validate(); err != nil {
		return fmt.Errorf(`validate "subscribe": %w`, err)
	}
	if err = w.PublishConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for ind, taskDefOverride := range w.TaskDefOverrides {
		if err = taskDefOverride.Validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if w.TaskConfig.IsWindows() {
		if err = validateWindows(validateWindowsOpts{
			execEnabled: aws.BoolValue(w.ExecuteCommand.Enable),
			efsVolumes:  w.Storage.Volumes,
		}); err != nil {
			return fmt.Errorf(`validate Windows: %w`, err)
		}
	}
	if w.TaskConfig.IsARM() {
		if err = validateARM(validateARMOpts{
			Spot:     w.Count.AdvancedCount.Spot,
			SpotFrom: w.Count.AdvancedCount.Range.RangeConfig.SpotFrom,
		}); err != nil {
			return fmt.Errorf("validate ARM: %w", err)
		}
	}
	return nil
}

// Validate returns nil if ScheduledJob is configured correctly.
func (s ScheduledJob) Validate() error {
	var err error
	if err = s.ScheduledJobConfig.Validate(); err != nil {
		return err
	}
	if err = s.Workload.Validate(); err != nil {
		return err
	}
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     s.Sidecars,
		imageConfig:       s.ImageConfig.Image,
		mainContainerName: aws.StringValue(s.Name),
		logging:           s.Logging,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	return nil
}

// Validate returns nil if ScheduledJobConfig is configured correctly.
func (s ScheduledJobConfig) Validate() error {
	var err error
	if err = s.ImageConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = s.ImageOverride.Validate(); err != nil {
		return err
	}
	if err = s.TaskConfig.Validate(); err != nil {
		return err
	}
	if err = s.Logging.Validate(); err != nil {
		return fmt.Errorf(`validate "logging": %w`, err)
	}
	for k, v := range s.Sidecars {
		if err = v.Validate(); err != nil {
			return fmt.Errorf(`validate "sidecars[%s]": %w`, k, err)
		}
	}
	if err = s.Network.Validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err = s.On.Validate(); err != nil {
		return fmt.Errorf(`validate "on": %w`, err)
	}
	if err = s.JobFailureHandlerConfig.Validate(); err != nil {
		return err
	}
	if err = s.PublishConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for ind, taskDefOverride := range s.TaskDefOverrides {
		if err = taskDefOverride.Validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if s.TaskConfig.IsWindows() {
		if err = validateWindows(validateWindowsOpts{
			execEnabled: aws.BoolValue(s.ExecuteCommand.Enable),
			efsVolumes:  s.Storage.Volumes,
		}); err != nil {
			return fmt.Errorf(`validate Windows: %w`, err)
		}
	}
	if s.TaskConfig.IsARM() {
		if err = validateARM(validateARMOpts{
			Spot:     s.Count.AdvancedCount.Spot,
			SpotFrom: s.Count.AdvancedCount.Range.RangeConfig.SpotFrom,
		}); err != nil {
			return fmt.Errorf("validate ARM: %w", err)
		}
	}
	return nil
}

// Validate returns nil if Workload is configured correctly.
func (w Workload) Validate() error {
	if w.Name == nil {
		return &errFieldMustBeSpecified{
			missingField: "name",
		}
	}
	return nil
}

// Validate returns nil if ImageWithPortAndHealthcheck is configured correctly.
func (i ImageWithPortAndHealthcheck) Validate() error {
	var err error
	if err = i.ImageWithPort.Validate(); err != nil {
		return err
	}
	if err = i.HealthCheck.Validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	return nil
}

// Validate returns nil if ImageWithHealthcheckAndOptionalPort is configured correctly.
func (i ImageWithHealthcheckAndOptionalPort) Validate() error {
	var err error
	if err = i.ImageWithOptionalPort.Validate(); err != nil {
		return err
	}
	if err = i.HealthCheck.Validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	return nil
}

// Validate returns nil if ImageWithHealthcheck is configured correctly.
func (i ImageWithHealthcheck) Validate() error {
	if err := i.Image.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate returns nil if ImageWithOptionalPort is configured correctly.
func (i ImageWithOptionalPort) Validate() error {
	if err := i.Image.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate returns nil if ImageWithPort is configured correctly.
func (i ImageWithPort) Validate() error {
	if err := i.Image.Validate(); err != nil {
		return err
	}
	if i.Port == nil {
		return &errFieldMustBeSpecified{
			missingField: "port",
		}
	}
	return nil
}

// Validate returns nil if Image is configured correctly.
func (i Image) Validate() error {
	var err error
	if err = i.Build.Validate(); err != nil {
		return fmt.Errorf(`validate "build": %w`, err)
	}
	if i.Build.isEmpty() == (i.Location == nil) {
		return &errFieldMutualExclusive{
			firstField:  "build",
			secondField: "location",
			mustExist:   true,
		}
	}
	if err = i.DependsOn.Validate(); err != nil {
		return fmt.Errorf(`validate "depends_on": %w`, err)
	}
	return nil
}

// Validate returns nil if DependsOn is configured correctly.
func (d DependsOn) Validate() error {
	if d == nil {
		return nil
	}
	for _, v := range d {
		status := strings.ToUpper(v)
		var isValid bool
		for _, allowed := range dependsOnValidStatuses {
			if status == allowed {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("container dependency status must be one of %s", english.WordSeries([]string{dependsOnStart, dependsOnComplete, dependsOnSuccess, dependsOnHealthy}, "or"))
		}
	}
	return nil
}

// Validate returns nil if BuildArgsOrString is configured correctly.
func (b BuildArgsOrString) Validate() error {
	if b.isEmpty() {
		return nil
	}
	if !b.BuildArgs.isEmpty() {
		return b.BuildArgs.Validate()
	}
	return nil
}

// Validate returns nil if DockerBuildArgs is configured correctly.
func (DockerBuildArgs) Validate() error {
	return nil
}

// Validate returns nil if ContainerHealthCheck is configured correctly.
func (ContainerHealthCheck) Validate() error {
	return nil
}

// Validate returns nil if ImageOverride is configured correctly.
func (i ImageOverride) Validate() error {
	var err error
	if err = i.EntryPoint.Validate(); err != nil {
		return fmt.Errorf(`validate "entrypoint": %w`, err)
	}
	if err = i.Command.Validate(); err != nil {
		return fmt.Errorf(`validate "command": %w`, err)
	}
	return nil
}

// Validate returns nil if EntryPointOverride is configured correctly.
func (EntryPointOverride) Validate() error {
	return nil
}

// Validate returns nil if CommandOverride is configured correctly.
func (CommandOverride) Validate() error {
	return nil
}

// Validate returns nil if RoutingRuleConfigOrBool is configured correctly.
func (r RoutingRuleConfigOrBool) Validate() error {
	if aws.BoolValue(r.Enabled) {
		return &errFieldMustBeSpecified{
			missingField: "path",
		}
	}
	if r.Enabled != nil {
		return nil
	}
	return r.RoutingRuleConfiguration.Validate()
}

// Validate returns nil if RoutingRuleConfiguration is configured correctly.
func (r RoutingRuleConfiguration) Validate() error {
	var err error
	if err = r.HealthCheck.Validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	if err = r.Alias.Validate(); err != nil {
		return fmt.Errorf(`validate "alias": %w`, err)
	}
	if r.TargetContainer != nil && r.TargetContainerCamelCase != nil {
		return &errFieldMutualExclusive{
			firstField:  "target_container",
			secondField: "targetContainer",
		}
	}
	for ind, ip := range r.AllowedSourceIps {
		if err = ip.Validate(); err != nil {
			return fmt.Errorf(`validate "allowed_source_ips[%d]": %w`, ind, err)
		}
	}
	if r.ProtocolVersion != nil {
		if !contains(strings.ToUpper(*r.ProtocolVersion), httpProtocolVersions) {
			return fmt.Errorf(`"version" field value '%s' must be one of %s`, *r.ProtocolVersion, english.WordSeries(httpProtocolVersions, "or"))
		}
	}
	if r.Path == nil {
		return &errFieldMustBeSpecified{
			missingField: "path",
		}
	}
	return nil
}

// Validate returns nil if HealthCheckArgsOrString is configured correctly.
func (h HealthCheckArgsOrString) Validate() error {
	if h.IsEmpty() {
		return nil
	}
	return h.HealthCheckArgs.Validate()
}

// Validate returns nil if HTTPHealthCheckArgs is configured correctly.
func (h HTTPHealthCheckArgs) Validate() error {
	if h.isEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if NLBHealthCheckArgs is configured correctly.
func (h NLBHealthCheckArgs) Validate() error {
	if h.isEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if Alias is configured correctly.
func (Alias) Validate() error {
	return nil
}

// Validate returns nil if IPNet is configured correctly.
func (ip IPNet) Validate() error {
	if _, _, err := net.ParseCIDR(string(ip)); err != nil {
		return fmt.Errorf("parse IPNet %s: %w", string(ip), err)
	}
	return nil
}

// Validate returns nil if NetworkLoadBalancerConfiguration is configured correctly.
func (c NetworkLoadBalancerConfiguration) Validate() error {
	if c.IsEmpty() {
		return nil
	}
	if aws.StringValue(c.Port) == "" {
		return &errFieldMustBeSpecified{
			missingField: "port",
		}
	}
	if err := validateNLBPort(c.Port); err != nil {
		return fmt.Errorf(`validate "port": %w`, err)
	}
	if err := c.HealthCheck.Validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	if err := c.Aliases.Validate(); err != nil {
		return fmt.Errorf(`validate "alias": %w`, err)
	}
	return nil
}

func validateNLBPort(port *string) error {
	_, protocol, err := ParsePortMapping(port)
	if err != nil {
		return err
	}
	if protocol == nil {
		return nil
	}
	protocolVal := aws.StringValue(protocol)
	var isValidProtocol bool
	for _, valid := range validProtocols {
		if strings.EqualFold(protocolVal, valid) {
			isValidProtocol = true
			break
		}
	}
	if !isValidProtocol {
		return fmt.Errorf(`unrecognized protocol %s`, protocolVal)
	}
	return nil
}

// Validate returns nil if TaskConfig is configured correctly.
func (t TaskConfig) Validate() error {
	var err error
	if err = t.Platform.Validate(); err != nil {
		return fmt.Errorf(`validate "platform": %w`, err)
	}
	if err = t.Count.Validate(); err != nil {
		return fmt.Errorf(`validate "count": %w`, err)
	}
	if err = t.ExecuteCommand.Validate(); err != nil {
		return fmt.Errorf(`validate "exec": %w`, err)
	}
	if err = t.Storage.Validate(); err != nil {
		return fmt.Errorf(`validate "storage": %w`, err)
	}
	if t.EnvFile != nil {
		envFile := aws.StringValue(t.EnvFile)
		if filepath.Ext(envFile) != envFileExt {
			return fmt.Errorf("environment file %s must have a %s file extension", envFile, envFileExt)
		}
	}
	return nil
}

// Validate returns nil if PlatformArgsOrString is configured correctly.
func (p PlatformArgsOrString) Validate() error {
	if p.IsEmpty() {
		return nil
	}
	if !p.PlatformArgs.isEmpty() {
		return p.PlatformArgs.Validate()
	}
	if p.PlatformString != nil {
		return p.PlatformString.Validate()
	}
	return nil
}

// Validate returns nil if PlatformArgs is configured correctly.
func (p PlatformArgs) Validate() error {
	if !p.bothSpecified() {
		return errors.New(`fields "osfamily" and "architecture" must either both be specified or both be empty`)
	}
	var ss []string
	for _, p := range validAdvancedPlatforms {
		ss = append(ss, p.String())
	}
	prettyValidPlatforms := strings.Join(ss, ", ")

	os := strings.ToLower(aws.StringValue(p.OSFamily))
	arch := strings.ToLower(aws.StringValue(p.Arch))
	for _, vap := range validAdvancedPlatforms {
		if os == aws.StringValue(vap.OSFamily) && arch == aws.StringValue(vap.Arch) {
			return nil
		}
	}
	return fmt.Errorf("platform pair %s is invalid: fields ('osfamily', 'architecture') must be one of %s", p.String(), prettyValidPlatforms)
}

// Validate returns nil if PlatformString is configured correctly.
func (p PlatformString) Validate() error {
	args := strings.Split(string(p), "/")
	if len(args) != 2 {
		return fmt.Errorf("platform '%s' must be in the format [OS]/[Arch]", string(p))
	}
	for _, validPlatform := range ValidShortPlatforms {
		if strings.ToLower(string(p)) == validPlatform {
			return nil
		}
	}
	return fmt.Errorf("platform '%s' is invalid; %s: %s", p, english.PluralWord(len(ValidShortPlatforms), "the valid platform is", "valid platforms are"), english.WordSeries(ValidShortPlatforms, "and"))
}

// Validate returns nil if Count is configured correctly.
func (c Count) Validate() error {
	return c.AdvancedCount.Validate()
}

// Validate returns nil if AdvancedCount is configured correctly.
func (a AdvancedCount) Validate() error {
	if a.IsEmpty() {
		return nil
	}
	if len(a.validScalingFields()) == 0 {
		return fmt.Errorf("cannot have autoscaling options for workloads of type '%s'", a.workloadType)
	}
	// Validate spot and remaining autoscaling fields.
	if a.Spot != nil && a.hasAutoscaling() {
		return &errFieldMutualExclusive{
			firstField:  "spot",
			secondField: fmt.Sprintf("range/%s", strings.Join(a.validScalingFields(), "/")),
		}
	}
	if err := a.Range.Validate(); err != nil {
		return fmt.Errorf(`validate "range": %w`, err)
	}

	// Validate combinations with "range".
	if a.Range.IsEmpty() && a.hasScalingFieldsSet() {
		return &errFieldMustBeSpecified{
			missingField:      "range",
			conditionalFields: a.validScalingFields(),
		}
	}
	if !a.Range.IsEmpty() && !a.hasScalingFieldsSet() {
		return &errAtLeastOneFieldMustBeSpecified{
			missingFields:    a.validScalingFields(),
			conditionalField: "range",
		}
	}

	// Validate individual custom autoscaling options.
	if err := a.QueueScaling.Validate(); err != nil {
		return fmt.Errorf(`validate "queue_delay": %w`, err)
	}
	if a.CPU != nil {
		if err := a.CPU.Validate(); err != nil {
			return fmt.Errorf(`validate "cpu_percentage": %w`, err)
		}
	}
	if a.Memory != nil {
		if err := a.Memory.Validate(); err != nil {
			return fmt.Errorf(`validate "memory_percentage": %w`, err)
		}
	}
	return nil
}

// Validate returns nil if Percentage is configured correctly.
func (p Percentage) Validate() error {
	if val := int(p); val < 0 || val > 100 {
		return fmt.Errorf("percentage value %v must be an integer from 0 to 100", val)
	}
	return nil
}

// Validate returns nil if QueueScaling is configured correctly.
func (qs QueueScaling) Validate() error {
	if qs.IsEmpty() {
		return nil
	}
	if qs.AcceptableLatency == nil {
		return &errFieldMustBeSpecified{
			missingField:      "acceptable_latency",
			conditionalFields: []string{"msg_processing_time"},
		}
	}
	if qs.AvgProcessingTime == nil {
		return &errFieldMustBeSpecified{
			missingField:      "msg_processing_time",
			conditionalFields: []string{"acceptable_latency"},
		}
	}
	latency, process := *qs.AcceptableLatency, *qs.AvgProcessingTime
	if latency == 0 {
		return errors.New(`"acceptable_latency" cannot be 0`)
	}
	if process == 0 {
		return errors.New(`"msg_processing_time" cannot be 0`)
	}
	if process > latency {
		return errors.New(`"msg_processing_time" cannot be longer than "acceptable_latency"`)
	}
	return nil
}

// Validate returns nil if Range is configured correctly.
func (r Range) Validate() error {
	if r.IsEmpty() {
		return nil
	}
	if !r.RangeConfig.IsEmpty() {
		return r.RangeConfig.Validate()
	}
	return r.Value.Validate()
}

// Validate returns nil if IntRangeBand is configured correctly.
func (r IntRangeBand) Validate() error {
	str := string(r)
	minMax := intRangeBandRegexp.FindStringSubmatch(str)
	// Valid minMax example: ["1-2", "1", "2"]
	if len(minMax) != 3 {
		return fmt.Errorf("invalid range value %s. Should be in format of ${min}-${max}", str)
	}
	// Guaranteed by intRangeBandRegexp.
	min, err := strconv.Atoi(minMax[1])
	if err != nil {
		return err
	}
	max, err := strconv.Atoi(minMax[2])
	if err != nil {
		return err
	}
	if min <= max {
		return nil
	}
	return &errMinGreaterThanMax{
		min: min,
		max: max,
	}
}

// Validate returns nil if RangeConfig is configured correctly.
func (r RangeConfig) Validate() error {
	if r.Min == nil || r.Max == nil {
		return &errFieldMustBeSpecified{
			missingField: "min/max",
		}
	}
	min, max := aws.IntValue(r.Min), aws.IntValue(r.Max)
	if min <= max {
		return nil
	}
	return &errMinGreaterThanMax{
		min: min,
		max: max,
	}
}

// Validate returns nil if ExecuteCommand is configured correctly.
func (e ExecuteCommand) Validate() error {
	if !e.Config.IsEmpty() {
		return e.Config.Validate()
	}
	return nil
}

// Validate returns nil if ExecuteCommandConfig is configured correctly.
func (ExecuteCommandConfig) Validate() error {
	return nil
}

// Validate returns nil if Storage is configured correctly.
func (s Storage) Validate() error {
	if s.IsEmpty() {
		return nil
	}
	if s.Ephemeral != nil {
		ephemeral := aws.IntValue(s.Ephemeral)
		if ephemeral < ephemeralMinValueGiB || ephemeral > ephemeralMaxValueGiB {
			return fmt.Errorf(`validate "ephemeral": ephemeral storage must be between 20 GiB and 200 GiB`)
		}
	}
	var hasManagedVolume bool
	for k, v := range s.Volumes {
		if err := v.Validate(); err != nil {
			return fmt.Errorf(`validate "volumes[%s]": %w`, k, err)
		}
		if !v.EmptyVolume() && v.EFS.UseManagedFS() {
			if hasManagedVolume {
				return fmt.Errorf("cannot specify more than one managed volume per service")
			}
			hasManagedVolume = true
		}
	}
	return nil
}

// Validate returns nil if Volume is configured correctly.
func (v Volume) Validate() error {
	if err := v.EFS.Validate(); err != nil {
		return fmt.Errorf(`validate "efs": %w`, err)
	}
	return v.MountPointOpts.Validate()
}

// Validate returns nil if MountPointOpts is configured correctly.
func (m MountPointOpts) Validate() error {
	path := aws.StringValue(m.ContainerPath)
	if path == "" {
		return &errFieldMustBeSpecified{
			missingField: "path",
		}
	}
	if err := validateVolumePath(path); err != nil {
		return fmt.Errorf(`validate "path": %w`, err)
	}
	return nil
}

// Validate returns nil if EFSConfigOrBool is configured correctly.
func (e EFSConfigOrBool) Validate() error {
	if e.IsEmpty() {
		return nil
	}
	return e.Advanced.Validate()
}

// Validate returns nil if EFSVolumeConfiguration is configured correctly.
func (e EFSVolumeConfiguration) Validate() error {
	if e.IsEmpty() {
		return nil
	}
	if !e.EmptyBYOConfig() && !e.EmptyUIDConfig() {
		return &errFieldMutualExclusive{
			firstField:  "uid/gid",
			secondField: "id/root_dir/auth",
		}
	}
	if e.UID != nil && e.GID == nil {
		return &errFieldMustBeSpecified{
			missingField:      "gid",
			conditionalFields: []string{"uid"},
		}
	}
	if e.UID == nil && e.GID != nil {
		return &errFieldMustBeSpecified{
			missingField:      "uid",
			conditionalFields: []string{"gid"},
		}
	}
	if e.UID != nil && *e.UID == 0 {
		return fmt.Errorf(`"uid" must not be 0`)
	}
	if err := e.AuthConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "auth": %w`, err)
	}
	if e.AuthConfig.AccessPointID != nil {
		if (aws.StringValue(e.RootDirectory) == "" || aws.StringValue(e.RootDirectory) == "/") &&
			(e.AuthConfig.IAM == nil || aws.BoolValue(e.AuthConfig.IAM)) {
			return nil
		}
		return fmt.Errorf(`"root_dir" must be either empty or "/" and "auth.iam" must be true when "access_point_id" is used`)
	}
	if e.RootDirectory != nil {
		if err := validateVolumePath(aws.StringValue(e.RootDirectory)); err != nil {
			return fmt.Errorf(`validate "root_dir": %w`, err)
		}
	}
	return nil
}

// Validate returns nil if AuthorizationConfig is configured correctly.
func (a AuthorizationConfig) Validate() error {
	if a.IsEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if Logging is configured correctly.
func (l Logging) Validate() error {
	if l.IsEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if SidecarConfig is configured correctly.
func (s SidecarConfig) Validate() error {
	for ind, mp := range s.MountPoints {
		if err := mp.Validate(); err != nil {
			return fmt.Errorf(`validate "mount_points[%d]": %w`, ind, err)
		}
	}
	if err := s.HealthCheck.Validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	if err := s.DependsOn.Validate(); err != nil {
		return fmt.Errorf(`validate "depends_on": %w`, err)
	}
	return s.ImageOverride.Validate()
}

// Validate returns nil if SidecarMountPoint is configured correctly.
func (s SidecarMountPoint) Validate() error {
	if aws.StringValue(s.SourceVolume) == "" {
		return &errFieldMustBeSpecified{
			missingField: "source_volume",
		}
	}
	return s.MountPointOpts.Validate()
}

// Validate returns nil if NetworkConfig is configured correctly.
func (n NetworkConfig) Validate() error {
	if n.IsEmpty() {
		return nil
	}
	if err := n.VPC.Validate(); err != nil {
		return fmt.Errorf(`validate "vpc": %w`, err)
	}
	return nil
}

// Validate returns nil if RequestDrivenWebServiceNetworkConfig is configured correctly.
func (n RequestDrivenWebServiceNetworkConfig) Validate() error {
	if n.IsEmpty() {
		return nil
	}
	if err := n.VPC.Validate(); err != nil {
		return fmt.Errorf(`validate "vpc": %w`, err)
	}
	return nil
}

// Validate returns nil if rdwsVpcConfig is configured correctly.
func (v rdwsVpcConfig) Validate() error {
	if v.isEmpty() {
		return nil
	}
	if v.Placement != nil {
		if err := v.Placement.Validate(); err != nil {
			return fmt.Errorf(`validate "placement": %w`, err)
		}
	}
	return nil
}

// Validate returns nil if vpcConfig is configured correctly.
func (v vpcConfig) Validate() error {
	if v.isEmpty() {
		return nil
	}
	if v.Placement != nil {
		if err := v.Placement.Validate(); err != nil {
			return fmt.Errorf(`validate "placement": %w`, err)
		}
	}
	return nil
}

// Validate returns nil if RequestDrivenWebServicePlacement is configured correctly.
func (p RequestDrivenWebServicePlacement) Validate() error {
	if err := (Placement)(p).Validate(); err != nil {
		return err
	}
	if string(p) == string(PrivateSubnetPlacement) {
		return nil
	}
	return fmt.Errorf(`placement "%s" is not supported for %s`, string(p), RequestDrivenWebServiceType)
}

// Validate returns nil if Placement is configured correctly.
func (p Placement) Validate() error {
	if string(p) == "" {
		return fmt.Errorf(`"placement" cannot be empty`)
	}
	for _, allowed := range subnetPlacements {
		if string(p) == allowed {
			return nil
		}
	}
	return fmt.Errorf(`"placement" %s must be one of %s`, string(p), strings.Join(subnetPlacements, ", "))
}

// Validate returns nil if AppRunnerInstanceConfig is configured correctly.
func (r AppRunnerInstanceConfig) Validate() error {
	if err := r.Platform.Validate(); err != nil {
		return fmt.Errorf(`validate "platform": %w`, err)
	}
	// Error out if user added Windows as platform in manifest.
	if isWindowsPlatform(r.Platform) {
		return ErrAppRunnerInvalidPlatformWindows
	}
	// This extra check is because ARM architectures won't work for App Runner services.
	if !r.Platform.IsEmpty() {
		if r.Platform.Arch() != ArchAMD64 || r.Platform.Arch() != ArchX86 {
			return fmt.Errorf("App Runner services can only build on %s and %s architectures", ArchAMD64, ArchX86)
		}
	}
	return nil
}

// Validate returns nil if RequestDrivenWebServiceHttpConfig is configured correctly.
func (r RequestDrivenWebServiceHttpConfig) Validate() error {
	return r.HealthCheckConfiguration.Validate()
}

// Validate returns nil if JobTriggerConfig is configured correctly.
func (c JobTriggerConfig) Validate() error {
	if c.Schedule == nil {
		return &errFieldMustBeSpecified{
			missingField: "schedule",
		}
	}
	return nil
}

// Validate returns nil if JobFailureHandlerConfig is configured correctly.
func (JobFailureHandlerConfig) Validate() error {
	return nil
}

// Validate returns nil if PublishConfig is configured correctly.
func (p PublishConfig) Validate() error {
	for ind, topic := range p.Topics {
		if err := topic.Validate(); err != nil {
			return fmt.Errorf(`validate "topics[%d]": %w`, ind, err)
		}
	}
	return nil
}

// Validate returns nil if Topic is configured correctly.
func (t Topic) Validate() error {
	return validatePubSubName(aws.StringValue(t.Name))
}

// Validate returns nil if SubscribeConfig is configured correctly.
func (s SubscribeConfig) Validate() error {
	if s.IsEmpty() {
		return nil
	}
	for ind, topic := range s.Topics {
		if err := topic.Validate(); err != nil {
			return fmt.Errorf(`validate "topics[%d]": %w`, ind, err)
		}
	}
	if err := s.Queue.Validate(); err != nil {
		return fmt.Errorf(`validate "queue": %w`, err)
	}
	return nil
}

// Validate returns nil if TopicSubscription is configured correctly.
func (t TopicSubscription) Validate() error {
	if err := validatePubSubName(aws.StringValue(t.Name)); err != nil {
		return err
	}
	svcName := aws.StringValue(t.Service)
	if svcName == "" {
		return &errFieldMustBeSpecified{
			missingField: "service",
		}
	}
	if !isValidSubSvcName(svcName) {
		return fmt.Errorf("service name must start with a letter, contain only lower-case letters, numbers, and hyphens, and have no consecutive or trailing hyphen")
	}
	if err := t.Queue.Validate(); err != nil {
		return fmt.Errorf(`validate "queue": %w`, err)
	}
	return nil
}

// Validate returns nil if SQSQueue is configured correctly.
func (q SQSQueueOrBool) Validate() error {
	if q.IsEmpty() {
		return nil
	}
	return q.Advanced.Validate()
}

// Validate returns nil if SQSQueue is configured correctly.
func (q SQSQueue) Validate() error {
	if q.IsEmpty() {
		return nil
	}
	if err := q.DeadLetter.Validate(); err != nil {
		return fmt.Errorf(`validate "dead_letter": %w`, err)
	}
	return nil
}

// Validate returns nil if DeadLetterQueue is configured correctly.
func (d DeadLetterQueue) Validate() error {
	if d.IsEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if OverrideRule is configured correctly.
func (r OverrideRule) Validate() error {
	for _, s := range invalidTaskDefOverridePathRegexp {
		re := regexp.MustCompile(fmt.Sprintf(`^%s$`, s))
		if re.MatchString(r.Path) {
			return fmt.Errorf(`"%s" cannot be overridden with a custom value`, s)
		}
	}
	return nil
}

type validateDependenciesOpts struct {
	mainContainerName string
	sidecarConfig     map[string]*SidecarConfig
	imageConfig       Image
	logging           Logging
}

type containerDependency struct {
	dependsOn   DependsOn
	isEssential bool
}

type validateTargetContainerOpts struct {
	mainContainerName string
	targetContainer   *string
	sidecarConfig     map[string]*SidecarConfig
}

type validateWindowsOpts struct {
	execEnabled bool
	efsVolumes  map[string]*Volume
}

type validateARMOpts struct {
	Spot     *int
	SpotFrom *int
}

func validateTargetContainer(opts validateTargetContainerOpts) error {
	if opts.targetContainer == nil {
		return nil
	}
	targetContainer := aws.StringValue(opts.targetContainer)
	if targetContainer == opts.mainContainerName {
		return nil
	}
	sidecar, ok := opts.sidecarConfig[targetContainer]
	if !ok {
		return fmt.Errorf("target container %s doesn't exist", targetContainer)
	}
	if sidecar.Port == nil {
		return fmt.Errorf("target container %s doesn't expose any port", targetContainer)
	}
	return nil
}

func validateContainerDeps(opts validateDependenciesOpts) error {
	containerDependencies := make(map[string]containerDependency)
	containerDependencies[opts.mainContainerName] = containerDependency{
		dependsOn:   opts.imageConfig.DependsOn,
		isEssential: true,
	}
	if !opts.logging.IsEmpty() {
		containerDependencies[firelensContainerName] = containerDependency{}
	}
	for name, config := range opts.sidecarConfig {
		containerDependencies[name] = containerDependency{
			dependsOn:   config.DependsOn,
			isEssential: config.Essential == nil || aws.BoolValue(config.Essential),
		}
	}
	if err := validateDepsForEssentialContainers(containerDependencies); err != nil {
		return err
	}
	return validateNoCircularDependencies(containerDependencies)
}

func validateDepsForEssentialContainers(deps map[string]containerDependency) error {
	for name, containerDep := range deps {
		for dep, status := range containerDep.dependsOn {
			if !deps[dep].isEssential {
				continue
			}
			if err := validateEssentialContainerDependency(dep, strings.ToUpper(status)); err != nil {
				return fmt.Errorf("validate %s container dependencies status: %w", name, err)
			}
		}
	}
	return nil
}

func validateEssentialContainerDependency(name, status string) error {
	for _, allowed := range essentialContainerDependsOnValidStatuses {
		if status == allowed {
			return nil
		}
	}
	return fmt.Errorf("essential container %s can only have status %s", name, english.WordSeries([]string{dependsOnStart, dependsOnHealthy}, "or"))
}

func validateNoCircularDependencies(deps map[string]containerDependency) error {
	dependencies, err := buildDependencyGraph(deps)
	if err != nil {
		return err
	}
	cycle, ok := dependencies.IsAcyclic()
	if ok {
		return nil
	}
	if len(cycle) == 1 {
		return fmt.Errorf("container %s cannot depend on itself", cycle[0])
	}
	// Stablize unit tests.
	sort.SliceStable(cycle, func(i, j int) bool { return cycle[i] < cycle[j] })
	return fmt.Errorf("circular container dependency chain includes the following containers: %s", cycle)
}

func buildDependencyGraph(deps map[string]containerDependency) (*graph.Graph, error) {
	dependencyGraph := graph.New()
	for name, containerDep := range deps {
		for dep := range containerDep.dependsOn {
			if _, ok := deps[dep]; !ok {
				return nil, fmt.Errorf("container %s does not exist", dep)
			}
			dependencyGraph.Add(graph.Edge{
				From: name,
				To:   dep,
			})
		}
	}
	return dependencyGraph, nil
}

// Validate that paths contain only an approved set of characters to guard against command injection.
// We can accept 0-9A-Za-z-_.
func validateVolumePath(input string) error {
	if len(input) == 0 {
		return nil
	}
	m := volumesPathRegexp.FindStringSubmatch(input)
	if len(m) == 0 {
		return fmt.Errorf("path can only contain the characters a-zA-Z0-9.-_/")
	}
	return nil
}

func validatePubSubName(name string) error {
	if name == "" {
		return &errFieldMustBeSpecified{
			missingField: "name",
		}
	}
	// Name must contain letters, numbers, and can't use special characters besides underscores and hyphens.
	if !awsSNSTopicRegexp.MatchString(name) {
		return fmt.Errorf(`"name" can only contain letters, numbers, underscores, and hypthens`)
	}
	return nil
}

func isValidSubSvcName(name string) bool {
	if !awsNameRegexp.MatchString(name) {
		return false
	}

	// Check for bad punctuation (no consecutive dashes or dots)
	formatMatch := punctuationRegExp.FindStringSubmatch(name)
	if len(formatMatch) != 0 {
		return false
	}

	trailingMatch := trailingPunctRegExp.FindStringSubmatch(name)
	return len(trailingMatch) == 0
}

func validateWindows(opts validateWindowsOpts) error {
	if opts.execEnabled {
		return errors.New(`'exec' is not supported when deploying a Windows container`)
	}
	for _, volume := range opts.efsVolumes {
		if !volume.EmptyVolume() {
			return errors.New(`'EFS' is not supported when deploying a Windows container`)
		}
	}
	return nil
}

func validateARM(opts validateARMOpts) error {
	if opts.Spot != nil || opts.SpotFrom != nil {
		return errors.New(`'Fargate Spot' is not supported when deploying on ARM architecture`)
	}
	return nil
}

func contains(name string, names []string) bool {
	for _, n := range names {
		if name == n {
			return true
		}
	}
	return false
}
