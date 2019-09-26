// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package styling

import (
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2/core"
	"github.com/fatih/color"
)

const colorEnvVar = "COLOR"

// DisableColorBasedOnEnvVar determines whether the CLI will produce color
// output based on the environment variable, COLOR.
func DisableColorBasedOnEnvVar() {
	value, exists := os.LookupEnv(colorEnvVar)
	if exists {
		if strings.ToLower(value) == "false" {
			core.DisableColor = true
			color.NoColor = true
		} else if strings.ToLower(value) == "true" {
			core.DisableColor = false
			color.NoColor = false
		}
	} else {
		// if the COLOR environment variable is not set
		// then follow the settings in the color library
		// since it's dynamitcally set based on the type of terminal
		// and whether stdout is connected to a terminal or not.
		core.DisableColor = color.NoColor
	}
}
