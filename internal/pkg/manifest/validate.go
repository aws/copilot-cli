// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"net"
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

	// In docker containers, max path length is 242.
	// https://github.com/moby/moby/issues/1413
	maxDockerContainerPathLength = 242
	// Max path length in EFS is 255 bytes.
	// https://docs.aws.amazon.com/efs/latest/ug/troubleshooting-efs-fileop-errors.html#filenametoolong
	maxEFSPathLength = 255
)

var (
	intRangeBandRegexp = regexp.MustCompile(`^(\d+)-(\d+)$`)
	pathRegexp         = regexp.MustCompile(`^[a-zA-Z0-9\-\.\_/]+$`)

	essentialContainerDependsOnValidStatuses = []string{dependsOnStart, dependsOnHealthy}
	dependsOnValidStatuses                   = []string{dependsOnStart, dependsOnComplete, dependsOnSuccess, dependsOnHealthy}

	invalidTaskDefOverridePathRegexp = []string{`Family`, `ContainerDefinitions\[\d+\].Name`}
)

// Validate returns nil if LoadBalancedWebServiceConfig is configured correctly.
func (l *LoadBalancedWebServiceConfig) Validate() error {
	var err error
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
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     l.Sidecars,
		imageConfig:       l.ImageConfig.Image,
		mainContainerName: l.name,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	return nil
}

// Validate returns nil if BackendServiceConfig is configured correctly.
func (b *BackendServiceConfig) Validate() error {
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
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     b.Sidecars,
		imageConfig:       b.ImageConfig.Image,
		mainContainerName: b.name,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	return nil
}

// Validate returns nil if RequestDrivenWebService is configured correctly.
func (r *RequestDrivenWebServiceConfig) Validate() error {
	var err error
	if err = r.ImageConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "image": %w`, err)
	}
	if err = r.RequestDrivenWebServiceHttpConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "http": %w`, err)
	}
	if err = r.PublishConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	return nil
}

// Validate returns nil if WorkerServiceConfig is configured correctly.
func (w *WorkerServiceConfig) Validate() error {
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
	for ind, taskDefOverride := range w.TaskDefOverrides {
		if err = taskDefOverride.Validate(); err != nil {
			return fmt.Errorf(`validate "taskdef_overrides[%d]": %w`, ind, err)
		}
	}
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     w.Sidecars,
		imageConfig:       w.ImageConfig.Image,
		mainContainerName: w.name,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	return nil
}

// Validate returns nil if ScheduledJobConfig is configured correctly.
func (s *ScheduledJobConfig) Validate() error {
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
	if err = validateContainerDeps(validateDependenciesOpts{
		sidecarConfig:     s.Sidecars,
		imageConfig:       s.ImageConfig.Image,
		mainContainerName: s.name,
	}); err != nil {
		return fmt.Errorf("validate container dependencies: %w", err)
	}
	return nil
}

// Validate returns nil if ImageWithPortAndHealthcheck is configured correctly.
func (i *ImageWithPortAndHealthcheck) Validate() error {
	var err error
	if err = i.ImageWithPort.Validate(); err != nil {
		return err
	}
	if err = i.HealthCheck.Validate(); err != nil {
		return fmt.Errorf(`validate "healthcheck": %w`, err)
	}
	return nil
}

