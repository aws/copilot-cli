// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import "fmt"

// ErrInvalidAppManifestType occurs when a user requested a manifest template type that doesn't exist.
type ErrInvalidAppManifestType struct {
	Type string
}

func (e *ErrInvalidAppManifestType) Error() string {
	return fmt.Sprintf("invalid manifest type: %s", e.Type)
}

// ErrInvalidPipelineManifestVersion occurs when the pipeline.yml file
// contains invalid schema version during unmarshalling.
type ErrInvalidPipelineManifestVersion struct {
	version PipelineSchemaMajorVersion
}

func (e *ErrInvalidPipelineManifestVersion) Error() string {
	return fmt.Sprintf("pipeline.yml contains invalid schema version: %d", e.version)
}

// ErrMissingProviderProperties occurs when the specified source provider
// hasn't been configured before pipeline creation.
type ErrMissingProviderProperties struct {
	provider Provider
}

func (e *ErrMissingProviderProperties) Error() string {
	return fmt.Sprintf("the provider has not been configured, provider: %s",
		e.provider)
}

// ErrProviderPropertiesMismatch occurs when the provided properties do not
// match the type of provider being configured.
type ErrProviderPropertiesMismatch struct {
	provider Provider
	newProps interface{}
}

func (e *ErrProviderPropertiesMismatch) Error() string {
	return fmt.Sprintf("mismatch between the property type and the provider, properties: %T%+v, provider: %s",
		e.newProps, e.newProps, e.provider)
}
