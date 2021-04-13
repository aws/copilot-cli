// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

var (
	errUnmarshalEFSOpts = errors.New(`cannot unmarshal efs field into bool or map`)
)

// Storage represents the options for external and native storage.
type Storage struct {
	Volumes map[string]Volume `yaml:"volumes"`
}

// Volume is an abstraction which merges the MountPoint and Volumes concepts from the ECS Task Definition
type Volume struct {
	EFS            *EFSConfigOrBool `yaml:"efs"`
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
	FileSystemID  *string              `yaml:"id"`       // Required. Can be specified as "copilot" or "managed" magic keys.
	RootDirectory *string              `yaml:"root_dir"` // Default "/". For BYO EFS.
	AuthConfig    *AuthorizationConfig `yaml:"auth"`     // Auth config for BYO EFS.
	UID           *uint32              `yaml:"uid"`      // UID for managed EFS.
	GID           *uint32              `yaml:"gid"`      // GID for managed EFS.
}

// IsEmpty returns empty if the struct has all zero members.
func (e *EFSVolumeConfiguration) IsEmpty() bool {
	if e.FileSystemID == nil && e.RootDirectory == nil && e.AuthConfig == nil && e.UID == nil && e.GID == nil {
		return true
	}
	return false
}

// EFSConfigOrBool contains custom unmarshaling logic for the `efs` field in the manifest.
type EFSConfigOrBool struct {
	Config  EFSVolumeConfiguration
	Enabled *bool
}

// UnmarshalYAML implements the yaml(v2) interface. It allows EFS to be specified as a
// string or a struct alternately.
func (e *EFSConfigOrBool) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&e.Config); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !e.Config.IsEmpty() {
		// Unmarshaled successfully to e.Config, unset e.ID, and return.
		e.Enabled = nil
		return nil
	}

	if err := unmarshal(&e.Enabled); err != nil {
		return errUnmarshalEFSOpts
	}
	return nil
}

// UseManagedFS returns true if the user has specified EFS as a bool, or has only specified UID and GID.
func (e *EFSConfigOrBool) UseManagedFS() bool {
	// Respect explicitly enabled or disabled value first.
	if e.Enabled != nil {
		return aws.BoolValue(e.Enabled)
	}
	// Check whether we're implicitly enabling managed EFS via UID/GID.
	if !e.Config.EmptyUIDConfig() {
		return true
	}

	return false
}

func (e *EFSVolumeConfiguration) EmptyBYOConfig() bool {
	return e.FileSystemID == nil && e.AuthConfig == nil && e.RootDirectory == nil
}

func (e *EFSVolumeConfiguration) EmptyUIDConfig() bool {
	return e.UID == nil && e.GID == nil
}

func (e *EFSConfigOrBool) EmptyVolume() bool {
	// Respect Bool value first: return true if Enabled is false; false if true.
	if e.Enabled != nil {
		return !aws.BoolValue(e.Enabled)
	}

	// If config is totally empty, the volume doesn't have an EFS config.
	if e.Config.EmptyBYOConfig() && e.Config.EmptyUIDConfig() {
		return true
	}

	return false
}

// AuthorizationConfig holds options relating to access points and IAM authorization.
type AuthorizationConfig struct {
	IAM           *bool   `yaml:"iam"`             // Default true
	AccessPointID *string `yaml:"access_point_id"` // Default ""
}
