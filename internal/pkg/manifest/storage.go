// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

var (
	errUnmarshalEFSOpts        = errors.New(`cannot unmarshal efs field into bool or map`)
	errInvalidEFSConfiguration = errors.New(`must specify one, not both, of "uid/gid" and "id/root_dir/auth"`)
)

// Storage represents the options for external and native storage.
type Storage struct {
	Ephemeral *int               `yaml:"ephemeral"`
	Volumes   map[string]*Volume `yaml:"volumes"`
}

// TODO: add comment and unit test
func (s *Storage) IsEmpty() bool {
	return s.Ephemeral == nil && s.Volumes == nil
}

// Volume is an abstraction which merges the MountPoint and Volumes concepts from the ECS Task Definition
type Volume struct {
	EFS            *EFSConfigOrBool `yaml:"efs"`
	MountPointOpts `yaml:",inline"`
}

// EmptyVolume returns true if the EFS configuration is nil or explicitly/implicitly disabled.
func (v *Volume) EmptyVolume() bool {
	if v.EFS == nil {
		return true
	}
	// Respect Bool value first: return true if EFS is explicitly disabled.
	if v.EFS.Disabled() {
		return true
	}

	return false
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
	return e.FileSystemID == nil && e.RootDirectory == nil && e.AuthConfig == nil && e.UID == nil && e.GID == nil
}

// EFSConfigOrBool contains custom unmarshaling logic for the `efs` field in the manifest.
type EFSConfigOrBool struct {
	Advanced EFSVolumeConfiguration
	Enabled  *bool
}

// UnmarshalYAML implements the yaml(v2) interface. It allows EFS to be specified as a
// string or a struct alternately.
func (e *EFSConfigOrBool) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&e.Advanced); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !e.Advanced.IsEmpty() {
		// Unmarshaled successfully to e.Config, unset e.ID, and return.
		if err := e.Advanced.isValid(); err != nil {
			return err
		}
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
	return !e.Advanced.EmptyUIDConfig()
}

// Disabled returns true if Enabled is explicitly set to false.
// This function is useful for checking that the EFS config has been intentionally turned off
// and whether we should ignore any values of the struct which have been populated erroneously.
func (e *EFSConfigOrBool) Disabled() bool {
	if e.Enabled != nil && !aws.BoolValue(e.Enabled) {
		return true
	}
	return false
}

// EmptyBYOConfig returns true if the `id`, `root_directory`, and `auth` fields are all empty.
// This would mean that no custom EFS information has been specified.
func (e *EFSVolumeConfiguration) EmptyBYOConfig() bool {
	return e.FileSystemID == nil && e.AuthConfig == nil && e.RootDirectory == nil
}

// EmptyUIDConfig returns true if the `uid` and `gid` fields are empty. These fields are mutually exclusive
// with BYO EFS. If they are nonempty, then we should use managed EFS instead.
func (e *EFSVolumeConfiguration) EmptyUIDConfig() bool {
	return e.UID == nil && e.GID == nil
}

func (e *EFSVolumeConfiguration) unsetBYOConfig() {
	e.FileSystemID = nil
	e.AuthConfig = nil
	e.RootDirectory = nil
}

func (e *EFSVolumeConfiguration) unsetUIDConfig() {
	e.UID = nil
	e.GID = nil
}

func (e *EFSVolumeConfiguration) isValid() error {
	if !e.EmptyBYOConfig() && !e.EmptyUIDConfig() {
		return errInvalidEFSConfiguration
	}
	return nil
}

// AuthorizationConfig holds options relating to access points and IAM authorization.
type AuthorizationConfig struct {
	IAM           *bool   `yaml:"iam"`             // Default true
	AccessPointID *string `yaml:"access_point_id"` // Default ""
}
