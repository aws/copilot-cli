// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrintSuccess(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintSuccess("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Success!")
	require.Contains(t, b.String(), "hello world")
}

func TestPrintSuccessln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintSuccessln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Success!")
	require.Contains(t, b.String(), "hello world\n")
}

func TestPrintSuccessf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintSuccessf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "Success!")
	require.Contains(t, b.String(), "hello world\n")
}

func TestPrintError(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintError("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Error!")
	require.Contains(t, b.String(), "hello world")
}

func TestPrintErrorln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintErrorln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Error!")
	require.Contains(t, b.String(), "hello world\n")
}

func TestPrintErrorf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintErrorf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "Error!")
	require.Contains(t, b.String(), "hello world\n")
}

func TestPrintWarning(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintWarning("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world")
}

func TestPrintWarningln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintWarningln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world\n")
}

func TestPrintWarningf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintWarningf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world\n")
}

func TestPrint(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Print("hello", " world")

	// THEN
	require.Equal(t, "hello world", b.String())
}

func TestPrintln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Println("hello", "world")

	// THEN
	require.Equal(t, "hello world\n", b.String())
}

func TestPrintf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	Printf("%s %s\n", "hello", "world")

	// THEN
	require.Equal(t, "hello world\n", b.String())
}

func TestPrintDebug(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintDebug("hello", " world")

	// THEN
	require.Contains(t, b.String(), "hello world")
}

func TestPrintDebugln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintDebugln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "hello world\n")
}

func TestPrintDebugf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	DiagnosticWriter = b

	// WHEN
	PrintDebugf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "hello world\n")
}
