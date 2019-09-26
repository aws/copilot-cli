// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package styling

import (
	"os"
	"testing"

	"github.com/AlecAivazis/survey/v2/core"
	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
)

func TestColorEnvVarSetToFalse(t *testing.T) {
	os.Setenv(colorEnvVar, "false")

	DisableColorBasedOnEnvVar()

	require.True(t, core.DisableColor, "expected to be true when COLOR is disabled")
	require.True(t, color.NoColor, "expected to be true when COLOR is disabled")
}

func TestColorEnvVarSetToTrue(t *testing.T) {
	os.Setenv(colorEnvVar, "True")

	DisableColorBasedOnEnvVar()

	require.False(t, core.DisableColor, "expected to be false when COLOR is enabled")
	require.False(t, color.NoColor, "expected to be true when COLOR is enabled")
}

func TestColorEnvVarNotSet(t *testing.T) {
	os.Clearenv()

	DisableColorBasedOnEnvVar()

	require.Equal(t, core.DisableColor, color.NoColor, "expected to be the same as color.NoColor")
}
