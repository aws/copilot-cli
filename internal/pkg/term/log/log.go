// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package log is a wrapper around the fmt package to print messages to the terminal.
package log

import (
	"fmt"

	"github.com/fatih/color"
)

// Color string formatting functions.
var (
	successSprintf = color.HiGreenString
	errorSprintf   = color.HiRedString
	warningSprintf = color.HiYellowString
	debugSprintf   = color.WhiteString
)

// Wrapper writers around standard error and standard output that work on windows.
var (
	DiagnosticWriter = color.Error
	OutputWriter     = color.Output
)

// PrintSuccess prefixes the message with a green "✔ Success!", and writes to standard error.
func PrintSuccess(args ...interface{}) {
	msg := fmt.Sprintf("%s %s", successSprintf(successPrefix), fmt.Sprint(args...))
	fmt.Fprint(DiagnosticWriter, msg)
}

// PrintSuccessln prefixes the message with a green "✔ Success!", and writes to standard error with a new line.
func PrintSuccessln(args ...interface{}) {
	msg := fmt.Sprintf("%s %s", successSprintf(successPrefix), fmt.Sprint(args...))
	fmt.Fprintln(DiagnosticWriter, msg)
}

// PrintSuccessf formats according to the specifier, prefixes the message with a green "✔ Success!", and writes to standard error.
func PrintSuccessf(format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", successSprintf(successPrefix), format)
	fmt.Fprintf(DiagnosticWriter, wrappedFormat, args...)
}

// PrintError prefixes the message with a red "✘ Error!", and writes to standard error.
func PrintError(args ...interface{}) {
	msg := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), fmt.Sprint(args...))
	fmt.Fprint(DiagnosticWriter, msg)
}

// PrintErrorln prefixes the message with a red "✘ Error!", and writes to standard error with a new line.
func PrintErrorln(args ...interface{}) {
	msg := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), fmt.Sprint(args...))
	fmt.Fprintln(DiagnosticWriter, msg)
}

// PrintErrorf formats according to the specifier, prefixes the message with a red "✘ Error!", and writes to standard error.
func PrintErrorf(format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), format)
	fmt.Fprintf(DiagnosticWriter, wrappedFormat, args...)
}

// PrintWarning prefixes the message with a "Note:", colors the *entire* message in yellow, writes to standard error.
func PrintWarning(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprint(DiagnosticWriter, warningSprintf(fmt.Sprintf("%s %s", warningPrefix, msg)))
}

// PrintWarningln prefixes the message with a "Note:", colors the *entire* message in yellow, writes to standard error with a new line.
func PrintWarningln(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprintln(DiagnosticWriter, warningSprintf(fmt.Sprintf("%s %s", warningPrefix, msg)))
}

// PrintWarningf formats according to the specifier, prefixes the message with a "Note:", colors the *entire* message in yellow, and writes to standard error.
func PrintWarningf(format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", warningPrefix, format)
	fmt.Fprintf(DiagnosticWriter, warningSprintf(wrappedFormat, args...))
}

// Print writes the message to standard error with the default color.
func Print(args ...interface{}) {
	fmt.Fprint(DiagnosticWriter, args...)
}

// Println writes the message to standard error with the default color and new line.
func Println(args ...interface{}) {
	fmt.Fprintln(DiagnosticWriter, args...)
}

// Printf formats according to the specifier, and writes to standard error with the default color.
func Printf(format string, args ...interface{}) {
	fmt.Fprintf(DiagnosticWriter, format, args...)
}

// PrintDebug writes the message to standard error in grey.
func PrintDebug(args ...interface{}) {
	fmt.Fprint(DiagnosticWriter, debugSprintf(fmt.Sprint(args...)))
}

// PrintDebugln writes the message to standard error in grey and with a new line.
func PrintDebugln(args ...interface{}) {
	fmt.Fprintln(DiagnosticWriter, debugSprintf(fmt.Sprint(args...)))
}

// PrintDebugf formats according to the specifier, colors the message in grey, and writes to standard error.
func PrintDebugf(format string, args ...interface{}) {
	fmt.Fprint(DiagnosticWriter, debugSprintf(format, args...))
}
