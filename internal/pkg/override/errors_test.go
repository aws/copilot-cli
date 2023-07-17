// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrPackageManagerUnavailable_RecommendActions(t *testing.T) {
	require.Equal(t, `Please follow the instructions to install either one of the package managers:
"npm": "https://docs.npmjs.com/downloading-and-installing-node-js-and-npm"
"yarn": "https://yarnpkg.com/getting-started/install"`,
		new(errPackageManagerUnavailable).RecommendActions())
}
