// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrManifestNotFoundInTemplate_Error(t *testing.T) {
	require.EqualError(t,
		&ErrManifestNotFoundInTemplate{app: "phonetool", env: "test", name: "api"},
		"manifest metadata not found in template of stack phonetool-test-api")
}
