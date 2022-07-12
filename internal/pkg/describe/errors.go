// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import "fmt"

type ErrManifestNotFoundInTemplate struct {
	app  string
	env  string
	name string
}

// Error implements the error interface.
func (err *ErrManifestNotFoundInTemplate) Error() string {
	return fmt.Sprintf("manifest metadata not found in template of stack %s-%s-%s", err.app, err.env, err.name)
}
