// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
)

var (
	errEmptyContainerPort        = errors.New(`"port" must be specified`)
	errInvalidBuildAndLocation   = errors.New(`must specify one, not both, not none, of "build" and "location"`)
	errDuplicatedTargetContainer = errors.New(`must specify one, not both, of "target_container" and "targetContainer"`)
	errInvalidRangeOpts          = errors.New(`must specify one, not both, of "range" and "min"/"max"`)
	errInvalidAdvancedCount      = errors.New(`must specify one, not both, of "spot" and autoscaling fields`)
	errInvalidAutoscaling        = errors.New(`must specify "range" if using autoscaling`)
	errInvalidEFSConfiguration   = errors.New(`must specify one, not both, of "uid/gid" and "id/root_dir/auth"`)
	errInvalidEFSAccessPoint     = errors.New("root_dir must be either empty or / and auth.iam must be true when access_point_id is in used")
)

// ErrInvalidWorkloadType occurs when a user requested a manifest template type that doesn't exist.
type ErrInvalidWorkloadType struct {
	Type string
}

func (e *ErrInvalidWorkloadType) Error() string {
	return fmt.Sprintf("invalid manifest type: %s", e.Type)
}

// ErrInvalidPipelineManifestVersion occurs when the pipeline.yml file
// contains invalid schema version during unmarshalling.
type ErrInvalidPipelineManifestVersion struct {
	invalidVersion PipelineSchemaMajorVersion
}

func (e *ErrInvalidPipelineManifestVersion) Error() string {
	return fmt.Sprintf("pipeline.yml contains invalid schema version: %d", e.invalidVersion)
}

// Is compares the 2 errors. Only returns true if the errors are of the same
// type and contain the same information.
func (e *ErrInvalidPipelineManifestVersion) Is(target error) bool {
	t, ok := target.(*ErrInvalidPipelineManifestVersion)
	return ok && t.invalidVersion == e.invalidVersion
}

// ErrUnknownProvider occurs CreateProvider() is called with configurations
// that do not map to any supported provider.
type ErrUnknownProvider struct {
	unknownProviderProperties interface{}
}

func (e *ErrUnknownProvider) Error() string {
	return fmt.Sprintf("no provider found for properties: %v",
		e.unknownProviderProperties)
}

// Is compares the 2 errors. Returns true if the errors are of the same
// type
func (e *ErrUnknownProvider) Is(target error) bool {
	_, ok := target.(*ErrUnknownProvider)
	return ok
}

type errInvalidIntRangeBand struct {
	value string
}

func (e *errInvalidIntRangeBand) Error() string {
	return fmt.Sprintf("invalid range value %s. Should be in format of ${min}-${max}", string(e.value))
}

type errMinGreaterThanMax struct {
	min int
	max int
}

func (e *errMinGreaterThanMax) Error() string {
	return fmt.Sprintf("min value %d cannot be greater than max value %d", e.min, e.max)
}
