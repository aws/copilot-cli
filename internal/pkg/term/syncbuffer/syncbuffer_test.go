// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package syncbuffer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncBuffer_Write(t *testing.T) {
	testCases := map[string]struct {
		input        []byte
		wantedOutput string
	}{
		"append to custom buffer with simple input": {
			input:        []byte("hello world"),
			wantedOutput: "hello world",
		},
		"append to custom buffer with empty input": {
			input:        []byte(""),
			wantedOutput: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			sb := &SyncBuffer{}

			// WHEN
			sb.Write(tc.input)

			// THEN
			require.Equal(t, tc.wantedOutput, sb.buf.String())
		})
	}
}

func TestSyncBuffer_IsDone(t *testing.T) {
	testCases := map[string]struct {
		buffer     *SyncBuffer
		wantedDone bool
	}{
		"Buffer is done": {
			buffer:     &SyncBuffer{done: make(chan struct{}), buf: bytes.Buffer{}},
			wantedDone: true,
		},
		"Buffer is not done": {
			buffer: &SyncBuffer{done: make(chan struct{}), buf: bytes.Buffer{}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			if tc.wantedDone {
				tc.buffer.MarkDone()
			}

			// WHEN
			actual := tc.buffer.IsDone()

			// THEN
			require.Equal(t, tc.wantedDone, actual)

		})
	}
}

func TestSyncBuffer_strings(t *testing.T) {
	testCases := map[string]struct {
		input  []byte
		wanted []string
	}{
		"single line in buffer": {
			input:  []byte("hello"),
			wanted: []string{"hello"},
		},
		"multiple lines in buffer": {
			input:  []byte("hello\nworld\n"),
			wanted: []string{"hello", "world"},
		},
		"empty buffer": {
			input: []byte(""),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			sb := &SyncBuffer{}
			sb.Write(tc.input)

			// WHEN
			actual := sb.strings()

			// THEN
			require.Equal(t, tc.wanted, actual)
		})
	}
}

type mockFileReader struct {
	data []byte
	err  error
}

func (r *mockFileReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, io.EOF
}

func TestSyncBuffer_CopyToBuffer(t *testing.T) {
	testCases := map[string]struct {
		reader       fileReader
		wantedOutput string
		wantedError  error
	}{
		"copy data from file reader to buffer": {
			reader: &mockFileReader{
				data: []byte("Building your container image"),
			},
			wantedOutput: "Building your container image",
		},
		"return an error when failed to copy data to buffer": {
			reader:       &mockFileReader{err: fmt.Errorf("some error")},
			wantedOutput: "",
			wantedError:  fmt.Errorf("failed to copy to buffer: some error"),
		},
		"return an EOF error": {
			reader: &mockFileReader{
				data: []byte("Building your container image"),
				err:  errors.New("EOF"),
			},
			wantedOutput: "Building your container image",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			sb := &SyncBuffer{
				done: make(chan struct{}),
			}

			// WHEN
			gotErr := sb.CopyToBuffer(tc.reader)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, gotErr.Error())
			}
			require.Equal(t, tc.wantedOutput, sb.buf.String())
		})
	}
}
