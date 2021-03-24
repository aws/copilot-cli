// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
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

const (
	ephemeralMinValueGiB = 20
	ephemeralMaxValueGiB = 16000
)

// Validation errors when rendering manifest into template.
var (
	errNoFSID                      = errors.New(`volume field efs/id cannot be empty`)
	errAcessPointWithRootDirectory = errors.New(`root directory must be empty or "/" when access point ID is specified`)
	errAccessPointWithoutIAM       = errors.New(`"iam" must be true when access point ID is specified`)

	errNoContainerPath = errors.New(`"path" cannot be empty`)
	errNoSourceVolume  = errors.New(`"source_volume" cannot be empty`)

	errEphemeralBadSize = errors.New("ephemeral storage must be between 20 GiB and 16000 GiB")
)

// convertSidecar converts the manifest sidecar configuration into a format parsable by the templates pkg.
func convertSidecar(s map[string]*manifest.SidecarConfig) ([]*template.SidecarOpts, error) {
	if s == nil {
		return nil, nil
	}
	var sidecars []*template.SidecarOpts
	for name, config := range s {
		port, protocol, err := parsePortMapping(config.Port)
		if err != nil {
			return nil, err
		}
		mp, err := convertSidecarMountPoints(config.MountPoints)
		if err != nil {
			return nil, err
		}
		sidecars = append(sidecars, &template.SidecarOpts{
			Name:        aws.String(name),
			Image:       config.Image,
			Port:        port,
			Protocol:    protocol,
			CredsParam:  config.CredsParam,
			Secrets:     config.Secrets,
			Variables:   config.Variables,
			MountPoints: mp,
		})
	}
	return sidecars, nil
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

// convertAutoscaling converts the service's Auto Scaling configuration into a format parsable
// by the templates pkg.
func convertAutoscaling(a *manifest.Autoscaling) (*template.AutoscalingOpts, error) {
	if a.IsEmpty() {
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
func convertStorageOpts(in *manifest.Storage) (*template.StorageOpts, error) {
	if in == nil {
		return nil, nil
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
		Ephemeral:   ephemeral,
		Volumes:     v,
		MountPoints: mp,
		EFSPerms:    perms,
	}, nil
}

func convertEphemeral(in *int) (*int, error) {
	if in == nil {
		return nil, nil
	}

	if aws.IntValue(in) < ephemeralMinValueGiB || aws.IntValue(in) > ephemeralMaxValueGiB {
		return nil, errEphemeralBadSize
	}

	return in, nil
}

// convertSidecarMountPoints is used to convert from manifest to template objects.
func convertSidecarMountPoints(in []manifest.SidecarMountPoint) ([]*template.MountPoint, error) {
	if len(in) == 0 {
		return nil, nil
	}
	var output []*template.MountPoint
	for _, smp := range in {
		mp, err := convertMountPoint(smp.SourceVolume, smp.ContainerPath, smp.ReadOnly)
		if err != nil {
			return nil, err
		}
		output = append(output, mp)
	}
	return output, nil
}

func convertMountPoint(sourceVolume, containerPath *string, readOnly *bool) (*template.MountPoint, error) {
	// containerPath must be specified.
	if aws.StringValue(containerPath) == "" {
		return nil, errNoContainerPath
	}
	path := aws.StringValue(containerPath)
	if err := validateContainerPath(path); err != nil {
		return nil, fmt.Errorf("validate container path %s: %w", path, err)
	}
	// readOnly defaults to true.
	oReadOnly := aws.Bool(defaultReadOnly)
	if readOnly != nil {
		oReadOnly = readOnly
	}
	// sourceVolume must be specified. This is only a concern for sidecars.
	if aws.StringValue(sourceVolume) == "" {
		return nil, errNoSourceVolume
	}
	return &template.MountPoint{
		ReadOnly:      oReadOnly,
		ContainerPath: containerPath,
		SourceVolume:  sourceVolume,
	}, nil
}

func convertMountPoints(input map[string]manifest.Volume) ([]*template.MountPoint, error) {
	if len(input) == 0 {
		return nil, nil
	}
	var output []*template.MountPoint
	for name, volume := range input {
		mp, err := convertMountPoint(aws.String(name), volume.ContainerPath, volume.ReadOnly)
		if err != nil {
			return nil, err
		}
		output = append(output, mp)
	}
	return output, nil
}

func convertEFSPermissions(input map[string]manifest.Volume) ([]*template.EFSPermission, error) {
	if len(input) == 0 {
		return nil, nil
	}
	output := []*template.EFSPermission{}
	for _, volume := range input {
		// Write defaults to false
		write := defaultWritePermission
		if volume.ReadOnly != nil {
			write = !aws.BoolValue(volume.ReadOnly)
		}
		if volume.EFS.FileSystemID == nil {
			return nil, errNoFSID
		}
		perm := template.EFSPermission{
			Write:         write,
			AccessPointID: volume.EFS.AuthConfig.AccessPointID,
			FilesystemID:  volume.EFS.FileSystemID,
		}
		output = append(output, &perm)
	}
	return output, nil
}

func convertVolumes(input map[string]manifest.Volume) ([]*template.Volume, error) {
	if len(input) == 0 {
		return nil, nil
	}
	output := []*template.Volume{}
	for name, volume := range input {
		// Set default values correctly.
		fsID := volume.EFS.FileSystemID
		if aws.StringValue(fsID) == "" {
			return nil, errNoFSID
		}

		rootDir := volume.EFS.RootDirectory

		// Validate that root directory path doesn't contain spaces or shell injection.
		if err := validateRootDirPath(aws.StringValue(rootDir)); err != nil {
			return nil, fmt.Errorf("validate root directory path %s: %w", aws.StringValue(rootDir), err)
		}

		if aws.StringValue(rootDir) == "" {
			rootDir = aws.String(defaultRootDirectory)
		}

		var iam *string
		if volume.EFS.AuthConfig.IAM == nil {
			iam = aws.String(defaultIAM)
		}
		if aws.BoolValue(volume.EFS.AuthConfig.IAM) {
			iam = aws.String(enabled)
		}

		// Validate ECS requirements: when an AP is specified, IAM MUST be true
		// and root directory MUST be either empty or "/".
		// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-efsvolumeconfiguration.html
		if aws.StringValue(volume.EFS.AuthConfig.AccessPointID) != "" {
			if !aws.BoolValue(volume.EFS.AuthConfig.IAM) {
				return nil, errAccessPointWithoutIAM
			}
			// Use rootDir var we previously identified.
			if !(aws.StringValue(rootDir) == "/") {
				return nil, errAcessPointWithRootDirectory
			}
		}

		v := template.Volume{
			Name: aws.String(name),

			Filesystem:    fsID,
			RootDirectory: rootDir,

			AccessPointID: volume.EFS.AuthConfig.AccessPointID,
			IAM:           iam,
		}
		output = append(output, &v)
	}
	return output, nil
}

func convertNetworkConfig(network manifest.NetworkConfig) *template.NetworkOpts {
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
