// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogger_Success(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Success("hello", " world")

	// THEN
	require.Equal(t, b.String(), fmt.Sprintf("%s hello world", successPrefix))
}

func TestLogger_Successln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Successln("hello", " world")

	// THEN
	require.Equal(t, b.String(), fmt.Sprintf("%s hello world\n", successPrefix))
}

func TestLogger_Successf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Successf("%s %s\n", "hello", "world")

	// THEN
	require.Equal(t, b.String(), fmt.Sprintf("%s hello world\n", successPrefix))
}

func TestLogger_Error(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Error("hello", " world")

	// THEN
	require.Contains(t, b.String(), fmt.Sprintf("%s hello world", errorPrefix))
}

func TestLogger_Errorln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Errorln("hello", " world")

	// THEN
	require.Contains(t, b.String(), fmt.Sprintf("%s hello world\n", errorPrefix))
}

func TestLogger_Errorf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Errorf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), fmt.Sprintf("%s hello world\n", errorPrefix))
}

func TestLogger_Warning(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Warning("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world")
}

func TestLogger_Warningln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Warningln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world\n")
}

func TestLogger_Warningf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Warningf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "Note:")
	require.Contains(t, b.String(), "hello world\n")
}

func TestLogger_Info(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Info("hello", " world")

	// THEN
	require.Equal(t, "hello world", b.String())
}

func TestLogger_Infoln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Infoln("hello", "world")

	// THEN
	require.Equal(t, "hello world\n", b.String())
}

func TestLogger_Infof(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Infof("%s %s\n", "hello", "world")

	// THEN
	require.Equal(t, "hello world\n", b.String())
}

func TestLogger_Debug(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Debug("hello", " world")

	// THEN
	require.Contains(t, b.String(), "hello world")
}

func TestLogger_Debugln(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Debugln("hello", " world")

	// THEN
	require.Contains(t, b.String(), "hello world\n")
}

func TestLogger_Debugf(t *testing.T) {
	// GIVEN
	b := &strings.Builder{}
	logger := New(b)

	// WHEN
	logger.Debugf("%s %s\n", "hello", "world")

	// THEN
	require.Contains(t, b.String(), "hello world\n")
}
