// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Pipeline represents the complete data model that can be serialized
// into a YAML manifest file or various other data format, such as
// CloudFormation template.
type Pipeline interface {
	// TODO: #244 Consolidate this interface and the archer.Manifest interface.
	// For now we use 2 interfaces to limit the scope of the changes to just
	// the pipeline commands so that they won't bleed into existing
	// application manifest.
	Marshal() ([]byte, error)
	StackConfiguration
}
