// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package color

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/core"
	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
)

type envVar struct {
	env map[string]string
}

func (e *envVar) lookupEnv(key string) (string, bool) {
	v, ok := e.env[key]
	return v, ok
}

func TestColorEnvVarSetToFalse(t *testing.T) {
	env := &envVar{
		env: map[string]string{colorEnvVar: "false"},
	}
	lookupEnv = env.lookupEnv

	DisableColorBasedOnEnvVar()

	require.True(t, core.DisableColor, "expected to be true when COLOR is disabled")
	require.True(t, color.NoColor, "expected to be true when COLOR is disabled")
}

func TestColorEnvVarSetToTrue(t *testing.T) {
	env := &envVar{
		env: map[string]string{colorEnvVar: "true"},
	}
	lookupEnv = env.lookupEnv

	DisableColorBasedOnEnvVar()

	require.False(t, core.DisableColor, "expected to be false when COLOR is enabled")
	require.False(t, color.NoColor, "expected to be true when COLOR is enabled")
}

func TestColorEnvVarNotSet(t *testing.T) {
	env := &envVar{
		env: make(map[string]string),
	}
	lookupEnv = env.lookupEnv

	DisableColorBasedOnEnvVar()

	require.Equal(t, core.DisableColor, color.NoColor, "expected to be the same as color.NoColor")
}

func TestColorGenerator(t *testing.T) {
	newColor := ColorGenerator()
	colors := make(map[*color.Color]struct{})
	for i := 0; i < 50; i++ {
		color := newColor()
		colors[color] = struct{}{}
		require.NotNil(t, color)
	}
	require.Equal(t, len(colors), 10)
}
