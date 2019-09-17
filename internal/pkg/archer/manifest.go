// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Manifest is the API for a manifest object.
type Manifest interface {
	ManifestMarshaller
	ManifestCFNTemplater
}

// ManifestMarshaller is the interface to serialize a manifest object into a YAML document.
type ManifestMarshaller interface {
	Marshal() ([]byte, error)
}

// ManifestCFNTemplater is the interface to serialize a manifest object into a CloudFormation template.
type ManifestCFNTemplater interface {
	CFNTemplate() (string, error)
}
