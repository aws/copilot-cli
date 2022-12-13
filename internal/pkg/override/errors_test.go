// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrNPMUnavailable_RecommendActions(t *testing.T) {
	require.Equal(t, `Please follow instructions at: "https://docs.npmjs.com/downloading-and-installing-node-js-and-npm" to install "npm"`, new(errNPMUnavailable).RecommendActions())
}
