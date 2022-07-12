// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuoteStringSlice(t *testing.T) {
	t.Run("returns a slice of quoted strings", func(t *testing.T) {
		in := []string{"running", "up", "that", "hill"}
		expected := []string{`"running"`, `"up"`, `"that"`, `"hill"`}
		got := QuoteStringSlice(in)
		require.Equal(t, expected, got)
	})
}
