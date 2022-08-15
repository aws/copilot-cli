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

// convertSidecar converts the manifest sidecar configuration into a format parsable by the templates pkg.
func convertSidecar(s map[string]*manifest.SidecarConfig) ([]*template.SidecarOpts, error) {
	if s == nil {
		return nil, nil
	}

	// Sort the sidecars so that the order is consistent and the integration test won't be flaky.
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sidecars []*template.SidecarOpts
	for _, name := range keys {
		config := s[name]
		port, protocol, err := manifest.ParsePortMapping(config.Port)
		if err != nil {
			return nil, err
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
			Name:       aws.String(name),
			Image:      config.Image,
			Essential:  config.Essential,
			Port:       port,
			Protocol:   protocol,
			CredsParam: config.CredsParam,
			Secrets:    convertSecrets(config.Secrets),
			Variables:  config.Variables,
			Storage: template.SidecarStorageOpts{
				MountPoints: mp,
			},
			DockerLabels: config.DockerLabels,
			DependsOn:    convertDependsOn(config.DependsOn),
			EntryPoint:   entrypoint,
			HealthCheck:  convertContainerHealthCheck(config.HealthCheck),
			Command:      command,
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

func convertHostedZone(m manifest.RoutingRuleConfiguration) (template.AliasesForHostedZone, error) {
	aliasesFor := make(map[string][]string)
	defaultHostedZone := m.HostedZone
	if len(m.Alias.AdvancedAliases) != 0 {
		for _, alias := range m.Alias.AdvancedAliases {
			if alias.HostedZone != nil {
				aliasesFor[*alias.HostedZone] = append(aliasesFor[*alias.HostedZone], *alias.Alias)
				continue
			}
			if defaultHostedZone != nil {
				aliasesFor[*defaultHostedZone] = append(aliasesFor[*defaultHostedZone], *alias.Alias)
			}
		}
		return aliasesFor, nil
	}
	if defaultHostedZone == nil {
		return aliasesFor, nil
	}
	aliases, err := m.Alias.ToStringSlice()
	if err != nil {
		return nil, err
	}
	aliasesFor[*defaultHostedZone] = aliases
	return aliasesFor, nil
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
	// Return if only spot is specifed as count
	if rc.SpotFrom == nil {
		return cps
	}
	// Scaling with spot
	spotFrom := aws.IntValue(rc.SpotFrom)
	min := aws.IntValue(rc.Min)
	// If spotFrom value is not equal to the autoscaling min, then
	// the base value on the Fargate Capacity provider must be set
	// to one less than spotFrom
	if spotFrom > min {
		base := spotFrom - 1
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
		HealthyThreshold:   hc.HealthCheckArgs.HealthyThreshold,
		UnhealthyThreshold: hc.HealthCheckArgs.UnhealthyThreshold,
		GracePeriod:        aws.Int64(manifest.DefaultHealthCheckGracePeriod),
	}
	if hc.HealthCheckArgs.Path != nil {
		opts.HealthCheckPath = *hc.HealthCheckArgs.Path
	} else if hc.HealthCheckPath != nil {
		opts.HealthCheckPath = *hc.HealthCheckPath
	}
	if hc.HealthCheckArgs.Port != nil {
		opts.Port = strconv.Itoa(aws.IntValue(hc.HealthCheckArgs.Port))
	}
	if hc.HealthCheckArgs.SuccessCodes != nil {
		opts.SuccessCodes = *hc.HealthCheckArgs.SuccessCodes
	}
	if hc.HealthCheckArgs.Interval != nil {
		opts.Interval = aws.Int64(int64(hc.HealthCheckArgs.Interval.Seconds()))
	}
	if hc.HealthCheckArgs.Timeout != nil {
		opts.Timeout = aws.Int64(int64(hc.HealthCheckArgs.Timeout.Seconds()))
	}
	if hc.HealthCheckArgs.GracePeriod != nil {
		opts.GracePeriod = aws.Int64(int64(hc.HealthCheckArgs.GracePeriod.Seconds()))
	}
	return opts
}

type networkLoadBalancerConfig struct {
	settings *template.NetworkLoadBalancer

	// If a domain is associated these values are not empty.
	appDNSDelegationRole *string
	appDNSName           *string
}

func convertELBAccessLogsConfig(mft *manifest.Environment) (*template.ELBAccessLogs, error) {
	elbAccessLogsArgs, isELBAccessLogsSet := mft.ELBAccessLogs()
	if !isELBAccessLogsSet {
		return nil, nil
	}

	if elbAccessLogsArgs == nil {
		return &template.ELBAccessLogs{}, nil
	}

	return &template.ELBAccessLogs{
		BucketName: aws.StringValue(elbAccessLogsArgs.BucketName),
		Prefix:     aws.StringValue(elbAccessLogsArgs.Prefix),
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

func (s *LoadBalancedWebService) convertNetworkLoadBalancer() (networkLoadBalancerConfig, error) {
	nlbConfig := s.manifest.NLBConfig
	if nlbConfig.IsEmpty() {
		return networkLoadBalancerConfig{}, nil
	}

	// Parse listener port and protocol.
	port, protocol, err := manifest.ParsePortMapping(nlbConfig.Port)
	if err != nil {
		return networkLoadBalancerConfig{}, err
	}
	if protocol == nil {
		protocol = aws.String(defaultNLBProtocol)
	}

	// Configure target container and port.
	targetContainer := s.name
	if nlbConfig.TargetContainer != nil {
		targetContainer = aws.StringValue(nlbConfig.TargetContainer)
	}

	// By default, the target port is the same as listener port.
	targetPort := aws.StringValue(port)
	if targetContainer != s.name {
		// If the target container is a sidecar container, the target port is the exposed sidecar port.
		sideCarPort := s.manifest.Sidecars[targetContainer].Port // We validated that a sidecar container exposes a port if it is a target container.
		port, _, err := manifest.ParsePortMapping(sideCarPort)
		if err != nil {
			return networkLoadBalancerConfig{}, err
		}
		targetPort = aws.StringValue(port)
	}
	// Finally, if a target port is explicitly specified, use that value.
	if nlbConfig.TargetPort != nil {
		targetPort = strconv.Itoa(aws.IntValue(nlbConfig.TargetPort))
	}

	aliases, err := convertAlias(nlbConfig.Aliases)
	if err != nil {
		return networkLoadBalancerConfig{}, fmt.Errorf(`convert "nlb.alias" to string slice: %w`, err)
	}

	hc := template.NLBHealthCheck{
		HealthyThreshold:   nlbConfig.HealthCheck.HealthyThreshold,
		UnhealthyThreshold: nlbConfig.HealthCheck.UnhealthyThreshold,
	}
	if nlbConfig.HealthCheck.Port != nil {
		hc.Port = strconv.Itoa(aws.IntValue(nlbConfig.HealthCheck.Port))
	}
	if nlbConfig.HealthCheck.Timeout != nil {
		hc.Timeout = aws.Int64(int64(nlbConfig.HealthCheck.Timeout.Seconds()))
	}
	if nlbConfig.HealthCheck.Interval != nil {
		hc.Interval = aws.Int64(int64(nlbConfig.HealthCheck.Interval.Seconds()))
	}
	config := networkLoadBalancerConfig{
		settings: &template.NetworkLoadBalancer{
			PublicSubnetCIDRs: s.publicSubnetCIDRBlocks,
			Listener: template.NetworkLoadBalancerListener{
				Port:            aws.StringValue(port),
				Protocol:        strings.ToUpper(aws.StringValue(protocol)),
				TargetContainer: targetContainer,
				TargetPort:      targetPort,
				SSLPolicy:       nlbConfig.SSLPolicy,
				Aliases:         aliases,
				HealthCheck:     hc,
				Stickiness:      nlbConfig.Stickiness,
			},
			MainContainerPort: s.containerPort(),
		},
	}

	if s.dnsDelegationEnabled {
		dnsDelegationRole, dnsName := convertAppInformation(s.appInfo)
		config.appDNSName = dnsName
		config.appDNSDelegationRole = dnsDelegationRole
	}
	return config, nil
}

func convertExecuteCommand(e *manifest.ExecuteCommand) *template.ExecuteCommandOpts {
	if e.Config.IsEmpty() && !aws.BoolValue(e.Enable) {
		return nil
	}
	return &template.ExecuteCommandOpts{}
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
		Variables:      lc.Variables,
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
			FilesystemID:  volume.EFS.Advanced.FileSystemID,
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
			Filesystem:    in.FileSystemID,
			RootDirectory: rootDir,
			IAM:           iam,
		}
	}
	// AuthConfig exists; check the properties.
	if aws.BoolValue(in.AuthConfig.IAM) {
		iam = aws.String(enabled)
	}

	return &template.EFSVolumeConfiguration{
		Filesystem:    in.FileSystemID,
		RootDirectory: rootDir,
		IAM:           iam,
		AccessPointID: in.AuthConfig.AccessPointID,
	}
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
	opts.SecurityGroups = network.VPC.SecurityGroups.GetIDs()
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

func convertDeploymentConfig(deploymentConfig manifest.DeploymentConfiguration) template.DeploymentConfigurationOpts {
	var deployConfigs template.DeploymentConfigurationOpts
	if strings.EqualFold(aws.StringValue(deploymentConfig.Rolling), manifest.ECSRecreateRollingUpdateStrategy) {
		deployConfigs.MinHealthyPercent = minHealthyPercentRecreate
		deployConfigs.MaxPercent = maxPercentRecreate
	} else {
		deployConfigs.MinHealthyPercent = minHealthyPercentDefault
		deployConfigs.MaxPercent = maxPercentDefault
	}
	return deployConfigs
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
		publishers.Topics = append(publishers.Topics, &template.Topic{
			Name:      topic.Name,
			AccountID: accountID,
			Partition: partition.ID(),
			Region:    region,
			App:       app,
			Env:       env,
			Svc:       svc,
		})
	}

	return &publishers, nil
}

func convertSubscribe(s manifest.SubscribeConfig) (*template.SubscribeOpts, error) {
	if s.Topics == nil {
		return nil, nil
	}
	var subscriptions template.SubscribeOpts
	for _, sb := range s.Topics {
		ts, err := convertTopicSubscription(sb)
		if err != nil {
			return nil, err
		}
		subscriptions.Topics = append(subscriptions.Topics, ts)
	}
	subscriptions.Queue = convertQueue(s.Queue)
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

func convertQueue(q manifest.SQSQueue) *template.SQSQueue {
	if q.IsEmpty() {
		return nil
	}
	return &template.SQSQueue{
		Retention:  convertRetention(q.Retention),
		Delay:      convertDelay(q.Delay),
		Timeout:    convertTimeout(q.Timeout),
		DeadLetter: convertDeadLetter(q.DeadLetter),
	}
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
		os = template.OSWindowsServerCore
	case manifest.OSWindowsServer2019Full:
		os = template.OSWindowsServerFull
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

func convertSecrets(secrets map[string]manifest.Secret) map[string]template.Secret {
	if len(secrets) == 0 {
		return nil
	}
	m := make(map[string]template.Secret)
	for name, mftSecret := range secrets {
		var tplSecret template.Secret = template.SecretFromSSMOrARN(mftSecret.Value())
		if mftSecret.IsSecretsManagerName() {
			tplSecret = template.SecretFromSecretsManager(mftSecret.Value())
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
