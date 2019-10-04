// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package log is a wrapper around the fmt package to print messages to the terminal.
package log

import (
	"fmt"

	"github.com/fatih/color"
)

// Colored string formatting functions.
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

// Success prefixes the message with a green "✔ Success!", and writes to standard error.
func Success(args ...interface{}) {
	msg := fmt.Sprintf("%s %s", successSprintf(successPrefix), fmt.Sprint(args...))
	fmt.Fprint(DiagnosticWriter, msg)
}

// Successln prefixes the message with a green "✔ Success!", and writes to standard error with a new line.
func Successln(args ...interface{}) {
	msg := fmt.Sprintf("%s %s", successSprintf(successPrefix), fmt.Sprint(args...))
	fmt.Fprintln(DiagnosticWriter, msg)
}

// Successf formats according to the specifier, prefixes the message with a green "✔ Success!", and writes to standard error.
func Successf(format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", successSprintf(successPrefix), format)
	fmt.Fprintf(DiagnosticWriter, wrappedFormat, args...)
}

// Error prefixes the message with a red "✘ Error!", and writes to standard error.
func Error(args ...interface{}) {
	msg := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), fmt.Sprint(args...))
	fmt.Fprint(DiagnosticWriter, msg)
}

// Errorln prefixes the message with a red "✘ Error!", and writes to standard error with a new line.
func Errorln(args ...interface{}) {
	msg := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), fmt.Sprint(args...))
	fmt.Fprintln(DiagnosticWriter, msg)
}

// Errorf formats according to the specifier, prefixes the message with a red "✘ Error!", and writes to standard error.
func Errorf(format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), format)
	fmt.Fprintf(DiagnosticWriter, wrappedFormat, args...)
}

// Warning prefixes the message with a "Note:", colors the *entire* message in yellow, writes to standard error.
func Warning(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprint(DiagnosticWriter, warningSprintf(fmt.Sprintf("%s %s", warningPrefix, msg)))
}

// Warningln prefixes the message with a "Note:", colors the *entire* message in yellow, writes to standard error with a new line.
func Warningln(args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprintln(DiagnosticWriter, warningSprintf(fmt.Sprintf("%s %s", warningPrefix, msg)))
}

// Warningf formats according to the specifier, prefixes the message with a "Note:", colors the *entire* message in yellow, and writes to standard error.
func Warningf(format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", warningPrefix, format)
	fmt.Fprintf(DiagnosticWriter, warningSprintf(wrappedFormat, args...))
}

// Info writes the message to standard error with the default color.
func Info(args ...interface{}) {
	fmt.Fprint(DiagnosticWriter, args...)
}

// Infoln writes the message to standard error with the default color and new line.
func Infoln(args ...interface{}) {
	fmt.Fprintln(DiagnosticWriter, args...)
}

// Infof formats according to the specifier, and writes to standard error with the default color.
func Infof(format string, args ...interface{}) {
	fmt.Fprintf(DiagnosticWriter, format, args...)
}

// Debug writes the message to standard error in grey.
func Debug(args ...interface{}) {
	fmt.Fprint(DiagnosticWriter, debugSprintf(fmt.Sprint(args...)))
}

// Debugln writes the message to standard error in grey and with a new line.
func Debugln(args ...interface{}) {
	fmt.Fprintln(DiagnosticWriter, debugSprintf(fmt.Sprint(args...)))
}

// Debugf formats according to the specifier, colors the message in grey, and writes to standard error.
func Debugf(format string, args ...interface{}) {
	fmt.Fprint(DiagnosticWriter, debugSprintf(format, args...))
}
