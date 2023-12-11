// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudfront"
	"github.com/aws/copilot-cli/internal/pkg/graph"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
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
	// TCP is the tcp protocol for NLB.
	TCP = "TCP"

	// TLS is the tls protocol for NLB.
	TLS = "TLS"

	// UDP is the udp protocol for NLB.
	UDP = "UDP"

	// Tracing vendors.
	awsXRAY = "awsxray"
)

const (
	defaultProtocol = TCP
)

const (
	// Listener rules have a quota of five condition values per rule.
	// Please refer to https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-limits.html.
	maxConditionsPerRule = 5
	rootPath             = "/"
)

var (
	intRangeBandRegexp  = regexp.MustCompile(`^(\d+)-(\d+)$`)
	volumesPathRegexp   = regexp.MustCompile(`^[a-zA-Z0-9\-\.\_/]+$`)
	awsSNSTopicRegexp   = regexp.MustCompile(`^[a-zA-Z0-9_-]*$`)   // Validates that an expression contains only letters, numbers, underscores, and hyphens.
	awsNameRegexp       = regexp.MustCompile(`^[a-z][a-z0-9\-]+$`) // Validates that an expression starts with a letter and only contains letters, numbers, and hyphens.
	punctuationRegExp   = regexp.MustCompile(`[\.\-]{2,}`)         // Check for consecutive periods or dashes.
	trailingPunctRegExp = regexp.MustCompile(`[\-\.]$`)            // Check for trailing dash or dot.

	essentialContainerDependsOnValidStatuses = []string{dependsOnStart, dependsOnHealthy}
	dependsOnValidStatuses                   = []string{dependsOnStart, dependsOnComplete, dependsOnSuccess, dependsOnHealthy}
	nlbValidProtocols                        = []string{TCP, UDP, TLS}
	validContainerProtocols                  = []string{TCP, UDP}
	validHealthCheckProtocols                = []string{TCP}
	tracingValidVendors                      = []string{awsXRAY}
	ecsRollingUpdateStrategies               = []string{ECSDefaultRollingUpdateStrategy, ECSRecreateRollingUpdateStrategy}

	httpProtocolVersions = []string{"GRPC", "HTTP1", "HTTP2"}

	invalidTaskDefOverridePathRegexp  = []string{`Family`, `ContainerDefinitions\[\d+\].Name`}
	validSQSDeduplicationScopeValues  = []string{sqsDeduplicationScopeMessageGroup, sqsDeduplicationScopeQueue}
	validSQSFIFOThroughputLimitValues = []string{sqsFIFOThroughputLimitPerMessageGroupID, sqsFIFOThroughputLimitPerQueue}
)

// Validate returns nil if DynamicLoadBalancedWebService is configured correctly.
func (l *DynamicWorkloadManifest) Validate() error {
	return l.mft.validate()
}

// validate returns nil if LoadBalancedWebService is configured correctly.
func (l LoadBalancedWebService) validate() error {
	var err error
	if err = l.LoadBalancedWebServiceConfig.validate(); err != nil {
		return err
	}
	if err = l.Workload.validate(); err != nil {
		return err
	}
	if err = validateTargetContainer(validateTargetContainerOpts{
		mainContainerName: aws.StringValue(l.Name),
		mainContainerPort: l.ImageConfig.Port,
		targetContainer:   l.HTTPOrBool.Main.TargetContainer,
		sidecarConfig:     l.Sidecars,
	}); err != nil {
		return fmt.Errorf(`validate load balancer target for "http": %w`, err)
	}
	for idx, rule := range l.HTTPOrBool.AdditionalRoutingRules {
		if err = validateTargetContainer(validateTargetContainerOpts{
			mainContainerName: aws.StringValue(l.Name),
			mainContainerPort: l.ImageConfig.Port,
			targetContainer:   rule.TargetContainer,
			sidecarConfig:     l.Sidecars,
		}); err != nil {
			return fmt.Errorf(`validate load balancer target for "http.additional_rules[%d]": %w`, idx, err)
		}
	}
	if err = validateTargetContainer(validateTargetContainerOpts{
		mainContainerName: aws.StringValue(l.Name),
		mainContainerPort: l.ImageConfig.Port,
		targetContainer:   l.NLBConfig.Listener.TargetContainer,
		sidecarConfig:     l.Sidecars,
	}); err != nil {
		return fmt.Errorf(`validate target for "nlb": %w`, err)
	}
	for idx, listener := range l.NLBConfig.AdditionalListeners {
		if err = validateTargetContainer(validateTargetContainerOpts{
			mainContainerName: aws.StringValue(l.Name),
			mainContainerPort: l.ImageConfig.Port,
			targetContainer:   listener.TargetContainer,
			sidecarConfig:     l.Sidecars,
		}); err != nil {
			return fmt.Errorf(`validate target for "nlb.additional_listeners[%d]": %w`, idx, err)
		}
	}
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     l.Sidecars,
		imageConfig:       l.ImageConfig.Image,
		mainContainerName: aws.StringValue(l.Name),
		logging:           l.Logging,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	if err = validateExposedPorts(validateExposedPortsOpts{
		mainContainerName: aws.StringValue(l.Name),
		mainContainerPort: l.ImageConfig.Port,
		sidecarConfig:     l.Sidecars,
		alb:               &l.HTTPOrBool.HTTP,
		nlb:               &l.NLBConfig,
	}); err != nil {
		return fmt.Errorf("validate unique exposed ports: %w", err)
	}
	ports, err := l.ExposedPorts()
	if err != nil {
		return err
	}
	if err = validateHealthCheckPorts(validateHealthCheckPortsOpts{
		exposedPorts:      ports,
		mainContainerPort: l.ImageConfig.Port,
		alb:               l.HTTPOrBool.HTTP,
		nlb:               l.NLBConfig,
	}); err != nil {
		return fmt.Errorf("validate load balancer health check ports: %w", err)
	}
	return nil
}

func (d DeploymentConfig) validate() error {
	if d.isEmpty() {
		return nil
	}
	if err := d.RollbackAlarms.validate(); err != nil {
		return fmt.Errorf(`validate "rollback_alarms": %w`, err)
	}
	if err := d.DeploymentControllerConfig.validate(); err != nil {
		return fmt.Errorf(`validate "rolling": %w`, err)
	}
	return nil
}

func (w WorkerDeploymentConfig) validate() error {
	if w.isEmpty() {
		return nil
	}
	if err := w.WorkerRollbackAlarms.validate(); err != nil {
		return fmt.Errorf(`validate "rollback_alarms": %w`, err)
	}
	if err := w.DeploymentControllerConfig.validate(); err != nil {
		return fmt.Errorf(`validate "deployment controller strategy": %w`, err)
	}
	return nil
}

func (d DeploymentControllerConfig) validate() error {
	if d.Rolling != nil {
		for _, validStrategy := range ecsRollingUpdateStrategies {
			if strings.EqualFold(aws.StringValue(d.Rolling), validStrategy) {
				return nil
			}
		}
		return fmt.Errorf("invalid rolling deployment strategy %q, must be one of %s",
			aws.StringValue(d.Rolling),
			english.WordSeries(ecsRollingUpdateStrategies, "or"))
	}
	return nil
}

func (a AlarmArgs) validate() error {
	return nil
}

func (w WorkerAlarmArgs) validate() error {
	return nil
}

