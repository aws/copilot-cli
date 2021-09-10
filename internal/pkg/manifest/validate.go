// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
)

var (
	intRangeBandRegexp = regexp.MustCompile(`^(\d+)-(\d+)$`)
)

// Validate returns if LoadBalancedWebServiceConfig is configured correctly.
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
			return fmt.Errorf(`validate sidecars[%s]: %w`, k, err)
		}
	}
	if err = l.Network.Validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err = l.PublishConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "publish": %w`, err)
	}
	for i := 0; i < len(l.TaskDefOverrides); i++ {
		if err = l.TaskDefOverrides[i].Validate(); err != nil {
			return fmt.Errorf(`validate taskdef_overrides[%d]: %w`, i, err)
		}
	}
	return nil
}

// Validate returns if ImageWithPortAndHealthcheck is configured correctly.
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

// Validate returns if ImageWithPort is configured correctly.
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

// Validate returns if Image is configured correctly.
func (i *Image) Validate() error {
	var err error
	if !i.Build.isEmpty() {
		if err = i.Build.Validate(); err != nil {
			return fmt.Errorf(`validate "build": %w`, err)
		}
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

// Validate returns if BuildArgsOrString is configured correctly.
func (b *BuildArgsOrString) Validate() error {
	return b.BuildArgs.Validate()
}

// Validate returns if DockerBuildArgs is configured correctly.
func (*DockerBuildArgs) Validate() error {
	return nil
}

// Validate returns if ContainerHealthCheck is configured correctly.
func (*ContainerHealthCheck) Validate() error {
	return nil
}

// Validate returns if ImageOverride is configured correctly.
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

// Validate returns if EntryPointOverride is configured correctly.
func (*EntryPointOverride) Validate() error {
	return nil
}

// Validate returns if CommandOverride is configured correctly.
func (*CommandOverride) Validate() error {
	return nil
}

// Validate returns if RoutingRule is configured correctly.
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
	return nil
}

// Validate returns if HealthCheckArgsOrString is configured correctly.
func (h *HealthCheckArgsOrString) Validate() error {
	return h.HealthCheckArgs.Validate()
}

// Validate returns if HTTPHealthCheckArgs is configured correctly.
func (h *HTTPHealthCheckArgs) Validate() error {
	return nil
}

// Validate returns if Alias is configured correctly.
func (a *Alias) Validate() error {
	return nil
}

// Validate returns if TaskConfig is configured correctly.
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

// Validate returns if PlatformArgsOrString is configured correctly.
func (p *PlatformArgsOrString) Validate() error {
	return p.PlatformArgs.Validate()
}

// Validate returns if PlatformArgsOrString is configured correctly.
// TODO: add validation once "feat/pencere" is merged.
func (*PlatformArgs) Validate() error {
	return nil
}

// Validate returns if Count is configured correctly.
func (c *Count) Validate() error {
	if c.Value != nil {
		return nil
	}
	return c.AdvancedCount.Validate()
}

// Validate returns if AdvancedCount is configured correctly.
func (a *AdvancedCount) Validate() error {
	if a.Spot != nil && a.hasAutoscaling() {
		return &errFieldMutualExclusive{
			firstField:  "spot",
			secondField: "range/cpu_percentage/memory_percentage/requests/response_time",
		}
	}
	if !a.Range.IsEmpty() {
		if err := a.Range.Validate(); err != nil {
			return fmt.Errorf(`validate "range": %w`, err)
		}
	}
	if a.CPU != nil || a.Memory != nil || a.Requests != nil || a.ResponseTime != nil {
		return &errFieldMustBeSpecified{
			missingField:      "range",
			conditionalFields: []string{"cpu_percentage", "memory_percentage", "requests", "response_time"},
		}
	}
	return nil
}

// Validate returns if Range is configured correctly.
func (r *Range) Validate() error {
	if r.Value != nil {
		return r.Value.Validate()
	}
	return r.RangeConfig.Validate()
}

// Validate returns if IntRangeBand is configured correctly.
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

// Validate returns if RangeConfig is configured correctly.
func (r *RangeConfig) Validate() error {
	min, max := aws.IntValue(r.Min), aws.IntValue(r.Max)
	if min <= max {
		return nil
	}
	return &errMinGreaterThanMax{
		min: min,
		max: max,
	}
}

// Validate returns if ExecuteCommand is configured correctly.
func (e *ExecuteCommand) Validate() error {
	if e.Enable != nil {
		return nil
	}
	return e.Config.Validate()
}

// Validate returns if ExecuteCommandConfig is configured correctly.
func (*ExecuteCommandConfig) Validate() error {
	return nil
}

// Validate returns if Storage is configured correctly.
func (s *Storage) Validate() error {
	for k, v := range s.Volumes {
		if err := v.Validate(); err != nil {
			return fmt.Errorf(`validate "volumes[%s]": %w`, k, err)
		}
	}
	return nil
}

// Validate returns if Volume is configured correctly.
func (v *Volume) Validate() error {
	if err := v.EFS.Validate(); err != nil {
		return fmt.Errorf(`validate "efs": %w`, err)
	}
	return v.MountPointOpts.Validate()
}

// Validate returns if MountPointOpts is configured correctly.
func (e *MountPointOpts) Validate() error {
	return nil
}

// Validate returns if EFSConfigOrBool is configured correctly.
func (e *EFSConfigOrBool) Validate() error {
	if e.Enabled != nil {
		return nil
	}
	return e.Advanced.Validate()
}

// Validate returns if EFSVolumeConfiguration is configured correctly.
func (e *EFSVolumeConfiguration) Validate() error {
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
	if err := e.AuthConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "auth": %w`, err)
	}
	if aws.StringValue(e.AuthConfig.AccessPointID) != "" {
		if (aws.StringValue(e.RootDirectory) == "" || aws.StringValue(e.RootDirectory) == "/") &&
			aws.BoolValue(e.AuthConfig.IAM) {
			return nil
		}
		return fmt.Errorf("root_dir must be either empty or / and auth.iam must be true when access_point_id is in used")
	}
	return nil
}

// Validate returns if AuthorizationConfig is configured correctly.
func (*AuthorizationConfig) Validate() error {
	return nil
}

// Validate returns if Logging is configured correctly.
func (*Logging) Validate() error {
	return nil
}

// Validate returns if SidecarConfig is configured correctly.
func (s *SidecarConfig) Validate() error {
	for ind, mp := range s.MountPoints {
		if err := mp.Validate(); err != nil {
			return fmt.Errorf(`validate "mount_points[%d]: %w`, ind, err)
		}
	}
	return s.ImageOverride.Validate()
}

// Validate returns if NetworkConfig is configured correctly.
func (n *NetworkConfig) Validate() error {
	if err := n.VPC.Validate(); err != nil {
		return fmt.Errorf(`validate "vpc": %w`, err)
	}
	return nil
}

// Validate returns if vpcConfig is configured correctly.
func (*vpcConfig) Validate() error {
	return nil
}

// Validate returns if PublishConfig is configured correctly.
func (p *PublishConfig) Validate() error {
	for ind, topic := range p.Topics {
		if err := topic.Validate(); err != nil {
			return fmt.Errorf(`validate "topics[%d]: %w`, ind, err)
		}
	}
	return nil
}

// Validate returns if Topic is configured correctly.
func (*Topic) Validate() error {
	return nil
}

// Validate returns if OverrideRule is configured correctly.
func (o *OverrideRule) Validate() error {
	return nil
}
