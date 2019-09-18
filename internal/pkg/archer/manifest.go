// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// Manifest is the API for a manifest object.
type Manifest interface {
	Marshaller
	CFNTemplater
}

// Marshaller is the interface to serialize an object into a YAML document.
type Marshaller interface {
	Marshal() ([]byte, error)
}

// CFNTemplater is the interface to serialize an object into a CloudFormation template.
type CFNTemplater interface {
	CFNTemplate() (string, error)
}
