// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsEmptyErr(t *testing.T) {
	testCases := map[string]struct {
		err    error
		wanted bool
	}{
		"should return true when ErrWorkspaceNotFound": {
			err:    &ErrWorkspaceNotFound{},
			wanted: true,
		},
		"should return true when ErrNoAssociatedApplication": {
			err:    &ErrNoAssociatedApplication{},
			wanted: true,
		},
		"should return true when an empty workspace error is wrapped": {
			err:    fmt.Errorf("burrito: %w", &ErrNoAssociatedApplication{}),
			wanted: true,
		},
		"should return false when a random error": {
			err: errors.New("unexpected"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, IsEmptyErr(tc.err))
		})
	}
}
