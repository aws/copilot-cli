// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package color provides functionality to displayed colored text on the terminal.
package color

import (
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2/core"
	"github.com/fatih/color"
)

// Predefined colors.
// Refer to https://en.wikipedia.org/wiki/ANSI_escape_code to validate if colors would
// be visible on white or black screen backgrounds.
var (
	Grey     = color.New(color.FgWhite)
	DarkGray = color.New(color.FgBlack)
	Red      = color.New(color.FgHiRed)
	DullRed  = color.New(color.FgRed)
	Green    = color.New(color.FgHiGreen)
	Yellow   = color.New(color.FgHiYellow)
	Magenta  = color.New(color.FgMagenta)
	Blue     = color.New(color.FgHiBlue)

	DullGreen   = color.New(color.FgGreen)
	DullBlue    = color.New(color.FgBlue)
	DullYellow  = color.New(color.FgYellow)
	DullMagenta = color.New(color.FgMagenta)
	DullCyan    = color.New(color.FgCyan)

	HiBlue       = color.New(color.FgHiBlue)
	Cyan         = color.New(color.FgHiCyan)
	Bold         = color.New(color.Bold)
	Faint        = color.New(color.Faint)
	BoldFgYellow = color.New(color.FgYellow).Add(color.Bold)
)

const colorEnvVar = "COLOR"

var lookupEnv = os.LookupEnv

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

// Help colors the string to denote that it's auxiliary helpful information, and returns it.
func Help(s string) string {
	return Faint.Sprint(s)
}

// Emphasize colors the string to denote that it as important, and returns it.
func Emphasize(s string) string {
	return Bold.Sprint(s)
}

// HighlightUserInput colors the string to denote it as an input from standard input, and returns it.
func HighlightUserInput(s string) string {
	return Emphasize(s)
}

// HighlightResource colors the string to denote it as a resource created by the CLI, and returns it.
func HighlightResource(s string) string {
	return HiBlue.Sprint(s)
}

// HighlightCode wraps the string s with the ` character, colors it to denote it's code, and returns it.
func HighlightCode(s string) string {
	return Cyan.Sprintf("`%s`", s)
}

// HighlightCodeBlock wraps the string s with ``` characters, colors it to denote it's a multi-line code block, and returns it.
func HighlightCodeBlock(s string) string {
	return Cyan.Sprintf("```\n%s\n```", s)
}

// Prod colors the string to mark it is a prod environment.
func Prod(s string) string {
	return BoldFgYellow.Sprint(s)
}

// ColorGenerator returns a generator function for colors.
// It doesn't return reds to avoid error-like formatting.
func ColorGenerator() func() *color.Color {
	colors := []*color.Color{
		Yellow,
		Green,
		Cyan,
		Blue,
		Magenta,
		DullYellow,
		DullGreen,
		DullCyan,
		DullBlue,
		DullMagenta,
	}
	i := 0
	return func() *color.Color {
		defer func() { i++ }()
		return colors[i%len(colors)]
	}
}
