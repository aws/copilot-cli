// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package manifest

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

// Validation errors when rendering manifest into template.
var (
	errNoFSID          = errors.New("volume field efs/id cannot be empty")
	errNoContainerPath = errors.New("volume field path cannot be empty")
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

// Storage represents the options for external and native storage.
type Storage struct {
	Volumes map[string]Volume `yaml:"volumes"`
}

// Volume is an abstraction which merges the MountPoint and Volumes concepts from the ECS Task Definition
type Volume struct {
	EFS            EFSVolumeConfiguration `yaml:"efs"`
	MountPointOpts `yaml:",inline"`
}

// MountPointOpts is shared between Volumes for the main container and MountPoints for sidecars.
type MountPointOpts struct {
	ContainerPath *string `yaml:"path"`
	ReadOnly      *bool   `yaml:"read_only"`
}

// SidecarMountPoint is used to let sidecars mount volumes defined in `storage`
type SidecarMountPoint struct {
	SourceVolume   *string `yaml:"source_volume"`
	MountPointOpts `yaml:",inline"`
}

// EFSVolumeConfiguration holds options which tell ECS how to reach out to the EFS filesystem.
type EFSVolumeConfiguration struct {
	FileSystemID  *string             `yaml:"id"`       // Required.
	RootDirectory *string             `yaml:"root_dir"` // Default "/"
	AuthConfig    AuthorizationConfig `yaml:"auth"`
}

// AuthorizationConfig holds options relating to access points and IAM authorization.
type AuthorizationConfig struct {
	IAM           *bool   `yaml:"iam"`             // Default true
	AccessPointID *string `yaml:"access_point_id"` // Default ""
}

// RenderStorageOpts converts a Storage field into template data structures which can be used
// to execute CFN templates
func RenderStorageOpts(in Storage) (*template.StorageOpts, error) {
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

// RenderSidecarMountPoints is used to convert from manifest to template objects.
func RenderSidecarMountPoints(in []SidecarMountPoint) []*template.MountPoint {
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

func renderStoragePermissions(input map[string]Volume) ([]*template.EFSPermission, error) {
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

func renderMountPoints(input map[string]Volume) ([]*template.MountPoint, error) {
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

func renderVolumes(input map[string]Volume) ([]*template.Volume, error) {
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
