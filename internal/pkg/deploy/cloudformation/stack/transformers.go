// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"time"

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
)

// Default value for Sidecar port.
const (
	defaultSidecarPort = "80"
)

// Min and Max values for task ephemeral storage in GiB.
const (
	ephemeralMinValueGiB = 21
	ephemeralMaxValueGiB = 200
)

// Supported capacityproviders for Fargate services
const (
	capacityProviderFargateSpot = "FARGATE_SPOT"
	capacityProviderFargate     = "FARGATE"
)

var (
	errEphemeralBadSize  = errors.New("ephemeral storage must be between 20 GiB and 200 GiB")
	errInvalidSpotConfig = errors.New(`"count.spot" and "count.range" cannot be specified together`)
)

type convertSidecarOpts struct {
	sidecarConfig map[string]*manifest.SidecarConfig
	imageConfig   *manifest.Image
	workloadName  string
}

// convertSidecar converts the manifest sidecar configuration into a format parsable by the templates pkg.
func convertSidecar(s convertSidecarOpts) ([]*template.SidecarOpts, error) {
	if s.sidecarConfig == nil {
		return nil, nil
	}
	if err := validateNoCircularDependencies(s); err != nil {
		return nil, err
	}
	var sidecars []*template.SidecarOpts
	for name, config := range s.sidecarConfig {
		port, protocol, err := parsePortMapping(config.Port)
		if err != nil {
			return nil, err
		}
		if err := validateSidecarMountPoints(config.MountPoints); err != nil {
			return nil, err
		}
		convertDependsOnStatus(&s)
		if err := validateSidecarDependsOn(*config, name, s); err != nil {
			return nil, err
		}
		mp := convertSidecarMountPoints(config.MountPoints)

		sidecars = append(sidecars, &template.SidecarOpts{
			Name:         aws.String(name),
			Image:        config.Image,
			Essential:    config.Essential,
			Port:         port,
			Protocol:     protocol,
			CredsParam:   config.CredsParam,
			Secrets:      config.Secrets,
			Variables:    config.Variables,
			MountPoints:  mp,
			DockerLabels: config.DockerLabels,
			DependsOn:    config.DependsOn,
		})
	}
	return sidecars, nil
}

// convertDependsOnStatus converts image and sidecar depends on fields to have upper case statuses
func convertDependsOnStatus(s *convertSidecarOpts) {
	if s.sidecarConfig != nil {
		for _, sidecar := range s.sidecarConfig {
			if sidecar.DependsOn == nil {
				continue
			}
			for name, status := range sidecar.DependsOn {
				sidecar.DependsOn[name] = strings.ToUpper(status)
			}
		}
	}
	if s.imageConfig != nil && s.imageConfig.DependsOn != nil {
		for name, status := range s.imageConfig.DependsOn {
			s.imageConfig.DependsOn[name] = strings.ToUpper(status)
		}
	}
}

// convertDependsOn converts an Image DependsOn field to a template DependsOn version
func convertImageDependsOn(s convertSidecarOpts) (map[string]string, error) {
	if s.imageConfig == nil || s.imageConfig.DependsOn == nil {
		return nil, nil
	}
	convertDependsOnStatus(&s)
	if err := validateImageDependsOn(s); err != nil {
		return nil, err
	}
	return s.imageConfig.DependsOn, nil
}

// Valid sidecar portMapping example: 2000/udp, or 2000 (default to be tcp).
func parsePortMapping(s *string) (port *string, protocol *string, err error) {
	if s == nil {
		// default port for sidecar container to be 80.
		return aws.String(defaultSidecarPort), nil, nil
	}
	portProtocol := strings.Split(*s, "/")
	switch len(portProtocol) {
	case 1:
		return aws.String(portProtocol[0]), nil, nil
	case 2:
		return aws.String(portProtocol[0]), aws.String(portProtocol[1]), nil
	default:
		return nil, nil, fmt.Errorf("cannot parse port mapping from %s", *s)
	}
}

