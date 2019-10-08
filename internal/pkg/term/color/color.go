// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package color provides utilities to globally enable/disable color
// output of the CLI
package color

import (
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2/core"
	"github.com/fatih/color"
)

const colorEnvVar = "COLOR"

var lookupEnv = os.LookupEnv

var (
	cyan               = color.New(color.FgHiCyan)
	whiteBoldUnderline = color.New(color.FgHiWhite, color.Bold, color.Underline)
	magenta            = color.New(color.FgHiMagenta)
)

// DisableColorBasedOnEnvVar determines whether the CLI will produce color
// output based on the environment variable, COLOR.
func DisableColorBasedOnEnvVar() {
	value, exists := lookupEnv(colorEnvVar)
	if !exists {
		// if the COLOR environment variable is not set
		// then follow the settings in the color library
		// since it's dynamically set based on the type of terminal
		// and whether stdout is connected to a terminal or not.
		core.DisableColor = color.NoColor
		return
	}

	if strings.ToLower(value) == "false" {
		core.DisableColor = true
		color.NoColor = true
	} else if strings.ToLower(value) == "true" {
		core.DisableColor = false
		color.NoColor = false
	}
}

// HighlightUserInput colors the string to denote it as an input from standard input, and returns it.
func HighlightUserInput(s string) string {
	return cyan.Sprint(s)
}

// HighlightResource colors the string to denote it as a resource created by the CLI, and returns it.
func HighlightResource(s string) string {
	return whiteBoldUnderline.Sprint(s)
}

// HighlightCode wraps the string with the ` character, colors it to denote it's a code block, and returns it.
func HighlightCode(s string) string {
	return magenta.Sprintf("`%s`", s)
}