// Validate returns nil if ImageWithHealthcheck is configured correctly.
func (i *ImageWithHealthcheck) Validate() error {
	if err := i.Image.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate returns nil if ImageWithPort is configured correctly.
func (i *ImageWithPort) Validate() error {
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
func (i *Image) Validate() error {
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
func (d *DependsOn) Validate() error {
	if d == nil {
		return nil
	}
	for _, v := range *d {
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
func (b *BuildArgsOrString) Validate() error {
	if b.isEmpty() {
		return nil
	}
	if !b.BuildArgs.isEmpty() {
		return b.BuildArgs.Validate()
	}
	return nil
}

// Validate returns nil if DockerBuildArgs is configured correctly.
func (*DockerBuildArgs) Validate() error {
	return nil
}

// Validate returns nil if ContainerHealthCheck is configured correctly.
func (*ContainerHealthCheck) Validate() error {
	return nil
}

// Validate returns nil if ImageOverride is configured correctly.
func (i *ImageOverride) Validate() error {
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
func (*EntryPointOverride) Validate() error {
	return nil
}

// Validate returns nil if CommandOverride is configured correctly.
func (*CommandOverride) Validate() error {
	return nil
}

// Validate returns nil if RoutingRule is configured correctly.
func (r *RoutingRule) Validate() error {
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

	return nil
}

// Validate returns nil if HealthCheckArgsOrString is configured correctly.
func (h *HealthCheckArgsOrString) Validate() error {
	if h.IsEmpty() {
		return nil
	}
	return h.HealthCheckArgs.Validate()
}

// Validate returns nil if HTTPHealthCheckArgs is configured correctly.
func (h *HTTPHealthCheckArgs) Validate() error {
	if h.isEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if Alias is configured correctly.
func (*Alias) Validate() error {
	return nil
}

// Validate returns nil if IPNet is configured correctly.
func (ip *IPNet) Validate() error {
	if ip == nil {
		return nil
	}
	if _, _, err := net.ParseCIDR(string(*ip)); err != nil {
		return fmt.Errorf("parse IPNet %s: %w", string(*ip), err)
	}
	return nil
}

// Validate returns nil if TaskConfig is configured correctly.
func (t *TaskConfig) Validate() error {
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
	return nil
}

// Validate returns nil if PlatformArgsOrString is configured correctly.
func (p *PlatformArgsOrString) Validate() error {
	if err := p.PlatformString.Validate(); err != nil {
		return err
	}
	return p.PlatformArgs.Validate()
}

// Validate returns nil if PlatformString is configured correctly.
func (p *PlatformString) Validate() error {
	if p == nil {
		return nil
	}
	if err := validatePlatform(p); err != nil {
		return err
	}
	return nil
}

// Validate returns nil if PlatformArgsOrString is configured correctly.
func (p *PlatformArgs) Validate() error {
	if p.isEmpty() {
		return nil
	}
	if !p.bothSpecified() {
		return errors.New(`fields "osfamily" and "architecture" must either both be specified or both be empty`)
	}
	if err := validateOS(p.OSFamily); err != nil {
		return err
	}
	if err := validateArch(p.Arch); err != nil {
		return err
	}
	return nil
}

// Validate returns nil if Count is configured correctly.
func (c *Count) Validate() error {
	return c.AdvancedCount.Validate()
}

// Validate returns nil if AdvancedCount is configured correctly.
func (a *AdvancedCount) Validate() error {
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

	if err := a.CPU.Validate(); err != nil {
		return fmt.Errorf(`validate "cpu_percentage": %w`, err)
	}
	if err := a.Memory.Validate(); err != nil {
		return fmt.Errorf(`validate "memory_percentage": %w`, err)
	}
	return nil
}

// Validate returns nil if Percentage is configured correctly.
func (p *Percentage) Validate() error {
	if p == nil {
		return nil
	}
	if val := int(*p); val < 0 || val > 100 {
		return fmt.Errorf("percentage value %v must be an integer from 0 to 100", val)
	}
	return nil
}

// Validate returns nil if QueueScaling is configured correctly.
func (qs *QueueScaling) Validate() error {
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
func (r *Range) Validate() error {
	if r.IsEmpty() {
		return nil
	}
	if !r.RangeConfig.IsEmpty() {
		return r.RangeConfig.Validate()
	}
	return r.Value.Validate()
}

// Validate returns nil if IntRangeBand is configured correctly.
func (r *IntRangeBand) Validate() error {
	str := string(*r)
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
func (r *RangeConfig) Validate() error {
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
func (e *ExecuteCommand) Validate() error {
	if !e.Config.IsEmpty() {
		return e.Config.Validate()
	}
	return nil
}

// Validate returns nil if ExecuteCommandConfig is configured correctly.
func (*ExecuteCommandConfig) Validate() error {
	return nil
}

// Validate returns nil if Storage is configured correctly.
func (s *Storage) Validate() error {
	if s.IsEmpty() {
		return nil
	}
	for k, v := range s.Volumes {
		if err := v.Validate(); err != nil {
			return fmt.Errorf(`validate "volumes[%s]": %w`, k, err)
		}
	}
	return nil
}

// Validate returns nil if Volume is configured correctly.
func (v *Volume) Validate() error {
	if err := v.EFS.Validate(); err != nil {
		return fmt.Errorf(`validate "efs": %w`, err)
	}
	return v.MountPointOpts.Validate()
}

// Validate returns nil if MountPointOpts is configured correctly.
func (m *MountPointOpts) Validate() error {
	if m == nil {
		return nil
	}
	path := aws.StringValue(m.ContainerPath)
	if path == "" {
		return &errFieldMustBeSpecified{
			missingField: "path",
		}
	}
	if err := validatePath(path, maxDockerContainerPathLength); err != nil {
		return fmt.Errorf(`validate "path": %w`, err)
	}
	return nil
}

// Validate returns nil if EFSConfigOrBool is configured correctly.
func (e *EFSConfigOrBool) Validate() error {
	if e.IsEmpty() {
		return nil
	}
	return e.Advanced.Validate()
}

// Validate returns nil if EFSVolumeConfiguration is configured correctly.
func (e *EFSVolumeConfiguration) Validate() error {
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
		if err := validatePath(aws.StringValue(e.RootDirectory), maxEFSPathLength); err != nil {
			return fmt.Errorf(`validate "root_dir": %w`, err)
		}
	}
	return nil
}

// Validate returns nil if AuthorizationConfig is configured correctly.
func (a *AuthorizationConfig) Validate() error {
	if a.IsEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if Logging is configured correctly.
func (l *Logging) Validate() error {
	if l.IsEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if SidecarConfig is configured correctly.
func (s *SidecarConfig) Validate() error {
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
func (s *SidecarMountPoint) Validate() error {
	if s == nil {
		return nil
	}
	if aws.StringValue(s.SourceVolume) == "" {
		return &errFieldMustBeSpecified{
			missingField: "source_volume",
		}
	}
	return s.MountPointOpts.Validate()
}

// Validate returns nil if NetworkConfig is configured correctly.
func (n *NetworkConfig) Validate() error {
	if n.IsEmpty() {
		return nil
	}
	if err := n.VPC.Validate(); err != nil {
		return fmt.Errorf(`validate "vpc": %w`, err)
	}
	return nil
}

// Validate returns nil if vpcConfig is configured correctly.
func (v *vpcConfig) Validate() error {
	if v.isEmpty() {
		return nil
	}
	if err := v.Placement.Validate(); err != nil {
		return fmt.Errorf(`validate "placement": %w`, err)
	}
	return nil
}

// Validate returns nil if Placement is configured correctly.
func (p *Placement) Validate() error {
	if p == nil {
		return fmt.Errorf(`"placement" cannot be empty`)
	}
	for _, allowed := range subnetPlacements {
		if string(*p) == allowed {
			return nil
		}
	}
	return fmt.Errorf(`"placement" %s must be one of %s`, string(*p), strings.Join(subnetPlacements, ", "))
}

// Validate returns nil if RequestDrivenWebServiceHttpConfig is configured correctly.
func (r *RequestDrivenWebServiceHttpConfig) Validate() error {
	return r.HealthCheckConfiguration.Validate()
}

// Validate returns nil if JobTriggerConfig is configured correctly.
func (c *JobTriggerConfig) Validate() error {
	if c.Schedule == nil {
		return &errFieldMustBeSpecified{
			missingField: "schedule",
		}
	}
	return nil
}

// Validate returns nil if JobFailureHandlerConfig is configured correctly.
func (*JobFailureHandlerConfig) Validate() error {
	return nil
}

// Validate returns nil if PublishConfig is configured correctly.
func (p *PublishConfig) Validate() error {
	for ind, topic := range p.Topics {
		if err := topic.Validate(); err != nil {
			return fmt.Errorf(`validate "topics[%d]: %w`, ind, err)
		}
	}
	return nil
}

// Validate returns nil if Topic is configured correctly.
func (*Topic) Validate() error {
	return nil
}

// Validate returns nil if SubscribeConfig is configured correctly.
func (p *SubscribeConfig) Validate() error {
	for ind, topic := range p.Topics {
		if err := topic.Validate(); err != nil {
			return fmt.Errorf(`validate "topics[%d]: %w`, ind, err)
		}
	}
	return nil
}

// Validate returns nil if TopicSubscription is configured correctly.
func (t *TopicSubscription) Validate() error {
	if err := t.Queue.Validate(); err != nil {
		return fmt.Errorf(`validate "queue": %w`, err)
	}
	return nil
}

// Validate returns nil if SQSQueue is configured correctly.
func (s *SQSQueue) Validate() error {
	if err := s.DeadLetter.Validate(); err != nil {
		return fmt.Errorf(`validate "dead_letter": %w`, err)
	}
	return nil
}

// Validate returns nil if DeadLetterQueue is configured correctly.
func (d *DeadLetterQueue) Validate() error {
	if d.IsEmpty() {
		return nil
	}
	return nil
}

// Validate returns nil if OverrideRule is configured correctly.
func (r *OverrideRule) Validate() error {
	if r == nil {
		return nil
	}
	for _, s := range invalidTaskDefOverridePathRegexp {
		re := regexp.MustCompile(fmt.Sprintf(`^%s$`, s))
		if re.MatchString(r.Path) {
			return fmt.Errorf("%s cannot be overidden with a custom value", s)
		}
	}
	return nil
}

type validateDependenciesOpts struct {
	mainContainerName string
	sidecarConfig     map[string]*SidecarConfig
	imageConfig       Image
}

type containerDependency struct {
	dependsOn   DependsOn
	isEssential bool
}

func validateContainerDeps(opts validateDependenciesOpts) error {
	containerDependencies := make(map[string]containerDependency)
	containerDependencies[opts.mainContainerName] = containerDependency{
		dependsOn:   opts.imageConfig.DependsOn,
		isEssential: true,
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
	return validateNoCircularDependencies(opts)
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

func validateNoCircularDependencies(opts validateDependenciesOpts) error {
	dependencies, err := buildDependencyGraph(opts)
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

func buildDependencyGraph(opts validateDependenciesOpts) (*graph.Graph, error) {
	dependencyGraph := graph.New()
	// Add any sidecar dependencies.
	for name, sidecar := range opts.sidecarConfig {
		for dep := range sidecar.DependsOn {
			if _, ok := opts.sidecarConfig[dep]; !ok && dep != opts.mainContainerName {
				return nil, fmt.Errorf("container %s does not exist", dep)
			}
			dependencyGraph.Add(graph.Edge{
				From: name,
				To:   dep,
			})
		}
	}
	// Add any image dependencies.
	for dep := range opts.imageConfig.DependsOn {
		if _, ok := opts.sidecarConfig[dep]; !ok && dep != opts.mainContainerName {
			return nil, fmt.Errorf("container %s does not exist", dep)
		}
		dependencyGraph.Add(graph.Edge{
			From: opts.mainContainerName,
			To:   dep,
		})
	}
	return dependencyGraph, nil
}

// Validate that paths contain only an approved set of characters to guard against command injection.
// We can accept 0-9A-Za-z-_.
func validatePath(input string, maxLength int) error {
	if len(input) > maxLength {
		return fmt.Errorf("path must be less than %d bytes in length", maxLength)
	}
	if len(input) == 0 {
		return nil
	}
	m := pathRegexp.FindStringSubmatch(input)
	if len(m) == 0 {
		return fmt.Errorf("path can only contain the characters a-zA-Z0-9.-_/")
	}
	return nil
}
