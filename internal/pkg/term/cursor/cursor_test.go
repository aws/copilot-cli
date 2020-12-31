// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEraseLine(t *testing.T) {
	testCases := map[string]struct {
		inWriter    func(writer io.Writer) io.Writer
		shouldErase bool
	}{
		"should not erase a line if the writer is not a file": {
			inWriter: func(writer io.Writer) io.Writer {
				return writer
			},
			shouldErase: false,
		},
		"should erase a line if the writer is a file": {
			inWriter: func(writer io.Writer) io.Writer {
				return &fakeFileWriter{w: writer}
			},
			shouldErase: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			buf := new(strings.Builder)

			// WHEN
			EraseLine(tc.inWriter(buf))

			// THEN
			isErased := buf.String() != ""
			require.Equal(t, tc.shouldErase, isErased)
		})
	}
}