func convertAdvancedCount(a *manifest.AdvancedCount) (*template.AdvancedCount, error) {
	if a == nil {
		return nil, nil
	}

	if a.IsEmpty() {
		return nil, nil
	}

	autoscaling, err := convertAutoscaling(a)
	if err != nil {
		return nil, err
	}

	cps, err := convertCapacityProviders(a)
	if err != nil {
		return nil, err
	}

	return &template.AdvancedCount{
		Spot:        a.Spot,
		Autoscaling: autoscaling,
		Cps:         cps,
	}, nil
}

// convertCapacityProviders transforms the manifest fields into a format
// parsable by the templates pkg.
func convertCapacityProviders(a *manifest.AdvancedCount) ([]*template.CapacityProviderStrategy, error) {
	if a.IsEmpty() {
		return nil, nil
	}

	if a.Spot != nil && a.Range != nil {
		return nil, errInvalidSpotConfig
	}

	// return if autoscaling range specified without spot scaling
	if a.Range != nil && a.Range.Value != nil {
		return nil, nil
	}

	var cps []*template.CapacityProviderStrategy

	// if Spot specified as count, then weight on Spot CPS should be 1
	cps = append(cps, &template.CapacityProviderStrategy{
		Weight:           aws.Int(1),
		CapacityProvider: capacityProviderFargateSpot,
	})

	// Return if only spot is specifed as count
	if a.Range == nil {
		return cps, nil
	}

	// Scaling with spot
	rc := a.Range.RangeConfig
	if !rc.IsEmpty() {
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
	}

	return cps, nil
}

// convertAutoscaling converts the service's Auto Scaling configuration into a format parsable
// by the templates pkg.
func convertAutoscaling(a *manifest.AdvancedCount) (*template.AutoscalingOpts, error) {
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
	if a.CPU != nil {
		autoscalingOpts.CPU = aws.Float64(float64(*a.CPU))
	}
	if a.Memory != nil {
		autoscalingOpts.Memory = aws.Float64(float64(*a.Memory))
	}
	if a.Requests != nil {
		autoscalingOpts.Requests = aws.Float64(float64(*a.Requests))
	}
	if a.ResponseTime != nil {
		responseTime := float64(*a.ResponseTime) / float64(time.Second)
		autoscalingOpts.ResponseTime = aws.Float64(responseTime)
	}
	return &autoscalingOpts, nil
}