// validate returns nil if LoadBalancedWebServiceConfig is configured correctly.
func (l LoadBalancedWebServiceConfig) validate() error {
	var err error
	if l.HTTPOrBool.Disabled() && l.NLBConfig.IsEmpty() {
		return &errAtLeastOneFieldMustBeSpecified{
			missingFields: []string{"http", "nlb"},
		}
	}
	if err = l.validateGracePeriod(); err != nil {
		return fmt.Errorf(`validate "grace_period": %w`, err)
	}
	if l.HTTPOrBool.Disabled() && (!l.Count.AdvancedCount.Requests.IsEmpty() || !l.Count.AdvancedCount.ResponseTime.IsEmpty()) {
		return errors.New(`scaling based on "nlb" requests or response time is not supported`)
	}
	if err = l.ImageConfig.validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = l.ImageOverride.validate(); err != nil {
		return err
	}
	if l.HTTPOrBool.isEmpty() {
		return &errFieldMustBeSpecified{
			missingField: "http",
		}
	}
	if err = l.HTTPOrBool.validate(); err != nil {
		return fmt.Errorf(`validate "http": %w`, err)
	}
	if err = l.TaskConfig.validate(); err != nil {
		return err
	}
	if err = l.Logging.validate(); err != nil {
		return fmt.Errorf(`validate "logging": %w`, err)
	}
	for k, v := range l.Sidecars {
		if err = v.validate(); err != nil {
			return fmt.Errorf(`validate "sidecars[%s]": %w`, k, err)
		}
	}
	if err = l.Network.validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err = l.PublishConfig.validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for ind, taskDefOverride := range l.TaskDefOverrides {
		if err = taskDefOverride.validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if l.TaskConfig.IsWindows() {
		if err = validateWindows(validateWindowsOpts{
			efsVolumes: l.Storage.Volumes,
			readOnlyFS: l.Storage.ReadonlyRootFS,
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
	if err = l.NLBConfig.validate(); err != nil {
		return fmt.Errorf(`validate "nlb": %w`, err)
	}
	if err = l.DeployConfig.validate(); err != nil {
		return fmt.Errorf(`validate "deployment": %w`, err)
	}
	return nil
}

// validate returns nil if BackendService is configured correctly.
func (b BackendService) validate() error {
	var err error
	if err = b.DeployConfig.validate(); err != nil {
		return fmt.Errorf(`validate "deployment": %w`, err)
	}
	if err = b.BackendServiceConfig.validate(); err != nil {
		return err
	}
	if err = b.Workload.validate(); err != nil {
		return err
	}
	if err = validateTargetContainer(validateTargetContainerOpts{
		mainContainerName: aws.StringValue(b.Name),
		mainContainerPort: b.ImageConfig.Port,
		targetContainer:   b.HTTP.Main.TargetContainer,
		sidecarConfig:     b.Sidecars,
	}); err != nil {
		return fmt.Errorf(`validate load balancer target for "http": %w`, err)
	}
	for idx, rule := range b.HTTP.AdditionalRoutingRules {
		if err = validateTargetContainer(validateTargetContainerOpts{
			mainContainerName: aws.StringValue(b.Name),
			mainContainerPort: b.ImageConfig.Port,
			targetContainer:   rule.TargetContainer,
			sidecarConfig:     b.Sidecars,
		}); err != nil {
			return fmt.Errorf(`validate load balancer target for "http.additional_rules[%d]": %w`, idx, err)
		}
	}
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     b.Sidecars,
		imageConfig:       b.ImageConfig.Image,
		mainContainerName: aws.StringValue(b.Name),
		logging:           b.Logging,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	if err = validateExposedPorts(validateExposedPortsOpts{
		mainContainerName: aws.StringValue(b.Name),
		mainContainerPort: b.ImageConfig.Port,
		sidecarConfig:     b.Sidecars,
		alb:               &b.HTTP,
	}); err != nil {
		return fmt.Errorf("validate unique exposed ports: %w", err)
	}
	exposedPortsIndex, err := b.ExposedPorts()
	if err != nil {
		return err
	}
	if err = validateHealthCheckPorts(validateHealthCheckPortsOpts{
		exposedPorts:      exposedPortsIndex,
		mainContainerPort: b.ImageConfig.Port,
		alb:               b.HTTP,
	}); err != nil {
		return fmt.Errorf("validate load balancer health check ports: %w", err)
	}
	return nil
}

// validate returns nil if BackendServiceConfig is configured correctly.
func (b BackendServiceConfig) validate() error {
	var err error
	if err = b.ImageConfig.validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = b.ImageOverride.validate(); err != nil {
		return err
	}
	if err = b.HTTP.validate(); err != nil {
		return fmt.Errorf(`validate "http": %w`, err)
	}
	if b.HTTP.IsEmpty() && (!b.Count.AdvancedCount.Requests.IsEmpty() || !b.Count.AdvancedCount.ResponseTime.IsEmpty()) {
		return &errFieldMustBeSpecified{
			missingField:      "http",
			conditionalFields: []string{"count.requests", "count.response_time"},
		}
	}
	if err = b.TaskConfig.validate(); err != nil {
		return err
	}
	if err = b.Logging.validate(); err != nil {
		return fmt.Errorf(`validate "logging": %w`, err)
	}
	for k, v := range b.Sidecars {
		if err = v.validate(); err != nil {
			return fmt.Errorf(`validate "sidecars[%s]": %w`, k, err)
		}
	}
	if err = b.Network.validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if b.Network.Connect.Alias != nil {
		if b.HTTP.Main.TargetContainer == nil && b.ImageConfig.Port == nil {
			return fmt.Errorf(`cannot set "network.connect.alias" when no ports are exposed`)
		}
	}
	if err = b.PublishConfig.validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for ind, taskDefOverride := range b.TaskDefOverrides {
		if err = taskDefOverride.validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if b.TaskConfig.IsWindows() {
		if err = validateWindows(validateWindowsOpts{
			efsVolumes: b.Storage.Volumes,
			readOnlyFS: b.Storage.ReadonlyRootFS,
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

// validate returns nil if RequestDrivenWebService is configured correctly.
func (r RequestDrivenWebService) validate() error {
	if err := r.RequestDrivenWebServiceConfig.validate(); err != nil {
		return err
	}
	return r.Workload.validate()
}

// validate returns nil if RequestDrivenWebServiceConfig is configured correctly.
func (r RequestDrivenWebServiceConfig) validate() error {
	var err error
	if err = r.ImageConfig.validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = r.InstanceConfig.validate(); err != nil {
		return err
	}
	if err = r.RequestDrivenWebServiceHttpConfig.validate(); err != nil {
		return fmt.Errorf(`validate "http": %w`, err)
	}
	if err = r.PublishConfig.validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	if err = r.Network.validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if r.Network.VPC.Placement.PlacementString != nil &&
		*r.Network.VPC.Placement.PlacementString != PrivateSubnetPlacement {
		return fmt.Errorf(`placement %q is not supported for %s`,
			*r.Network.VPC.Placement.PlacementString, manifestinfo.RequestDrivenWebServiceType)
	}
	if err = r.Observability.validate(); err != nil {
		return fmt.Errorf(`validate "observability": %w`, err)
	}
	return nil
}

// validate returns nil if WorkerService is configured correctly.
func (w WorkerService) validate() error {
	var err error
	if err = w.WorkerServiceConfig.validate(); err != nil {
		return err
	}
	if err = w.Workload.validate(); err != nil {
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
	if err = validateExposedPorts(validateExposedPortsOpts{
		sidecarConfig: w.Sidecars,
	}); err != nil {
		return fmt.Errorf("validate unique exposed ports: %w", err)
	}
	return nil
}

// validate returns nil if WorkerServiceConfig is configured correctly.
func (w WorkerServiceConfig) validate() error {
	var err error
	if err = w.DeployConfig.validate(); err != nil {
		return fmt.Errorf(`validate "deployment": %w`, err)
	}
	if err = w.ImageConfig.validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = w.ImageOverride.validate(); err != nil {
		return err
	}
	if err = w.TaskConfig.validate(); err != nil {
		return err
	}
	if err = w.Logging.validate(); err != nil {
		return fmt.Errorf(`validate "logging": %w`, err)
	}
	for k, v := range w.Sidecars {
		if err = v.validate(); err != nil {
			return fmt.Errorf(`validate "sidecars[%s]": %w`, k, err)
		}
	}
	if err = w.Network.validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if w.Network.Connect.Alias != nil {
		return fmt.Errorf(`cannot set "network.connect.alias" when no ports are exposed`)
	}
	if err = w.Subscribe.validate(); err != nil {
		return fmt.Errorf(`validate "subscribe": %w`, err)
	}
	if err = w.PublishConfig.validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for ind, taskDefOverride := range w.TaskDefOverrides {
		if err = taskDefOverride.validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if w.TaskConfig.IsWindows() {
		if err = validateWindows(validateWindowsOpts{
			efsVolumes: w.Storage.Volumes,
			readOnlyFS: w.Storage.ReadonlyRootFS,
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

// validate returns nil if ScheduledJob is configured correctly.
func (s ScheduledJob) validate() error {
	var err error
	if err = s.ScheduledJobConfig.validate(); err != nil {
		return err
	}
	if err = s.Workload.validate(); err != nil {
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
	if err = validateExposedPorts(validateExposedPortsOpts{
		sidecarConfig: s.Sidecars,
	}); err != nil {
		return fmt.Errorf("validate unique exposed ports: %w", err)
	}
	return nil
}

// validate returns nil if ScheduledJobConfig is configured correctly.
func (s ScheduledJobConfig) validate() error {
	var err error
	if err = s.ImageConfig.validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = s.ImageOverride.validate(); err != nil {
		return err
	}
	if err = s.TaskConfig.validate(); err != nil {
		return err
	}
	if err = s.Logging.validate(); err != nil {
		return fmt.Errorf(`validate "logging": %w`, err)
	}
	for k, v := range s.Sidecars {
		if err = v.validate(); err != nil {
			return fmt.Errorf(`validate "sidecars[%s]": %w`, k, err)
		}
	}
	if err = s.Network.validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err = s.On.validate(); err != nil {
		return fmt.Errorf(`validate "on": %w`, err)
	}
	if err = s.JobFailureHandlerConfig.validate(); err != nil {
		return err
	}
	if err = s.PublishConfig.validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for ind, taskDefOverride := range s.TaskDefOverrides {
		if err = taskDefOverride.validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if s.TaskConfig.IsWindows() {
		if err = validateWindows(validateWindowsOpts{
			efsVolumes: s.Storage.Volumes,
			readOnlyFS: s.Storage.ReadonlyRootFS,
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

// validate returns nil if StaticSite is configured correctly.
func (s StaticSite) validate() error {
	if err := s.StaticSiteConfig.validate(); err != nil {
		return err
	}
	return s.Workload.validate()
}

func (s StaticSiteConfig) validate() error {
	if err := s.HTTP.validate(); err != nil {
		return fmt.Errorf(`validate "http": %w`, err)
	}
	for idx, fileupload := range s.FileUploads {
		if err := fileupload.validate(); err != nil {
			return fmt.Errorf(`validate "files[%d]": %w`, idx, err)
		}
	}
	return nil
}

func (f FileUpload) validate() error {
	return f.validateSource()
}

func (s StaticSiteHTTP) validate() error {
	if s.Certificate != "" {
		if s.Alias == "" {
			return &errFieldMustBeSpecified{
				missingField:      "alias",
				conditionalFields: []string{"certificate"},
			}
		}
		certARN, err := arn.Parse(s.Certificate)
		if err != nil {
			return fmt.Errorf(`parse cdn certificate: %w`, err)
		}
		if certARN.Region != cloudfront.CertRegion {
			return &errInvalidCloudFrontRegion{}
		}
	}
	return nil
}

// validateSource returns nil if Source is configured correctly.
func (f FileUpload) validateSource() error {
	if f.Source == "" {
		return &errFieldMustBeSpecified{
			missingField: "source",
		}
	}
	return nil
}

// Validate returns nil if the pipeline manifest is configured correctly.
func (p Pipeline) Validate() error {
	if len(p.Name) > 100 {
		return fmt.Errorf(`pipeline name '%s' must be shorter than 100 characters`, p.Name)
	}
	for _, stg := range p.Stages {
		if err := stg.validate(); err != nil {
			return fmt.Errorf(`validate stage %q for pipeline %q: %w`, stg.Name, p.Name, err)
		}
		if err := stg.Deployments.validate(); err != nil {
			return fmt.Errorf(`validate "deployments" for pipeline stage %s: %w`, stg.Name, err)
		}
	}
	return nil
}

// validate returns nil if stages are configured correctly.
func (s PipelineStage) validate() error {
	if len(s.TestCommands) != 0 && s.PostDeployments != nil {
		return &errFieldMutualExclusive{
			firstField:  "post_deployments",
			secondField: "test_commands",
			mustExist:   false,
		}
	}
	if s.PreDeployments != nil {
		for _, preDep := range s.PreDeployments {
			if preDep.BuildspecPath == "" {
				return &errFieldMustBeSpecified{
					missingField: "buildspec",
				}
			}
		}

	}
	if s.PostDeployments != nil {
		for _, postDep := range s.PostDeployments {
			if postDep.BuildspecPath == "" {
				return &errFieldMustBeSpecified{
					missingField: "buildspec",
				}
			}
		}

	}
	return nil
}

// validate returns nil if deployments are configured correctly.
func (d Deployments) validate() error {
	names := make(map[string]bool)
	for name := range d {
		names[name] = true
	}

	for name, conf := range d {
		if conf == nil {
			continue
		}
		for _, dependency := range conf.DependsOn {
			if _, ok := names[dependency]; !ok {
				return fmt.Errorf("dependency deployment named '%s' of '%s' does not exist", dependency, name)
			}
		}
	}
	return nil
}

// validate returns nil if Workload is configured correctly.
func (w Workload) validate() error {
	if w.Name == nil {
		return &errFieldMustBeSpecified{
			missingField: "name",
		}
	}
	return nil
}

// validate returns nil if ImageWithPortAndHealthcheck is configured correctly.
func (i ImageWithPortAndHealthcheck) validate() error {
	var err error
	if err = i.ImageWithPort.validate(); err != nil {
		return err
	}
	if err = i.HealthCheck.validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	return nil
}

// validate returns nil if ImageWithHealthcheckAndOptionalPort is configured correctly.
func (i ImageWithHealthcheckAndOptionalPort) validate() error {
	var err error
	if err = i.ImageWithOptionalPort.validate(); err != nil {
		return err
	}
	if err = i.HealthCheck.validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	return nil
}

// validate returns nil if ImageWithHealthcheck is configured correctly.
func (i ImageWithHealthcheck) validate() error {
	if err := i.Image.validate(); err != nil {
		return err
	}
	return nil
}

// validate returns nil if ImageWithOptionalPort is configured correctly.
func (i ImageWithOptionalPort) validate() error {
	if err := i.Image.validate(); err != nil {
		return err
	}
	return nil
}

// validate returns nil if ImageWithPort is configured correctly.
func (i ImageWithPort) validate() error {
	if err := i.Image.validate(); err != nil {
		return err
	}
	if i.Port == nil {
		return &errFieldMustBeSpecified{
			missingField: "port",
		}
	}
	return nil
}

// validate returns nil if Image is configured correctly.
func (i Image) validate() error {
	var err error
	if err := i.ImageLocationOrBuild.validate(); err != nil {
		return err
	}
	if err = i.DependsOn.validate(); err != nil {
		return fmt.Errorf(`validate "depends_on": %w`, err)
	}
	return nil
}

// validate returns nil if DependsOn is configured correctly.
func (d DependsOn) validate() error {
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

// validate returns nil if BuildArgsOrString is configured correctly.
func (b BuildArgsOrString) validate() error {
	if b.isEmpty() {
		return nil
	}
	if !b.BuildArgs.isEmpty() {
		return b.BuildArgs.validate()
	}
	return nil
}

// validate returns nil if DockerBuildArgs is configured correctly.
func (DockerBuildArgs) validate() error {
	return nil
}

// validate returns nil if ContainerHealthCheck is configured correctly.
func (ContainerHealthCheck) validate() error {
	return nil
}

// validate returns nil if ImageOverride is configured correctly.
func (i ImageOverride) validate() error {
	var err error
	if err = i.EntryPoint.validate(); err != nil {
		return fmt.Errorf(`validate "entrypoint": %w`, err)
	}
	if err = i.Command.validate(); err != nil {
		return fmt.Errorf(`validate "command": %w`, err)
	}
	return nil
}

// validate returns nil if EntryPointOverride is configured correctly.
func (EntryPointOverride) validate() error {
	return nil
}

// validate returns nil if CommandOverride is configured correctly.
func (CommandOverride) validate() error {
	return nil
}

// validate returns nil if HTTP is configured correctly.
func (r HTTP) validate() error {
	if r.IsEmpty() {
		return nil
	}
	// we consider the fact that primary routing rule is mandatory before you write any additional routing rules.
	if err := r.Main.validate(); err != nil {
		return err
	}
	if r.Main.TargetContainer != nil && r.TargetContainerCamelCase != nil {
		return &errFieldMutualExclusive{
			firstField:  "target_container",
			secondField: "targetContainer",
		}
	}

	for idx, rule := range r.AdditionalRoutingRules {
		if err := rule.validate(); err != nil {
			return fmt.Errorf(`validate "additional_rules[%d]": %w`, idx, err)
		}
	}
	return nil
}

// validate returns nil if HTTPOrBool is configured correctly.
func (r HTTPOrBool) validate() error {
	if r.Disabled() {
		return nil
	}

	return r.HTTP.validate()
}

func (l LoadBalancedWebServiceConfig) validateGracePeriod() error {
	gracePeriodForALB, err := l.validateGracePeriodForALB()
	if err != nil {
		return err
	}
	gracePeriodForNLB, err := l.validateGracePeriodForNLB()
	if err != nil {
		return err
	}
	if gracePeriodForALB && gracePeriodForNLB {
		return &errGracePeriodsInBothALBAndNLB{
			errFieldMutualExclusive: errFieldMutualExclusive{
				firstField:  "http.healthcheck.grace_period",
				secondField: "nlb.healthcheck.grace_period",
			},
		}
	}

	return nil
}

// validateGracePeriodForALB validates if ALB has grace period mentioned in their additional listeners rules.
func (cfg *LoadBalancedWebServiceConfig) validateGracePeriodForALB() (bool, error) {
	var exist bool
	if cfg.HTTPOrBool.Main.HealthCheck.Advanced.GracePeriod != nil {
		exist = true
	}
	for idx, rule := range cfg.HTTPOrBool.AdditionalRoutingRules {
		if rule.HealthCheck.Advanced.GracePeriod != nil {
			return exist, &errGracePeriodSpecifiedInAdditionalRule{
				index: idx,
			}
		}
	}
	return exist, nil
}

// validateGracePeriodForNLB validates if NLB has grace period mentioned in their additional listeners.
func (cfg *LoadBalancedWebServiceConfig) validateGracePeriodForNLB() (bool, error) {
	var exist bool
	if cfg.NLBConfig.Listener.HealthCheck.GracePeriod != nil {
		exist = true
	}
	for idx, listener := range cfg.NLBConfig.AdditionalListeners {
		if listener.HealthCheck.GracePeriod != nil {
			return exist, &errGracePeriodSpecifiedInAdditionalListener{
				index: idx,
			}
		}
	}
	return exist, nil
}

// validate returns nil if HTTP is configured correctly.
func (r RoutingRule) validate() error {
	if r.Path == nil {
		return &errFieldMustBeSpecified{
			missingField: "path",
		}
	}
	if err := r.HealthCheck.validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	if err := r.Alias.validate(); err != nil {
		return fmt.Errorf(`validate "alias": %w`, err)
	}
	for ind, ip := range r.AllowedSourceIps {
		if err := ip.validate(); err != nil {
			return fmt.Errorf(`validate "allowed_source_ips[%d]": %w`, ind, err)
		}
	}
	if r.ProtocolVersion != nil {
		if !slices.Contains(httpProtocolVersions, strings.ToUpper(*r.ProtocolVersion)) {
			return fmt.Errorf(`"version" field value '%s' must be one of %s`, *r.ProtocolVersion, english.WordSeries(httpProtocolVersions, "or"))
		}
	}
	if r.HostedZone != nil && r.Alias.IsEmpty() {
		return &errFieldMustBeSpecified{
			missingField:      "alias",
			conditionalFields: []string{"hosted_zone"},
		}
	}
	if err := r.validateConditionValuesPerRule(); err != nil {
		return fmt.Errorf("validate condition values per listener rule: %w", err)
	}
	return nil
}

// validate returns nil if HTTPHealthCheckArgs is configured correctly.
func (h HTTPHealthCheckArgs) validate() error {
	return nil
}

// validate returns nil if NLBHealthCheckArgs is configured correctly.
func (h NLBHealthCheckArgs) validate() error {
	if h.isEmpty() {
		return nil
	}
	return nil
}

// validate returns nil if Alias is configured correctly.
func (a Alias) validate() error {
	if a.IsEmpty() {
		return nil
	}
	if err := a.StringSliceOrString.validate(); err != nil {
		return err
	}
	for _, alias := range a.AdvancedAliases {
		if err := alias.validate(); err != nil {
			return err
		}
	}
	return nil
}

// validate returns nil if AdvancedAlias is configured correctly.
func (a AdvancedAlias) validate() error {
	if a.Alias == nil {
		return &errFieldMustBeSpecified{
			missingField: "name",
		}
	}
	return nil
}

// validate is a no-op for StringSliceOrString.
func (StringSliceOrString) validate() error {
	return nil
}

// validate returns nil if IPNet is configured correctly.
func (ip IPNet) validate() error {
	if _, _, err := net.ParseCIDR(string(ip)); err != nil {
		return fmt.Errorf("parse IPNet %s: %w", string(ip), err)
	}
	return nil
}

// validate returns nil if NetworkLoadBalancerConfiguration is configured correctly.
func (c NetworkLoadBalancerConfiguration) validate() error {
	if c.IsEmpty() {
		return nil
	}
	if err := c.Listener.validate(); err != nil {
		return err
	}
	if err := c.Aliases.validate(); err != nil {
		return fmt.Errorf(`validate "alias": %w`, err)
	}
	if !c.Aliases.IsEmpty() {
		for _, advancedAlias := range c.Aliases.AdvancedAliases {
			if advancedAlias.HostedZone != nil {
				return fmt.Errorf(`"hosted_zone" is not supported for Network Load Balancer`)
			}
		}
	}
	for idx, listener := range c.AdditionalListeners {
		if err := listener.validate(); err != nil {
			return fmt.Errorf(`validate "additional_listeners[%d]": %w`, idx, err)
		}
	}
	return nil
}

func (c NetworkLoadBalancerListener) validate() error {
	if aws.StringValue(c.Port) == "" {
		return &errFieldMustBeSpecified{
			missingField: "port",
		}
	}
	if err := validateNLBPort(c.Port); err != nil {
		return fmt.Errorf(`validate "port": %w`, err)
	}
	if err := c.HealthCheck.validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
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
	for _, valid := range nlbValidProtocols {
		if strings.EqualFold(protocolVal, valid) {
			isValidProtocol = true
			break
		}
	}
	if !isValidProtocol {
		return fmt.Errorf(`invalid protocol %s; valid protocols include %s`, protocolVal, english.WordSeries(nlbValidProtocols, "and"))
	}
	return nil
}

// validate returns nil if TaskConfig is configured correctly.
func (t TaskConfig) validate() error {
	var err error
	if err = t.Platform.validate(); err != nil {
		return fmt.Errorf(`validate "platform": %w`, err)
	}
	if err = t.Count.validate(); err != nil {
		return fmt.Errorf(`validate "count": %w`, err)
	}
	if err = t.ExecuteCommand.validate(); err != nil {
		return fmt.Errorf(`validate "exec": %w`, err)
	}
	if err = t.Storage.validate(); err != nil {
		return fmt.Errorf(`validate "storage": %w`, err)
	}
	for n, v := range t.Variables {
		if err := v.validate(); err != nil {
			return fmt.Errorf(`validate %q "variables": %w`, n, err)
		}
	}
	for _, v := range t.Secrets {
		if err := v.validate(); err != nil {
			return fmt.Errorf(`validate "secret": %w`, err)
		}
	}
	if t.EnvFile != nil {
		envFile := aws.StringValue(t.EnvFile)
		if filepath.Ext(envFile) != envFileExt {
			return fmt.Errorf("environment file %s must have a %s file extension", envFile, envFileExt)
		}
	}
	return nil
}

// validate returns nil if PlatformArgsOrString is configured correctly.
func (p PlatformArgsOrString) validate() error {
	if p.IsEmpty() {
		return nil
	}
	if !p.PlatformArgs.isEmpty() {
		return p.PlatformArgs.validate()
	}
	if p.PlatformString != nil {
		return p.PlatformString.validate()
	}
	return nil
}

// validate returns nil if PlatformArgs is configured correctly.
func (p PlatformArgs) validate() error {
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

// validate returns nil if PlatformString is configured correctly.
func (p PlatformString) validate() error {
	args := strings.Split(string(p), "/")
	if len(args) != 2 {
		return fmt.Errorf("platform '%s' must be in the format [OS]/[Arch]", string(p))
	}
	for _, validPlatform := range validShortPlatforms {
		if strings.ToLower(string(p)) == validPlatform {
			return nil
		}
	}
	return fmt.Errorf("platform '%s' is invalid; %s: %s", p, english.PluralWord(len(validShortPlatforms), "the valid platform is", "valid platforms are"), english.WordSeries(validShortPlatforms, "and"))
}

// validate returns nil if Count is configured correctly.
func (c Count) validate() error {
	return c.AdvancedCount.validate()
}

// validate returns nil if AdvancedCount is configured correctly.
func (a AdvancedCount) validate() error {
	if a.IsEmpty() {
		return nil
	}
	if len(a.validScalingFields()) == 0 {
		return fmt.Errorf("cannot have autoscaling options for workloads of type '%s'", a.workloadType)
	}

	// validate if incorrect autoscaling fields are set
	if fields := a.getInvalidFieldsSet(); fields != nil {
		return &errInvalidAutoscalingFieldsWithWkldType{
			invalidFields: fields,
			workloadType:  a.workloadType,
		}
	}

	// validate spot and remaining autoscaling fields.
	if a.Spot != nil && a.hasAutoscaling() {
		return &errFieldMutualExclusive{
			firstField:  "spot",
			secondField: fmt.Sprintf("range/%s", strings.Join(a.validScalingFields(), "/")),
		}
	}
	if err := a.Range.validate(); err != nil {
		return fmt.Errorf(`validate "range": %w`, err)
	}

	// validate combinations with "range".
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

	// validate combinations with cooldown
	if !a.Cooldown.IsEmpty() && !a.hasScalingFieldsSet() {
		return &errAtLeastOneFieldMustBeSpecified{
			missingFields:    a.validScalingFields(),
			conditionalField: "cooldown",
		}
	}

	// validate individual custom autoscaling options.
	if err := a.QueueScaling.validate(); err != nil {
		return fmt.Errorf(`validate "queue_delay": %w`, err)
	}
	if err := a.CPU.validate(); err != nil {
		return fmt.Errorf(`validate "cpu_percentage": %w`, err)
	}
	if err := a.Memory.validate(); err != nil {
		return fmt.Errorf(`validate "memory_percentage": %w`, err)
	}

	return nil
}

// validate returns nil if Percentage is configured correctly.
func (p Percentage) validate() error {
	if val := int(p); val < 0 || val > 100 {
		return fmt.Errorf("percentage value %v must be an integer from 0 to 100", val)
	}
	return nil
}

// validate returns nil if ScalingConfigOrT is configured correctly.
func (r ScalingConfigOrT[_]) validate() error {
	if r.IsEmpty() {
		return nil
	}
	if r.Value != nil {
		switch any(r.Value).(type) {
		case *Percentage:
			return any(r.Value).(*Percentage).validate()
		default:
			return nil
		}
	}
	return r.ScalingConfig.validate()
}

// validate returns nil if AdvancedScalingConfig is configured correctly.
func (r AdvancedScalingConfig[_]) validate() error {
	if r.IsEmpty() {
		return nil
	}
	switch any(r.Value).(type) {
	case *Percentage:
		if err := any(r.Value).(*Percentage).validate(); err != nil {
			return err
		}
	}
	return r.Cooldown.validate()
}

// Validation is a no-op for Cooldown.
func (c Cooldown) validate() error {
	return nil
}

// validate returns nil if QueueScaling is configured correctly.
func (qs QueueScaling) validate() error {
	if qs.IsEmpty() {
		return nil
	}
	if qs.AcceptableLatency == nil && qs.AvgProcessingTime != nil {
		return &errFieldMustBeSpecified{
			missingField:      "acceptable_latency",
			conditionalFields: []string{"msg_processing_time"},
		}
	}
	if qs.AvgProcessingTime == nil && qs.AcceptableLatency != nil {
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
	return qs.Cooldown.validate()
}

// validate returns nil if Range is configured correctly.
func (r Range) validate() error {
	if r.IsEmpty() {
		return nil
	}
	if !r.RangeConfig.IsEmpty() {
		return r.RangeConfig.validate()
	}
	return r.Value.validate()
}

type errInvalidRange struct {
	value       string
	validFormat string
}

func (e *errInvalidRange) Error() string {
	return fmt.Sprintf("invalid range value %s: valid format is %s", e.value, e.validFormat)
}

// validate returns nil if IntRangeBand is configured correctly.
func (r IntRangeBand) validate() error {
	str := string(r)
	minMax := intRangeBandRegexp.FindStringSubmatch(str)
	// Valid minMax example: ["1-2", "1", "2"]
	if len(minMax) != 3 {
		return &errInvalidRange{
			value:       str,
			validFormat: "${min}-${max}",
		}
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

// validate returns nil if RangeConfig is configured correctly.
func (r RangeConfig) validate() error {
	if r.Min == nil || r.Max == nil {
		return &errFieldMustBeSpecified{
			missingField: "min/max",
		}
	}
	min, max, spotFrom := aws.IntValue(r.Min), aws.IntValue(r.Max), aws.IntValue(r.SpotFrom)
	if min < 0 || max < 0 || spotFrom < 0 {
		return &errRangeValueLessThanZero{
			min:      min,
			max:      max,
			spotFrom: spotFrom,
		}
	}
	if min <= max {
		return nil
	}
	return &errMinGreaterThanMax{
		min: min,
		max: max,
	}
}

// validate returns nil if ExecuteCommand is configured correctly.
func (e ExecuteCommand) validate() error {
	if !e.Config.IsEmpty() {
		return e.Config.validate()
	}
	return nil
}

// validate returns nil if ExecuteCommandConfig is configured correctly.
func (ExecuteCommandConfig) validate() error {
	return nil
}

// validate returns nil if Storage is configured correctly.
func (s Storage) validate() error {
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
		if err := v.validate(); err != nil {
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

// validate returns nil if Volume is configured correctly.
func (v Volume) validate() error {
	if err := v.EFS.validate(); err != nil {
		return fmt.Errorf(`validate "efs": %w`, err)
	}
	return v.MountPointOpts.validate()
}

// validate returns nil if MountPointOpts is configured correctly.
func (m MountPointOpts) validate() error {
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

// validate returns nil if EFSConfigOrBool is configured correctly.
func (e EFSConfigOrBool) validate() error {
	if e.IsEmpty() {
		return nil
	}
	return e.Advanced.validate()
}

// validate returns nil if EFSVolumeConfiguration is configured correctly.
func (e EFSVolumeConfiguration) validate() error {
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
	if err := e.AuthConfig.validate(); err != nil {
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

// validate returns nil if AuthorizationConfig is configured correctly.
func (a AuthorizationConfig) validate() error {
	if a.IsEmpty() {
		return nil
	}
	return nil
}

// validate returns nil if Logging is configured correctly.
func (l Logging) validate() error {
	if l.IsEmpty() {
		return nil
	}
	if l.EnvFile != nil {
		envFile := aws.StringValue(l.EnvFile)
		if filepath.Ext(envFile) != envFileExt {
			return fmt.Errorf("environment file %s must have a %s file extension", envFile, envFileExt)
		}
	}
	return nil
}

// validate returns nil if SidecarConfig is configured correctly.
func (s SidecarConfig) validate() error {
	if err := s.validateImage(); err != nil {
		return err
	}
	for ind, mp := range s.MountPoints {
		if err := mp.validate(); err != nil {
			return fmt.Errorf(`validate "mount_points[%d]": %w`, ind, err)
		}
	}
	_, protocol, err := ParsePortMapping(s.Port)
	if err != nil {
		return err
	}
	if protocol != nil {
		protocolVal := aws.StringValue(protocol)
		var isValidProtocol bool
		for _, valid := range validContainerProtocols {
			if strings.EqualFold(protocolVal, valid) {
				isValidProtocol = true
				break
			}
		}
		if !isValidProtocol {
			return fmt.Errorf(`invalid protocol %s; valid protocols include %s`, protocolVal, english.WordSeries(validContainerProtocols, "and"))
		}
	}
	if err := s.HealthCheck.validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	if err := s.DependsOn.validate(); err != nil {
		return fmt.Errorf(`validate "depends_on": %w`, err)
	}
	if s.EnvFile != nil {
		envFile := aws.StringValue(s.EnvFile)
		if filepath.Ext(envFile) != envFileExt {
			return fmt.Errorf("environment file %s must have a %s file extension", envFile, envFileExt)
		}
	}
	return s.ImageOverride.validate()
}
func (s SidecarConfig) validateImage() error {
	if s.Image.IsZero() {
		return fmt.Errorf(`must specify one of "image", "image.build, or "image.location"`)
	}
	if err := s.Image.validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	return nil
}

// validate returns nil if SidecarMountPoint is configured correctly.
func (s SidecarMountPoint) validate() error {
	if aws.StringValue(s.SourceVolume) == "" {
		return &errFieldMustBeSpecified{
			missingField: "source_volume",
		}
	}
	return s.MountPointOpts.validate()
}

// validate returns nil if NetworkConfig is configured correctly.
func (n NetworkConfig) validate() error {
	if n.IsEmpty() {
		return nil
	}
	if err := n.VPC.validate(); err != nil {
		return fmt.Errorf(`validate "vpc": %w`, err)
	}
	if err := n.Connect.validate(); err != nil {
		return fmt.Errorf(`validate "connect": %w`, err)
	}
	return nil
}

// validate returns nil if ServiceConnectBoolOrArgs is configured correctly.
func (s ServiceConnectBoolOrArgs) validate() error {
	return s.ServiceConnectArgs.validate()
}

// validate is a no-op for ServiceConnectArgs.
func (ServiceConnectArgs) validate() error {
	return nil
}

// validate returns nil if RequestDrivenWebServiceNetworkConfig is configured correctly.
func (n RequestDrivenWebServiceNetworkConfig) validate() error {
	if n.IsEmpty() {
		return nil
	}
	if err := n.VPC.validate(); err != nil {
		return fmt.Errorf(`validate "vpc": %w`, err)
	}
	return nil
}

// validate returns nil if rdwsVpcConfig is configured correctly.
func (v rdwsVpcConfig) validate() error {
	if v.isEmpty() {
		return nil
	}
	if err := v.Placement.validate(); err != nil {
		return fmt.Errorf(`validate "placement": %w`, err)
	}
	return nil
}

// validate returns nil if vpcConfig is configured correctly.
func (v vpcConfig) validate() error {
	if v.isEmpty() {
		return nil
	}
	if err := v.Placement.validate(); err != nil {
		return fmt.Errorf(`validate "placement": %w`, err)
	}
	if err := v.SecurityGroups.validate(); err != nil {
		return fmt.Errorf(`validate "security_groups": %w`, err)
	}
	return nil
}

// validate returns nil if PlacementArgOrString is configured correctly.
func (p PlacementArgOrString) validate() error {
	if p.IsEmpty() {
		return nil
	}
	if p.PlacementString != nil {
		return p.PlacementString.validate()
	}
	return p.PlacementArgs.validate()
}

// validate returns nil if PlacementArgs is configured correctly.
func (p PlacementArgs) validate() error {
	if !p.Subnets.isEmpty() {
		return p.Subnets.validate()
	}
	return nil
}

// validate returns nil if SubnetArgs is configured correctly.
func (s SubnetArgs) validate() error {
	if s.isEmpty() {
		return nil
	}
	return s.FromTags.validate()
}

// validate returns nil if Tags is configured correctly.
func (t Tags) validate() error {
	for _, v := range t {
		if err := v.validate(); err != nil {
			return err
		}
	}
	return nil
}

// validate returns nil if PlacementString is configured correctly.
func (p PlacementString) validate() error {
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

// validate is a no-op for SecurityGroupsIDsOrConfig.
func (s SecurityGroupsIDsOrConfig) validate() error {
	if s.isEmpty() {
		return nil
	}
	return s.AdvancedConfig.validate()
}

// validate is a no-op for SecurityGroupsConfig.
func (SecurityGroupsConfig) validate() error {
	return nil
}

// validate returns nil if AppRunnerInstanceConfig is configured correctly.
func (r AppRunnerInstanceConfig) validate() error {
	if err := r.Platform.validate(); err != nil {
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

// validate returns nil if RequestDrivenWebServiceHttpConfig is configured correctly.
func (r RequestDrivenWebServiceHttpConfig) validate() error {
	if err := r.HealthCheckConfiguration.validate(); err != nil {
		return err
	}
	return r.Private.validate()
}

func (v VPCEndpoint) validate() error {
	return nil
}

// validate returns nil if Observability is configured correctly.
func (o Observability) validate() error {
	if o.isEmpty() {
		return nil
	}
	for _, validVendor := range tracingValidVendors {
		if strings.EqualFold(aws.StringValue(o.Tracing), validVendor) {
			return nil
		}
	}
	return fmt.Errorf("invalid tracing vendor %s: %s %s",
		aws.StringValue(o.Tracing),
		english.PluralWord(len(tracingValidVendors), "the valid vendor is", "valid vendors are"),
		english.WordSeries(tracingValidVendors, "and"))
}

// validate returns nil if JobTriggerConfig is configured correctly.
func (c JobTriggerConfig) validate() error {
	if c.Schedule == nil {
		return &errFieldMustBeSpecified{
			missingField: "schedule",
		}
	}
	return nil
}

// validate returns nil if JobFailureHandlerConfig is configured correctly.
func (JobFailureHandlerConfig) validate() error {
	return nil
}

// validate returns nil if PublishConfig is configured correctly.
func (p PublishConfig) validate() error {
	for ind, topic := range p.Topics {
		if err := topic.validate(); err != nil {
			return fmt.Errorf(`validate "topics[%d]": %w`, ind, err)
		}
	}
	return nil
}

// validate returns nil if Topic is configured correctly.
func (t Topic) validate() error {
	if err := validatePubSubName(aws.StringValue(t.Name)); err != nil {
		return err
	}
	return t.FIFO.validate()
}

// validate returns nil if FIFOTopicAdvanceConfigOrBool is configured correctly.
func (f FIFOTopicAdvanceConfigOrBool) validate() error {
	if f.IsEmpty() {
		return nil
	}
	return f.Advanced.validate()
}

// validate returns nil if FIFOTopicAdvanceConfig is configured correctly.
func (a FIFOTopicAdvanceConfig) validate() error {
	return nil
}

// validate returns nil if SubscribeConfig is configured correctly.
func (s SubscribeConfig) validate() error {
	if s.IsEmpty() {
		return nil
	}
	for ind, topic := range s.Topics {
		if err := topic.validate(); err != nil {
			return fmt.Errorf(`validate "topics[%d]": %w`, ind, err)
		}
	}
	if err := s.Queue.validate(); err != nil {
		return fmt.Errorf(`validate "queue": %w`, err)
	}
	return nil
}

// validate returns nil if TopicSubscription is configured correctly.
func (t TopicSubscription) validate() error {
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
	if err := t.Queue.validate(); err != nil {
		return fmt.Errorf(`validate "queue": %w`, err)
	}
	return nil
}

// validate returns nil if SQSQueue is configured correctly.
func (q SQSQueueOrBool) validate() error {
	if q.IsEmpty() {
		return nil
	}
	return q.Advanced.validate()
}

// validate returns nil if SQSQueue is configured correctly.
func (q SQSQueue) validate() error {
	if q.IsEmpty() {
		return nil
	}
	if err := q.DeadLetter.validate(); err != nil {
		return fmt.Errorf(`validate "dead_letter": %w`, err)
	}
	return q.FIFO.validate()
}

// validate returns nil if FIFOAdvanceConfig is configured correctly.
func (q FIFOAdvanceConfig) validate() error {
	if q.IsEmpty() {
		return nil
	}

	if err := q.validateHighThroughputFIFO(); err != nil {
		return err
	}
	if err := q.validateDeduplicationScope(); err != nil {
		return err
	}
	if err := q.validateFIFOThroughputLimit(); err != nil {
		return err
	}
	if aws.StringValue(q.FIFOThroughputLimit) == sqsFIFOThroughputLimitPerMessageGroupID && aws.StringValue(q.DeduplicationScope) == sqsDeduplicationScopeQueue {
		return fmt.Errorf(`"throughput_limit" must be set to "perQueue" when "deduplication_scope" is set to "queue"`)
	}
	return nil
}

// validateFIFO returns nil if FIFOAdvanceConfigOrBool is configured correctly.
func (q FIFOAdvanceConfigOrBool) validate() error {
	if q.IsEmpty() {
		return nil
	}
	return q.Advanced.validate()
}

func (q FIFOAdvanceConfig) validateHighThroughputFIFO() error {
	if q.HighThroughputFifo == nil {
		return nil
	}
	if q.FIFOThroughputLimit != nil {
		return &errFieldMutualExclusive{
			firstField:  "high_throughput",
			secondField: "throughput_limit",
			mustExist:   false,
		}
	}

	if q.DeduplicationScope != nil {
		return &errFieldMutualExclusive{
			firstField:  "high_throughput",
			secondField: "deduplication_scope",
			mustExist:   false,
		}
	}
	return nil
}

func (q FIFOAdvanceConfig) validateDeduplicationScope() error {
	if q.DeduplicationScope != nil && !slices.Contains(validSQSDeduplicationScopeValues, aws.StringValue(q.DeduplicationScope)) {
		return fmt.Errorf(`validate "deduplication_scope": deduplication scope value must be one of %s`, english.WordSeries(validSQSDeduplicationScopeValues, "or"))
	}
	return nil
}

func (q FIFOAdvanceConfig) validateFIFOThroughputLimit() error {
	if q.FIFOThroughputLimit != nil && !slices.Contains(validSQSFIFOThroughputLimitValues, aws.StringValue(q.FIFOThroughputLimit)) {
		return fmt.Errorf(`validate "throughput_limit": fifo throughput limit value must be one of %s`, english.WordSeries(validSQSFIFOThroughputLimitValues, "or"))
	}
	return nil
}

// validate returns nil if DeadLetterQueue is configured correctly.
func (d DeadLetterQueue) validate() error {
	if d.IsEmpty() {
		return nil
	}
	return nil
}

// validate returns nil if OverrideRule is configured correctly.
func (r OverrideRule) validate() error {
	for _, s := range invalidTaskDefOverridePathRegexp {
		re := regexp.MustCompile(fmt.Sprintf(`^%s$`, s))
		if re.MatchString(r.Path) {
			return fmt.Errorf(`"%s" cannot be overridden with a custom value`, s)
		}
	}
	return nil
}

// validate returns nil if Variable is configured correctly.
func (v Variable) validate() error {
	if err := v.FromCFN.validate(); err != nil {
		return fmt.Errorf(`validate "from_cfn": %w`, err)
	}
	return nil
}

// validate returns nil if StringOrFromCFN is configured correctly.
func (s StringOrFromCFN) validate() error {
	if s.isEmpty() {
		return nil
	}
	return s.FromCFN.validate()
}

// validate returns nil if fromCFN is configured correctly.
func (cfg fromCFN) validate() error {
	if cfg.isEmpty() {
		return nil
	}
	if len(aws.StringValue(cfg.Name)) == 0 {
		return errors.New("name cannot be an empty string")
	}
	return nil
}

// validate is a no-op for Secrets.
func (s Secret) validate() error {
	return nil
}

type validateHealthCheckPortsOpts struct {
	exposedPorts      ExposedPortsIndex
	mainContainerPort *uint16
	alb               HTTP
	nlb               NetworkLoadBalancerConfiguration
}

type validateExposedPortsOpts struct {
	mainContainerName string
	mainContainerPort *uint16
	alb               *HTTP
	nlb               *NetworkLoadBalancerConfiguration
	sidecarConfig     map[string]*SidecarConfig
}

type validateDependenciesOpts struct {
	mainContainerName string
	sidecarConfig     map[string]*SidecarConfig
	imageConfig       Image
	logging           Logging
}

type validateTargetContainerOpts struct {
	mainContainerName string
	mainContainerPort *uint16
	targetContainer   *string
	sidecarConfig     map[string]*SidecarConfig
}

type validateWindowsOpts struct {
	readOnlyFS *bool
	efsVolumes map[string]*Volume
}

type validateARMOpts struct {
	Spot     *int
	SpotFrom *int
}

func validateHealthCheckPorts(opts validateHealthCheckPortsOpts) error {
	for _, rule := range opts.alb.RoutingRules() {
		healthCheckPort := rule.HealthCheckPort(opts.mainContainerPort)
		if err := validateHealthCheckPort(healthCheckPort, opts.exposedPorts); err != nil {
			return err
		}
	}

	for _, listener := range opts.nlb.NLBListeners() {
		healthCheckPort, err := listener.HealthCheckPort(opts.mainContainerPort)
		if err != nil {
			return err
		}
		if err := validateHealthCheckPort(healthCheckPort, opts.exposedPorts); err != nil {
			return err
		}
	}
	return nil
}

func validateHealthCheckPort(port uint16, ports ExposedPortsIndex) error {
	container := ports.ContainerForPort[port]
	containerPorts := ports.PortsForContainer[container]
	for _, exposedPort := range containerPorts {
		if exposedPort.Port != port {
			continue
		}

		if !slices.Contains(validHealthCheckProtocols, strings.ToUpper(exposedPort.Protocol)) {
			return &errHealthCheckPortExposedWithInvalidProtocol{
				healthCheckPort: port,
				container:       container,
				protocol:        exposedPort.Protocol,
			}
		}
	}
	return nil
}

func validateTargetContainer(opts validateTargetContainerOpts) error {
	if opts.targetContainer == nil {
		return nil
	}
	targetContainer := aws.StringValue(opts.targetContainer)
	if targetContainer == opts.mainContainerName {
		if opts.mainContainerPort == nil {
			return fmt.Errorf("target container %q doesn't expose a port", targetContainer)
		}
		return nil
	}
	sidecar, ok := opts.sidecarConfig[targetContainer]
	if !ok {
		return fmt.Errorf("target container %q doesn't exist", targetContainer)
	}
	if sidecar.Port == nil {
		return fmt.Errorf("target container %q doesn't expose a port", targetContainer)
	}
	return nil
}

func validateContainerDeps(opts validateDependenciesOpts) error {
	containerDependencies := containerDependencies(opts.mainContainerName, opts.imageConfig, opts.logging, opts.sidecarConfig)
	if err := validateDepsForEssentialContainers(containerDependencies); err != nil {
		return err
	}
	return validateNoCircularDependencies(containerDependencies)
}

func validateDepsForEssentialContainers(deps map[string]ContainerDependency) error {
	for name, containerDep := range deps {
		for dep, status := range containerDep.DependsOn {
			if !deps[dep].IsEssential {
				continue
			}
			if err := validateEssentialContainerDependency(dep, strings.ToUpper(status)); err != nil {
				return fmt.Errorf("validate %s container dependencies status: %w", name, err)
			}
		}
	}
	return nil
}

type containerNameAndProtocol struct {
	containerName     string
	containerProtocol string
}

func validateExposedPorts(opts validateExposedPortsOpts) error {
	portExposedTo := make(map[uint16]containerNameAndProtocol)

	if err := validateAndPopulateSidecarContainerPorts(portExposedTo, opts); err != nil {
		return err
	}
	if err := validateAndPopulateALBPorts(portExposedTo, opts); err != nil {
		return err
	}
	if err := validateAndPopulateNLBPorts(portExposedTo, opts); err != nil {
		return err
	}
	if err := validateAndPopulateMainContainerPort(portExposedTo, opts); err != nil {
		return err
	}
	return nil
}

func validateAndPopulateMainContainerPort(portExposedTo map[uint16]containerNameAndProtocol, opts validateExposedPortsOpts) error {
	if opts.mainContainerPort == nil {
		return nil
	}

	targetPort := aws.Uint16Value(opts.mainContainerPort)
	targetProtocol := defaultProtocol
	if existingContainerNameAndProtocol, ok := portExposedTo[targetPort]; ok {
		targetProtocol = existingContainerNameAndProtocol.containerProtocol
	}

	return validateAndPopulateExposedPortMapping(portExposedTo, targetPort, targetProtocol, opts.mainContainerName)
}

func validateAndPopulateSidecarContainerPorts(portExposedTo map[uint16]containerNameAndProtocol, opts validateExposedPortsOpts) error {
	for name, sidecar := range opts.sidecarConfig {
		if sidecar.Port == nil {
			continue
		}
		sidecarPort, sidecarProtocol, err := ParsePortMapping(sidecar.Port)
		if err != nil {
			return err
		}
		parsedPort, err := strconv.ParseUint(aws.StringValue(sidecarPort), 10, 16)
		if err != nil {
			return err
		}
		protocol := defaultProtocol
		if sidecarProtocol != nil {
			protocol = aws.StringValue(sidecarProtocol)
		}

		if err = validateAndPopulateExposedPortMapping(portExposedTo, uint16(parsedPort), protocol, name); err != nil {
			return err
		}
	}
	return nil
}

func validateAndPopulateALBPorts(portExposedTo map[uint16]containerNameAndProtocol, opts validateExposedPortsOpts) error {
	if opts.alb == nil || opts.alb.IsEmpty() {
		return nil
	}

	alb := opts.alb
	for _, rule := range alb.RoutingRules() {
		if rule.TargetPort == nil {
			continue
		}
		targetPort := aws.Uint16Value(rule.TargetPort)

		// Prefer `http.target_container`, then existing exposed port mapping, then fallback on name of main container
		targetContainer := opts.mainContainerName
		if existingContainerNameAndProtocol, ok := portExposedTo[targetPort]; ok {
			targetContainer = existingContainerNameAndProtocol.containerName
		}
		if rule.TargetContainer != nil {
			targetContainer = aws.StringValue(rule.TargetContainer)
		}

		if err := validateAndPopulateExposedPortMapping(portExposedTo, targetPort, TCP, targetContainer); err != nil {
			return err
		}
	}
	return nil
}

func validateAndPopulateNLBPorts(portExposedTo map[uint16]containerNameAndProtocol, opts validateExposedPortsOpts) error {
	if opts.nlb == nil || opts.nlb.IsEmpty() {
		return nil
	}

	nlb := opts.nlb
	if err := validateAndPopulateNLBListenerPorts(nlb.Listener, portExposedTo, opts.mainContainerName); err != nil {
		return fmt.Errorf(`validate "nlb": %w`, err)
	}

	for idx, listener := range nlb.AdditionalListeners {
		if err := validateAndPopulateNLBListenerPorts(listener, portExposedTo, opts.mainContainerName); err != nil {
			return fmt.Errorf(`validate "nlb.additional_listeners[%d]": %w`, idx, err)
		}
	}
	return nil
}

func validateAndPopulateNLBListenerPorts(listener NetworkLoadBalancerListener, portExposedTo map[uint16]containerNameAndProtocol, mainContainerName string) error {
	nlbReceiverPort, nlbProtocol, err := ParsePortMapping(listener.Port)
	if err != nil {
		return err
	}

	port, err := strconv.ParseUint(aws.StringValue(nlbReceiverPort), 10, 16)
	if err != nil {
		return err
	}

	targetPort := uint16(port)
	if listener.TargetPort != nil {
		targetPort = uint16(aws.IntValue(listener.TargetPort))
	}

	// Prefer `nlb.port`, then fallback on default protocol
	targetProtocol := defaultProtocol
	if nlbProtocol != nil {
		targetProtocol = strings.ToUpper(aws.StringValue(nlbProtocol))
	}

	// Handle TLS termination of container exposed port protocol
	if targetProtocol == TLS {
		targetProtocol = TCP
	}

	// Prefer `nlb.target_container`, then existing exposed port mapping, then fallback on name of main container
	targetContainer := mainContainerName
	if existingContainerNameAndProtocol, ok := portExposedTo[targetPort]; ok {
		targetContainer = existingContainerNameAndProtocol.containerName
	}
	if listener.TargetContainer != nil {
		targetContainer = aws.StringValue(listener.TargetContainer)
	}

	return validateAndPopulateExposedPortMapping(portExposedTo, targetPort, targetProtocol, targetContainer)
}

func validateAndPopulateExposedPortMapping(portExposedTo map[uint16]containerNameAndProtocol, targetPort uint16, targetProtocol string, targetContainer string) error {
	exposedContainerAndProtocol, alreadyExposed := portExposedTo[targetPort]
	targetProtocol = strings.ToUpper(targetProtocol)

	// Port is not associated with container and protocol, populate map
	if !alreadyExposed {
		portExposedTo[targetPort] = containerNameAndProtocol{
			containerName:     targetContainer,
			containerProtocol: targetProtocol,
		}
		return nil
	}

	exposedContainer := exposedContainerAndProtocol.containerName
	exposedProtocol := exposedContainerAndProtocol.containerProtocol
	if exposedContainer != targetContainer {
		return &errContainersExposingSamePort{
			firstContainer:  targetContainer,
			secondContainer: exposedContainer,
			port:            targetPort,
		}
	}
	if exposedProtocol != targetProtocol {
		return &errContainerPortExposedWithMultipleProtocol{
			container:      exposedContainer,
			port:           targetPort,
			firstProtocol:  targetProtocol,
			secondProtocol: exposedProtocol,
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

func validateNoCircularDependencies(deps map[string]ContainerDependency) error {
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
	// Stabilize unit tests.
	sort.SliceStable(cycle, func(i, j int) bool { return cycle[i] < cycle[j] })
	return fmt.Errorf("circular container dependency chain includes the following containers: %s", cycle)
}

func buildDependencyGraph(deps map[string]ContainerDependency) (*graph.Graph[string], error) {
	dependencyGraph := graph.New[string]()
	for name, containerDep := range deps {
		for dep := range containerDep.DependsOn {
			if _, ok := deps[dep]; !ok {
				return nil, fmt.Errorf("container %s does not exist", dep)
			}
			dependencyGraph.Add(graph.Edge[string]{
				From: name,
				To:   dep,
			})
		}
	}
	return dependencyGraph, nil
}

// validate that paths contain only an approved set of characters to guard against command injection.
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
	// Name must contain letters, numbers, and can't use special characters besides underscores, and hyphens.
	if !awsSNSTopicRegexp.MatchString(name) {
		return fmt.Errorf(`"name" can only contain letters, numbers, underscores, and hyphens`)
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
	if aws.BoolValue(opts.readOnlyFS) {
		return fmt.Errorf(`%q can not be set to 'true' when deploying a Windows container`, "readonly_fs")
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

// validate returns nil if ImageLocationOrBuild is configured correctly.
func (i ImageLocationOrBuild) validate() error {
	if err := i.Build.validate(); err != nil {
		return fmt.Errorf(`validate "build": %w`, err)
	}
	if i.Build.isEmpty() == (i.Location == nil) {
		return &errFieldMutualExclusive{
			firstField:  "build",
			secondField: "location",
			mustExist:   true,
		}
	}
	return nil
}

func (r *RoutingRule) validateConditionValuesPerRule() error {
	aliases, err := r.Alias.ToStringSlice()
	if err != nil {
		return fmt.Errorf("convert aliases to string slice: %w", err)
	}
	allowedSourceIps := make([]string, len(r.AllowedSourceIps))
	for idx, ip := range r.AllowedSourceIps {
		allowedSourceIps[idx] = string(ip)
	}
	if len(aliases)+len(allowedSourceIps) >= maxConditionsPerRule {
		return &errMaxConditionValuesPerRule{
			path:             aws.StringValue(r.Path),
			aliases:          aliases,
			allowedSourceIps: allowedSourceIps,
		}
	}
	return nil
}

type errMaxConditionValuesPerRule struct {
	path             string
	aliases          []string
	allowedSourceIps []string
}

func (e *errMaxConditionValuesPerRule) Error() string {
	return fmt.Sprintf("listener rule has more than five conditions %s %s", english.WordSeries(e.aliases, "and"),
		english.WordSeries(e.allowedSourceIps, "and"))
}

func (e *errMaxConditionValuesPerRule) RecommendActions() string {
	cgList := e.generateConditionGroups()
	var fmtListenerRules strings.Builder
	fmtListenerRules.WriteString(fmt.Sprintf(`http:
  path: %s
  alias: %s
  allowed_source_ips: %s
  additional_rules:`, e.path, fmtStringArray(cgList[0].aliases), fmtStringArray(cgList[0].allowedSourceIps)))
	for i := 1; i < len(cgList); i++ {
		fmtListenerRules.WriteString(fmt.Sprintf(`
    - path: %s
      alias: %s
      allowed_source_ips: %s`, e.path, fmtStringArray(cgList[i].aliases), fmtStringArray(cgList[i].allowedSourceIps)))
	}
	return fmt.Sprintf(`You can split the "alias" and "allowed_source_ips" field into separate rules, so that each rule contains up to 5 values: 
%s`, color.HighlightCodeBlock(fmtListenerRules.String()))
}

func fmtStringArray(arr []string) string {
	return fmt.Sprintf("[%s]", strings.Join(arr, ","))
}

// conditionGroup represents groups of conditions per listener rule.
type conditionGroup struct {
	allowedSourceIps []string
	aliases          []string
}

func (e *errMaxConditionValuesPerRule) generateConditionGroups() []conditionGroup {
	remaining := calculateRemainingConditions(e.path)
	if len(e.aliases) != 0 && len(e.allowedSourceIps) != 0 {
		return e.generateConditionsWithSourceIPsAndAlias(remaining)
	}
	if len(e.aliases) != 0 {
		return e.generateConditionsWithAliasOnly(remaining)
	}
	return e.generateConditionWithSourceIPsOnly(remaining)
}

func calculateRemainingConditions(path string) int {
	rcPerRule := maxConditionsPerRule
	if path != rootPath {
		return rcPerRule - 2
	}
	return rcPerRule - 1
}

func (e *errMaxConditionValuesPerRule) generateConditionsWithSourceIPsAndAlias(remaining int) []conditionGroup {
	var groups []conditionGroup
	for i := 0; i < len(e.allowedSourceIps); i++ {
		var group conditionGroup
		group.allowedSourceIps = []string{e.allowedSourceIps[i]}
		groups = append(groups, e.generateConditionsGroups(remaining-1, true, group)...)
	}
	return groups
}

func (e *errMaxConditionValuesPerRule) generateConditionsWithAliasOnly(remaining int) []conditionGroup {
	var group conditionGroup
	return e.generateConditionsGroups(remaining, true, group)
}

func (e *errMaxConditionValuesPerRule) generateConditionWithSourceIPsOnly(remaining int) []conditionGroup {
	var group conditionGroup
	return e.generateConditionsGroups(remaining, false, group)
}

func (e *errMaxConditionValuesPerRule) generateConditionsGroups(remaining int, isAlias bool, group conditionGroup) []conditionGroup {
	var groups []conditionGroup
	var conditions []string
	if isAlias {
		conditions = e.aliases
	} else {
		conditions = e.allowedSourceIps
	}
	for i := 0; i < len(conditions); i += remaining {
		end := i + remaining
		if end > len(conditions) {
			end = len(conditions)
		}
		if isAlias {
			group.aliases = conditions[i:end]
			groups = append(groups, group)
			continue
		}
		group.allowedSourceIps = conditions[i:end]
		groups = append(groups, group)
	}
	return groups
}
