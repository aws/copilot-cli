// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import "fmt"

type errNPMUnavailable struct {
	parent error
}

func (err *errNPMUnavailable) Error() string {
	return fmt.Sprintf(`"npm" is required to override with the Cloud Development Kit: %v`, err.parent)
}

// RecommendActions implements the cli.actionRecommender interface.
func (err *errNPMUnavailable) RecommendActions() string {
	return fmt.Sprintf(`Please follow instructions at: %q to install "npm"`, "https://docs.npmjs.com/downloading-and-installing-node-js-and-npm")
}