// convertHTTPHealthCheck converts the ALB health check configuration into a format parsable by the templates pkg.
func convertHTTPHealthCheck(hc *manifest.HealthCheckArgsOrString) template.HTTPHealthCheckOpts {
	opts := template.HTTPHealthCheckOpts{
		HealthCheckPath:    manifest.DefaultHealthCheckPath,
		HealthyThreshold:   hc.HealthCheckArgs.HealthyThreshold,
		UnhealthyThreshold: hc.HealthCheckArgs.UnhealthyThreshold,
	}
	if hc.HealthCheckArgs.Path != nil {
		opts.HealthCheckPath = *hc.HealthCheckArgs.Path
	} else if hc.HealthCheckPath != nil {
		opts.HealthCheckPath = *hc.HealthCheckPath
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
	return opts
}

func convertExecuteCommand(e *manifest.ExecuteCommand) *template.ExecuteCommandOpts {
	if e.Config.IsEmpty() && !aws.BoolValue(e.Enable) {
		return nil
	}
	return &template.ExecuteCommandOpts{}
}

func convertLogging(lc *manifest.Logging) *template.LogConfigOpts {
	if lc == nil {
		return nil
	}
	return logConfigOpts(lc)
}

func logConfigOpts(lc *manifest.Logging) *template.LogConfigOpts {
	return &template.LogConfigOpts{
		Image:          lc.LogImage(),
		ConfigFile:     lc.ConfigFile,
		EnableMetadata: lc.GetEnableMetadata(),
		Destination:    lc.Destination,
		SecretOptions:  lc.SecretOptions,
	}
}

// convertStorageOpts converts a manifest Storage field into template data structures which can be used
// to execute CFN templates
func convertStorageOpts(wlName *string, in *manifest.Storage) (*template.StorageOpts, error) {
	if in == nil {
		return nil, nil
	}
	if err := validateStorageConfig(in); err != nil {
		return nil, err
	}
	mv, err := convertManagedFSInfo(wlName, in.Volumes)
	if err != nil {
		return nil, err
	}
	v, err := convertVolumes(in.Volumes)
	if err != nil {
		return nil, err
	}
	mp, err := convertMountPoints(in.Volumes)
	if err != nil {
		return nil, err
	}
	perms, err := convertEFSPermissions(in.Volumes)
	if err != nil {
		return nil, err
	}
	ephemeral, err := convertEphemeral(in.Ephemeral)
	if err != nil {
		return nil, err
	}
	return &template.StorageOpts{
		Ephemeral:         ephemeral,
		Volumes:           v,
		MountPoints:       mp,
		EFSPerms:          perms,
		ManagedVolumeInfo: mv,
	}, nil
}

func convertEphemeral(in *int) (*int, error) {
	if in == nil {
		return nil, nil
	}

	// Min value for extensible ephemeral storage is 21; if customer specifies 20, which is the default size,
	// we shouldn't let CF error out. Instead, we'll just omit it from the config.
	if aws.IntValue(in) == 20 {
		return nil, nil
	}

	if aws.IntValue(in) < ephemeralMinValueGiB || aws.IntValue(in) > ephemeralMaxValueGiB {
		return nil, errEphemeralBadSize
	}

	return in, nil
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

func convertMountPoints(input map[string]manifest.Volume) ([]*template.MountPoint, error) {
	if len(input) == 0 {
		return nil, nil
	}
	var output []*template.MountPoint
	for name, volume := range input {
		output = append(output, convertMountPoint(aws.String(name), volume.ContainerPath, volume.ReadOnly))
	}
	return output, nil
}

func convertEFSPermissions(input map[string]manifest.Volume) ([]*template.EFSPermission, error) {
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
		var accessPointID *string
		if volume.EFS.Advanced.AuthConfig != nil {
			accessPointID = volume.EFS.Advanced.AuthConfig.AccessPointID
		}
		output = append(output, &template.EFSPermission{
			Write:         write,
			AccessPointID: accessPointID,
			FilesystemID:  volume.EFS.Advanced.FileSystemID,
		})
	}
	return output, nil
}

func convertManagedFSInfo(wlName *string, input map[string]manifest.Volume) (*template.ManagedVolumeCreationInfo, error) {
	var output *template.ManagedVolumeCreationInfo
	for name, volume := range input {
		if volume.EmptyVolume() {
			continue
		}
		if !volume.EFS.UseManagedFS() {
			continue
		}

		if output != nil {
			return nil, fmt.Errorf("cannot specify more than one managed volume per service")
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
	return output, nil
}

// getRandomUIDGID returns the 32-bit checksum of the service name for use as CreationInfo in the EFS Access Point.
// See https://stackoverflow.com/a/14210379/5890422 for discussion of the possibility of collisions in CRC32 with
// small numbers of hashes.
func getRandomUIDGID(name *string) uint32 {
	return crc32.ChecksumIEEE([]byte(aws.StringValue(name)))
}

func convertVolumes(input map[string]manifest.Volume) ([]*template.Volume, error) {
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
	return output, nil
}

func convertEFSConfiguration(in manifest.EFSVolumeConfiguration) *template.EFSVolumeConfiguration {
	// Set default values correctly.
	rootDir := in.RootDirectory
	if aws.StringValue(rootDir) == "" {
		rootDir = aws.String(defaultRootDirectory)
	}
	// Set default values for IAM and AccessPointID
	iam := aws.String(defaultIAM)
	if in.AuthConfig == nil {
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

func convertNetworkConfig(network *manifest.NetworkConfig) *template.NetworkOpts {
	if network == nil || network.VPC == nil {
		return &template.NetworkOpts{
			AssignPublicIP: template.EnablePublicIP,
			SubnetsType:    template.PublicSubnetsPlacement,
		}
	}
	opts := &template.NetworkOpts{
		AssignPublicIP: template.EnablePublicIP,
		SubnetsType:    template.PublicSubnetsPlacement,
		SecurityGroups: network.VPC.SecurityGroups,
	}
	if aws.StringValue(network.VPC.Placement) != manifest.PublicSubnetPlacement {
		opts.AssignPublicIP = template.DisablePublicIP
		opts.SubnetsType = template.PrivateSubnetsPlacement
	}
	return opts
}
