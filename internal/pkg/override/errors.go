// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"
)

type errPackageManagerUnavailable struct{}

func (err *errPackageManagerUnavailable) Error() string {
	return "cannot find a JavaScript package manager to override with the Cloud Development Kit"
}

// RecommendActions implements the cli.actionRecommender interface.
func (err *errPackageManagerUnavailable) RecommendActions() string {
	return fmt.Sprintf(`Please follow the instructions to install either one of the package managers:
%q: %q
%q: %q`,
		"npm", "https://docs.npmjs.com/downloading-and-installing-node-js-and-npm",
		"yarn", "https://yarnpkg.com/getting-started/install")
}

// ErrNotExist occurs when the path of the file associated with an Overrider does not exist.
type ErrNotExist struct {
	parent error
}

func (err *ErrNotExist) Error() string {
	return fmt.Sprintf("overrider does not exist: %v", err.parent)
}
