// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuccess(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Success("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Success!")
	require.Contains(t, b.String(), "hello world")
}

func TestSuccessln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Successln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Success!")
	require.Contains(t, b.String(), "hello world\n")
}

func TestSuccessf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Successf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "Success!")
	require.Contains(t, b.String(), "hello world\n")
}

func TestSsuccess(t *testing.T) {
	s := Ssuccess("hello", " world")

	require.Contains(t, s, "Success!")
	require.Contains(t, s, "hello world")
}

func TestSsuccessln(t *testing.T) {
	s := Ssuccessln("hello", " world")

	// THEN
	require.Contains(t, s, "Success!")
	require.Contains(t, s, "hello world\n")
}

func TestSsuccessf(t *testing.T) {
	s := Ssuccessf("%s %s\n", "hello", "world")

	require.Contains(t, s, "Success!")
	require.Contains(t, s, "hello world\n")
}

func TestError(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Error("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Error!")
	require.Contains(t, b.String(), "hello world")
}

func TestErrorln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Errorln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Error!")
	require.Contains(t, b.String(), "hello world\n")
}

func TestErrorf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Errorf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "Error!")
	require.Contains(t, b.String(), "hello world\n")
}

func TestSerror(t *testing.T) {
	s := Serror("hello", " world")

	require.Contains(t, s, "Error!")
	require.Contains(t, s, "hello world")
}

func TestSerrorln(t *testing.T) {
	s := Serrorln("hello", " world")

	require.Contains(t, s, "Error!")
	require.Contains(t, s, "hello world\n")
}

func TestSerrorf(t *testing.T) {
	s := Serrorf("%s %s\n", "hello", "world")

	require.Contains(t, s, "Error!")
	require.Contains(t, s, "hello world\n")
}

func TestWarning(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Warning("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world")
}

func TestWarningln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Warningln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world\n")
}

func TestWarningf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Warningf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world\n")
}

func TestInfo(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Info("hello", " world")

	// THEN
	require.Equal(t, "hello world", b.String())
}

func TestInfoln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Infoln("hello", "world")

	// THEN
	require.Equal(t, "hello world\n", b.String())
}

func TestInfof(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Infof("%s %s\n", "hello", "world")

	// THEN
	require.Equal(t, "hello world\n", b.String())
}

func TestDebug(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Debug("hello", " world")

	// THEN
	require.Contains(t, b.String(), "hello world")
}

func TestDebugln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Debugln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "hello world\n")
}

func TestDebugf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Debugf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "hello world\n")
}
