// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import "gopkg.in/yaml.v3"

// Storage represents the options for external and native storage.
type Storage struct {
	Volumes map[string]Volume `yaml:"volumes"`
}

// Volume is an abstraction which merges the MountPoint and Volumes concepts from the ECS Task Definition
type Volume struct {
	EFS            *EFSVolumeConfiguration `yaml:"efs"`
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
	FileSystemID  *string              `yaml:"id"`       // Required.
	RootDirectory *string              `yaml:"root_dir"` // Default "/"
	AuthConfig    *AuthorizationConfig `yaml:"auth"`
}

func (e *EFSVolumeConfiguration) isEmpty() bool {
	return e.FileSystemID == nil && e.RootDirectory == nil && e.AuthConfig == nil
}

type EFSConfigOrID struct {
	Config EFSVolumeConfiguration
	ID     *string
}

func (e *EFSConfigOrID) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&e.Config); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !e.Config.isEmpty() {
		// Unmarshaled successfully to b.BuildArgs, unset b.BuildString, and return.
		e.ID = nil
		return nil
	}

	if err := unmarshal(&e.ID); err != nil {
		return errUnmarshalBuildOpts
	}
	return nil
}

// AuthorizationConfig holds options relating to access points and IAM authorization.
type AuthorizationConfig struct {
	IAM           *bool   `yaml:"iam"`             // Default true
	AccessPointID *string `yaml:"access_point_id"` // Default ""
}
