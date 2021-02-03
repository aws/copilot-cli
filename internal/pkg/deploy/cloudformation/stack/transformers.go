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

var (
	pEnabled  = aws.String("ENABLED")
	pDisabled = aws.String("DISABLED")
)

// Default values for EFS options
var (
	defaultRootDirectory   = aws.String("/")
	defaultIAM             = pDisabled
	defaultReadOnly        = aws.Bool(true)
	defaultWritePermission = false
)

// Default value for Sidecar port.
const (
	defaultSidecarPort = "80"
)

// Validation errors when rendering manifest into template.
var (
	errNoFSID          = errors.New("volume field efs/id cannot be empty")
	errNoContainerPath = errors.New("volume field path cannot be empty")
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
		sidecars = append(sidecars, &template.SidecarOpts{
			Name:       aws.String(name),
			Image:      config.Image,
			Port:       port,
			Protocol:   protocol,
			CredsParam: config.CredsParam,
			Secrets:    config.Secrets,
			Variables:  config.Variables,
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
func convertStorageOpts(in manifest.Storage) (*template.StorageOpts, error) {
	v, err := renderVolumes(in.Volumes)
	if err != nil {
		return nil, err
	}
	mp, err := renderMountPoints(in.Volumes)
	if err != nil {
		return nil, err
	}
	perms, err := renderStoragePermissions(in.Volumes)
	if err != nil {
		return nil, err
	}
	return &template.StorageOpts{
		Volumes:     v,
		MountPoints: mp,
		EFSPerms:    perms,
	}, nil
}

// renderSidecarMountPoints is used to convert from manifest to template objects.
func renderSidecarMountPoints(in []manifest.SidecarMountPoint) []*template.MountPoint {
	if len(in) == 0 {
		return nil
	}
	output := []*template.MountPoint{}
	for _, smp := range in {
		mp := template.MountPoint{
			ContainerPath: smp.ContainerPath,
			SourceVolume:  smp.SourceVolume,
			ReadOnly:      smp.ReadOnly,
		}
		output = append(output, &mp)
	}
	return output
}

func renderStoragePermissions(input map[string]manifest.Volume) ([]*template.EFSPermission, error) {
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

func renderMountPoints(input map[string]manifest.Volume) ([]*template.MountPoint, error) {
	if len(input) == 0 {
		return nil, nil
	}
	output := []*template.MountPoint{}
	for name, volume := range input {
		// ContainerPath must be specified.
		if volume.ContainerPath == nil {
			return nil, errNoContainerPath
		}
		// ReadOnly defaults to true.
		readOnly := defaultReadOnly
		if volume.ReadOnly != nil {
			readOnly = volume.ReadOnly
		}
		mp := template.MountPoint{
			ReadOnly:      readOnly,
			ContainerPath: volume.ContainerPath,
			SourceVolume:  aws.String(name),
		}
		output = append(output, &mp)
	}
	return output, nil
}

func renderVolumes(input map[string]manifest.Volume) ([]*template.Volume, error) {
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
		if aws.StringValue(rootDir) == "" {
			rootDir = defaultRootDirectory
		}
		var iam *string
		if volume.EFS.AuthConfig.IAM == nil {
			iam = defaultIAM
		}
		if aws.BoolValue(volume.EFS.AuthConfig.IAM) {
			iam = pEnabled
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
