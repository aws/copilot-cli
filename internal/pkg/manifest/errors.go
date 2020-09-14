// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
)

// ErrInvalidWkldManifestType occurs when a user requested a manifest template type that doesn't exist.
type ErrInvalidWkldManifestType struct {
	Type string
}

func (e *ErrInvalidWkldManifestType) Error() string {
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
