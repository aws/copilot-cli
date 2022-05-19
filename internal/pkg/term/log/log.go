// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package log is a wrapper around the fmt package to print messages to the terminal.
package log

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

// Decorated io.Writers around standard error and standard output that work on Windows.
var (
	DiagnosticWriter = color.Error
	OutputWriter     = color.Output
)

// Colored string formatting functions.
var (
	successSprintf = color.HiGreenString
	errorSprintf   = color.HiRedString
	warningSprintf = color.YellowString
	debugSprintf   = color.New(color.Faint).Sprintf
)

// Log message prefixes.
const (
	warningPrefix = "Note:"
)

// Success prefixes the message with a green "✔ Success!", and writes to standard error.
func Success(args ...interface{}) {
	success(DiagnosticWriter, args...)
}

// Successln prefixes the message with a green "✔ Success!", and writes to standard error with a new line.
func Successln(args ...interface{}) {
	successln(DiagnosticWriter, args...)
}

// Successf formats according to the specifier, prefixes the message with a green "✔ Success!", and writes to standard error.
func Successf(format string, args ...interface{}) {
	successf(DiagnosticWriter, format, args...)
}

// Ssuccess prefixes the message with a green "✔ Success!", and returns it.
func Ssuccess(args ...interface{}) string {
	return fmt.Sprintf("%s %s", successSprintf(successPrefix), fmt.Sprint(args...))
}

// Ssuccessln prefixes the message with a green "✔ Success!", appends a new line, and returns it.
func Ssuccessln(args ...interface{}) string {
	msg := fmt.Sprintf("%s %s", successSprintf(successPrefix), fmt.Sprint(args...))
	return fmt.Sprintln(msg)
}

// Ssuccessf formats according to the specifier, prefixes the message with a green "✔ Success!", and returns it.
func Ssuccessf(format string, args ...interface{}) string {
	wrappedFormat := fmt.Sprintf("%s %s", successSprintf(successPrefix), format)
	return fmt.Sprintf(wrappedFormat, args...)
}

// Error prefixes the message with a red "✘ Error!", and writes to standard error.
func Error(args ...interface{}) {
	err(DiagnosticWriter, args...)
}

// Errorln prefixes the message with a red "✘ Error!", and writes to standard error with a new line.
func Errorln(args ...interface{}) {
	errln(DiagnosticWriter, args...)
}

// Errorf formats according to the specifier, prefixes the message with a red "✘ Error!", and writes to standard error.
func Errorf(format string, args ...interface{}) {
	errf(DiagnosticWriter, format, args...)
}

// Serror prefixes the message with a red "✘ Error!", and returns it.
func Serror(args ...interface{}) string {
	return fmt.Sprintf("%s %s", errorSprintf(errorPrefix), fmt.Sprint(args...))
}

// Serrorln prefixes the message with a red "✘ Error!", appends a new line, and returns it.
func Serrorln(args ...interface{}) string {
	msg := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), fmt.Sprint(args...))
	return fmt.Sprintln(msg)
}

// Serrorf formats according to the specifier, prefixes the message with a red "✘ Error!", and returns it.
func Serrorf(format string, args ...interface{}) string {
	wrappedFormat := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), format)
	return fmt.Sprintf(wrappedFormat, args...)
}

// Warning prefixes the message with a "Note:", colors the *entire* message in yellow, writes to standard error.
func Warning(args ...interface{}) {
	warning(DiagnosticWriter, args...)
}

// Warningln prefixes the message with a "Note:", colors the *entire* message in yellow, writes to standard error with a new line.
func Warningln(args ...interface{}) {
	warningln(DiagnosticWriter, args...)
}

// Warningf formats according to the specifier, prefixes the message with a "Note:", colors the *entire* message in yellow, and writes to standard error.
func Warningf(format string, args ...interface{}) {
	warningf(DiagnosticWriter, format, args...)
}

// Info writes the message to standard error with the default color.
func Info(args ...interface{}) {
	info(DiagnosticWriter, args...)
}

// Infoln writes the message to standard error with the default color and new line.
func Infoln(args ...interface{}) {
	infoln(DiagnosticWriter, args...)
}

// Infof formats according to the specifier, and writes to standard error with the default color.
func Infof(format string, args ...interface{}) {
	infof(DiagnosticWriter, format, args...)
}

// Debug writes the message to standard error in grey.
func Debug(args ...interface{}) {
	debug(DiagnosticWriter, args...)
}

// Debugln writes the message to standard error in grey and with a new line.
func Debugln(args ...interface{}) {
	debugln(DiagnosticWriter, args...)
}

// Debugf formats according to the specifier, colors the message in grey, and writes to standard error.
func Debugf(format string, args ...interface{}) {
	debugf(DiagnosticWriter, format, args...)
}

func success(w io.Writer, args ...interface{}) {
	msg := fmt.Sprintf("%s %s", successSprintf(successPrefix), fmt.Sprint(args...))
	fmt.Fprint(w, msg)
}

func successln(w io.Writer, args ...interface{}) {
	msg := fmt.Sprintf("%s %s", successSprintf(successPrefix), fmt.Sprint(args...))
	fmt.Fprintln(w, msg)
}

func successf(w io.Writer, format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", successSprintf(successPrefix), format)
	fmt.Fprintf(w, wrappedFormat, args...)
}

func err(w io.Writer, args ...interface{}) {
	msg := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), fmt.Sprint(args...))
	fmt.Fprint(w, msg)
}

func errln(w io.Writer, args ...interface{}) {
	msg := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), fmt.Sprint(args...))
	fmt.Fprintln(w, msg)
}

func errf(w io.Writer, format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", errorSprintf(errorPrefix), format)
	fmt.Fprintf(w, wrappedFormat, args...)
}

func warning(w io.Writer, args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprint(w, warningSprintf(fmt.Sprintf("%s %s", warningPrefix, msg)))
}

func warningln(w io.Writer, args ...interface{}) {
	msg := fmt.Sprint(args...)
	fmt.Fprintln(w, warningSprintf(fmt.Sprintf("%s %s", warningPrefix, msg)))
}

func warningf(w io.Writer, format string, args ...interface{}) {
	wrappedFormat := fmt.Sprintf("%s %s", warningPrefix, format)
	fmt.Fprint(w, warningSprintf(wrappedFormat, args...))
}

func info(w io.Writer, args ...interface{}) {
	fmt.Fprint(w, args...)
}

func infoln(w io.Writer, args ...interface{}) {
	fmt.Fprintln(w, args...)
}

func infof(w io.Writer, format string, args ...interface{}) {
	fmt.Fprintf(w, format, args...)
}

func debug(w io.Writer, args ...interface{}) {
	fmt.Fprint(w, debugSprintf(fmt.Sprint(args...)))
}

func debugln(w io.Writer, args ...interface{}) {
	fmt.Fprintln(w, debugSprintf(fmt.Sprint(args...)))
}

func debugf(w io.Writer, format string, args ...interface{}) {
	fmt.Fprint(w, debugSprintf(format, args...))
}
