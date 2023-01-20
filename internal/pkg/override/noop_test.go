// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoop_Override(t *testing.T) {
	// GIVEN
	overrider := new(Noop)

	// WHEN
	out, err := overrider.Override([]byte("hello, world!"))

	// THEN
	require.NoError(t, err)
	require.Equal(t, "hello, world!", string(out))
}
