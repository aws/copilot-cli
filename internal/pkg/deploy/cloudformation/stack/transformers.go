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

// Validation errors when rendering manifest into template.
var (
	errNoFSID                      = errors.New(`volume field efs/id cannot be empty`)
	errAcessPointWithRootDirectory = errors.New(`root directory must be empty or "/" when access point ID is specified`)
	errAccessPointWithoutIAM       = errors.New(`"iam" must be true when access point ID is specified`)

	errNoContainerPath = errors.New(`"path" cannot be empty`)
	errNoSourceVolume  = errors.New(`"source_volume" cannot be empty`)

	errUIDWithNonManagedFS = errors.New("UID and GID cannot be specified with non-managed EFS")
	errInvalidUIDGIDConfig = errors.New("set managed filesystem access point creation info: must specify both UID and GID, or neither")
	errReservedUID         = errors.New("set managed filesystem access point creation info: UID must not be 0")
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
func convertStorageOpts(wlName *string, in *manifest.Storage) (*template.StorageOpts, error) {
	if in == nil {
		return nil, nil
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
	return &template.StorageOpts{
		Volumes:           v,
		MountPoints:       mp,
		EFSPerms:          perms,
		ManagedVolumeInfo: mv,
	}, nil
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
	var output []*template.EFSPermission
	for _, volume := range input {
		if volume.EFS == nil {
			continue
		}
		if volume.EFS.UseManagedFS() {
			continue
		}
		// Write defaults to false.
		write := defaultWritePermission
		if volume.ReadOnly != nil {
			write = !aws.BoolValue(volume.ReadOnly)
		}

		id := volume.EFS.FSID()
		if id == nil {
			return nil, errNoFSID
		}

		var accessPointID *string
		if volume.EFS.Config.AuthConfig != nil {
			accessPointID = volume.EFS.Config.AuthConfig.AccessPointID
		}
		perm := template.EFSPermission{
			Write:         write,
			AccessPointID: accessPointID,
			FilesystemID:  id,
		}
		output = append(output, &perm)
	}
	return output, nil
}

func convertManagedFSInfo(wlName *string, input map[string]manifest.Volume) (*template.ManagedVolumeCreationInfo, error) {
	var output *template.ManagedVolumeCreationInfo
	for name, volume := range input {
		if volume.EFS == nil {
			continue
		}

		if !volume.EFS.UseManagedFS() {
			continue
		}

		if output != nil {
			return nil, fmt.Errorf("validate managed EFS: cannot specify more than one managed volume per service")
		}

		uid := volume.EFS.Config.UID
		gid := volume.EFS.Config.GID

		if err := validateUIDGID(uid, gid); err != nil {
			return nil, err
		}

		if uid == nil && gid == nil {
			crc := aws.Uint32(getRandomUIDGID(aws.StringValue(wlName)))
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
func getRandomUIDGID(name string) uint32 {
	return crc32.ChecksumIEEE([]byte(name))
}

func validateUIDGID(uid, gid *uint32) error {
	if uid == nil && gid == nil {
		return nil
	}
	if (uid == nil) != (gid == nil) {
		return errInvalidUIDGIDConfig
	}
	// Check for root UID.
	if aws.Uint32Value(uid) == 0 {
		return errReservedUID
	}
	return nil
}

func convertVolumes(input map[string]manifest.Volume) ([]*template.Volume, error) {
	var output []*template.Volume
	for name, volume := range input {
		// Volumes can contain either:
		//   a) an EFS configuration, which must be valid
		//   b) no EFS configuration, in which case the volume is created using task scratch storage in order to share
		//      data between containers.
		if volume.EFS != nil && volume.EFS.UseManagedFS() {
			continue
		}
		// Convert EFS configuration to template struct.
		efs, err := convertEFS(volume.EFS)
		if err != nil {
			return nil, err
		}
		v := template.Volume{
			Name: aws.String(name),
			EFS:  efs,
		}
		output = append(output, &v)
	}
	return output, nil
}

// convertEFS converts a volume from a manfiest object to a template object. This function
// should not be called on non-managed volumes.
func convertEFS(in *manifest.EFSConfigOrID) (*template.EFSVolumeConfiguration, error) {
	// If there is no EFS information, just add the Name to the volume.
	if in == nil {
		return nil, nil
	}
	// UID and GID should not be specified for non-managed volumes.
	if !in.Config.IsEmpty() {
		if in.Config.UID != nil {
			return nil, errUIDWithNonManagedFS
		}
		if in.Config.GID != nil {
			return nil, errUIDWithNonManagedFS
		}
	}
	// EFS is specified as a string with just the filesystem ID.
	if in.ID != "" {
		return &template.EFSVolumeConfiguration{
			Filesystem:    aws.String(in.ID),
			IAM:           aws.String(defaultIAM),
			RootDirectory: aws.String(defaultRootDirectory),
		}, nil
	}
	// ID is nil and we received a value; therefore Config must be not nil.
	return convertEFSConfiguration(in.Config)

}

func convertEFSConfiguration(in manifest.EFSVolumeConfiguration) (*template.EFSVolumeConfiguration, error) {
	// Set default values correctly.
	fsID := in.FileSystemID
	if aws.StringValue(fsID) == "" {
		return nil, errNoFSID
	}

	rootDir := in.RootDirectory
	// Validate that root directory path doesn't contain spaces or shell injection.
	if err := validateRootDirPath(aws.StringValue(rootDir)); err != nil {
		return nil, fmt.Errorf("validate root directory path %s: %w", aws.StringValue(rootDir), err)
	}
	if aws.StringValue(rootDir) == "" {
		rootDir = aws.String(defaultRootDirectory)
	}

	// Set default values for IAM and AccessPointID
	iam := aws.String(defaultIAM)
	if in.AuthConfig == nil {
		return &template.EFSVolumeConfiguration{
			Filesystem:    fsID,
			RootDirectory: rootDir,
			IAM:           iam,
		}, nil
	}
	// AuthConfig exists; check the properties.
	if aws.BoolValue(in.AuthConfig.IAM) {
		iam = aws.String(enabled)
	}
	// Validate ECS requirements: when an AP is specified, IAM MUST be true
	// and root directory MUST be either empty or "/".
	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ecs-taskdefinition-efsvolumeconfiguration.html
	if aws.StringValue(in.AuthConfig.AccessPointID) != "" {
		if !aws.BoolValue(in.AuthConfig.IAM) {
			return nil, errAccessPointWithoutIAM
		}
		// Use rootDir value we previously identified.
		if !(aws.StringValue(rootDir) == "/") {
			return nil, errAcessPointWithRootDirectory
		}
	}

	return &template.EFSVolumeConfiguration{
		Filesystem:    fsID,
		RootDirectory: rootDir,
		IAM:           iam,
		AccessPointID: in.AuthConfig.AccessPointID,
	}, nil
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
