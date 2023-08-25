// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"gopkg.in/yaml.v3"
)

var (
	errUnmarshalEFSOpts = errors.New(`cannot unmarshal "efs" field into bool or map`)
)

// Storage represents the options for external and native storage.
type Storage struct {
	Ephemeral      *int               `yaml:"ephemeral"`
	ReadonlyRootFS *bool              `yaml:"readonly_fs"`
	Volumes        map[string]*Volume `yaml:"volumes"` // NOTE: keep the pointers because `mergo` doesn't automatically deep merge map's value unless it's a pointer type.
}

// IsEmpty returns empty if the struct has all zero members.
func (s *Storage) IsEmpty() bool {
	return s.Ephemeral == nil && s.Volumes == nil && s.ReadonlyRootFS == nil
}

func (s *Storage) requiredEnvFeatures() []string {
	if s.hasManagedFS() {
		return []string{template.EFSFeatureName}
	}
	return nil
}

func (s *Storage) hasManagedFS() bool {
	for _, v := range s.Volumes {
		if v.EmptyVolume() || !v.EFS.UseManagedFS() {
			continue
		}
		return true
	}
	return false
}

// Volume is an abstraction which merges the MountPoint and Volumes concepts from the ECS Task Definition
type Volume struct {
	EFS            EFSConfigOrBool `yaml:"efs"`
	MountPointOpts `yaml:",inline"`
}

// EmptyVolume returns true if the EFS configuration is nil or explicitly/implicitly disabled.
func (v *Volume) EmptyVolume() bool {
	if v.EFS.IsEmpty() {
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
	FileSystemID  StringOrFromCFN     `yaml:"id"`       // Required. Can be specified as "copilot" or "managed" magic keys.
	RootDirectory *string             `yaml:"root_dir"` // Default "/". For BYO EFS.
	AuthConfig    AuthorizationConfig `yaml:"auth"`     // Auth config for BYO EFS.
	UID           *uint32             `yaml:"uid"`      // UID for managed EFS.
	GID           *uint32             `yaml:"gid"`      // GID for managed EFS.
}

// IsEmpty returns empty if the struct has all zero members.
func (e *EFSVolumeConfiguration) IsEmpty() bool {
	return e.FileSystemID.isEmpty() && e.RootDirectory == nil && e.AuthConfig.IsEmpty() && e.UID == nil && e.GID == nil
}

// EFSConfigOrBool contains custom unmarshaling logic for the `efs` field in the manifest.
type EFSConfigOrBool struct {
	Advanced EFSVolumeConfiguration
	Enabled  *bool
}

// IsEmpty returns empty if the struct has all zero members.
func (e *EFSConfigOrBool) IsEmpty() bool {
	return e.Advanced.IsEmpty() && e.Enabled == nil
}

// UnmarshalYAML implements the yaml(v3) interface. It allows EFS to be specified as a
// string or a struct alternately.
func (e *EFSConfigOrBool) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&e.Advanced); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !e.Advanced.IsEmpty() {
		if err := e.Advanced.isValid(); err != nil {
			// NOTE: `e.Advanced` contains exclusive fields.
			// Validating that exclusive fields cannot be set simultaneously is necessary during `UnmarshalYAML`
			// because the `ApplyEnv` stage assumes that no exclusive fields are set together.
			// Not validating it during `UnmarshalYAML` would potentially cause an invalid manifest being deemed valid.
			return err
		}
		// Unmarshaled successfully to e.Config, unset e.ID, and return.
		e.Enabled = nil
		return nil
	}

	if err := value.Decode(&e.Enabled); err != nil {
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
	return e.FileSystemID.isEmpty() && e.AuthConfig.IsEmpty() && e.RootDirectory == nil
}

// EmptyUIDConfig returns true if the `uid` and `gid` fields are empty. These fields are mutually exclusive
// with BYO EFS. If they are nonempty, then we should use managed EFS instead.
func (e *EFSVolumeConfiguration) EmptyUIDConfig() bool {
	return e.UID == nil && e.GID == nil
}

func (e *EFSVolumeConfiguration) unsetBYOConfig() {
	e.FileSystemID = StringOrFromCFN{}
	e.AuthConfig = AuthorizationConfig{}
	e.RootDirectory = nil
}

func (e *EFSVolumeConfiguration) unsetUIDConfig() {
	e.UID = nil
	e.GID = nil
}

func (e *EFSVolumeConfiguration) isValid() error {
	if !e.EmptyBYOConfig() && !e.EmptyUIDConfig() {
		return &errFieldMutualExclusive{
			firstField:  "uid/gid",
			secondField: "id/root_dir/auth",
		}
	}
	return nil
}

// AuthorizationConfig holds options relating to access points and IAM authorization.
type AuthorizationConfig struct {
	IAM           *bool   `yaml:"iam"`             // Default true
	AccessPointID *string `yaml:"access_point_id"` // Default ""
}

// IsEmpty returns empty if the struct has all zero members.
func (a *AuthorizationConfig) IsEmpty() bool {
	return a.IAM == nil && a.AccessPointID == nil
}
