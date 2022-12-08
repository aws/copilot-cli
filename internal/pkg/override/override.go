// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override defines functionality to interact with the "overrides/" directory
// for accessing and mutating the Copilot generated AWS CloudFormation templates.
package override

import "fmt"

// An Overrider is the interface to apply transformations to a CloudFormation template body.
type Overrider interface {
	Override(body []byte) (transformed []byte, err error)
}

// Bytes applies an Overrider to a given byte slice and returns the transformed byte slice.
// If the Overrider has to install any dependencies, then Install is invoked before Override.
// If the Overrider has to clean up data from the transformed template, then CleanUp is invoked before exiting.
func Bytes(body []byte, ovrdr Overrider) ([]byte, error) {
	if installer, ok := ovrdr.(interface{ Install() error }); ok {
		if err := installer.Install(); err != nil {
			return nil, fmt.Errorf("install dependencies before overriding: %w", err)
		}
	}

	out, err := ovrdr.Override(body)
	if err != nil {
		return nil, fmt.Errorf("override document: %w", err)
	}

	if cleaner, ok := ovrdr.(interface{ CleanUp([]byte) ([]byte, error) }); ok {
		out, err = cleaner.CleanUp(out)
		if err != nil {
			return nil, fmt.Errorf("clean up overriden document: %w", err)
		}
	}
	return out, nil
}
