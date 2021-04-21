// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

var managedFSIDKeys = []string{"copilot", "managed"}

// Storage represents the options for external and native storage.
type Storage struct {
	Volumes map[string]Volume `yaml:"volumes"`
}

// Volume is an abstraction which merges the MountPoint and Volumes concepts from the ECS Task Definition
type Volume struct {
	EFS            *EFSConfigOrID `yaml:"efs"`
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
	return e.FileSystemID == nil && e.RootDirectory == nil && e.AuthConfig == nil && e.UID == nil && e.GID == nil
}

// EFSConfigOrID contains custom unmarshaling logic for the `efs` field in the manifest.
type EFSConfigOrID struct {
	Config EFSVolumeConfiguration
	ID     string
}

// UnmarshalYAML implements the yaml(v2) interface. It allows EFS to be specified as a
// string or a struct alternately.
func (e *EFSConfigOrID) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
		e.ID = ""
		return nil
	}

	if err := unmarshal(&e.ID); err != nil {
		return errUnmarshalBuildOpts
	}
	return nil
}

// UseManagedFS returns true if the user has specified "copilot" or "managed" as a FSID; false otherwise.
func (e *EFSConfigOrID) UseManagedFS() bool {
	if contains(managedFSIDKeys, e.ID) {
		return true
	}
	if contains(managedFSIDKeys, aws.StringValue(e.Config.FileSystemID)) {
		return true
	}
	return false
}

// FSID returns the correct value of the EFS filesystem ID. If the ID is set improperly (via a bad merge
// of environment overrides), it throws an error.
func (e *EFSConfigOrID) FSID() *string {
	fromID := e.ID
	// If config is empty, use ID; otherwise ignore string and use struct.
	if e.Config.IsEmpty() {
		return aws.String(fromID)
	}

	return e.Config.FileSystemID
}

// AuthorizationConfig holds options relating to access points and IAM authorization.
type AuthorizationConfig struct {
	IAM           *bool   `yaml:"iam"`             // Default true
	AccessPointID *string `yaml:"access_point_id"` // Default ""
}

func contains(l []string, k string) bool {
	for _, i := range l {
		if i == strings.ToLower(k) {
			return true
		}
	}
	return false
}
