// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"
	"sort"
	"strings"
)

type errorExecutableNotFound struct {
	executable string
	error      error
}

func (e *errorExecutableNotFound) Error() string {
	return fmt.Sprintf("look up %q: %s", e.executable, e.error.Error())
}

type errPackageManagerUnavailable struct {
	parentErrors []error
}

func (err *errPackageManagerUnavailable) Error() string {
	var parentErrStrings []string
	for _, e := range err.parentErrors {
		parentErrStrings = append(parentErrStrings, e.Error())
	}
	sort.Sort((sort.StringSlice)(parentErrStrings))
	return fmt.Sprintf("cannot find a package manager to override with the Cloud Development Kit: %s",
		strings.Join(parentErrStrings, "; "))
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
